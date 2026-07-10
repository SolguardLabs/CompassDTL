package risk

import (
	"fmt"
	"sort"
	"sync"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type RouteLimit struct {
	RouteID       domain.RouteID
	Corridor      domain.CorridorID
	MaxExposure   int64
	MaxTicketSize int64
}

type Book struct {
	mu                 sync.RWMutex
	routeLimits        map[domain.RouteID]RouteLimit
	corridorLimits     map[domain.CorridorID]domain.CorridorLimit
	routeExposure      map[domain.RouteID]int64
	corridorExposure   map[domain.CorridorID]int64
	corridorDailyGross map[domain.CorridorID]int64
}

func NewBook() *Book {
	return &Book{
		routeLimits:        make(map[domain.RouteID]RouteLimit),
		corridorLimits:     make(map[domain.CorridorID]domain.CorridorLimit),
		routeExposure:      make(map[domain.RouteID]int64),
		corridorExposure:   make(map[domain.CorridorID]int64),
		corridorDailyGross: make(map[domain.CorridorID]int64),
	}
}

func (b *Book) RegisterRoute(route domain.Route) error {
	if err := route.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routeLimits[route.ID] = RouteLimit{
		RouteID:       route.ID,
		Corridor:      route.Corridor,
		MaxExposure:   route.MaxExposure,
		MaxTicketSize: route.MaxTicketSize,
	}
	if _, ok := b.routeExposure[route.ID]; !ok {
		b.routeExposure[route.ID] = 0
	}
	if _, ok := b.corridorExposure[route.Corridor]; !ok {
		b.corridorExposure[route.Corridor] = 0
	}
	return nil
}

func (b *Book) RegisterCorridor(limit domain.CorridorLimit) error {
	if err := limit.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.corridorLimits[limit.Corridor] = limit
	if _, ok := b.corridorExposure[limit.Corridor]; !ok {
		b.corridorExposure[limit.Corridor] = 0
	}
	if _, ok := b.corridorDailyGross[limit.Corridor]; !ok {
		b.corridorDailyGross[limit.Corridor] = 0
	}
	return nil
}

func (b *Book) UpdateRouteLimit(route domain.Route) error {
	if err := route.Validate(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	limit, ok := b.routeLimits[route.ID]
	if !ok {
		return domain.NotFound(fmt.Sprintf("route %s not registered in risk book", route.ID))
	}
	limit.Corridor = route.Corridor
	limit.MaxExposure = route.MaxExposure
	limit.MaxTicketSize = route.MaxTicketSize
	b.routeLimits[route.ID] = limit
	return nil
}

func (b *Book) Authorize(intent domain.Intent, route domain.Route, quote domain.RouteQuote, epoch uint64, ttl uint64) domain.RiskAdmission {
	b.mu.RLock()
	defer b.mu.RUnlock()
	routeLimit := b.routeLimits[route.ID]
	corridorLimit := b.corridorLimits[route.Corridor]
	routeBefore := b.routeExposure[route.ID]
	corridorBefore := b.corridorExposure[route.Corridor]
	routeAfter := routeBefore + intent.Amount
	corridorAfter := corridorBefore + intent.Amount
	admission := domain.RiskAdmission{
		RouteID:             route.ID,
		Corridor:            route.Corridor,
		Granted:             true,
		Score:               quote.Score.Total,
		ExposureBefore:      routeBefore,
		ExposureAfter:       routeAfter,
		RouteMaxExposure:    routeLimit.MaxExposure,
		CorridorBefore:      corridorBefore,
		CorridorAfter:       corridorAfter,
		CorridorMaxExposure: corridorLimit.MaxExposure,
		ValidUntilEpoch:     epoch + ttl,
	}
	if routeLimit.RouteID == "" {
		admission.Granted = false
		admission.Reason = "route limit is not configured"
		return admission
	}
	if corridorLimit.Corridor == "" {
		admission.Granted = false
		admission.Reason = "corridor limit is not configured"
		return admission
	}
	if intent.Amount > routeLimit.MaxTicketSize {
		admission.Granted = false
		admission.Reason = "ticket exceeds route max size"
		return admission
	}
	if routeAfter > routeLimit.MaxExposure {
		admission.Granted = false
		admission.Reason = "route exposure cap reached"
		return admission
	}
	if corridorAfter > corridorLimit.MaxExposure {
		admission.Granted = false
		admission.Reason = "corridor exposure cap reached"
		return admission
	}
	if b.corridorDailyGross[route.Corridor]+intent.Amount > corridorLimit.MaxDailyGross {
		admission.Granted = false
		admission.Reason = "corridor daily gross cap reached"
		return admission
	}
	admission.Reason = "admitted"
	return admission
}

func (b *Book) ApplySettlement(route domain.Route, amount int64) error {
	if amount < 0 {
		return domain.Invalid("settlement exposure amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.routeLimits[route.ID]; !ok {
		return domain.NotFound(fmt.Sprintf("route %s not registered", route.ID))
	}
	b.routeExposure[route.ID] += amount
	b.corridorExposure[route.Corridor] += amount
	b.corridorDailyGross[route.Corridor] += amount
	return nil
}

func (b *Book) ReleaseExposure(route domain.Route, amount int64) error {
	if amount < 0 {
		return domain.Invalid("release exposure amount cannot be negative")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.routeExposure[route.ID] = domain.MaxInt64(0, b.routeExposure[route.ID]-amount)
	b.corridorExposure[route.Corridor] = domain.MaxInt64(0, b.corridorExposure[route.Corridor]-amount)
	return nil
}

func (b *Book) AdjustExposure(route domain.Route, delta int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.routeLimits[route.ID]; !ok {
		return domain.NotFound(fmt.Sprintf("route %s not registered", route.ID))
	}
	nextRoute := b.routeExposure[route.ID] + delta
	nextCorridor := b.corridorExposure[route.Corridor] + delta
	if nextRoute < 0 || nextCorridor < 0 {
		return domain.Invalid("exposure adjustment would make exposure negative")
	}
	b.routeExposure[route.ID] = nextRoute
	b.corridorExposure[route.Corridor] = nextCorridor
	return nil
}

func (b *Book) ResetDailyGross() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for corridor := range b.corridorDailyGross {
		b.corridorDailyGross[corridor] = 0
	}
}

func (b *Book) RouteExposure(routeID domain.RouteID) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.routeExposure[routeID]
}

func (b *Book) CorridorExposure(corridor domain.CorridorID) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.corridorExposure[corridor]
}

func (b *Book) CorridorGross(corridor domain.CorridorID) int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.corridorDailyGross[corridor]
}

