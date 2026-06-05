#!/usr/bin/env node
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const uiRoot = path.resolve(__dirname, "..");
const repoRoot = path.resolve(uiRoot, "..");
const openApiPath = path.join(repoRoot, "openapi/v1alpha1/inventory.yaml");
const outDir = path.join(uiRoot, "src/mocks/generated");
const outFile = path.join(outDir, "openapi-manifest.json");

const source = readFileSync(openApiPath, "utf8");
const sha256 = createHash("sha256").update(source).digest("hex");

const manifest = {
  schemaVersion: "kollect.dev/v1alpha1",
  source: "openapi/v1alpha1/inventory.yaml",
  sha256,
  paths: [
    "/v1alpha1/inventory",
    "/v1alpha1/inventory/{namespace}/{name}",
    "/v1alpha1/inventory/watch",
    "/v1alpha1/status/targets",
    "/v1alpha1/status/inventories",
  ],
  generatedAt: new Date().toISOString(),
};

mkdirSync(outDir, { recursive: true });
writeFileSync(outFile, `${JSON.stringify(manifest, null, 2)}\n`);
console.log(`wrote ${path.relative(repoRoot, outFile)} (sha256=${sha256.slice(0, 12)}…)`);
