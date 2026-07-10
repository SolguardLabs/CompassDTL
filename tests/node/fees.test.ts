import assert from "node:assert/strict";
import test from "node:test";
import { balanceOf, resultByLabel, runFixture } from "../helpers/runner.ts";

test("quotes route fees and reserves exact debit funding", () => {
  const result = runFixture("fee_quote");
  const ticketAction = resultByLabel(result, "ticket");
  const submit = ticketAction.submit as {
    quote: { fees: { baseFee: number; operatorFee: number; networkFee: number; totalFee: number } };
    ticket: { totalDebit?: number };
  };

  assert.deepEqual(submit.quote.fees, {
    baseFee: 80,
    operatorFee: 20,
    networkFee: 10,
    totalFee: 110,
  });

  const alice = balanceOf(result, "acct:alice", "usdc");
  assert.equal(alice.available, 1899890);
  assert.equal(alice.reserved, 100110);
});
