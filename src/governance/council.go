package governance

import (
	"fmt"
	"sort"
	"sync"
)

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusReady     Status = "ready"
	StatusExecuted  Status = "executed"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
)

type Record struct {
	ID             string        `json:"id"`
	Spec           OperationSpec `json:"spec"`
	Approvals      []string      `json:"approvals"`
	Status         Status        `json:"status"`
	ScheduledEpoch uint64        `json:"scheduledEpoch"`
	ExecutedEpoch  uint64        `json:"executedEpoch,omitempty"`
	CancelledEpoch uint64        `json:"cancelledEpoch,omitempty"`
	CancelReason   string        `json:"cancelReason,omitempty"`
}

type storedRecord struct {
	record    Record
	approvals map[string]struct{}
}

type Council struct {
	mu         sync.Mutex
	governors  map[string]struct{}
	guardian   string
	quorum     int
	operations map[string]*storedRecord
}

func NewCouncil(governors []string, quorum int, guardian string) (*Council, error) {
	if len(governors) == 0 {
		return nil, fmt.Errorf("at least one governor is required")
	}
	if quorum <= 0 || quorum > len(governors) {
		return nil, fmt.Errorf("quorum must be within the governor set")
	}
	set := make(map[string]struct{}, len(governors))
	for _, governor := range governors {
		if governor == "" {
			return nil, fmt.Errorf("governor identity is required")
		}
		if _, duplicate := set[governor]; duplicate {
			return nil, fmt.Errorf("duplicate governor %s", governor)
		}
		set[governor] = struct{}{}
	}
	if guardian == "" {
		return nil, fmt.Errorf("guardian identity is required")
	}
	return &Council{
		governors:  set,
		guardian:   guardian,
		quorum:     quorum,
		operations: make(map[string]*storedRecord),
	}, nil
}

func (c *Council) Schedule(spec OperationSpec, proposer string, epoch uint64) (Record, error) {
	if _, ok := c.governors[proposer]; !ok {
		return Record{}, fmt.Errorf("proposer is not a governor")
	}
	if spec.EarliestEpoch <= epoch {
		return Record{}, fmt.Errorf("earliest execution must be in the future")
	}
	id, err := spec.ID()
	if err != nil {
		return Record{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.operations[id]; exists {
		return Record{}, fmt.Errorf("operation is already scheduled")
	}
	stored := &storedRecord{
		record: Record{
			ID:             id,
			Spec:           spec,
			Status:         StatusScheduled,
			ScheduledEpoch: epoch,
		},
		approvals: map[string]struct{}{proposer: {}},
	}
	c.operations[id] = stored
	return c.snapshot(stored, epoch), nil
}

func (c *Council) Approve(id string, governor string, epoch uint64) (Record, error) {
	if _, ok := c.governors[governor]; !ok {
		return Record{}, fmt.Errorf("approver is not a governor")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stored, ok := c.operations[id]
	if !ok {
		return Record{}, fmt.Errorf("operation not found")
	}
	if terminal(stored.record.Status) {
		return Record{}, fmt.Errorf("operation is terminal")
	}
	if epoch >= stored.record.Spec.ExpiresEpoch {
		stored.record.Status = StatusExpired
		return c.snapshot(stored, epoch), fmt.Errorf("operation has expired")
	}
	stored.approvals[governor] = struct{}{}
	return c.snapshot(stored, epoch), nil
}

func (c *Council) Cancel(id string, guardian string, reason string, epoch uint64) (Record, error) {
	if guardian != c.guardian {
		return Record{}, fmt.Errorf("caller is not the guardian")
	}
	if reason == "" {
		return Record{}, fmt.Errorf("cancellation reason is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stored, ok := c.operations[id]
	if !ok {
		return Record{}, fmt.Errorf("operation not found")
	}
	if terminal(stored.record.Status) {
		return Record{}, fmt.Errorf("operation is terminal")
	}
	stored.record.Status = StatusCancelled
	stored.record.CancelledEpoch = epoch
	stored.record.CancelReason = reason
	return c.snapshot(stored, epoch), nil
}

func (c *Council) Execute(id string, epoch uint64) (Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored, ok := c.operations[id]
	if !ok {
		return Record{}, fmt.Errorf("operation not found")
	}
	if terminal(stored.record.Status) {
		return Record{}, fmt.Errorf("operation is terminal")
	}
	if epoch >= stored.record.Spec.ExpiresEpoch {
		stored.record.Status = StatusExpired
		return c.snapshot(stored, epoch), fmt.Errorf("operation has expired")
	}
	if epoch < stored.record.Spec.EarliestEpoch {
		return c.snapshot(stored, epoch), fmt.Errorf("timelock has not elapsed")
	}
	if len(stored.approvals) < c.quorum {
		return c.snapshot(stored, epoch), fmt.Errorf("operation has not reached quorum")
	}
	if predecessor := stored.record.Spec.Predecessor; predecessor != "" {
		prior, exists := c.operations[predecessor]
		if !exists || prior.record.Status != StatusExecuted {
			return c.snapshot(stored, epoch), fmt.Errorf("predecessor has not executed")
		}
	}
	stored.record.Status = StatusExecuted
	stored.record.ExecutedEpoch = epoch
	return c.snapshot(stored, epoch), nil
}

func (c *Council) Get(id string, epoch uint64) (Record, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored, ok := c.operations[id]
	if !ok {
		return Record{}, false
	}
	return c.snapshot(stored, epoch), true
}

func (c *Council) snapshot(stored *storedRecord, epoch uint64) Record {
	record := stored.record
	if !terminal(record.Status) {
		switch {
		case epoch >= record.Spec.ExpiresEpoch:
			record.Status = StatusExpired
		case epoch >= record.Spec.EarliestEpoch && len(stored.approvals) >= c.quorum:
			record.Status = StatusReady
		default:
			record.Status = StatusScheduled
		}
	}
	record.Approvals = make([]string, 0, len(stored.approvals))
	for approval := range stored.approvals {
		record.Approvals = append(record.Approvals, approval)
	}
	sort.Strings(record.Approvals)
	return record
}

func terminal(status Status) bool {
	return status == StatusExecuted || status == StatusCancelled || status == StatusExpired
}
