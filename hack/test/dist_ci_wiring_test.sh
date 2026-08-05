#!/usr/bin/env bash
# Locks dist_* meta-tests into CI lint job (alongside sonar_ko glob).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok - $*"; }

[[ -f "${CI_WORKFLOW}" ]] || fail "expected ${CI_WORKFLOW}"

grep -q 'hack/test/dist_\*_test\.sh' "${CI_WORKFLOW}" ||
  fail "ci.yaml must glob hack/test/dist_*_test.sh"

grep -q 'Hub distribution meta-tests' "${CI_WORKFLOW}" ||
  fail "ci.yaml must name the Hub distribution meta-tests step"

glob_line="$(grep -n 'hack/test/dist_\*_test\.sh' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
lint_line="$(grep -n '^\s*run: task lint\s*$' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
[[ -n "${glob_line}" && -n "${lint_line}" ]] || fail "could not locate dist glob or lint step"
if [[ "${glob_line}" -ge "${lint_line}" ]]; then
  fail "dist_* glob step (line ${glob_line}) must come before task lint (line ${lint_line})"
fi
pass "ci.yaml globs dist_*_test.sh before lint"
echo "All dist CI wiring tests passed."
