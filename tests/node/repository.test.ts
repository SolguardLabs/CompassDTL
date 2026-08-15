import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { projectRoot } from "../helpers/runner.ts";

const read = (path: string): string => readFileSync(join(projectRoot, path), "utf8");

test("repository ships the approved visual identity", () => {
  const digest = createHash("sha256")
    .update(readFileSync(join(projectRoot, "assets", "banner.png")))
    .digest("hex");
  assert.equal(digest, "80dc1783d690799b3a493c32b8de15ada850769cfdcfab35612bbeae411b766b");
  assert.match(read("README.md"), /assets\/banner\.png/);
});

test("documentation contract contains seven linked guides and rich diagrams", () => {
  const docs = readdirSync(join(projectRoot, "docs"))
    .filter((name) => name.endsWith(".md"))
    .sort();
  assert.deepEqual(docs, [
    "01-arquitectura.md",
    "02-modelo-economico.md",
    "03-seguridad-operativa.md",
    "04-api-sdk.md",
    "05-operacion.md",
    "06-gobernanza.md",
    "07-observabilidad.md",
  ]);
  const markdown = [
    read("README.md"),
    read("SECURITY.md"),
    ...docs.map((name) => read(`docs/${name}`)),
  ];
  const diagrams = markdown.reduce(
    (count, content) => count + (content.match(/```mermaid\b/g) ?? []).length,
    0,
  );
  assert.ok(diagrams >= 18, `expected at least 18 Mermaid diagrams, found ${diagrams}`);
  for (const name of docs) assert.match(markdown[0], new RegExp(name.replace(".", "\\.")));
});

test("automation validates candidates and immutable delivery references", () => {
  const ci = read(".github/workflows/ci.yml");
  const integrity = read(".github/workflows/release-integrity.yml");
  assert.match(ci, /ubuntu-latest/);
  assert.match(ci, /windows-latest/);
  assert.match(ci, /actions\/checkout@v7/);
  assert.match(ci, /actions\/setup-go@v7/);
  assert.match(ci, /actions\/setup-node@v7/);
  assert.match(integrity, /branches: \[main, production\]/);
  assert.match(integrity, /types: \[published\]/);
  assert.match(integrity, /cat-file -t/);
  assert.match(integrity, /rev-parse.*\^\{\}/);
});
