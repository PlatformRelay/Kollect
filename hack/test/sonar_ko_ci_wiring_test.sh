#!/usr/bin/env bash
# Locks in that .github/workflows/ci.yaml runs every hack/test/sonar_ko_*_test.sh
# script automatically (glob), so each SEC-04a..i lane's meta-test actually
# executes in CI without a per-lane ci.yaml edit.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${CI_WORKFLOW}" ]] || fail "expected ${CI_WORKFLOW} to exist"

if ! grep -q 'hack/test/sonar_ko_\*_test\.sh' "${CI_WORKFLOW}"; then
  fail "ci.yaml no longer globs hack/test/sonar_ko_*_test.sh -- new SEC-04 lane scripts would silently never run in CI"
fi
pass "ci.yaml globs hack/test/sonar_ko_*_test.sh"

# The glob step must run before the generic 'Lint' step, not after -- a
# security meta-test failure should surface as its own named step, not get
# buried inside the catch-all lint output.
glob_line="$(grep -n 'hack/test/sonar_ko_\*_test\.sh' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
lint_line="$(grep -n '^\s*run: task lint\s*$' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
[[ -n "${glob_line}" && -n "${lint_line}" ]] || fail "could not locate glob step or lint step line numbers"
if [[ "${glob_line}" -ge "${lint_line}" ]]; then
  fail "sonar_ko glob step (line ${glob_line}) must come before the 'task lint' step (line ${lint_line})"
fi
pass "sonar_ko glob step runs before the generic lint step"

echo "All CI sonar_ko wiring tests passed."
