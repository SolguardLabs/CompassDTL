package governance

import "testing"

func testSpec() OperationSpec {
	return OperationSpec{
		Domain:        "compass.governance.v1",
		ChainID:       "settlement-eu-1",
		Target:        "route:atlantic-fast",
		Method:        "set_route_limit",
		PayloadHash:   HashPayload([]byte(`{"maxExposure":"900000"}`)),
		Salt:          "limit-2026-08",
		EarliestEpoch: 10,
		ExpiresEpoch:  20,
	}
}

func TestOperationIDIsDeterministicAndDomainSeparated(t *testing.T) {
	spec := testSpec()
	first, err := spec.ID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := spec.ID()
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("unexpected operation ids: %s %s", first, second)
	}
	spec.ChainID = "settlement-us-1"
	third, err := spec.ID()
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("chain separation did not change operation id")
	}
}

func TestCouncilRequiresQuorumAndTimelock(t *testing.T) {
	council, err := NewCouncil([]string{"alice", "bob", "carol"}, 2, "guardian")
	if err != nil {
		t.Fatal(err)
	}
	record, err := council.Schedule(testSpec(), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := council.Execute(record.ID, 10); err == nil {
		t.Fatal("execution without quorum should fail")
	}
	if _, err := council.Approve(record.ID, "bob", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := council.Execute(record.ID, 9); err == nil {
		t.Fatal("execution before timelock should fail")
	}
	executed, err := council.Execute(record.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if executed.Status != StatusExecuted || executed.ExecutedEpoch != 10 {
		t.Fatalf("unexpected executed record: %+v", executed)
	}
}

func TestCouncilEnforcesPredecessor(t *testing.T) {
	council, _ := NewCouncil([]string{"alice", "bob"}, 1, "guardian")
	first, err := council.Schedule(testSpec(), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	secondSpec := testSpec()
	secondSpec.Salt = "dependent-operation"
	secondSpec.Predecessor = first.ID
	second, err := council.Schedule(secondSpec, "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := council.Execute(second.ID, 10); err == nil {
		t.Fatal("dependent operation executed before predecessor")
	}
	if _, err := council.Execute(first.ID, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := council.Execute(second.ID, 10); err != nil {
		t.Fatal(err)
	}
}

func TestGuardianCancellationIsTerminal(t *testing.T) {
	council, _ := NewCouncil([]string{"alice", "bob"}, 2, "guardian")
	record, _ := council.Schedule(testSpec(), "alice", 1)
	if _, err := council.Cancel(record.ID, "mallory", "invalid", 2); err == nil {
		t.Fatal("non-guardian cancellation should fail")
	}
	cancelled, err := council.Cancel(record.ID, "guardian", "capacity incident", 2)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %s", cancelled.Status)
	}
	if _, err := council.Approve(record.ID, "bob", 3); err == nil {
		t.Fatal("cancelled operation accepted approval")
	}
}

func TestExpiredOperationCannotExecute(t *testing.T) {
	council, _ := NewCouncil([]string{"alice"}, 1, "guardian")
	record, _ := council.Schedule(testSpec(), "alice", 1)
	if _, err := council.Execute(record.ID, 20); err == nil {
		t.Fatal("expired operation executed")
	}
	snapshot, ok := council.Get(record.ID, 20)
	if !ok || snapshot.Status != StatusExpired {
		t.Fatalf("unexpected expired record: %+v", snapshot)
	}
}
