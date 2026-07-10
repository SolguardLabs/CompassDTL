package routing

import (
	"sort"

	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/risk"
)

type Selector struct {
	Catalog *Catalog
	Scorer  Scorer
	Risk    *risk.Book
	Policy  risk.AdmissionPolicy
}

func NewSelector(catalog *Catalog, scorer Scorer, riskBook *risk.Book, policy risk.AdmissionPolicy) Selector {
	return Selector{
		Catalog: catalog,
		Scorer:  scorer,
		Risk:    riskBook,
		Policy:  policy,
	}
}

func (s Selector) Select(intent domain.Intent, epoch uint64) (domain.SettlementPlan, error) {
	if err := intent.Validate(); err != nil {
		return domain.SettlementPlan{}, err
	}
	candidates := s.Catalog.Candidates(intent)
	if len(candidates) == 0 {
		return domain.SettlementPlan{}, domain.RouteUnavailable("no route accepts the intent")
	}
	evaluated := make([]evaluatedRoute, 0, len(candidates))
	for _, route := range candidates {
		quote, err := s.Scorer.Quote(intent, route, s.Risk, epoch, s.Policy.TTL)
		if err != nil {
			continue
		}
		admission := s.Risk.Authorize(intent, route, quote, epoch, s.Policy.TTL)
		evaluated = append(evaluated, evaluatedRoute{
			route:     route,
			quote:     quote,
			admission: admission,
		})
	}
	sort.SliceStable(evaluated, func(i, j int) bool {
		left := evaluated[i]
		right := evaluated[j]
		if left.admission.Granted != right.admission.Granted {
			return left.admission.Granted
		}
		if left.quote.Score.Total == right.quote.Score.Total {
			return left.route.ID < right.route.ID
		}
		return left.quote.Score.Total > right.quote.Score.Total
	})
	for _, candidate := range evaluated {
		if !candidate.admission.Granted {
			continue
		}
		if !s.Policy.Accepts(candidate.admission) {
			continue
		}
		plan := domain.SettlementPlan{
			Intent:      intent,
			Quote:       candidate.quote,
			Admission:   candidate.admission,
			CreatedAt:   epoch,
			ExecuteFrom: epoch + candidate.route.SettlementDelay,
			ExpiresAt:   epoch + s.Policy.TTL,
		}
		if err := plan.Validate(); err != nil {
			return domain.SettlementPlan{}, err
		}
		return plan, nil
	}
	if len(evaluated) > 0 {
		return domain.SettlementPlan{}, domain.LimitExceeded(evaluated[0].admission.Reason)
	}
	return domain.SettlementPlan{}, domain.RouteUnavailable("routes could not produce a quote")
}

func (s Selector) QuoteAll(intent domain.Intent, epoch uint64) ([]domain.RouteQuote, error) {
	if err := intent.Validate(); err != nil {
		return nil, err
	}
	candidates := s.Catalog.Candidates(intent)
	quotes := make([]domain.RouteQuote, 0, len(candidates))
	for _, route := range candidates {
		quote, err := s.Scorer.Quote(intent, route, s.Risk, epoch, s.Policy.TTL)
		if err != nil {
			continue
		}
		quotes = append(quotes, quote)
	}
	sort.Slice(quotes, func(i, j int) bool {
		if quotes[i].Score.Total == quotes[j].Score.Total {
			return quotes[i].RouteID < quotes[j].RouteID
		}
		return quotes[i].Score.Total > quotes[j].Score.Total
	})
	return quotes, nil
}

type evaluatedRoute struct {
	route     domain.Route
	quote     domain.RouteQuote
	admission domain.RiskAdmission
}
