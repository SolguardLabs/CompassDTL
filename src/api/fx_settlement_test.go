package api_test

import (
	"testing"

	"github.com/solguardlabs/compassdtl/src/api"
	"github.com/solguardlabs/compassdtl/src/domain"
	"github.com/solguardlabs/compassdtl/src/ledger"
	"github.com/solguardlabs/compassdtl/src/scenario"
)

func fxBootstrap(liquidity int64) api.Bootstrap {
	bootstrap := scenario.DefaultBootstrap()
	bootstrap.Routes = bootstrap.Routes[:1]
	bootstrap.Routes[0].FXRatePPM = 1_200_000
	bootstrap.Routes[0].Liquidity = liquidity
	bootstrap.Routes[0].MaxExposure = 500_000
	bootstrap.Routes[0].MaxTicketSize = 200_000
	bootstrap.Accounts = append(bootstrap.Accounts, domain.Account{
		ID: "acct:bob-source", DisplayName: "Bob source", Role: domain.RoleCustomer, Enabled: true,
	})
	bootstrap.Balances = append(bootstrap.Balances, ledger.SeedBalance{
		Account: "acct:bob-source", Asset: "usdc", Available: 500_000,
	})
	for i := range bootstrap.Balances {
		if bootstrap.Balances[i].Account == "acct:route-a-settle" && bootstrap.Balances[i].Asset == "eurc" {
			bootstrap.Balances[i].Available = liquidity
		}
	}
	return bootstrap
}

func TestFundedFXQueueSettlesInDestinationUnits(t *testing.T) {
	service, err := api.NewService(fxBootstrap(300_000))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id     domain.IntentID
		source domain.AccountID
	}{
		{"intent:alice-fx", "acct:alice"},
		{"intent:bob-fx", "acct:bob-source"},
	} {
		accepted, submitErr := service.SubmitIntent(domain.SubmitIntentRequest{Intent: domain.Intent{
			ID: item.id, SourceAccount: item.source, DestinationAccount: "acct:bob",
			SourceAsset: "usdc", DestinationAsset: "eurc", Amount: 100_000, MaxFee: 500,
			Priority: domain.PriorityUrgent,
		}})
		if submitErr != nil {
			t.Fatal(submitErr)
		}
		if accepted.Quote.DestinationAmount != 120_000 {
			t.Fatalf("destination quote = %d", accepted.Quote.DestinationAmount)
		}
	}
	service.AdvanceEpoch(1)
	result, err := service.Execute(domain.ExecuteRequest{Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Receipts) != 2 || len(result.Skipped) != 0 {
		t.Fatalf("unexpected execution result: receipts=%d skipped=%d", len(result.Receipts), len(result.Skipped))
	}
	var beneficiary, reserve int64
	for _, balance := range result.Snapshot.Balances {
		if balance.Account == "acct:bob" && balance.Asset == "eurc" {
			beneficiary = balance.Available
		}
		if balance.Account == "acct:route-a-settle" && balance.Asset == "eurc" {
			reserve = balance.Available
		}
	}
	if beneficiary != 240_000 || reserve != 60_000 {
		t.Fatalf("beneficiary=%d reserve=%d", beneficiary, reserve)
	}
}

func TestCurrentExposureIsCheckedBeforeSettlement(t *testing.T) {
	service, err := api.NewService(scenario.DefaultBootstrap())
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.SubmitIntent(domain.SubmitIntentRequest{Intent: domain.Intent{
		ID: "intent:limit-check", SourceAccount: "acct:alice", DestinationAccount: "acct:bob",
		SourceAsset: "usdc", DestinationAsset: "eurc", Amount: 100_000, MaxFee: 500,
		Priority: domain.PriorityUrgent,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AdjustExposure(domain.ExposureAdjustmentRequest{
		RouteID: "route:atlantic-fast", Delta: 650_000, Reason: "route reconciliation",
	}); err != nil {
		t.Fatal(err)
	}
	service.AdvanceEpoch(1)
	result, err := service.Execute(domain.ExecuteRequest{Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Receipts) != 0 || len(result.Skipped) != 1 || result.Skipped[0].Status != domain.TicketRejected {
		t.Fatalf("stale admission must not settle: %+v", result)
	}
	var beneficiary, available, reserved int64
	for _, balance := range result.Snapshot.Balances {
		if balance.Account == "acct:bob" && balance.Asset == "eurc" {
			beneficiary = balance.Available
		}
		if balance.Account == "acct:alice" && balance.Asset == "usdc" {
			available, reserved = balance.Available, balance.Reserved
		}
	}
	if beneficiary != 0 || available != 2_000_000 || reserved != 0 {
		t.Fatalf("beneficiary=%d source available=%d reserved=%d", beneficiary, available, reserved)
	}
}

func TestInventoryImpactQuotesAndSettlesAtParity(t *testing.T) {
	bootstrap := fxBootstrap(1_000_000)
	bootstrap.Routes[0].FXRatePPM = 1_000_000
	bootstrap.Routes[0].LiquidityImpactBPS = 5_000
	bootstrap.Routes[0].MaxExposure = 1_000_000
	bootstrap.Routes[0].MaxTicketSize = 600_000
	service, err := api.NewService(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := service.SubmitIntent(domain.SubmitIntentRequest{Intent: domain.Intent{
		ID: "intent:inventory-parity", SourceAccount: "acct:alice", DestinationAccount: "acct:bob",
		SourceAsset: "usdc", DestinationAsset: "eurc", Amount: 500_000, MaxFee: 1_000,
		Priority: domain.PriorityUrgent,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Quote.DestinationAmount != 375_000 {
		t.Fatalf("quoted payout = %d", accepted.Quote.DestinationAmount)
	}
	service.AdvanceEpoch(1)
	response, err := service.Execute(domain.ExecuteRequest{Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Receipts) != 1 || response.Receipts[0].DestinationAmount != 375_000 {
		t.Fatalf("unexpected inventory settlement: %+v", response)
	}
	var reserve int64
	for _, balance := range response.Snapshot.Balances {
		if balance.Account == "acct:route-a-settle" && balance.Asset == "eurc" {
			reserve = balance.Available
		}
	}
	if reserve != 625_000 {
		t.Fatalf("route reserve = %d", reserve)
	}
}
