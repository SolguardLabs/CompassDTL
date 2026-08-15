package observability

import (
	"fmt"
	"sort"

	"github.com/solguardlabs/compassdtl/src/domain"
)

const BPS int64 = 10_000

type RouteIndicator struct {
	RouteID             domain.RouteID `json:"routeId"`
	Exposure            int64          `json:"exposure"`
	MaxExposure         int64          `json:"maxExposure"`
	ExposureUtilization int64          `json:"exposureUtilizationBps"`
	Liquidity           int64          `json:"liquidity"`
	QueuedPrincipal     int64          `json:"queuedPrincipal"`
}

type Metrics struct {
	Epoch                     uint64           `json:"epoch"`
	QueueDepth                int              `json:"queueDepth"`
	QueuedPrincipal           int64            `json:"queuedPrincipal"`
	OldestQueuedEpochs        uint64           `json:"oldestQueuedEpochs"`
	SettledReceipts           int              `json:"settledReceipts"`
	SettledPrincipal          int64            `json:"settledPrincipal"`
	AuditIssuesBySeverity     map[string]int   `json:"auditIssuesBySeverity"`
	MaxExposureUtilizationBPS int64            `json:"maxExposureUtilizationBps"`
	RouteExposureHHIBPS       int64            `json:"routeExposureHhiBps"`
	Routes                    []RouteIndicator `json:"routes"`
}

func Build(snapshot domain.SystemSnapshot) (Metrics, error) {
	metrics := Metrics{
		Epoch:                 snapshot.Epoch,
		QueueDepth:            len(snapshot.Queue),
		SettledReceipts:       len(snapshot.Receipts),
		AuditIssuesBySeverity: make(map[string]int),
		Routes:                make([]RouteIndicator, 0, len(snapshot.Routes)),
	}
	queuedByRoute := make(map[domain.RouteID]int64)
	for _, ticket := range snapshot.Queue {
		if ticket.Amount < 0 {
			return Metrics{}, fmt.Errorf("ticket %s has negative amount", ticket.TicketID)
		}
		metrics.QueuedPrincipal += ticket.Amount
		queuedByRoute[ticket.RouteID] += ticket.Amount
		if snapshot.Epoch >= ticket.SubmittedAt {
			age := snapshot.Epoch - ticket.SubmittedAt
			if age > metrics.OldestQueuedEpochs {
				metrics.OldestQueuedEpochs = age
			}
		}
	}
	var totalExposure int64
	for _, route := range snapshot.Routes {
		if route.Exposure < 0 || route.MaxExposure <= 0 {
			return Metrics{}, fmt.Errorf("route %s has invalid exposure state", route.ID)
		}
		utilization := route.Exposure * BPS / route.MaxExposure
		if utilization > metrics.MaxExposureUtilizationBPS {
			metrics.MaxExposureUtilizationBPS = utilization
		}
		metrics.Routes = append(metrics.Routes, RouteIndicator{
			RouteID:             route.ID,
			Exposure:            route.Exposure,
			MaxExposure:         route.MaxExposure,
			ExposureUtilization: utilization,
			Liquidity:           route.Liquidity,
			QueuedPrincipal:     queuedByRoute[route.ID],
		})
		totalExposure += route.Exposure
	}
	if totalExposure > 0 {
		for _, route := range snapshot.Routes {
			share := route.Exposure * BPS / totalExposure
			metrics.RouteExposureHHIBPS += share * share / BPS
		}
	}
	for _, receipt := range snapshot.Receipts {
		if receipt.Amount < 0 {
			return Metrics{}, fmt.Errorf("receipt %s has negative amount", receipt.ID)
		}
		metrics.SettledPrincipal += receipt.Amount
	}
	for _, issue := range snapshot.AuditIssues {
		metrics.AuditIssuesBySeverity[issue.Severity]++
	}
	sort.Slice(metrics.Routes, func(i, j int) bool { return metrics.Routes[i].RouteID < metrics.Routes[j].RouteID })
	return metrics, nil
}
