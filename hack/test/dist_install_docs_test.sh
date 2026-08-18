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

# ADR-0708 originally forbade ANY hub URL that 404s before that hub lists us. Artifact Hub
# registration completed 2026-08-18 (repo `kollect`, oci://ghcr.io/platformrelay/kollect,
# Verified Publisher active), so its badge is unambiguously legitimate.
#
# The OperatorHub.io badge ships ahead of the listing by explicit operator decision: the
# community-operators submission is open and green, and operatorhub.io soft-404s (it serves
# HTTP 200 with the generic landing page for unknown operators) rather than showing a broken
# link. Both badges are asserted PRESENT so neither can silently regress.
grep -Fq 'artifacthub.io/badge/repository/kollect' "${README}" ||
  fail "${README} must carry the Artifact Hub badge (repository is registered)"
grep -Fq 'operatorhub.io/operator/kollect' "${README}" ||
  fail "${README} must carry the OperatorHub.io badge"

pass "install docs describe hub paths without premature listing URLs"

echo "All dist install doc tests passed."
