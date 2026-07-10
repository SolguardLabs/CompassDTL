package api

import (
	"sort"

	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/ledger"
)

type Bootstrap struct {
	Assets         []domain.Asset         `json:"assets"`
	Accounts       []domain.Account       `json:"accounts"`
	Routes         []domain.Route         `json:"routes"`
	CorridorLimits []domain.CorridorLimit `json:"corridorLimits"`
	Balances       []ledger.SeedBalance   `json:"balances"`
	Config         domain.EngineConfig    `json:"config"`
}

func (b Bootstrap) Validate() error {
	if len(b.Assets) == 0 {
		return domain.Invalid("at least one asset is required")
	}
	if len(b.Accounts) == 0 {
		return domain.Invalid("at least one account is required")
	}
	if len(b.Routes) == 0 {
		return domain.Invalid("at least one route is required")
	}
	if len(b.CorridorLimits) == 0 {
		return domain.Invalid("at least one corridor limit is required")
	}
	for _, asset := range b.Assets {
		if err := asset.Validate(); err != nil {
			return err
		}
	}
	for _, account := range b.Accounts {
		if err := account.Validate(); err != nil {
			return err
		}
	}
	for _, route := range b.Routes {
		if err := route.Validate(); err != nil {
			return err
		}
	}
	for _, limit := range b.CorridorLimits {
		if err := limit.Validate(); err != nil {
			return err
		}
	}
	config := b.Config.Normalize()
	if err := config.Validate(); err != nil {
		return err
	}
	return nil
}

func sortedAssets(values map[domain.AssetID]domain.Asset) []domain.Asset {
	assets := make([]domain.Asset, 0, len(values))
	for _, asset := range values {
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].ID < assets[j].ID
	})
	return assets
}

func sortedAccounts(values map[domain.AccountID]domain.Account) []domain.Account {
	accounts := make([]domain.Account, 0, len(values))
	for _, account := range values {
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].ID < accounts[j].ID
	})
	return accounts
}
