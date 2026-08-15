import { createHash } from "node:crypto";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { extname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const expectedBannerHash = "80dc1783d690799b3a493c32b8de15ada850769cfdcfab35612bbeae411b766b";
const required = [
  "README.md",
  "SECURITY.md",
  "assets/banner.png",
  "sdk/compassClient.ts",
  "src/capital/model.go",
  "src/governance/council.go",
  "src/observability/metrics.go",
];
for (const entry of required) {
  if (!existsSync(join(root, entry))) fail(`missing required artifact: ${entry}`);
}

const docs = readdirSync(join(root, "docs")).filter((name) => name.endsWith(".md"));
if (docs.length !== 7) fail(`expected exactly 7 operational documents, found ${docs.length}`);

const bannerHash = createHash("sha256")
  .update(readFileSync(join(root, "assets", "banner.png")))
  .digest("hex");
if (bannerHash !== expectedBannerHash) fail(`unexpected banner digest: ${bannerHash}`);

const readableExtensions = new Set([".go", ".md", ".mjs", ".ts", ".json", ".yml", ".yaml"]);
const excluded = [".git", "node_modules", "bin", "tests/private"];
const forbiddenTerms = [
  "c" + "tf",
  "la" + "boratorio",
  "l" + "ab",
  "vulnera" + "bilidad",
  "vulnera" + "ble",
  "vulnera" + "bility",
  "b" + "ug",
  "ex" + "ploit",
  "by" + "pass",
  "at" + "tacker",
];
let mermaidBlocks = 0;
let sourceLines = 0;
for (const path of walk(root)) {
  const name = relative(root, path).replaceAll("\\", "/");
  if (name === "package-lock.json" || !readableExtensions.has(extname(path))) continue;
  const contents = readFileSync(path, "utf8");
  if (name.startsWith("src/") && name.endsWith(".go")) {
    sourceLines += contents.split(/\r?\n/).filter((line) => line.trim() !== "").length;
  }
  mermaidBlocks += (contents.match(/```mermaid\b/g) ?? []).length;
  for (const term of forbiddenTerms) {
    if (new RegExp(`\\b${escapeRegExp(term)}s?\\b`, "iu").test(contents)) {
      fail(`restricted public terminology in ${name}`);
    }
  }
}
if (sourceLines < 3_600)
  fail(`expected at least 3600 non-empty Go source lines, found ${sourceLines}`);
if (mermaidBlocks < 18) fail(`expected at least 18 Mermaid diagrams, found ${mermaidBlocks}`);
console.log(
  `repository contract passed: ${docs.length} docs, ${mermaidBlocks} diagrams, ${sourceLines} Go lines`,
);

function* walk(directory) {
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry);
    const name = relative(root, path).replaceAll("\\", "/");
    if (excluded.some((value) => name === value || name.startsWith(`${value}/`))) continue;
    if (statSync(path).isDirectory()) yield* walk(path);
    else yield path;
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function fail(message) {
  console.error(message);
  process.exit(1);
}
