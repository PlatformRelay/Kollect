#!/usr/bin/env bash
# LAB-DOC-02: publishable lab evidence bundle schema + redaction contract must stay
# on the tracked docs page and in MkDocs navigation.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PAGE="${ROOT}/docs/operator-manual/lab-evidence-bundle.md"

fail() {
  printf 'lab evidence contract docs: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PAGE}" ]] || fail "${PAGE} is missing"

required_headings=(
  "## Evidence bundle schema"
  "### Manifest fields"
  "### Scenario result rows"
  "## Limitations section"
  "## Redaction rules"
  "## What may be published"
)

for heading in "${required_headings[@]}"; do
  grep -qF -- "${heading}" "${PAGE}" || fail "missing heading: ${heading}"
done
pass "required schema/redaction/limitations headings present"

required_terms=(
  "RUN_ID"
  "PASS_WITH_LIMITATION"
  "SKIPPED"
  "LIMIT_REACHED"
  "kubeconfig"
  "Secret"
  "token"
  "pprof"
  "lab-protocols"
  "artifacts/lab"
)

for term in "${required_terms[@]}"; do
  grep -qF -- "${term}" "${PAGE}" || fail "missing contract term: ${term}"
done
pass "required contract terms present"

# Local-only pointer: raw protocols must not be described as committed public artifacts.
grep -Eqi 'local.?only|not committed|gitignored' "${PAGE}" ||
  fail "page must state that raw protocols/artifacts are local-only / not committed"

grep -qF -- "Lab evidence bundle: operator-manual/lab-evidence-bundle.md" "${ROOT}/mkdocs.yml" ||
  fail "lab evidence bundle page is missing from MkDocs navigation"

pass "mkdocs navigation entry present"

echo "All lab evidence contract docs tests passed."
