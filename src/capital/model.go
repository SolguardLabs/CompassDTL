package capital

import (
	"fmt"
	"math"
	"math/big"

	"github.com/solguardlabs/compassdtl/src/domain"
)

const BPS int64 = 10_000

type RouteInput struct {
	RouteID               domain.RouteID    `json:"routeId"`
	Corridor              domain.CorridorID `json:"corridor"`
	Liquidity             int64             `json:"liquidity"`
	ReservedLiquidity     int64             `json:"reservedLiquidity"`
	Exposure              int64             `json:"exposure"`
	MaxExposure           int64             `json:"maxExposure"`
	QueuedPrincipal       int64             `json:"queuedPrincipal"`
	SettlementDelayEpochs uint64            `json:"settlementDelayEpochs"`
	LiquidityHaircutBPS   int64             `json:"liquidityHaircutBps"`
	SettlementShockBPS    int64             `json:"settlementShockBps"`
	OperationalBufferBPS  int64             `json:"operationalBufferBps"`
}

type RouteMetrics struct {
	RouteID                domain.RouteID    `json:"routeId"`
	Corridor               domain.CorridorID `json:"corridor"`
	AvailableLiquidity     int64             `json:"availableLiquidity"`
	EffectiveLiquidity     int64             `json:"effectiveLiquidity"`
	StressedOutflows       int64             `json:"stressedOutflows"`
	OperationalBuffer      int64             `json:"operationalBuffer"`
	RequiredLiquidity      int64             `json:"requiredLiquidity"`
	LiquidityShortfall     int64             `json:"liquidityShortfall"`
	ExposureHeadroom       int64             `json:"exposureHeadroom"`
	SettlementCapacity     int64             `json:"settlementCapacity"`
	LiquidityCoverageBPS   int64             `json:"liquidityCoverageBps"`
	ExposureUtilizationBPS int64             `json:"exposureUtilizationBps"`
	SettlementDelayEpochs  uint64            `json:"settlementDelayEpochs"`
	Compliant              bool              `json:"compliant"`
}

func EvaluateRoute(input RouteInput) (RouteMetrics, error) {
	if err := input.Validate(); err != nil {
		return RouteMetrics{}, err
	}
	available := input.Liquidity - input.ReservedLiquidity
	effective, err := mulDivFloor(available, BPS-input.LiquidityHaircutBPS, BPS)
	if err != nil {
		return RouteMetrics{}, err
	}
	stressedOutflows, err := mulDivCeil(input.QueuedPrincipal, BPS+input.SettlementShockBPS, BPS)
	if err != nil {
		return RouteMetrics{}, err
	}
	buffer, err := mulDivCeil(input.Exposure, input.OperationalBufferBPS, BPS)
	if err != nil {
		return RouteMetrics{}, err
	}
	required, err := checkedAdd(stressedOutflows, buffer)
	if err != nil {
		return RouteMetrics{}, err
	}
	shortfall := positiveDifference(required, effective)
	headroom := positiveDifference(input.MaxExposure, input.Exposure)
	freeLiquidity := positiveDifference(effective, required)
	capacity := min64(freeLiquidity, headroom)
	coverage, err := ratioBPS(effective, required)
	if err != nil {
		return RouteMetrics{}, err
	}
	utilization, err := ratioBPS(input.Exposure, input.MaxExposure)
	if err != nil {
		return RouteMetrics{}, err
	}
	return RouteMetrics{
		RouteID:                input.RouteID,
		Corridor:               input.Corridor,
		AvailableLiquidity:     available,
		EffectiveLiquidity:     effective,
		StressedOutflows:       stressedOutflows,
		OperationalBuffer:      buffer,
		RequiredLiquidity:      required,
		LiquidityShortfall:     shortfall,
		ExposureHeadroom:       headroom,
		SettlementCapacity:     capacity,
		LiquidityCoverageBPS:   coverage,
		ExposureUtilizationBPS: utilization,
		SettlementDelayEpochs:  input.SettlementDelayEpochs,
		Compliant:              shortfall == 0 && input.Exposure <= input.MaxExposure,
	}, nil
}

func (input RouteInput) Validate() error {
	if err := input.RouteID.Validate(); err != nil {
		return err
	}
	if err := input.Corridor.Validate(); err != nil {
		return err
	}
	values := map[string]int64{
		"liquidity":          input.Liquidity,
		"reserved liquidity": input.ReservedLiquidity,
		"exposure":           input.Exposure,
		"max exposure":       input.MaxExposure,
		"queued principal":   input.QueuedPrincipal,
		"liquidity haircut":  input.LiquidityHaircutBPS,
		"settlement shock":   input.SettlementShockBPS,
		"operational buffer": input.OperationalBufferBPS,
	}
	for name, value := range values {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	if input.MaxExposure == 0 {
		return fmt.Errorf("max exposure must be positive")
	}
	if input.ReservedLiquidity > input.Liquidity {
		return fmt.Errorf("reserved liquidity exceeds route liquidity")
	}
	if input.LiquidityHaircutBPS > BPS {
		return fmt.Errorf("liquidity haircut exceeds 10000 bps")
	}
	return nil
}

func mulDivFloor(value int64, numerator int64, denominator int64) (int64, error) {
	if value < 0 || numerator < 0 || denominator <= 0 {
		return 0, fmt.Errorf("mulDiv requires non-negative values and a positive denominator")
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(numerator))
	result := product.Quo(product, big.NewInt(denominator))
	if !result.IsInt64() {
		return 0, fmt.Errorf("mulDiv result exceeds int64")
	}
	return result.Int64(), nil
}

func mulDivCeil(value int64, numerator int64, denominator int64) (int64, error) {
	if value == 0 || numerator == 0 {
		return 0, nil
	}
	floor, err := mulDivFloor(value, numerator, denominator)
	if err != nil {
		return 0, err
	}
	product := new(big.Int).Mul(big.NewInt(value), big.NewInt(numerator))
	remainder := new(big.Int).Mod(product, big.NewInt(denominator))
	if remainder.Sign() == 0 {
		return floor, nil
	}
	return checkedAdd(floor, 1)
}

func ratioBPS(numerator int64, denominator int64) (int64, error) {
	if denominator == 0 {
		if numerator == 0 {
			return 0, nil
		}
		return math.MaxInt64, nil
	}
	return mulDivFloor(numerator, BPS, denominator)
}

func checkedAdd(left int64, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, fmt.Errorf("integer addition overflow")
	}
	return left + right, nil
}

func positiveDifference(left int64, right int64) int64 {
	if left <= right {
		return 0
	}
	return left - right
}

func min64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
