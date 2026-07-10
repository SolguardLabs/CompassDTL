package domain

type PriorityClass string

const (
	PriorityNormal   PriorityClass = "normal"
	PriorityElevated PriorityClass = "elevated"
	PriorityUrgent   PriorityClass = "urgent"
)

func (p PriorityClass) Weight() int64 {
	switch p {
	case PriorityUrgent:
		return 300
	case PriorityElevated:
		return 150
	default:
		return 0
	}
}

type Intent struct {
	ID                 IntentID          `json:"id"`
	SourceAccount      AccountID         `json:"sourceAccount"`
	DestinationAccount AccountID         `json:"destinationAccount"`
	SourceAsset        AssetID           `json:"sourceAsset"`
	DestinationAsset   AssetID           `json:"destinationAsset"`
	Amount             int64             `json:"amount"`
	MaxFee             int64             `json:"maxFee"`
	Priority           PriorityClass     `json:"priority"`
	RequestedEpoch     uint64            `json:"requestedEpoch"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

func (i Intent) Validate() error {
	if err := i.ID.Validate(); err != nil {
		return err
	}
	if err := i.SourceAccount.Validate(); err != nil {
		return err
	}
	if err := i.DestinationAccount.Validate(); err != nil {
		return err
	}
	if err := i.SourceAsset.Validate(); err != nil {
		return err
	}
	if err := i.DestinationAsset.Validate(); err != nil {
		return err
	}
	if i.Amount <= 0 {
		return Invalid("intent amount must be positive")
	}
	if i.MaxFee < 0 {
		return Invalid("intent maxFee cannot be negative")
	}
	if i.Priority == "" {
		return Invalid("intent priority is required")
	}
	return nil
}

type SubmitIntentRequest struct {
	Intent Intent `json:"intent"`
}

type ExecuteRequest struct {
	Count uint64 `json:"count"`
}

type ExposureAdjustmentRequest struct {
	RouteID RouteID `json:"routeId"`
	Delta   int64   `json:"delta"`
	Reason  string  `json:"reason"`
}

func (r ExposureAdjustmentRequest) Validate() error {
	if err := r.RouteID.Validate(); err != nil {
		return err
	}
	if r.Delta == 0 {
		return Invalid("exposure delta cannot be zero")
	}
	if r.Reason == "" {
		return Invalid("exposure reason is required")
	}
	return nil
}

type RouteUpdateRequest struct {
	RouteID           RouteID      `json:"routeId"`
	Status            *RouteStatus `json:"status,omitempty"`
	LiquidityDelta    int64        `json:"liquidityDelta,omitempty"`
	MaxExposure       *int64       `json:"maxExposure,omitempty"`
	MaxTicketSize     *int64       `json:"maxTicketSize,omitempty"`
	PriorityBiasDelta int64        `json:"priorityBiasDelta,omitempty"`
	Reason            string       `json:"reason"`
}

func (r RouteUpdateRequest) Validate() error {
	if err := r.RouteID.Validate(); err != nil {
		return err
	}
	if r.Reason == "" {
		return Invalid("route update reason is required")
	}
	if r.MaxExposure != nil && *r.MaxExposure <= 0 {
		return Invalid("maxExposure must be positive")
	}
	if r.MaxTicketSize != nil && *r.MaxTicketSize <= 0 {
		return Invalid("maxTicketSize must be positive")
	}
	return nil
}
