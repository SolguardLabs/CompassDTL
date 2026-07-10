package risk

import "github.com/solguardlabs/compassdtl/src/domain"

type AdmissionPolicy struct {
	MinScore int64
	TTL      uint64
}

func NewAdmissionPolicy(config domain.EngineConfig) AdmissionPolicy {
	normalized := config.Normalize()
	return AdmissionPolicy{
		MinScore: normalized.MinScore,
		TTL:      normalized.DefaultSettlementTTL,
	}
}

func (p AdmissionPolicy) Accepts(admission domain.RiskAdmission) bool {
	if !admission.Granted {
		return false
	}
	return admission.Score >= p.MinScore
}

func (p AdmissionPolicy) ValidateExecution(epoch uint64, ticket domain.SettlementTicket) error {
	if ticket.Status != domain.TicketQueued && ticket.Status != domain.TicketExecuting {
		return domain.Conflict("ticket is not queued")
	}
	if ticket.Plan.ExpiresAt <= epoch {
		return domain.Conflict("ticket has expired")
	}
	if ticket.Plan.Admission.Score < p.MinScore {
		return domain.LimitExceeded("ticket score is below execution threshold")
	}
	return nil
}
