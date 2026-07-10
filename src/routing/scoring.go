package routing

import (
	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/fees"
	"github.com/solguardlabs/compassdtl/src/risk"
)

type Scorer struct {
	FeeCalculator fees.Calculator
}

func NewScorer(calculator fees.Calculator) Scorer {
	return Scorer{FeeCalculator: calculator}
}

func (s Scorer) Quote(intent domain.Intent, route domain.Route, exposures *risk.Book, epoch uint64, ttl uint64) (domain.RouteQuote, error) {
	breakdown, err := s.FeeCalculator.Quote(intent, route)
	if err != nil {
		return domain.RouteQuote{}, err
	}
	exposure := exposures.RouteExposure(route.ID)
	score := s.Score(intent, route, breakdown, exposure)
	return domain.RouteQuote{
		RouteID:           route.ID,
		Corridor:          route.Corridor,
		Amount:            intent.Amount,
		DestinationAmount: fees.DestinationAmount(intent, route),
		Fees:              breakdown,
		Score:             score,
		ObservedExposure:  exposure,
		ObservedLiquidity: route.Liquidity,
		QuoteEpoch:        epoch,
		ExpiresEpoch:      epoch + ttl,
	}, nil
}

func (s Scorer) Score(intent domain.Intent, route domain.Route, breakdown domain.FeeBreakdown, exposure int64) domain.ScoreBreakdown {
	liquidityScore := liquidityScore(route.Liquidity, intent.Amount)
	feeScore := feeScore(intent.Amount, breakdown.TotalFee)
	latencyScore := latencyScore(route.SettlementDelay)
	exposureScore := exposureScore(route, exposure)
	priorityScore := intent.Priority.Weight() + route.PriorityBias
	return domain.ScoreBreakdown{
		LiquidityScore: liquidityScore,
		FeeScore:       feeScore,
		LatencyScore:   latencyScore,
		ExposureScore:  exposureScore,
		PriorityScore:  priorityScore,
	}.Normalize()
}

func liquidityScore(liquidity int64, amount int64) int64 {
	if amount <= 0 {
		return 0
	}
	if liquidity <= 0 {
		return -1_000
	}
	ratio := liquidity * 100 / amount
	switch {
	case ratio >= 8_00:
		return 300
	case ratio >= 4_00:
		return 220
	case ratio >= 2_00:
		return 140
	case ratio >= 1_50:
		return 80
	case ratio >= 1_00:
		return 30
	default:
		return -300
	}
}

func feeScore(amount int64, totalFee int64) int64 {
	if amount <= 0 {
		return 0
	}
	bps := totalFee * domain.BpsDenominator / amount
	switch {
	case bps <= 5:
		return 240
	case bps <= 10:
		return 180
	case bps <= 25:
		return 80
	case bps <= 50:
		return 20
	default:
		return -120
	}
}

func latencyScore(delay uint64) int64 {
	switch {
	case delay == 0:
		return 180
	case delay <= 1:
		return 140
	case delay <= 3:
		return 80
	case delay <= 8:
		return 20
	default:
		return -80
	}
}

func exposureScore(route domain.Route, exposure int64) int64 {
	if route.MaxExposure <= 0 {
		return -1_000
	}
	usage := exposure * 100 / route.MaxExposure
	score := int64(220 - usage*3)
	if route.PreferLowExposure && usage > 60 {
		score -= 80
	}
	return domain.ClampInt64(score, -260, 220)
}
