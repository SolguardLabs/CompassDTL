package fees

import (
	"math/big"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type Calculator struct {
	NetworkFeeBpsNormal   int64
	NetworkFeeBpsElevated int64
	NetworkFeeBpsUrgent   int64
}

func NewCalculator() Calculator {
	return Calculator{
		NetworkFeeBpsNormal:   1,
		NetworkFeeBpsElevated: 3,
		NetworkFeeBpsUrgent:   5,
	}
}

func (c Calculator) Quote(intent domain.Intent, route domain.Route) (domain.FeeBreakdown, error) {
	if err := intent.Validate(); err != nil {
		return domain.FeeBreakdown{}, err
	}
	if err := route.Validate(); err != nil {
		return domain.FeeBreakdown{}, err
	}
	base, err := domain.NewAmount(intent.SourceAsset, intent.Amount).ScaleBps(route.BaseFeeBps)
	if err != nil {
		return domain.FeeBreakdown{}, err
	}
	operator, err := domain.NewAmount(intent.SourceAsset, intent.Amount).ScaleBps(route.OperatorFeeBps)
	if err != nil {
		return domain.FeeBreakdown{}, err
	}
	network, err := domain.NewAmount(intent.SourceAsset, intent.Amount).ScaleBps(c.priorityBps(intent.Priority))
	if err != nil {
		return domain.FeeBreakdown{}, err
	}
	if base.Units < route.MinFee {
		base.Units = route.MinFee
	}
	breakdown := domain.FeeBreakdown{
		BaseFee:     base.Units,
		OperatorFee: operator.Units,
		NetworkFee:  network.Units,
	}
	breakdown.TotalFee = breakdown.BaseFee + breakdown.OperatorFee + breakdown.NetworkFee
	if err := breakdown.Validate(intent.MaxFee); err != nil {
		return domain.FeeBreakdown{}, err
	}
	return breakdown, nil
}

func (c Calculator) priorityBps(priority domain.PriorityClass) int64 {
	switch priority {
	case domain.PriorityUrgent:
		return c.NetworkFeeBpsUrgent
	case domain.PriorityElevated:
		return c.NetworkFeeBpsElevated
	default:
		return c.NetworkFeeBpsNormal
	}
}

func DestinationAmount(intent domain.Intent, route domain.Route) (int64, error) {
	gross := new(big.Int).Mul(big.NewInt(intent.Amount), big.NewInt(route.EffectiveFXRatePPM()))
	gross.Quo(gross, big.NewInt(1_000_000))
	if route.LiquidityImpactBPS > 0 {
		if route.Liquidity <= 0 {
			return 0, domain.Insufficient("route has no pricing liquidity")
		}
		impact := new(big.Int).Mul(gross, big.NewInt(intent.Amount))
		impact.Mul(impact, big.NewInt(route.LiquidityImpactBPS))
		divisor := new(big.Int).Mul(big.NewInt(route.Liquidity), big.NewInt(10_000))
		impact.Add(impact, new(big.Int).Sub(divisor, big.NewInt(1)))
		impact.Quo(impact, divisor)
		gross.Sub(gross, impact)
	}
	if !gross.IsInt64() || gross.Sign() <= 0 {
		return 0, domain.Invalid("destination quote exceeds supported amount")
	}
	return gross.Int64(), nil
}

func TotalDebit(intent domain.Intent, breakdown domain.FeeBreakdown) int64 {
	return intent.Amount + breakdown.TotalFee
}
