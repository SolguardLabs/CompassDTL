import assert from "node:assert/strict";
import test from "node:test";
import { resultByLabel, runFixture } from "../helpers/runner.ts";

test("selects the highest scoring eligible route", () => {
  const result = runFixture("route_selection");
  const quotesAction = resultByLabel(result, "quotes");
  const quotes = quotesAction.quotes as Array<{ routeId: string; score: { total: number } }>;

  assert.equal(quotes[0].routeId, "route:atlantic-fast");
  assert.ok(quotes[0].score.total > quotes[1].score.total);

  const ticketAction = resultByLabel(result, "ticket");
  const submit = ticketAction.submit as {
    ticket: { plan: { quote: { routeId: string; observedExposure: number } } };
  };
  assert.equal(submit.ticket.plan.quote.routeId, "route:atlantic-fast");
  assert.equal(submit.ticket.plan.quote.observedExposure, 0);
});
