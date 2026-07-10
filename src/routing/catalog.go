package routing

import (
	"fmt"
	"sort"
	"sync"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type Catalog struct {
	mu     sync.RWMutex
	routes map[domain.RouteID]domain.Route
}

func NewCatalog(routes []domain.Route) (*Catalog, error) {
	catalog := &Catalog{routes: make(map[domain.RouteID]domain.Route)}
	for _, route := range routes {
		if err := catalog.Upsert(route); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

func (c *Catalog) Upsert(route domain.Route) error {
	if err := route.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.routes[route.ID] = route
	return nil
}

func (c *Catalog) Get(routeID domain.RouteID) (domain.Route, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	route, ok := c.routes[routeID]
	return route, ok
}

func (c *Catalog) MustGet(routeID domain.RouteID) (domain.Route, error) {
	route, ok := c.Get(routeID)
	if !ok {
		return domain.Route{}, domain.NotFound(fmt.Sprintf("route %s not found", routeID))
	}
	return route, nil
}

func (c *Catalog) List() []domain.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	routes := make([]domain.Route, 0, len(c.routes))
	for _, route := range c.routes {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].ID < routes[j].ID
	})
	return routes
}

func (c *Catalog) Candidates(intent domain.Intent) []domain.Route {
	c.mu.RLock()
	defer c.mu.RUnlock()
	routes := make([]domain.Route, 0)
	for _, route := range c.routes {
		if !route.ActiveForSubmission() {
			continue
		}
		if !route.Accepts(intent.SourceAsset, intent.DestinationAsset) {
			continue
		}
		if route.Liquidity < intent.Amount {
			continue
		}
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].PriorityBias == routes[j].PriorityBias {
			return routes[i].ID < routes[j].ID
		}
		return routes[i].PriorityBias > routes[j].PriorityBias
	})
	return routes
}

func (c *Catalog) Update(request domain.RouteUpdateRequest) (domain.Route, error) {
	if err := request.Validate(); err != nil {
		return domain.Route{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	route, ok := c.routes[request.RouteID]
	if !ok {
		return domain.Route{}, domain.NotFound(fmt.Sprintf("route %s not found", request.RouteID))
	}
	if request.Status != nil {
		route.Status = *request.Status
	}
	if request.LiquidityDelta != 0 {
		route.Liquidity += request.LiquidityDelta
		if route.Liquidity < 0 {
			return domain.Route{}, domain.Invalid("liquidity update would make route negative")
		}
	}
	if request.MaxExposure != nil {
		route.MaxExposure = *request.MaxExposure
	}
	if request.MaxTicketSize != nil {
		route.MaxTicketSize = *request.MaxTicketSize
	}
	if request.PriorityBiasDelta != 0 {
		route.PriorityBias += request.PriorityBiasDelta
	}
	c.routes[request.RouteID] = route
	return route, nil
}

func (c *Catalog) ConsumeLiquidity(routeID domain.RouteID, amount int64) (domain.Route, error) {
	if amount < 0 {
		return domain.Route{}, domain.Invalid("liquidity amount cannot be negative")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	route, ok := c.routes[routeID]
	if !ok {
		return domain.Route{}, domain.NotFound(fmt.Sprintf("route %s not found", routeID))
	}
	if route.Liquidity < amount {
		return domain.Route{}, domain.Insufficient(fmt.Sprintf("route %s liquidity is insufficient", routeID))
	}
	route.Liquidity -= amount
	c.routes[routeID] = route
	return route, nil
}

func (c *Catalog) AddLiquidity(routeID domain.RouteID, amount int64) (domain.Route, error) {
	if amount < 0 {
		return domain.Route{}, domain.Invalid("liquidity amount cannot be negative")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	route, ok := c.routes[routeID]
	if !ok {
		return domain.Route{}, domain.NotFound(fmt.Sprintf("route %s not found", routeID))
	}
	route.Liquidity += amount
	c.routes[routeID] = route
	return route, nil
}
