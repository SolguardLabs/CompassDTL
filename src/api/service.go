package api

import (
	"fmt"
	"sync"

	"github.com/solguardlabs/compassdtl/src/audit"
	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/fees"
	"github.com/solguardlabs/compassdtl/src/ledger"
	"github.com/solguardlabs/compassdtl/src/risk"
	"github.com/solguardlabs/compassdtl/src/routing"
	"github.com/solguardlabs/compassdtl/src/settlement"
)

type Service struct {
	mu       sync.Mutex
	epoch    uint64
	eventSeq uint64
	assets   map[domain.AssetID]domain.Asset
	accounts map[domain.AccountID]domain.Account
	config   domain.EngineConfig
	book     *ledger.Book
	risk     *risk.Book
	catalog  *routing.Catalog
	selector routing.Selector
	engine   *settlement.Engine
	events   []domain.Event
}

func NewService(bootstrap Bootstrap) (*Service, error) {
	if err := bootstrap.Validate(); err != nil {
		return nil, err
	}
	assets := make(map[domain.AssetID]domain.Asset)
	for _, asset := range bootstrap.Assets {
		assets[asset.ID] = asset
	}
	accounts := make(map[domain.AccountID]domain.Account)
	for _, account := range bootstrap.Accounts {
		accounts[account.ID] = account
	}
	book := ledger.NewBook()
	if err := ledger.ApplySeeds(book, bootstrap.Balances); err != nil {
		return nil, err
	}
	riskBook := risk.NewBook()
	for _, limit := range bootstrap.CorridorLimits {
		if err := riskBook.RegisterCorridor(limit); err != nil {
			return nil, err
		}
	}
	for _, route := range bootstrap.Routes {
		if err := riskBook.RegisterRoute(route); err != nil {
			return nil, err
		}
	}
	catalog, err := routing.NewCatalog(bootstrap.Routes)
	if err != nil {
		return nil, err
	}
	config := bootstrap.Config.Normalize()
	policy := risk.NewAdmissionPolicy(config)
	scorer := routing.NewScorer(fees.NewCalculator())
	selector := routing.NewSelector(catalog, scorer, riskBook, policy)
	engine := settlement.NewEngine(book, catalog, riskBook, policy)
	service := &Service{
		assets:   assets,
		accounts: accounts,
		config:   config,
		book:     book,
		risk:     riskBook,
		catalog:  catalog,
		selector: selector,
		engine:   engine,
		events:   make([]domain.Event, 0, 128),
	}
	service.record(domain.EventSnapshot, "", "", "", 0, "service initialized", nil)
	return service, nil
}

func (s *Service) SubmitIntent(request domain.SubmitIntentRequest) (domain.SubmitIntentResponse, error) {
	if err := s.validateIntentAccounts(request.Intent); err != nil {
		return domain.SubmitIntentResponse{}, err
	}
	s.mu.Lock()
	epoch := s.epoch
	s.mu.Unlock()
	plan, err := s.selector.Select(request.Intent, epoch)
	if err != nil {
		return domain.SubmitIntentResponse{}, err
	}
	ticket, _, err := s.engine.Enqueue(plan, epoch)
	if err != nil {
		return domain.SubmitIntentResponse{}, err
	}
	s.record(domain.EventIntentSubmitted, plan.Quote.RouteID, ticket.ID, request.Intent.ID, request.Intent.Amount, "intent accepted", map[string]string{
		"priority": string(request.Intent.Priority),
	})
	s.record(domain.EventTicketQueued, plan.Quote.RouteID, ticket.ID, request.Intent.ID, ticket.QueueScore, "ticket queued", nil)
	return domain.SubmitIntentResponse{
		Ticket: ticket,
		Quote:  plan.Quote,
	}, nil
}

func (s *Service) QuoteIntent(intent domain.Intent) ([]domain.RouteQuote, error) {
	if err := s.validateIntentAccounts(intent); err != nil {
		return nil, err
	}
	s.mu.Lock()
	epoch := s.epoch
	s.mu.Unlock()
	return s.selector.QuoteAll(intent, epoch)
}

