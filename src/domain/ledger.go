package domain

type BalanceSnapshot struct {
	Account   AccountID `json:"account"`
	Asset     AssetID   `json:"asset"`
	Available int64     `json:"available"`
	Reserved  int64     `json:"reserved"`
}

type LedgerEntryType string

const (
	EntryDebit       LedgerEntryType = "debit"
	EntryCredit      LedgerEntryType = "credit"
	EntryReserve     LedgerEntryType = "reserve"
	EntryRelease     LedgerEntryType = "release"
	EntryFee         LedgerEntryType = "fee"
	EntryRouteDebit  LedgerEntryType = "route_debit"
	EntryRouteCredit LedgerEntryType = "route_credit"
)

type LedgerEntry struct {
	ID       string          `json:"id"`
	Type     LedgerEntryType `json:"type"`
	Account  AccountID       `json:"account"`
	Asset    AssetID         `json:"asset"`
	Amount   int64           `json:"amount"`
	Balance  int64           `json:"balance"`
	TicketID TicketID        `json:"ticketId,omitempty"`
	RouteID  RouteID         `json:"routeId,omitempty"`
	Memo     string          `json:"memo"`
	Epoch    uint64          `json:"epoch"`
}

func (e LedgerEntry) Validate() error {
	if e.Type == "" {
		return Invalid("ledger entry type is required")
	}
	if err := e.Account.Validate(); err != nil {
		return err
	}
	if err := e.Asset.Validate(); err != nil {
		return err
	}
	if e.Amount < 0 {
		return Invalid("ledger entry amount cannot be negative")
	}
	return nil
}

type EventType string

const (
	EventIntentSubmitted  EventType = "intent_submitted"
	EventTicketQueued     EventType = "ticket_queued"
	EventTicketSettled    EventType = "ticket_settled"
	EventTicketRejected   EventType = "ticket_rejected"
	EventRouteAdjusted    EventType = "route_adjusted"
	EventExposureAdjusted EventType = "exposure_adjusted"
	EventEpochAdvanced    EventType = "epoch_advanced"
	EventSnapshot         EventType = "snapshot"
)

type Event struct {
	ID       EventID           `json:"id"`
	Type     EventType         `json:"type"`
	Epoch    uint64            `json:"epoch"`
	RouteID  RouteID           `json:"routeId,omitempty"`
	TicketID TicketID          `json:"ticketId,omitempty"`
	IntentID IntentID          `json:"intentId,omitempty"`
	Amount   int64             `json:"amount,omitempty"`
	Message  string            `json:"message"`
	Fields   map[string]string `json:"fields,omitempty"`
}

type AuditIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}
