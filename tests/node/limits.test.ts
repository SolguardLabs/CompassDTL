import assert from "node:assert/strict";
import test from "node:test";
import { resultByLabel, runFixture } from "../helpers/runner.ts";

test("rejects tickets that exceed configured economic limits", () => {
  const result = runFixture("limit_rejection");
  const rejected = resultByLabel(result, "oversized-ticket");
  const error = rejected.error as { code: string; message: string };

  assert.equal(error.code, "limit_exceeded");
  assert.match(error.message, /max size|cap/);
  assert.equal(result.snapshot.queue.length, 0);
  assert.equal(result.snapshot.receipts.length, 0);
});
