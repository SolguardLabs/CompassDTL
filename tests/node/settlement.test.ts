import assert from "node:assert/strict";
import test from "node:test";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("executes ready tickets by priority and posts settlement entries", () => {
  const result = runFixture("priority_settlement");
  const execution = resultByLabel(result, "first-execution").execute as {
    receipts: Array<{
      intentId: string;
      routeId: string;
      amount: number;
      fees: { totalFee: number };
    }>;
  };

  assert.equal(execution.receipts.length, 1);
  assert.equal(execution.receipts[0].intentId, "intent:settle-urgent");
  assert.equal(execution.receipts[0].routeId, "route:atlantic-fast");
  assert.equal(execution.receipts[0].amount, 100000);
  assert.equal(execution.receipts[0].fees.totalFee, 150);

  assert.equal(balanceOf(result, "acct:bob", "eurc").available, 100000);
  assert.equal(balanceOf(result, "acct:route-a-settle", "eurc").available, 700000);
  assert.equal(balanceOf(result, "acct:route-a-treasury", "usdc").available, 100000);
  assert.equal(balanceOf(result, "acct:fees", "usdc").available, 150);

  const route = result.snapshot.routes.find((entry) => entry.id === "route:atlantic-fast");
  assert.equal(route?.exposure, 100000);
  assert.equal(result.snapshot.queue.length, 1);
  assert.equal(result.snapshot.auditIssues.length, 0);
});
