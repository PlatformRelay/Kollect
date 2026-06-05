#!/usr/bin/env bash
# Fail when OpenAPI hash drifts from committed ui mock manifest (Phase 2 drift gate).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${ROOT}/ui/src/mocks/generated/openapi-manifest.json"
OPENAPI="${ROOT}/openapi/v1alpha1/inventory.yaml"

if [[ ! -f "${MANIFEST}" ]]; then
  echo "verify-ui-mock: missing ${MANIFEST} — run task ui-mock-sync" >&2
  exit 1
fi

expected="$(sha256sum "${OPENAPI}" | awk '{print $1}')"
actual="$(node -e "process.stdout.write(JSON.parse(require('node:fs').readFileSync('${MANIFEST}','utf8')).sha256)")"

if [[ "${expected}" != "${actual}" ]]; then
  echo "verify-ui-mock: OpenAPI drift — expected sha256 ${expected}, manifest has ${actual}" >&2
  echo "verify-ui-mock: run task ui-mock-sync and commit ui/src/mocks/generated/" >&2
  exit 1
fi

echo "verify-ui-mock: ok (${actual:0:12}…)"
