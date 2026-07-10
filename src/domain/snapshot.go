package domain

type RouteSnapshot struct {
	ID                RouteID     `json:"id"`
	Corridor          CorridorID  `json:"corridor"`
	Status            RouteStatus `json:"status"`
	SourceAsset       AssetID     `json:"sourceAsset"`
	DestinationAsset  AssetID     `json:"destinationAsset"`
	Liquidity         int64       `json:"liquidity"`
	MaxExposure       int64       `json:"maxExposure"`
	Exposure          int64       `json:"exposure"`
	MaxTicketSize     int64       `json:"maxTicketSize"`
	PriorityBias      int64       `json:"priorityBias"`
	BaseFeeBps        int64       `json:"baseFeeBps"`
	OperatorFeeBps    int64       `json:"operatorFeeBps"`
	LastObservedEpoch uint64      `json:"lastObservedEpoch"`
}

type CorridorSnapshot struct {
	ID            CorridorID `json:"id"`
	Exposure      int64      `json:"exposure"`
	MaxExposure   int64      `json:"maxExposure"`
	DailyGross    int64      `json:"dailyGross"`
	MaxDailyGross int64      `json:"maxDailyGross"`
}

type QueueSnapshot struct {
	TicketID    TicketID      `json:"ticketId"`
	IntentID    IntentID      `json:"intentId"`
	RouteID     RouteID       `json:"routeId"`
	Status      TicketStatus  `json:"status"`
	QueueScore  int64         `json:"queueScore"`
	Amount      int64         `json:"amount"`
	SubmittedAt uint64        `json:"submittedAt"`
	ExpiresAt   uint64        `json:"expiresAt"`
	Priority    PriorityClass `json:"priority"`
}

type SystemSnapshot struct {
	Epoch       uint64              `json:"epoch"`
	Assets      []Asset             `json:"assets"`
	Accounts    []Account           `json:"accounts"`
	Balances    []BalanceSnapshot   `json:"balances"`
	Routes      []RouteSnapshot     `json:"routes"`
	Corridors   []CorridorSnapshot  `json:"corridors"`
	Queue       []QueueSnapshot     `json:"queue"`
	Receipts    []SettlementReceipt `json:"receipts"`
	Events      []Event             `json:"events"`
	AuditIssues []AuditIssue        `json:"auditIssues"`
}

type SubmitIntentResponse struct {
	Ticket SettlementTicket `json:"ticket"`
	Quote  RouteQuote       `json:"quote"`
}

type ExecuteResponse struct {
	Receipts []SettlementReceipt `json:"receipts"`
	Skipped  []SettlementTicket  `json:"skipped"`
	Snapshot SystemSnapshot      `json:"snapshot"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Epoch   uint64 `json:"epoch"`
}
