import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
export const projectRoot = join(here, "..", "..");

export type ScenarioResult = {
  name: string;
  results: Array<Record<string, unknown>>;
  snapshot: {
    epoch: number;
    balances: Array<{ account: string; asset: string; available: number; reserved: number }>;
    routes: Array<{ id: string; exposure: number; liquidity: number; maxExposure: number }>;
    queue: Array<{ ticketId: string; intentId: string; routeId: string; queueScore: number }>;
    receipts: Array<{
      id: string;
      ticketId: string;
      intentId: string;
      routeId: string;
      amount: number;
      destinationAmount: number;
      fees: { baseFee: number; operatorFee: number; networkFee: number; totalFee: number };
    }>;
    auditIssues: Array<{ code: string; severity: string; message: string }>;
  };
};

export function runFixture(name: string): ScenarioResult {
  const fixturePath = join(projectRoot, "tests", "fixtures", `${name}.json`);
  const child = spawnSync("go", ["run", "./cmd/compassdtl", "run", fixturePath], {
    cwd: projectRoot,
    encoding: "utf8",
  });
  if (child.status !== 0) {
    throw new Error(
      [
        `fixture ${name} failed`,
        `status: ${child.status}`,
        `stdout: ${child.stdout}`,
        `stderr: ${child.stderr}`,
      ].join("\n"),
    );
  }
  return JSON.parse(child.stdout) as ScenarioResult;
}

export function resultByLabel(result: ScenarioResult, label: string): Record<string, unknown> {
  const found = result.results.find((entry) => entry.label === label);
  if (!found) {
    throw new Error(`missing action label ${label}`);
  }
  return found;
}

export function balanceOf(
  result: ScenarioResult,
  account: string,
  asset: string,
): { available: number; reserved: number } {
  const found = result.snapshot.balances.find(
    (balance) => balance.account === account && balance.asset === asset,
  );
  return found ?? { available: 0, reserved: 0 };
}
