package settlement

import (
	"sort"
	"sync"

	"github.com/solguardlabs/compassdtl/src/domain"
)

type Queue struct {
	mu      sync.RWMutex
	tickets map[domain.TicketID]domain.SettlementTicket
	order   []domain.TicketID
}

func NewQueue() *Queue {
	return &Queue{
		tickets: make(map[domain.TicketID]domain.SettlementTicket),
		order:   make([]domain.TicketID, 0, 64),
	}
}

func (q *Queue) Enqueue(ticket domain.SettlementTicket) error {
	if err := ticket.Plan.Validate(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.tickets[ticket.ID]; ok {
		return domain.Conflict("ticket already exists")
	}
	q.tickets[ticket.ID] = ticket
	q.order = append(q.order, ticket.ID)
	q.sortLocked()
	return nil
}

func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.order)
}

func (q *Queue) Get(ticketID domain.TicketID) (domain.SettlementTicket, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	ticket, ok := q.tickets[ticketID]
	return ticket, ok
}

func (q *Queue) PopReady(epoch uint64) (domain.SettlementTicket, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sortLocked()
	for index, ticketID := range q.order {
		ticket := q.tickets[ticketID]
		if ticket.Status != domain.TicketQueued {
			continue
		}
		if ticket.Plan.ExecuteFrom > epoch {
			continue
		}
		q.order = append(q.order[:index], q.order[index+1:]...)
		delete(q.tickets, ticketID)
		ticket.Status = domain.TicketExecuting
		ticket.LastUpdatedAt = epoch
		return ticket, true
	}
	return domain.SettlementTicket{}, false
}

func (q *Queue) RemoveExpired(epoch uint64) []domain.SettlementTicket {
	q.mu.Lock()
	defer q.mu.Unlock()
	expired := make([]domain.SettlementTicket, 0)
	next := make([]domain.TicketID, 0, len(q.order))
	for _, ticketID := range q.order {
		ticket := q.tickets[ticketID]
		if ticket.Status == domain.TicketQueued && ticket.Plan.ExpiresAt <= epoch {
			ticket.Status = domain.TicketExpired
			ticket.LastUpdatedAt = epoch
			expired = append(expired, ticket)
			delete(q.tickets, ticketID)
			continue
		}
		next = append(next, ticketID)
	}
	q.order = next
	return expired
}

func (q *Queue) Snapshot() []domain.QueueSnapshot {
	q.mu.RLock()
	defer q.mu.RUnlock()
	snapshots := make([]domain.QueueSnapshot, 0, len(q.order))
	for _, ticketID := range q.order {
		ticket := q.tickets[ticketID]
		snapshots = append(snapshots, domain.QueueSnapshot{
			TicketID:    ticket.ID,
			IntentID:    ticket.IntentID(),
			RouteID:     ticket.RouteID(),
			Status:      ticket.Status,
			QueueScore:  ticket.QueueScore,
			Amount:      ticket.Amount(),
			SubmittedAt: ticket.SubmittedAt,
			ExpiresAt:   ticket.Plan.ExpiresAt,
			Priority:    ticket.Plan.Intent.Priority,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].QueueScore == snapshots[j].QueueScore {
			return snapshots[i].TicketID < snapshots[j].TicketID
		}
		return snapshots[i].QueueScore > snapshots[j].QueueScore
	})
	return snapshots
}

func (q *Queue) Tickets() []domain.SettlementTicket {
	q.mu.RLock()
	defer q.mu.RUnlock()
	tickets := make([]domain.SettlementTicket, 0, len(q.order))
	for _, ticketID := range q.order {
		tickets = append(tickets, q.tickets[ticketID])
	}
	sort.Slice(tickets, func(i, j int) bool {
		if tickets[i].QueueScore == tickets[j].QueueScore {
			return tickets[i].ID < tickets[j].ID
		}
		return tickets[i].QueueScore > tickets[j].QueueScore
	})
	return tickets
}

func (q *Queue) sortLocked() {
	sort.SliceStable(q.order, func(i, j int) bool {
		left := q.tickets[q.order[i]]
		right := q.tickets[q.order[j]]
		if left.QueueScore == right.QueueScore {
			if left.Plan.ExecuteFrom == right.Plan.ExecuteFrom {
				return left.Sequence < right.Sequence
			}
			return left.Plan.ExecuteFrom < right.Plan.ExecuteFrom
		}
		return left.QueueScore > right.QueueScore
	})
}