func (b *Book) RouteLimit(routeID domain.RouteID) (RouteLimit, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	limit, ok := b.routeLimits[routeID]
	return limit, ok
}

func (b *Book) Snapshots() []domain.CorridorSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snapshots := make([]domain.CorridorSnapshot, 0, len(b.corridorLimits))
	for corridor, limit := range b.corridorLimits {
		snapshots = append(snapshots, domain.CorridorSnapshot{
			ID:            corridor,
			Exposure:      b.corridorExposure[corridor],
			MaxExposure:   limit.MaxExposure,
			DailyGross:    b.corridorDailyGross[corridor],
			MaxDailyGross: limit.MaxDailyGross,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ID < snapshots[j].ID
	})
	return snapshots
}

func (b *Book) RouteExposureSnapshots(routes []domain.Route) []domain.RouteSnapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snapshots := make([]domain.RouteSnapshot, 0, len(routes))
	for _, route := range routes {
		snapshots = append(snapshots, domain.RouteSnapshot{
			ID:                route.ID,
			Corridor:          route.Corridor,
			Status:            route.Status,
			SourceAsset:       route.SourceAsset,
			DestinationAsset:  route.DestinationAsset,
			Liquidity:         route.Liquidity,
			MaxExposure:       route.MaxExposure,
			Exposure:          b.routeExposure[route.ID],
			MaxTicketSize:     route.MaxTicketSize,
			PriorityBias:      route.PriorityBias,
			BaseFeeBps:        route.BaseFeeBps,
			OperatorFeeBps:    route.OperatorFeeBps,
			LastObservedEpoch: route.LastObservedEpoch,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ID < snapshots[j].ID
	})
	return snapshots
}

func (b *Book) Issues(routes []domain.Route) []domain.AuditIssue {
	b.mu.RLock()
	defer b.mu.RUnlock()
	issues := make([]domain.AuditIssue, 0)
	for _, route := range routes {
		limit := b.routeLimits[route.ID]
		exposure := b.routeExposure[route.ID]
		if exposure > limit.MaxExposure {
			issues = append(issues, domain.AuditIssue{
				Code:     "route_exposure_above_limit",
				Severity: "high",
				Message:  fmt.Sprintf("route %s exposure is above configured limit", route.ID),
			})
		}
	}
	for corridor, limit := range b.corridorLimits {
		exposure := b.corridorExposure[corridor]
		if exposure > limit.MaxExposure {
			issues = append(issues, domain.AuditIssue{
				Code:     "corridor_exposure_above_limit",
				Severity: "high",
				Message:  fmt.Sprintf("corridor %s exposure is above configured limit", corridor),
			})
		}
	}
	return issues
}
