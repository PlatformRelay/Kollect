#!/usr/bin/env bash
# DIST-DOC-01: install docs mention hub paths without live 404 badge URLs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALL="${ROOT}/docs/getting-started/install.md"
README="${ROOT}/README.md"

fail() {
  printf 'dist install docs: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${INSTALL}" ]] || fail "${INSTALL} is missing"
[[ -f "${README}" ]] || fail "${README} is missing"

for file in "${INSTALL}" "${README}"; do
  grep -Eiq 'artifact hub|artifacthub' "${file}" ||
    fail "${file} must mention Artifact Hub discoverability"
  grep -Eiq 'operatorhub|operator hub' "${file}" ||
    fail "${file} must mention OperatorHub discoverability"
done

# ADR-0708 forbids shipping hub URLs that 404 before that hub lists us. Artifact Hub
# registration completed 2026-08-18 (repo `kollect`, oci://ghcr.io/platformrelay/kollect),
# so AH badge/listing URLs are now legitimate and are asserted below. OperatorHub has NOT
# listed yet — its listing URLs stay banned until the community-operators PR merges.
for file in "${INSTALL}" "${README}"; do
  if grep -E 'operatorhub\.io/operator/kollect' "${file}"; then
    fail "${file} must not ship live OperatorHub listing URLs before that listing exists"
  fi
done

grep -Fq 'artifacthub.io/badge/repository/kollect' "${README}" ||
  fail "${README} must carry the Artifact Hub badge now that the repository is registered"

pass "install docs describe hub paths without premature listing URLs"

echo "All dist install doc tests passed."
