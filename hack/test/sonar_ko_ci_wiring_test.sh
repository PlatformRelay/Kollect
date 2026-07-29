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

# Several sonar_ko_* scripts render Helm charts (helm template) or parse YAML
# (yq) -- the lint job otherwise only installs Go tooling + shellcheck. If a
# future edit drops or reorders these installs, the glob step would hit
# "command not found" the moment a tool-dependent script lands, and nothing
# else in this repo would catch it until that happens on a shared main.
helm_line="$(grep -n 'name: Install Helm' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
yq_line="$(grep -n 'name: Ensure yq is available' "${CI_WORKFLOW}" | head -1 | cut -d: -f1)"
[[ -n "${helm_line}" ]] || fail "ci.yaml no longer installs Helm before running sonar_ko tests -- helm-dependent scripts would fail with 'command not found'"
[[ -n "${yq_line}" ]] || fail "ci.yaml no longer ensures yq is available before running sonar_ko tests -- yq-dependent scripts would fail with 'command not found'"
pass "ci.yaml installs Helm and ensures yq is available"

if [[ "${helm_line}" -ge "${glob_line}" ]]; then
  fail "Install Helm step (line ${helm_line}) must come before the sonar_ko glob step (line ${glob_line})"
fi
if [[ "${yq_line}" -ge "${glob_line}" ]]; then
  fail "Ensure yq step (line ${yq_line}) must come before the sonar_ko glob step (line ${glob_line})"
fi
pass "Install Helm and Ensure yq steps both run before the sonar_ko glob step"

echo "All CI sonar_ko wiring tests passed."
