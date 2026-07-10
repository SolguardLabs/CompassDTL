package scenario

import (
	"github.com/solguardlabs/compassdtl/src/api"
	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/ledger"
)

func DefaultBootstrap() api.Bootstrap {
	return api.Bootstrap{
		Assets: []domain.Asset{
			{ID: "usdc", Symbol: "USDC", Decimals: 6},
			{ID: "eurc", Symbol: "EURC", Decimals: 6},
		},
		Accounts: []domain.Account{
			{ID: "acct:alice", DisplayName: "Alice Treasury", Role: domain.RoleCustomer, Enabled: true},
			{ID: "acct:bob", DisplayName: "Bob Receiver", Role: domain.RoleCustomer, Enabled: true},
			{ID: "acct:ops", DisplayName: "Compass Operator", Role: domain.RoleOperator, Enabled: true},
			{ID: "acct:fees", DisplayName: "Fee Sink", Role: domain.RoleFeeSink, Enabled: true},
			{ID: "acct:route-a-settle", DisplayName: "Route A Settlement", Role: domain.RoleTreasury, Enabled: true},
			{ID: "acct:route-a-treasury", DisplayName: "Route A Treasury", Role: domain.RoleTreasury, Enabled: true},
			{ID: "acct:route-b-settle", DisplayName: "Route B Settlement", Role: domain.RoleTreasury, Enabled: true},
			{ID: "acct:route-b-treasury", DisplayName: "Route B Treasury", Role: domain.RoleTreasury, Enabled: true},
		},
		Routes: []domain.Route{
			{
				ID:                "route:atlantic-fast",
				Corridor:          "corridor:us-eu",
				SourceAsset:       "usdc",
				DestinationAsset:  "eurc",
				TreasuryAccount:   "acct:route-a-treasury",
				SettlementAccount: "acct:route-a-settle",
				FeeAccount:        "acct:fees",
				Status:            domain.RouteEnabled,
				PriorityBias:      40,
				BaseFeeBps:        8,
				OperatorFeeBps:    2,
				MinFee:            25,
				Liquidity:         800_000,
				MaxExposure:       700_000,
				MaxTicketSize:     300_000,
				SettlementDelay:   1,
				PreferLowExposure: true,
			},
			{
				ID:                "route:iberia-deep",
				Corridor:          "corridor:us-eu",
				SourceAsset:       "usdc",
				DestinationAsset:  "eurc",
				TreasuryAccount:   "acct:route-b-treasury",
				SettlementAccount: "acct:route-b-settle",
				FeeAccount:        "acct:fees",
				Status:            domain.RouteEnabled,
				PriorityBias:      5,
				BaseFeeBps:        15,
				OperatorFeeBps:    2,
				MinFee:            20,
				Liquidity:         1_500_000,
				MaxExposure:       1_200_000,
				MaxTicketSize:     500_000,
				SettlementDelay:   3,
			},
		},
		CorridorLimits: []domain.CorridorLimit{
			{Corridor: "corridor:us-eu", MaxExposure: 1_600_000, MaxDailyGross: 3_000_000},
		},
		Balances: []ledger.SeedBalance{
			{Account: "acct:alice", Asset: "usdc", Available: 2_000_000},
			{Account: "acct:route-a-settle", Asset: "eurc", Available: 800_000},
			{Account: "acct:route-b-settle", Asset: "eurc", Available: 1_500_000},
		},
		Config: domain.EngineConfig{
			MinScore:             50,
			DefaultSettlementTTL: 8,
			OperatorAccount:      "acct:ops",
			NativeAsset:          "usdc",
		},
	}
}