func (s *Service) Execute(request domain.ExecuteRequest) (domain.ExecuteResponse, error) {
	s.mu.Lock()
	epoch := s.epoch
	s.mu.Unlock()
	receipts, skipped, err := s.engine.ExecuteReady(epoch, request.Count)
	if err != nil {
		return domain.ExecuteResponse{}, err
	}
	for _, skippedTicket := range skipped {
		s.record(domain.EventTicketRejected, skippedTicket.RouteID(), skippedTicket.ID, skippedTicket.IntentID(), skippedTicket.Amount(), skippedTicket.RejectionReason, nil)
	}
	for _, receipt := range receipts {
		s.record(domain.EventTicketSettled, receipt.RouteID, receipt.TicketID, receipt.IntentID, receipt.Amount, "ticket settled", map[string]string{
			"receipt": receipt.ID.String(),
		})
	}
	return domain.ExecuteResponse{
		Receipts: receipts,
		Skipped:  skipped,
		Snapshot: s.Snapshot(),
	}, nil
}

func (s *Service) AdjustExposure(request domain.ExposureAdjustmentRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	route, err := s.catalog.MustGet(request.RouteID)
	if err != nil {
		return err
	}
	if err := s.risk.AdjustExposure(route, request.Delta); err != nil {
		return err
	}
	s.record(domain.EventExposureAdjusted, route.ID, "", "", request.Delta, request.Reason, nil)
	return nil
}

func (s *Service) UpdateRoute(request domain.RouteUpdateRequest) (domain.Route, error) {
	route, err := s.catalog.Update(request)
	if err != nil {
		return domain.Route{}, err
	}
	if err := s.risk.UpdateRouteLimit(route); err != nil {
		return domain.Route{}, err
	}
	s.record(domain.EventRouteAdjusted, route.ID, "", "", request.LiquidityDelta, request.Reason, nil)
	return route, nil
}

func (s *Service) AdvanceEpoch(delta uint64) uint64 {
	if delta == 0 {
		delta = 1
	}
	s.mu.Lock()
	s.epoch += delta
	epoch := s.epoch
	s.mu.Unlock()
	if epoch%24 == 0 {
		s.risk.ResetDailyGross()
	}
	s.record(domain.EventEpochAdvanced, "", "", "", int64(delta), "epoch advanced", nil)
	return epoch
}

func (s *Service) Snapshot() domain.SystemSnapshot {
	s.mu.Lock()
	epoch := s.epoch
	assets := sortedAssets(s.assets)
	accounts := sortedAccounts(s.accounts)
	events := append([]domain.Event(nil), s.events...)
	s.mu.Unlock()
	routes := s.catalog.List()
	issues := make([]domain.AuditIssue, 0)
	issues = append(issues, s.book.AssertSolvent()...)
	issues = append(issues, s.risk.Issues(routes)...)
	snapshot := domain.SystemSnapshot{
		Epoch:       epoch,
		Assets:      assets,
		Accounts:    accounts,
		Balances:    s.book.Snapshots(),
		Routes:      s.risk.RouteExposureSnapshots(routes),
		Corridors:   s.risk.Snapshots(),
		Queue:       s.engine.Queue.Snapshot(),
		Receipts:    s.engine.ReceiptsSnapshot(),
		Events:      events,
		AuditIssues: issues,
	}
	snapshot.AuditIssues = append(snapshot.AuditIssues, audit.Evaluate(snapshot)...)
	return snapshot
}

func (s *Service) Health() domain.HealthResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.HealthResponse{
		Service: "CompassDTL",
		Status:  "ok",
		Epoch:   s.epoch,
	}
}

func (s *Service) validateIntentAccounts(intent domain.Intent) error {
	if err := intent.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	source, ok := s.accounts[intent.SourceAccount]
	if !ok || !source.Enabled {
		return domain.NotFound(fmt.Sprintf("source account %s is unavailable", intent.SourceAccount))
	}
	destination, ok := s.accounts[intent.DestinationAccount]
	if !ok || !destination.Enabled {
		return domain.NotFound(fmt.Sprintf("destination account %s is unavailable", intent.DestinationAccount))
	}
	if _, ok := s.assets[intent.SourceAsset]; !ok {
		return domain.NotFound(fmt.Sprintf("source asset %s is unavailable", intent.SourceAsset))
	}
	if _, ok := s.assets[intent.DestinationAsset]; !ok {
		return domain.NotFound(fmt.Sprintf("destination asset %s is unavailable", intent.DestinationAsset))
	}
	return nil
}

func (s *Service) record(eventType domain.EventType, routeID domain.RouteID, ticketID domain.TicketID, intentID domain.IntentID, amount int64, message string, fields map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventSeq++
	event := domain.Event{
		ID:       domain.NewEventID(s.epoch, s.eventSeq),
		Type:     eventType,
		Epoch:    s.epoch,
		RouteID:  routeID,
		TicketID: ticketID,
		IntentID: intentID,
		Amount:   amount,
		Message:  message,
		Fields:   fields,
	}
	s.events = append(s.events, event)
}
