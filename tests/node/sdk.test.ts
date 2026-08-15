import assert from "node:assert/strict";
import test from "node:test";
import {
  CompassClient,
  CompassHTTPError,
  canonicalJSON,
  computeRouteCapital,
  payloadHash,
  type IntentInput,
} from "../../sdk/compassClient.ts";

const intent: IntentInput = {
  id: "intent:sdk-001",
  sourceAccount: "acct:alice",
  destinationAccount: "acct:bob",
  sourceAsset: "usdc",
  destinationAsset: "eurc",
  amount: 100_000n,
  maxFee: 500n,
  priority: "urgent",
};

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  headers.set("content-type", "application/json; charset=utf-8");
  return new Response(JSON.stringify(body), { ...init, headers });
}

test("capital preview uses conservative integer rounding", () => {
  const metrics = computeRouteCapital({
    liquidity: 1_000_000n,
    reservedLiquidity: 100_000n,
    exposure: 400_000n,
    maxExposure: 900_000n,
    queuedPrincipal: 300_000n,
    liquidityHaircutBps: 500n,
    settlementShockBps: 2_000n,
    operationalBufferBps: 800n,
  });
  assert.equal(metrics.effectiveLiquidity, 855_000n);
  assert.equal(metrics.requiredLiquidity, 392_000n);
  assert.equal(metrics.settlementCapacity, 463_000n);
  assert.equal(metrics.compliant, true);
});

test("canonical payloads preserve bigint precision and stable ordering", () => {
  const left = canonicalJSON({ z: 2n ** 63n, a: { y: 2, x: 1 } });
  const right = canonicalJSON({ a: { x: 1, y: 2 }, z: 2n ** 63n });
  assert.equal(left, right);
  assert.equal(
    payloadHash({ z: 2n ** 63n, a: { y: 2, x: 1 } }),
    payloadHash({ a: { x: 1, y: 2 }, z: 2n ** 63n }),
  );
});

test("client enforces transport policy", () => {
  assert.throws(() => new CompassClient("http://payments.example"), /requires HTTPS/);
  assert.throws(() => new CompassClient("https://user:secret@payments.example"), /credentials/);
  assert.doesNotThrow(
    () => new CompassClient("http://127.0.0.1:8087", { allowInsecureLocalhost: true }),
  );
});

test("submit sends canonical decimal fields and idempotency", async () => {
  let captured: { url?: string; init?: RequestInit } = {};
  const fetchImpl: typeof fetch = async (input, init) => {
    captured = { url: String(input), init };
    return jsonResponse({ accepted: true }, { status: 202 });
  };
  const client = new CompassClient("https://payments.example/api/", { fetchImpl });
  await client.submit(intent, "idem-sdk-001");
  assert.equal(captured.url, "https://payments.example/api/v1/intents");
  assert.equal(new Headers(captured.init?.headers).get("idempotency-key"), "idem-sdk-001");
  const body = JSON.parse(String(captured.init?.body)) as { intent: Record<string, unknown> };
  assert.equal(body.intent.amount, 100000);
  assert.equal(body.intent.maxFee, 500);
  assert.equal(body.intent.sourceAccount, "acct:alice");
});

test("client rejects non-JSON and oversized responses", async () => {
  const nonJSON: typeof fetch = async () => new Response("ok", { status: 200 });
  await assert.rejects(
    new CompassClient("https://payments.example", { fetchImpl: nonJSON }).snapshot(),
    /content type/,
  );
  const oversized: typeof fetch = async () =>
    jsonResponse({ ok: true }, { headers: { "content-length": "9000" } });
  await assert.rejects(
    new CompassClient("https://payments.example", {
      fetchImpl: oversized,
      maxResponseBytes: 1024,
    }).snapshot(),
    /size limit/,
  );
});

test("client exposes structured HTTP failures", async () => {
  const fetchImpl: typeof fetch = async () =>
    jsonResponse({ error: { code: "limit_exceeded" } }, { status: 409, statusText: "Conflict" });
  const client = new CompassClient("https://payments.example", { fetchImpl });
  await assert.rejects(client.submit(intent, "idem-sdk-002"), (error: unknown) => {
    assert.ok(error instanceof CompassHTTPError);
    assert.equal(error.status, 409);
    assert.deepEqual(error.body, { error: { code: "limit_exceeded" } });
    return true;
  });
});

test("capital and request inputs fail closed", async () => {
  assert.throws(
    () =>
      computeRouteCapital({
        liquidity: 1n,
        reservedLiquidity: 2n,
        exposure: 0n,
        maxExposure: 1n,
        queuedPrincipal: 0n,
        liquidityHaircutBps: 0n,
        settlementShockBps: 0n,
        operationalBufferBps: 0n,
      }),
    /reserved liquidity/,
  );
  const client = new CompassClient("https://payments.example", {
    fetchImpl: async () => jsonResponse({}),
  });
  assert.throws(() => client.submit({ ...intent, id: " intent:sdk-001" }), /normalized/);
  assert.throws(() => client.execute(0), /count/);
});
