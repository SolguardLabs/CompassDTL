package capital

import (
	"fmt"
	"sort"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type CorridorMetrics struct {
	Corridor           domain.CorridorID `json:"corridor"`
	Routes             int               `json:"routes"`
	Exposure           int64             `json:"exposure"`
	EffectiveLiquidity int64             `json:"effectiveLiquidity"`
	RequiredLiquidity  int64             `json:"requiredLiquidity"`
	Shortfall          int64             `json:"shortfall"`
	CoverageBPS        int64             `json:"coverageBps"`
}

type PortfolioMetrics struct {
	Routes                        []RouteMetrics    `json:"routes"`
	Corridors                     []CorridorMetrics `json:"corridors"`
	TotalExposure                 int64             `json:"totalExposure"`
	TotalEffectiveLiquidity       int64             `json:"totalEffectiveLiquidity"`
	TotalRequiredLiquidity        int64             `json:"totalRequiredLiquidity"`
	TotalShortfall                int64             `json:"totalShortfall"`
	TotalSettlementCapacity       int64             `json:"totalSettlementCapacity"`
	PortfolioCoverageBPS          int64             `json:"portfolioCoverageBps"`
	ExposureHHIBPS                int64             `json:"exposureHhiBps"`
	LargestRouteConcentrationBPS  int64             `json:"largestRouteConcentrationBps"`
	WeightedSettlementDelayEpochs int64             `json:"weightedSettlementDelayEpochs"`
	CompliantRoutes               int               `json:"compliantRoutes"`
	Compliant                     bool              `json:"compliant"`
}

func EvaluatePortfolio(inputs []RouteInput) (PortfolioMetrics, error) {
	if len(inputs) == 0 {
		return PortfolioMetrics{}, fmt.Errorf("portfolio requires at least one route")
	}
	result := PortfolioMetrics{Routes: make([]RouteMetrics, 0, len(inputs))}
	type corridorAccumulator struct {
		routes    int
		exposure  int64
		effective int64
		required  int64
	}
	corridors := make(map[domain.CorridorID]corridorAccumulator)
	var weightedDelayNumerator int64
	for _, input := range inputs {
		metrics, err := EvaluateRoute(input)
		if err != nil {
			return PortfolioMetrics{}, fmt.Errorf("route %s: %w", input.RouteID, err)
		}
		result.Routes = append(result.Routes, metrics)
		if result.TotalExposure, err = checkedAdd(result.TotalExposure, input.Exposure); err != nil {
			return PortfolioMetrics{}, err
		}
		if result.TotalEffectiveLiquidity, err = checkedAdd(result.TotalEffectiveLiquidity, metrics.EffectiveLiquidity); err != nil {
			return PortfolioMetrics{}, err
		}
		if result.TotalRequiredLiquidity, err = checkedAdd(result.TotalRequiredLiquidity, metrics.RequiredLiquidity); err != nil {
			return PortfolioMetrics{}, err
		}
		if result.TotalShortfall, err = checkedAdd(result.TotalShortfall, metrics.LiquidityShortfall); err != nil {
			return PortfolioMetrics{}, err
		}
		if result.TotalSettlementCapacity, err = checkedAdd(result.TotalSettlementCapacity, metrics.SettlementCapacity); err != nil {
			return PortfolioMetrics{}, err
		}
		if metrics.Compliant {
			result.CompliantRoutes++
		}
		delayContribution, err := mulDivFloor(metrics.RequiredLiquidity, int64(metrics.SettlementDelayEpochs), 1)
		if err != nil {
			return PortfolioMetrics{}, err
		}
		if weightedDelayNumerator, err = checkedAdd(weightedDelayNumerator, delayContribution); err != nil {
			return PortfolioMetrics{}, err
		}
		accumulator := corridors[input.Corridor]
		accumulator.routes++
		accumulator.exposure, err = checkedAdd(accumulator.exposure, input.Exposure)
		if err != nil {
			return PortfolioMetrics{}, err
		}
		accumulator.effective, err = checkedAdd(accumulator.effective, metrics.EffectiveLiquidity)
		if err != nil {
			return PortfolioMetrics{}, err
		}
		accumulator.required, err = checkedAdd(accumulator.required, metrics.RequiredLiquidity)
		if err != nil {
			return PortfolioMetrics{}, err
		}
		corridors[input.Corridor] = accumulator
	}
	sort.Slice(result.Routes, func(i, j int) bool { return result.Routes[i].RouteID < result.Routes[j].RouteID })
	coverage, err := ratioBPS(result.TotalEffectiveLiquidity, result.TotalRequiredLiquidity)
	if err != nil {
		return PortfolioMetrics{}, err
	}
	result.PortfolioCoverageBPS = coverage
	if result.TotalRequiredLiquidity > 0 {
		result.WeightedSettlementDelayEpochs, err = mulDivFloor(weightedDelayNumerator, 1, result.TotalRequiredLiquidity)
		if err != nil {
			return PortfolioMetrics{}, err
		}
	}
	for corridor, accumulator := range corridors {
		corridorCoverage, err := ratioBPS(accumulator.effective, accumulator.required)
		if err != nil {
			return PortfolioMetrics{}, err
		}
		result.Corridors = append(result.Corridors, CorridorMetrics{
			Corridor:           corridor,
			Routes:             accumulator.routes,
			Exposure:           accumulator.exposure,
			EffectiveLiquidity: accumulator.effective,
			RequiredLiquidity:  accumulator.required,
			Shortfall:          positiveDifference(accumulator.required, accumulator.effective),
			CoverageBPS:        corridorCoverage,
		})
	}
	sort.Slice(result.Corridors, func(i, j int) bool { return result.Corridors[i].Corridor < result.Corridors[j].Corridor })
	if result.TotalExposure > 0 {
		for _, input := range inputs {
			share, err := ratioBPS(input.Exposure, result.TotalExposure)
			if err != nil {
				return PortfolioMetrics{}, err
			}
			squared, err := mulDivFloor(share, share, BPS)
			if err != nil {
				return PortfolioMetrics{}, err
			}
			result.ExposureHHIBPS, err = checkedAdd(result.ExposureHHIBPS, squared)
			if err != nil {
				return PortfolioMetrics{}, err
			}
			if share > result.LargestRouteConcentrationBPS {
				result.LargestRouteConcentrationBPS = share
			}
		}
	}
	result.Compliant = result.TotalShortfall == 0 && result.CompliantRoutes == len(inputs)
	return result, nil
}

type StressPoint struct {
	LiquidityHaircutBPS int64 `json:"liquidityHaircutBps"`
	SettlementShockBPS  int64 `json:"settlementShockBps"`
	CoverageBPS         int64 `json:"coverageBps"`
	Shortfall           int64 `json:"shortfall"`
	SettlementCapacity  int64 `json:"settlementCapacity"`
}

func BuildStressGrid(base []RouteInput, haircuts []int64, shocks []int64) ([]StressPoint, error) {
	points := make([]StressPoint, 0, len(haircuts)*len(shocks))
	for _, haircut := range haircuts {
		for _, shock := range shocks {
			inputs := append([]RouteInput(nil), base...)
			for index := range inputs {
				inputs[index].LiquidityHaircutBPS = haircut
				inputs[index].SettlementShockBPS = shock
			}
			metrics, err := EvaluatePortfolio(inputs)
			if err != nil {
				return nil, err
			}
			points = append(points, StressPoint{
				LiquidityHaircutBPS: haircut,
				SettlementShockBPS:  shock,
				CoverageBPS:         metrics.PortfolioCoverageBPS,
				Shortfall:           metrics.TotalShortfall,
				SettlementCapacity:  metrics.TotalSettlementCapacity,
			})
		}
	}
	return points, nil
}
