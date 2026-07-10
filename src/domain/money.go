package domain

import (
	"fmt"
	"math"
)

const BpsDenominator int64 = 10_000

type Amount struct {
	Asset AssetID `json:"asset"`
	Units int64   `json:"units"`
}

func NewAmount(asset AssetID, units int64) Amount {
	return Amount{Asset: asset, Units: units}
}

func (a Amount) ValidatePositive() error {
	if err := a.Asset.Validate(); err != nil {
		return err
	}
	if a.Units <= 0 {
		return Invalid("amount must be positive")
	}
	return nil
}

func (a Amount) ValidateNonNegative() error {
	if err := a.Asset.Validate(); err != nil {
		return err
	}
	if a.Units < 0 {
		return Invalid("amount cannot be negative")
	}
	return nil
}

func (a Amount) SameAsset(other Amount) bool {
	return a.Asset == other.Asset
}

func (a Amount) Add(other Amount) (Amount, error) {
	if !a.SameAsset(other) {
		return Amount{}, Invalid("asset mismatch")
	}
	if (other.Units > 0 && a.Units > math.MaxInt64-other.Units) ||
		(other.Units < 0 && a.Units < math.MinInt64-other.Units) {
		return Amount{}, Invalid("amount overflow")
	}
	return Amount{Asset: a.Asset, Units: a.Units + other.Units}, nil
}

func (a Amount) Sub(other Amount) (Amount, error) {
	if !a.SameAsset(other) {
		return Amount{}, Invalid("asset mismatch")
	}
	if other.Units == math.MinInt64 {
		return Amount{}, Invalid("amount overflow")
	}
	return a.Add(Amount{Asset: other.Asset, Units: -other.Units})
}

func (a Amount) ScaleBps(bps int64) (Amount, error) {
	if bps < 0 {
		return Amount{}, Invalid("bps cannot be negative")
	}
	if a.Units > math.MaxInt64/BpsDenominator {
		return Amount{}, Invalid("amount overflow")
	}
	scaled := (a.Units*bps + BpsDenominator - 1) / BpsDenominator
	return Amount{Asset: a.Asset, Units: scaled}, nil
}

func (a Amount) String() string {
	return fmt.Sprintf("%d %s", a.Units, a.Asset)
}

func MaxInt64(values ...int64) int64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func MinInt64(values ...int64) int64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func ClampInt64(value int64, floor int64, ceil int64) int64 {
	if value < floor {
		return floor
	}
	if value > ceil {
		return ceil
	}
	return value
}
