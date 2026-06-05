import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const src = path.join(root, "..", "openapi", "v1alpha1", "inventory.yaml");
const dest = path.join(root, "openapi", "inventory.yaml");
const logoSrc = path.join(root, "..", "docs", "assets", "logo.svg");
const logoDest = path.join(root, "public", "logo.svg");

fs.mkdirSync(path.dirname(dest), { recursive: true });
fs.copyFileSync(src, dest);

fs.mkdirSync(path.dirname(logoDest), { recursive: true });
fs.copyFileSync(logoSrc, logoDest);

console.log("linked openapi + logo assets");
