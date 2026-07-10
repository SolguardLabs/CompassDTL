package domain

type Asset struct {
	ID       AssetID `json:"id"`
	Symbol   string  `json:"symbol"`
	Decimals int     `json:"decimals"`
}

func (a Asset) Validate() error {
	if err := a.ID.Validate(); err != nil {
		return err
	}
	if a.Symbol == "" {
		return Invalid("asset symbol is required")
	}
	if a.Decimals < 0 || a.Decimals > 18 {
		return Invalid("asset decimals out of range")
	}
	return nil
}

type AccountRole string

const (
	RoleTreasury AccountRole = "treasury"
	RoleCustomer AccountRole = "customer"
	RoleOperator AccountRole = "operator"
	RoleFeeSink  AccountRole = "fee_sink"
)

type Account struct {
	ID          AccountID   `json:"id"`
	DisplayName string      `json:"displayName"`
	Role        AccountRole `json:"role"`
	Enabled     bool        `json:"enabled"`
}

func (a Account) Validate() error {
	if err := a.ID.Validate(); err != nil {
		return err
	}
	if a.DisplayName == "" {
		return Invalid("account displayName is required")
	}
	if a.Role == "" {
		return Invalid("account role is required")
	}
	return nil
}

type RouteStatus string

const (
	RouteEnabled    RouteStatus = "enabled"
	RoutePaused     RouteStatus = "paused"
	RouteDraining   RouteStatus = "draining"
	RouteDeprecated RouteStatus = "deprecated"
)

type Route struct {
	ID                 RouteID     `json:"id"`
	Corridor           CorridorID  `json:"corridor"`
	SourceAsset        AssetID     `json:"sourceAsset"`
	DestinationAsset   AssetID     `json:"destinationAsset"`
	TreasuryAccount    AccountID   `json:"treasuryAccount"`
	SettlementAccount  AccountID   `json:"settlementAccount"`
	FeeAccount         AccountID   `json:"feeAccount"`
	Status             RouteStatus `json:"status"`
	PriorityBias       int64       `json:"priorityBias"`
	BaseFeeBps         int64       `json:"baseFeeBps"`
	OperatorFeeBps     int64       `json:"operatorFeeBps"`
	MinFee             int64       `json:"minFee"`
	Liquidity          int64       `json:"liquidity"`
	MaxExposure        int64       `json:"maxExposure"`
	MaxTicketSize      int64       `json:"maxTicketSize"`
	SettlementDelay    uint64      `json:"settlementDelay"`
	LastObservedEpoch  uint64      `json:"lastObservedEpoch"`
	PreferLowExposure  bool        `json:"preferLowExposure"`
	ManualReviewAmount int64       `json:"manualReviewAmount"`
}

func (r Route) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return err
	}
	if err := r.Corridor.Validate(); err != nil {
		return err
	}
	if err := r.SourceAsset.Validate(); err != nil {
		return err
	}
	if err := r.DestinationAsset.Validate(); err != nil {
		return err
	}
	if err := r.TreasuryAccount.Validate(); err != nil {
		return err
	}
	if err := r.SettlementAccount.Validate(); err != nil {
		return err
	}
	if err := r.FeeAccount.Validate(); err != nil {
		return err
	}
	if r.Status == "" {
		return Invalid("route status is required")
	}
	if r.BaseFeeBps < 0 || r.OperatorFeeBps < 0 {
		return Invalid("route fee bps cannot be negative")
	}
	if r.MinFee < 0 {
		return Invalid("route min fee cannot be negative")
	}
	if r.Liquidity < 0 {
		return Invalid("route liquidity cannot be negative")
	}
	if r.MaxExposure <= 0 {
		return Invalid("route maxExposure must be positive")
	}
	if r.MaxTicketSize <= 0 {
		return Invalid("route maxTicketSize must be positive")
	}
	return nil
}

func (r Route) Accepts(asset AssetID, dest AssetID) bool {
	return r.SourceAsset == asset && r.DestinationAsset == dest
}

func (r Route) ActiveForSubmission() bool {
	return r.Status == RouteEnabled || r.Status == RouteDraining
}

func (r Route) ActiveForExecution() bool {
	return r.Status == RouteEnabled || r.Status == RouteDraining
}

type CorridorLimit struct {
	Corridor      CorridorID `json:"corridor"`
	MaxExposure   int64      `json:"maxExposure"`
	MaxDailyGross int64      `json:"maxDailyGross"`
}

func (l CorridorLimit) Validate() error {
	if err := l.Corridor.Validate(); err != nil {
		return err
	}
	if l.MaxExposure <= 0 {
		return Invalid("corridor maxExposure must be positive")
	}
	if l.MaxDailyGross <= 0 {
		return Invalid("corridor maxDailyGross must be positive")
	}
	return nil
}

type EngineConfig struct {
	MinScore             int64     `json:"minScore"`
	DefaultSettlementTTL uint64    `json:"defaultSettlementTtl"`
	OperatorAccount      AccountID `json:"operatorAccount"`
	NativeAsset          AssetID   `json:"nativeAsset"`
}

func (c EngineConfig) Normalize() EngineConfig {
	if c.MinScore == 0 {
		c.MinScore = 50
	}
	if c.DefaultSettlementTTL == 0 {
		c.DefaultSettlementTTL = 8
	}
	return c
}

func (c EngineConfig) Validate() error {
	if err := c.OperatorAccount.Validate(); err != nil {
		return err
	}
	if err := c.NativeAsset.Validate(); err != nil {
		return err
	}
	if c.MinScore < 0 || c.MinScore > 1_000 {
		return Invalid("minScore out of range")
	}
	return nil
}
