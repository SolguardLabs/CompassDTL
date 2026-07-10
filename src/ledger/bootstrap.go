package ledger

import "github.com/solguardlabs/compassdtl/src/domain"

type SeedBalance struct {
	Account   domain.AccountID `json:"account"`
	Asset     domain.AssetID   `json:"asset"`
	Available int64            `json:"available"`
}

func ApplySeeds(book *Book, seeds []SeedBalance) error {
	for _, seed := range seeds {
		if err := seed.Account.Validate(); err != nil {
			return err
		}
		if err := seed.Asset.Validate(); err != nil {
			return err
		}
		if err := book.Seed(seed.Account, seed.Asset, seed.Available); err != nil {
			return err
		}
	}
	return nil
}

func RequiredDebit(intent domain.Intent, fees domain.FeeBreakdown) int64 {
	return intent.Amount + fees.TotalFee
}

func HasAvailable(book *Book, account domain.AccountID, asset domain.AssetID, amount int64) bool {
	return book.Available(account, asset) >= amount
}
