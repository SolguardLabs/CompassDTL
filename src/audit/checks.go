package audit

import (
	"fmt"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type SnapshotCheck struct {
	Name string
	Run  func(domain.SystemSnapshot) []domain.AuditIssue
}

func Evaluate(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	checks := []SnapshotCheck{
		{Name: "balances", Run: checkBalances},
		{Name: "routes", Run: checkRoutes},
		{Name: "corridors", Run: checkCorridors},
		{Name: "queue", Run: checkQueue},
		{Name: "receipts", Run: checkReceipts},
	}
	issues := make([]domain.AuditIssue, 0)
	for _, check := range checks {
		issues = append(issues, check.Run(snapshot)...)
	}
	return issues
}

func checkBalances(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	issues := make([]domain.AuditIssue, 0)
	seen := make(map[string]struct{}, len(snapshot.Balances))
	for _, balance := range snapshot.Balances {
		key := fmt.Sprintf("%s/%s", balance.Account, balance.Asset)
		if _, ok := seen[key]; ok {
			issues = append(issues, domain.AuditIssue{
				Code:     "duplicate_balance_row",
				Severity: "medium",
				Message:  fmt.Sprintf("duplicate balance row for %s", key),
			})
		}
		seen[key] = struct{}{}
		if balance.Available < 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "negative_available_balance",
				Severity: "critical",
				Message:  fmt.Sprintf("%s has negative available %s", balance.Account, balance.Asset),
			})
		}
		if balance.Reserved < 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "negative_reserved_balance",
				Severity: "critical",
				Message:  fmt.Sprintf("%s has negative reserved %s", balance.Account, balance.Asset),
			})
		}
	}
	return issues
}

func checkRoutes(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	issues := make([]domain.AuditIssue, 0)
	for _, route := range snapshot.Routes {
		if route.Liquidity < 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "negative_route_liquidity",
				Severity: "critical",
				Message:  fmt.Sprintf("route %s has negative liquidity", route.ID),
			})
		}
		if route.MaxExposure <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_route_limit",
				Severity: "high",
				Message:  fmt.Sprintf("route %s has invalid max exposure", route.ID),
			})
		}
		if route.MaxTicketSize <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_ticket_limit",
				Severity: "high",
				Message:  fmt.Sprintf("route %s has invalid ticket size", route.ID),
			})
		}
		if route.Exposure > route.MaxExposure {
			issues = append(issues, domain.AuditIssue{
				Code:     "route_exposure_limit_mismatch",
				Severity: "high",
				Message:  fmt.Sprintf("route %s exposure exceeds configured max", route.ID),
			})
		}
	}
	return issues
}

func checkCorridors(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	issues := make([]domain.AuditIssue, 0)
	for _, corridor := range snapshot.Corridors {
		if corridor.MaxExposure <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_corridor_limit",
				Severity: "high",
				Message:  fmt.Sprintf("corridor %s has invalid max exposure", corridor.ID),
			})
		}
		if corridor.MaxDailyGross <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_corridor_gross_limit",
				Severity: "high",
				Message:  fmt.Sprintf("corridor %s has invalid daily gross limit", corridor.ID),
			})
		}
		if corridor.Exposure > corridor.MaxExposure {
			issues = append(issues, domain.AuditIssue{
				Code:     "corridor_exposure_limit_mismatch",
				Severity: "high",
				Message:  fmt.Sprintf("corridor %s exposure exceeds configured max", corridor.ID),
			})
		}
		if corridor.DailyGross > corridor.MaxDailyGross {
			issues = append(issues, domain.AuditIssue{
				Code:     "corridor_gross_limit_mismatch",
				Severity: "high",
				Message:  fmt.Sprintf("corridor %s daily gross exceeds configured max", corridor.ID),
			})
		}
	}
	return issues
}

func checkQueue(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	issues := make([]domain.AuditIssue, 0)
	seen := make(map[domain.TicketID]struct{}, len(snapshot.Queue))
	for _, item := range snapshot.Queue {
		if _, ok := seen[item.TicketID]; ok {
			issues = append(issues, domain.AuditIssue{
				Code:     "duplicate_queue_ticket",
				Severity: "high",
				Message:  fmt.Sprintf("ticket %s appears multiple times in queue", item.TicketID),
			})
		}
		seen[item.TicketID] = struct{}{}
		if item.Amount <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_queue_amount",
				Severity: "medium",
				Message:  fmt.Sprintf("ticket %s has invalid queued amount", item.TicketID),
			})
		}
		if item.ExpiresAt <= item.SubmittedAt {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_queue_expiry",
				Severity: "medium",
				Message:  fmt.Sprintf("ticket %s has invalid expiry", item.TicketID),
			})
		}
	}
	return issues
}

func checkReceipts(snapshot domain.SystemSnapshot) []domain.AuditIssue {
	issues := make([]domain.AuditIssue, 0)
	seenReceipts := make(map[domain.ReceiptID]struct{}, len(snapshot.Receipts))
	seenTickets := make(map[domain.TicketID]struct{}, len(snapshot.Receipts))
	for _, receipt := range snapshot.Receipts {
		if _, ok := seenReceipts[receipt.ID]; ok {
			issues = append(issues, domain.AuditIssue{
				Code:     "duplicate_receipt",
				Severity: "high",
				Message:  fmt.Sprintf("receipt %s appears multiple times", receipt.ID),
			})
		}
		seenReceipts[receipt.ID] = struct{}{}
		if _, ok := seenTickets[receipt.TicketID]; ok {
			issues = append(issues, domain.AuditIssue{
				Code:     "duplicate_ticket_receipt",
				Severity: "high",
				Message:  fmt.Sprintf("ticket %s produced multiple receipts", receipt.TicketID),
			})
		}
		seenTickets[receipt.TicketID] = struct{}{}
		if receipt.Amount <= 0 || receipt.DestinationAmount <= 0 {
			issues = append(issues, domain.AuditIssue{
				Code:     "invalid_receipt_amount",
				Severity: "medium",
				Message:  fmt.Sprintf("receipt %s has invalid amount", receipt.ID),
			})
		}
		if receipt.Fees.TotalFee != receipt.Fees.BaseFee+receipt.Fees.OperatorFee+receipt.Fees.NetworkFee {
			issues = append(issues, domain.AuditIssue{
				Code:     "receipt_fee_mismatch",
				Severity: "medium",
				Message:  fmt.Sprintf("receipt %s fee total does not match components", receipt.ID),
			})
		}
	}
	return issues
}
