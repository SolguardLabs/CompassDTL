import { spawnSync } from "node:child_process";

const result = spawnSync("gofmt", ["-l", "cmd", "src"], { encoding: "utf8" });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
if (result.status !== 0) {
  process.stderr.write(result.stderr);
  process.exit(result.status ?? 1);
}
if (result.stdout.trim() !== "") {
  console.error("gofmt is required for:");
  console.error(result.stdout.trim());
  process.exit(1);
}
