import { createHash, randomUUID } from "node:crypto";

export const BPS = 10_000n;

export type ClientOptions = {
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
  maxResponseBytes?: number;
  allowInsecureLocalhost?: boolean;
};

export type IntentInput = {
  id: string;
  sourceAccount: string;
  destinationAccount: string;
  sourceAsset: string;
  destinationAsset: string;
  amount: bigint;
  maxFee: bigint;
  priority: "normal" | "elevated" | "urgent";
  requestedEpoch?: bigint;
  metadata?: Record<string, string>;
};

export type RouteCapitalInput = {
  liquidity: bigint;
  reservedLiquidity: bigint;
  exposure: bigint;
  maxExposure: bigint;
  queuedPrincipal: bigint;
  liquidityHaircutBps: bigint;
  settlementShockBps: bigint;
  operationalBufferBps: bigint;
};

export type RouteCapitalMetrics = {
  availableLiquidity: bigint;
  effectiveLiquidity: bigint;
  stressedOutflows: bigint;
  operationalBuffer: bigint;
  requiredLiquidity: bigint;
  liquidityShortfall: bigint;
  exposureHeadroom: bigint;
  settlementCapacity: bigint;
  liquidityCoverageBps: bigint;
  exposureUtilizationBps: bigint;
  compliant: boolean;
};

export class CompassClient {
  readonly #baseURL: URL;
  readonly #fetch: typeof fetch;
  readonly #timeoutMs: number;
  readonly #maxResponseBytes: number;

  constructor(baseURL: string, options: ClientOptions = {}) {
    const parsed = new URL(baseURL);
    const local =
      parsed.hostname === "localhost" ||
      parsed.hostname === "127.0.0.1" ||
      parsed.hostname === "::1";
    if (parsed.protocol !== "https:" && !(local && options.allowInsecureLocalhost)) {
      throw new Error("CompassClient requires HTTPS outside an explicitly allowed local endpoint");
    }
    if (parsed.username || parsed.password || parsed.search || parsed.hash) {
      throw new Error("CompassClient base URL cannot include credentials, query, or fragment");
    }
    parsed.pathname = parsed.pathname.replace(/\/+$/, "") + "/";
    this.#baseURL = parsed;
    this.#fetch = options.fetchImpl ?? fetch;
    this.#timeoutMs = boundedInteger(options.timeoutMs ?? 8_000, 100, 60_000, "timeoutMs");
    this.#maxResponseBytes = boundedInteger(
      options.maxResponseBytes ?? 1_000_000,
      1_024,
      8_000_000,
      "maxResponseBytes",
    );
  }

  snapshot<T = unknown>(signal?: AbortSignal): Promise<T> {
    return this.#request<T>("v1/snapshot", { method: "GET" }, undefined, signal);
  }

  quotes<T = unknown>(intent: IntentInput, signal?: AbortSignal): Promise<T> {
    return this.#request<T>(
      "v1/quotes",
      { method: "POST", body: canonicalJSON({ intent: encodeIntent(intent) }) },
      undefined,
      signal,
    );
  }

  submit<T = unknown>(
    intent: IntentInput,
    idempotencyKey: string = randomUUID(),
    signal?: AbortSignal,
  ): Promise<T> {
    return this.#request<T>(
      "v1/intents",
      { method: "POST", body: canonicalJSON({ intent: encodeIntent(intent) }) },
      normalizedToken(idempotencyKey, "idempotency key"),
      signal,
    );
  }

  execute<T = unknown>(
    count: number,
    idempotencyKey: string = randomUUID(),
    signal?: AbortSignal,
  ): Promise<T> {
    return this.#request<T>(
      "v1/execute",
      { method: "POST", body: canonicalJSON({ count: boundedInteger(count, 1, 1_000, "count") }) },
      normalizedToken(idempotencyKey, "idempotency key"),
      signal,
    );
  }

  async #request<T>(
    path: string,
    init: RequestInit,
    idempotencyKey?: string,
    signal?: AbortSignal,
  ): Promise<T> {
    const url = new URL(path, this.#baseURL);
    if (url.origin !== this.#baseURL.origin || !url.pathname.startsWith(this.#baseURL.pathname)) {
      throw new Error("request path escapes the configured CompassDTL endpoint");
    }
    const controller = new AbortController();
    const timeout = setTimeout(
      () => controller.abort(new Error("request timeout")),
      this.#timeoutMs,
    );
    const relayAbort = () => controller.abort(signal?.reason);
    signal?.addEventListener("abort", relayAbort, { once: true });
    try {
      const headers = new Headers(init.headers);
      headers.set("accept", "application/json");
      if (init.body !== undefined) headers.set("content-type", "application/json");
      if (idempotencyKey !== undefined) headers.set("idempotency-key", idempotencyKey);
      const response = await this.#fetch(url, {
        ...init,
        headers,
        cache: "no-store",
        credentials: "omit",
        redirect: "error",
        signal: controller.signal,
      });
      const contentType = response.headers.get("content-type")?.toLowerCase() ?? "";
      if (!contentType.startsWith("application/json")) {
        throw new Error(`unexpected response content type: ${contentType || "missing"}`);
      }
      const declared = Number(response.headers.get("content-length") ?? "0");
      if (Number.isFinite(declared) && declared > this.#maxResponseBytes) {
        throw new Error("response exceeds configured size limit");
      }
      const text = await response.text();
      if (Buffer.byteLength(text, "utf8") > this.#maxResponseBytes) {
        throw new Error("response exceeds configured size limit");
      }
      const body = text === "" ? null : (JSON.parse(text) as unknown);
      if (!response.ok) throw new CompassHTTPError(response.status, response.statusText, body);
      return body as T;
    } finally {
      clearTimeout(timeout);
      signal?.removeEventListener("abort", relayAbort);
    }
  }
}

