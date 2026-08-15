package capital

import (
	"testing"

	"github.com/solguardlabs/compassdtl/src/domain"
)

func routeInput(id string, corridor string, exposure int64) RouteInput {
	return RouteInput{
		RouteID:               domain.RouteID(id),
		Corridor:              domain.CorridorID(corridor),
		Liquidity:             1_000_000,
		ReservedLiquidity:     100_000,
		Exposure:              exposure,
		MaxExposure:           900_000,
		QueuedPrincipal:       300_000,
		SettlementDelayEpochs: 2,
		LiquidityHaircutBPS:   500,
		SettlementShockBPS:    2_000,
		OperationalBufferBPS:  800,
	}
}

func TestEvaluateRouteUsesConservativeRounding(t *testing.T) {
	metrics, err := EvaluateRoute(routeInput("route:alpha", "corridor:a", 400_000))
	if err != nil {
		t.Fatal(err)
	}
	if metrics.EffectiveLiquidity != 855_000 {
		t.Fatalf("effective liquidity = %d", metrics.EffectiveLiquidity)
	}
	if metrics.StressedOutflows != 360_000 || metrics.OperationalBuffer != 32_000 {
		t.Fatalf("unexpected stress result: %+v", metrics)
	}
	if metrics.RequiredLiquidity != 392_000 || metrics.LiquidityShortfall != 0 {
		t.Fatalf("unexpected requirement: %+v", metrics)
	}
	if metrics.SettlementCapacity != 463_000 || !metrics.Compliant {
		t.Fatalf("unexpected route capacity: %+v", metrics)
	}
}

func TestEvaluateRouteReportsShortfallAndExposureBreach(t *testing.T) {
	input := routeInput("route:alpha", "corridor:a", 950_000)
	input.Liquidity = 500_000
	input.ReservedLiquidity = 200_000
	input.QueuedPrincipal = 500_000
	metrics, err := EvaluateRoute(input)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.LiquidityShortfall == 0 || metrics.ExposureHeadroom != 0 || metrics.Compliant {
		t.Fatalf("expected non-compliant route: %+v", metrics)
	}
}

func TestEvaluatePortfolioMeasuresConcentration(t *testing.T) {
	left := routeInput("route:alpha", "corridor:a", 750_000)
	right := routeInput("route:beta", "corridor:b", 250_000)
	metrics, err := EvaluatePortfolio([]RouteInput{left, right})
	if err != nil {
		t.Fatal(err)
	}
	if metrics.ExposureHHIBPS != 6_250 {
		t.Fatalf("HHI = %d", metrics.ExposureHHIBPS)
	}
	if metrics.LargestRouteConcentrationBPS != 7_500 {
		t.Fatalf("largest concentration = %d", metrics.LargestRouteConcentrationBPS)
	}
	if len(metrics.Corridors) != 2 {
		t.Fatalf("corridors = %d", len(metrics.Corridors))
	}
}

func TestStressGridIsMonotonic(t *testing.T) {
	points, err := BuildStressGrid(
		[]RouteInput{routeInput("route:alpha", "corridor:a", 400_000)},
		[]int64{0, 1_000},
		[]int64{0, 2_000},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 4 {
		t.Fatalf("points = %d", len(points))
	}
	if points[3].CoverageBPS >= points[0].CoverageBPS {
		t.Fatalf("stress should reduce coverage: %+v", points)
	}
}

func TestRouteInputRejectsInvalidReserveState(t *testing.T) {
	input := routeInput("route:alpha", "corridor:a", 100_000)
	input.ReservedLiquidity = input.Liquidity + 1
	if _, err := EvaluateRoute(input); err == nil {
		t.Fatal("expected reserved liquidity validation error")
	}
}
