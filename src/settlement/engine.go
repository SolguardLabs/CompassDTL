package settlement

import (
	"fmt"
	"sync"

	"github.com/solguardlabs/compassdtl/src/domain"
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
	if count == 0 {
		count = 1
	}
	receipts := make([]domain.SettlementReceipt, 0, count)
	skipped := e.Queue.RemoveExpired(epoch)
	for uint64(len(receipts)) < count {
		ticket, ok := e.Queue.PopReady(epoch)
		if !ok {
			break
		}
		receipt, err := e.executeTicket(ticket, epoch)
		if err != nil {
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
	if err := e.Risk.ApplySettlement(route, ticket.Plan.Intent.Amount); err != nil {
		return domain.SettlementReceipt{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
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
	return receipt, nil
}

func queueScore(plan domain.SettlementPlan) int64 {
	score := plan.Quote.Score.Total
	score += plan.Intent.Priority.Weight()
	if plan.ExecuteFrom > plan.CreatedAt {
		score -= int64(plan.ExecuteFrom-plan.CreatedAt) * 10
	}
	return score
}
