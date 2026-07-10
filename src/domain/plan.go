package domain

type FeeBreakdown struct {
	BaseFee     int64 `json:"baseFee"`
	OperatorFee int64 `json:"operatorFee"`
	NetworkFee  int64 `json:"networkFee"`
	TotalFee    int64 `json:"totalFee"`
}

func (f FeeBreakdown) Validate(maxFee int64) error {
	if f.BaseFee < 0 || f.OperatorFee < 0 || f.NetworkFee < 0 || f.TotalFee < 0 {
		return Invalid("fees cannot be negative")
	}
	if f.TotalFee != f.BaseFee+f.OperatorFee+f.NetworkFee {
		return Invalid("fee total does not match components")
	}
	if maxFee >= 0 && f.TotalFee > maxFee {
		return LimitExceeded("fee exceeds maxFee")
	}
	return nil
}

type ScoreBreakdown struct {
	LiquidityScore int64 `json:"liquidityScore"`
	FeeScore       int64 `json:"feeScore"`
	LatencyScore   int64 `json:"latencyScore"`
	ExposureScore  int64 `json:"exposureScore"`
	PriorityScore  int64 `json:"priorityScore"`
	Total          int64 `json:"total"`
}

func (s ScoreBreakdown) Normalize() ScoreBreakdown {
	s.Total = s.LiquidityScore + s.FeeScore + s.LatencyScore + s.ExposureScore + s.PriorityScore
	return s
}

type RouteQuote struct {
	RouteID           RouteID        `json:"routeId"`
	Corridor          CorridorID     `json:"corridor"`
	Amount            int64          `json:"amount"`
	DestinationAmount int64          `json:"destinationAmount"`
	Fees              FeeBreakdown   `json:"fees"`
	Score             ScoreBreakdown `json:"score"`
	ObservedExposure  int64          `json:"observedExposure"`
	ObservedLiquidity int64          `json:"observedLiquidity"`
	QuoteEpoch        uint64         `json:"quoteEpoch"`
	ExpiresEpoch      uint64         `json:"expiresEpoch"`
}

type RiskAdmission struct {
	RouteID             RouteID    `json:"routeId"`
	Corridor            CorridorID `json:"corridor"`
	Granted             bool       `json:"granted"`
	Reason              string     `json:"reason"`
	Score               int64      `json:"score"`
	ExposureBefore      int64      `json:"exposureBefore"`
	ExposureAfter       int64      `json:"exposureAfter"`
	RouteMaxExposure    int64      `json:"routeMaxExposure"`
	CorridorBefore      int64      `json:"corridorBefore"`
	CorridorAfter       int64      `json:"corridorAfter"`
	CorridorMaxExposure int64      `json:"corridorMaxExposure"`
	ValidUntilEpoch     uint64     `json:"validUntilEpoch"`
}

type SettlementPlan struct {
	Intent      Intent        `json:"intent"`
	Quote       RouteQuote    `json:"quote"`
	Admission   RiskAdmission `json:"admission"`
	CreatedAt   uint64        `json:"createdAt"`
	ExecuteFrom uint64        `json:"executeFrom"`
	ExpiresAt   uint64        `json:"expiresAt"`
}

func (p SettlementPlan) Validate() error {
	if err := p.Intent.Validate(); err != nil {
		return err
	}
	if !p.Admission.Granted {
		return LimitExceeded(p.Admission.Reason)
	}
	if p.Quote.RouteID == "" {
		return Invalid("plan route is required")
	}
	if p.Quote.Fees.TotalFee > p.Intent.MaxFee {
		return LimitExceeded("plan fee exceeds maxFee")
	}
	if p.ExpiresAt <= p.CreatedAt {
		return Invalid("plan expiration must be after creation")
	}
	return nil
}

type TicketStatus string

const (
	TicketQueued    TicketStatus = "queued"
	TicketExecuting TicketStatus = "executing"
	TicketSettled   TicketStatus = "settled"
	TicketExpired   TicketStatus = "expired"
	TicketRejected  TicketStatus = "rejected"
)

type SettlementTicket struct {
	ID              TicketID       `json:"id"`
	Status          TicketStatus   `json:"status"`
	Plan            SettlementPlan `json:"plan"`
	QueueScore      int64          `json:"queueScore"`
	Sequence        uint64         `json:"sequence"`
	SubmittedAt     uint64         `json:"submittedAt"`
	LastUpdatedAt   uint64         `json:"lastUpdatedAt"`
	RejectionReason string         `json:"rejectionReason,omitempty"`
}

func (t SettlementTicket) RouteID() RouteID {
	return t.Plan.Quote.RouteID
}

func (t SettlementTicket) IntentID() IntentID {
	return t.Plan.Intent.ID
}

func (t SettlementTicket) Amount() int64 {
	return t.Plan.Intent.Amount
}

func (t SettlementTicket) TotalDebit() int64 {
	return t.Plan.Intent.Amount + t.Plan.Quote.Fees.TotalFee
}

type ReceiptStatus string

const (
	ReceiptSettled ReceiptStatus = "settled"
	ReceiptSkipped ReceiptStatus = "skipped"
)

type SettlementReceipt struct {
	ID                ReceiptID     `json:"id"`
	TicketID          TicketID      `json:"ticketId"`
	IntentID          IntentID      `json:"intentId"`
	RouteID           RouteID       `json:"routeId"`
	Status            ReceiptStatus `json:"status"`
	Amount            int64         `json:"amount"`
	DestinationAmount int64         `json:"destinationAmount"`
	Fees              FeeBreakdown  `json:"fees"`
	SettledAt         uint64        `json:"settledAt"`
	LedgerEntries     []LedgerEntry `json:"ledgerEntries"`
}
