package settlement

import (
	"fmt"
	"math"
	"sync"

	"github.com/solguardlabs/compassdtl/src/capital"
	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/fees"
	"github.com/solguardlabs/compassdtl/src/ledger"
	"github.com/solguardlabs/compassdtl/src/risk"
	"github.com/solguardlabs/compassdtl/src/routing"
)

type Engine struct {
	mu         sync.Mutex
	Book       *ledger.Book
	Catalog    *routing.Catalog
	Risk       *risk.Book
	Queue      *Queue
	Policy     risk.AdmissionPolicy
	Receipts   []domain.SettlementReceipt
	ticketSeq  uint64
	receiptSeq uint64
}

func NewEngine(book *ledger.Book, catalog *routing.Catalog, riskBook *risk.Book, policy risk.AdmissionPolicy) *Engine {
	return &Engine{
		Book:     book,
		Catalog:  catalog,
		Risk:     riskBook,
		Queue:    NewQueue(),
		Policy:   policy,
		Receipts: make([]domain.SettlementReceipt, 0, 64),
	}
}

func (e *Engine) Enqueue(plan domain.SettlementPlan, epoch uint64) (domain.SettlementTicket, []domain.LedgerEntry, error) {
	if err := plan.Validate(); err != nil {
		return domain.SettlementTicket{}, nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	totalDebit := plan.Intent.Amount + plan.Quote.Fees.TotalFee
	entries, err := e.Book.Reserve(
		plan.Intent.SourceAccount,
		plan.Intent.SourceAsset,
		totalDebit,
		"intent funding reserve",
		epoch,
		ledger.NoRefs(),
	)
	if err != nil {
		return domain.SettlementTicket{}, nil, err
	}
	e.ticketSeq++
	ticket := domain.SettlementTicket{
		ID:            domain.NewTicketID(plan.Intent.ID, e.ticketSeq),
		Status:        domain.TicketQueued,
		Plan:          plan,
		QueueScore:    queueScore(plan),
		Sequence:      e.ticketSeq,
		SubmittedAt:   epoch,
		LastUpdatedAt: epoch,
	}
	if err := e.Queue.Enqueue(ticket); err != nil {
		_, _ = e.Book.Release(plan.Intent.SourceAccount, plan.Intent.SourceAsset, totalDebit, "queue rollback", epoch, ledger.NoRefs())
		return domain.SettlementTicket{}, nil, err
	}
	return ticket, entries, nil
}

func (e *Engine) ExecuteReady(epoch uint64, count uint64) ([]domain.SettlementReceipt, []domain.SettlementTicket, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if count == 0 {
		count = 1
	}
	receipts := make([]domain.SettlementReceipt, 0, count)
	skipped := e.Queue.RemoveExpired(epoch)
	for _, ticket := range skipped {
		if err := e.releaseTicket(ticket, epoch); err != nil {
			return receipts, skipped, err
		}
	}
	for uint64(len(receipts)) < count {
		ticket, ok := e.Queue.PopReady(epoch)
		if !ok {
			break
		}
		receipt, err := e.executeTicket(ticket, epoch)
		if err != nil {
			if releaseErr := e.releaseTicket(ticket, epoch); releaseErr != nil {
				return receipts, skipped, releaseErr
			}
			ticket.Status = domain.TicketRejected
			ticket.RejectionReason = err.Error()
			ticket.LastUpdatedAt = epoch
			skipped = append(skipped, ticket)
			continue
		}
		receipts = append(receipts, receipt)
	}
	if len(receipts) == 0 && len(skipped) == 0 {
		return nil, nil, domain.QueueEmpty("no executable tickets")
	}
	return receipts, skipped, nil
}

func (e *Engine) ReceiptsSnapshot() []domain.SettlementReceipt {
	e.mu.Lock()
	defer e.mu.Unlock()
	receipts := make([]domain.SettlementReceipt, 0, len(e.Receipts))
	return append(receipts, e.Receipts...)
}

func (e *Engine) executeTicket(ticket domain.SettlementTicket, epoch uint64) (domain.SettlementReceipt, error) {
	if err := e.Policy.ValidateExecution(epoch, ticket); err != nil {
		return domain.SettlementReceipt{}, err
	}
	route, err := e.Catalog.MustGet(ticket.RouteID())
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	if !route.ActiveForExecution() {
		return domain.SettlementReceipt{}, domain.RouteUnavailable(fmt.Sprintf("route %s is not executable", route.ID))
	}
	if route.SettlementAccount == "" || route.TreasuryAccount == "" || route.FeeAccount == "" {
		return domain.SettlementReceipt{}, domain.Invalid("route settlement accounts are incomplete")
	}
	expectedPayout, err := fees.DestinationAmount(ticket.Plan.Intent, route)
	if err != nil || ticket.Plan.Quote.RouteID != route.ID ||
		ticket.Plan.Quote.Corridor != route.Corridor ||
		ticket.Plan.Quote.DestinationAmount != expectedPayout {
		return domain.SettlementReceipt{}, domain.Invalid("ticket quote does not match executable route")
	}
	if e.Book.Reserved(ticket.Plan.Intent.SourceAccount, ticket.Plan.Intent.SourceAsset) < ticket.TotalDebit() {
		return domain.SettlementReceipt{}, domain.Insufficient("ticket source reserve is unavailable")
	}
	if e.Book.Available(route.SettlementAccount, route.DestinationAsset) < ticket.Plan.Quote.DestinationAmount {
		return domain.SettlementReceipt{}, domain.Insufficient("route settlement account is underfunded")
	}
	_, err = e.Risk.AdmitExecution(ticket.Plan.Intent, route, ticket.Plan.Quote, epoch, e.Policy.TTL)
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	riskBooked := true
	defer func() {
		if riskBooked {
			_ = e.Risk.RevertExecution(route, ticket.Plan.Intent.Amount)
		}
	}()
	if err := e.checkQueuedCoverage(route, ticket); err != nil {
		return domain.SettlementReceipt{}, err
	}
	consumedRoute, err := e.Catalog.ConsumeLiquidity(route.ID, ticket.Plan.Quote.DestinationAmount)
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	route = consumedRoute
	refs := ledger.Refs(ticket.ID, route.ID)
	entries := make([]domain.LedgerEntry, 0, 8)
	payoutEntries, err := e.Book.Transfer(
		route.SettlementAccount,
		ticket.Plan.Intent.DestinationAccount,
		route.DestinationAsset,
		ticket.Plan.Quote.DestinationAmount,
		"route payout",
		epoch,
		refs,
	)
	if err != nil {
		_, _ = e.Catalog.AddLiquidity(route.ID, ticket.Plan.Quote.DestinationAmount)
		return domain.SettlementReceipt{}, err
	}
	entries = append(entries, payoutEntries...)
	reimburseEntries, err := e.Book.TransferReserved(
		ticket.Plan.Intent.SourceAccount,
		route.TreasuryAccount,
		ticket.Plan.Intent.SourceAsset,
		ticket.Plan.Intent.SourceAsset,
		ticket.Plan.Intent.Amount,
		ticket.Plan.Intent.Amount,
		"source reimbursement",
		epoch,
		refs,
	)
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	entries = append(entries, reimburseEntries...)
	feeEntries, err := e.Book.PayFeeFromReserved(
		ticket.Plan.Intent.SourceAccount,
		route.FeeAccount,
		ticket.Plan.Intent.SourceAsset,
		ticket.Plan.Quote.Fees.TotalFee,
		"route fees",
		epoch,
		refs,
	)
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	entries = append(entries, feeEntries...)
	e.receiptSeq++
	receipt := domain.SettlementReceipt{
		ID:                domain.NewReceiptID(ticket.ID, e.receiptSeq),
		TicketID:          ticket.ID,
		IntentID:          ticket.IntentID(),
		RouteID:           route.ID,
		Status:            domain.ReceiptSettled,
		Amount:            ticket.Plan.Intent.Amount,
		DestinationAmount: ticket.Plan.Quote.DestinationAmount,
		Fees:              ticket.Plan.Quote.Fees,
		SettledAt:         epoch,
		LedgerEntries:     entries,
	}
	e.Receipts = append(e.Receipts, receipt)
	riskBooked = false
	return receipt, nil
}

func (e *Engine) releaseTicket(ticket domain.SettlementTicket, epoch uint64) error {
	_, err := e.Book.Release(
		ticket.Plan.Intent.SourceAccount,
		ticket.Plan.Intent.SourceAsset,
		ticket.TotalDebit(),
		"ticket reserve released",
		epoch,
		ledger.Refs(ticket.ID, ticket.RouteID()),
	)
	return err
}

func (e *Engine) checkQueuedCoverage(route domain.Route, ticket domain.SettlementTicket) error {
	var queuedPrincipal int64
	for _, pending := range e.Queue.Tickets() {
		if pending.RouteID() != route.ID {
			continue
		}
		if pending.Plan.Quote.DestinationAmount > math.MaxInt64-queuedPrincipal {
			return domain.Invalid("queued principal exceeds supported amount")
		}
		queuedPrincipal += pending.Plan.Quote.DestinationAmount
	}
	if queuedPrincipal == 0 {
		return nil
	}
	liquidBacking := domain.MinInt64(
		route.Liquidity,
		e.Book.Available(route.SettlementAccount, route.DestinationAsset),
	)
	remaining := liquidBacking - ticket.Plan.Quote.DestinationAmount
	if remaining < 0 {
		return domain.Insufficient("route liquidity cannot cover the current payout")
	}
	limit, ok := e.Risk.RouteLimit(route.ID)
	if !ok {
		return domain.NotFound("route risk limit is unavailable")
	}
	metrics, err := capital.EvaluateRoute(capital.RouteInput{
		RouteID:               route.ID,
		Corridor:              route.Corridor,
		Liquidity:             remaining,
		Exposure:              e.Risk.RouteExposure(route.ID),
		MaxExposure:           limit.MaxExposure,
		QueuedPrincipal:       queuedPrincipal,
		SettlementDelayEpochs: route.SettlementDelay,
		LiquidityHaircutBPS:   500,
		SettlementShockBPS:    1_000,
		OperationalBufferBPS:  200,
	})
	if err != nil {
		return domain.Invalid(fmt.Sprintf("queued coverage evaluation failed: %v", err))
	}
	if !metrics.Compliant {
		return domain.LimitExceeded("queued settlement coverage is below policy")
	}
	return nil
}

func queueScore(plan domain.SettlementPlan) int64 {
	score := plan.Quote.Score.Total
	score += plan.Intent.Priority.Weight()
	if plan.ExecuteFrom > plan.CreatedAt {
		score -= int64(plan.ExecuteFrom-plan.CreatedAt) * 10
	}
	return score
}