export class CompassHTTPError extends Error {
  readonly status: number;
  readonly statusText: string;
  readonly body: unknown;

  constructor(status: number, statusText: string, body: unknown) {
    super(`CompassDTL request failed with HTTP ${status}${statusText ? ` ${statusText}` : ""}`);
    this.name = "CompassHTTPError";
    this.status = status;
    this.statusText = statusText;
    this.body = body;
  }
}

export function computeRouteCapital(input: RouteCapitalInput): RouteCapitalMetrics {
  for (const [name, value] of Object.entries(input)) {
    if (value < 0n) throw new Error(`${name} must be non-negative`);
  }
  if (input.maxExposure === 0n) throw new Error("max exposure must be positive");
  if (input.reservedLiquidity > input.liquidity) {
    throw new Error("reserved liquidity exceeds route liquidity");
  }
  if (input.liquidityHaircutBps > BPS) throw new Error("liquidity haircut exceeds 10000 bps");
  const availableLiquidity = input.liquidity - input.reservedLiquidity;
  const effectiveLiquidity = mulDivFloor(availableLiquidity, BPS - input.liquidityHaircutBps, BPS);
  const stressedOutflows = mulDivCeil(input.queuedPrincipal, BPS + input.settlementShockBps, BPS);
  const operationalBuffer = mulDivCeil(input.exposure, input.operationalBufferBps, BPS);
  const requiredLiquidity = stressedOutflows + operationalBuffer;
  const liquidityShortfall = positiveDifference(requiredLiquidity, effectiveLiquidity);
  const exposureHeadroom = positiveDifference(input.maxExposure, input.exposure);
  const freeLiquidity = positiveDifference(effectiveLiquidity, requiredLiquidity);
  return {
    availableLiquidity,
    effectiveLiquidity,
    stressedOutflows,
    operationalBuffer,
    requiredLiquidity,
    liquidityShortfall,
    exposureHeadroom,
    settlementCapacity: freeLiquidity < exposureHeadroom ? freeLiquidity : exposureHeadroom,
    liquidityCoverageBps: ratioBps(effectiveLiquidity, requiredLiquidity),
    exposureUtilizationBps: ratioBps(input.exposure, input.maxExposure),
    compliant: liquidityShortfall === 0n && input.exposure <= input.maxExposure,
  };
}

export function canonicalJSON(value: unknown): string {
  return encodeCanonical(value);
}

export function payloadHash(value: unknown): string {
  return createHash("sha256").update(canonicalJSON(value), "utf8").digest("hex");
}

function encodeIntent(intent: IntentInput): Record<string, unknown> {
  return {
    amount: positiveInteger(intent.amount, "amount"),
    destinationAccount: normalizedToken(intent.destinationAccount, "destination account"),
    destinationAsset: normalizedToken(intent.destinationAsset, "destination asset"),
    id: normalizedToken(intent.id, "intent id"),
    maxFee: nonNegativeInteger(intent.maxFee, "max fee"),
    metadata: intent.metadata,
    priority: intent.priority,
    requestedEpoch: intent.requestedEpoch ?? 0n,
    sourceAccount: normalizedToken(intent.sourceAccount, "source account"),
    sourceAsset: normalizedToken(intent.sourceAsset, "source asset"),
  };
}

function encodeCanonical(value: unknown): string {
  if (value === null) return "null";
  if (typeof value === "bigint") return value.toString(10);
  if (typeof value === "string" || typeof value === "boolean") return JSON.stringify(value);
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value)) {
      throw new Error("canonical JSON accepts only safe integer numbers");
    }
    return String(value);
  }
  if (Array.isArray(value)) return `[${value.map(encodeCanonical).join(",")}]`;
  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, entry]) => entry !== undefined)
      .sort(([left], [right]) => left.localeCompare(right));
    return `{${entries
      .map(([key, entry]) => `${JSON.stringify(key)}:${encodeCanonical(entry)}`)
      .join(",")}}`;
  }
  throw new Error(`unsupported canonical JSON value: ${typeof value}`);
}

function mulDivFloor(value: bigint, numerator: bigint, denominator: bigint): bigint {
  if (denominator <= 0n) throw new Error("denominator must be positive");
  return (value * numerator) / denominator;
}

function mulDivCeil(value: bigint, numerator: bigint, denominator: bigint): bigint {
  if (denominator <= 0n) throw new Error("denominator must be positive");
  const product = value * numerator;
  return product === 0n ? 0n : (product + denominator - 1n) / denominator;
}

function ratioBps(numerator: bigint, denominator: bigint): bigint {
  if (denominator === 0n) return numerator === 0n ? 0n : (1n << 255n) - 1n;
  return mulDivFloor(numerator, BPS, denominator);
}

function positiveDifference(left: bigint, right: bigint): bigint {
  return left > right ? left - right : 0n;
}

function positiveInteger(value: bigint, field: string): bigint {
  if (value <= 0n) throw new Error(`${field} must be positive`);
  return value;
}

function nonNegativeInteger(value: bigint, field: string): bigint {
  if (value < 0n) throw new Error(`${field} must be non-negative`);
  return value;
}

function normalizedToken(value: string, field: string): string {
  if (value === "" || value !== value.trim() || /\s/.test(value) || value.length > 128) {
    throw new Error(`${field} must be normalized, non-empty, and at most 128 characters`);
  }
  return value;
}

function boundedInteger(value: number, minimum: number, maximum: number, field: string): number {
  if (!Number.isInteger(value) || value < minimum || value > maximum) {
    throw new Error(`${field} must be an integer within [${minimum}, ${maximum}]`);
  }
  return value;
}
