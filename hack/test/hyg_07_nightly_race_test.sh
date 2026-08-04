#!/usr/bin/env bash
# HY-07 / TEST-02: e2e-nightly must carry an advisory race job.
# Asserts continue-on-error, coverage:race invocation, CGO/CI/COVERAGE_MIN=0,
# and race/flake filing guidance in the failure summary.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/e2e-nightly.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

if ! command -v yq >/dev/null 2>&1; then
  echo "yq not found; install yq (mikefarah/yq v4) to run this test" >&2
  exit 1
fi

[[ -f "${WORKFLOW}" ]] || fail "workflow file not found: ${WORKFLOW}"

# Prefer job id `race`; accept legacy `race-detector` if still present.
JOB_ID=""
for candidate in race race-detector; do
  if [[ "$(yq eval ".jobs.${candidate} | type" "${WORKFLOW}")" == "!!map" ]]; then
    JOB_ID="${candidate}"
    break
  fi
done
[[ -n "${JOB_ID}" ]] || fail "e2e-nightly.yaml missing race / race-detector job"

COO="$(yq eval ".jobs.${JOB_ID}.[\"continue-on-error\"]" "${WORKFLOW}")"
[[ "${COO}" == "true" ]] || fail "job '${JOB_ID}' must set continue-on-error: true (got: ${COO})"
pass "job '${JOB_ID}' is advisory (continue-on-error: true)"

TIMEOUT="$(yq eval ".jobs.${JOB_ID}.[\"timeout-minutes\"]" "${WORKFLOW}")"
if [[ "${TIMEOUT}" == "null" ]] || [[ "${TIMEOUT}" -lt 30 ]]; then
  fail "job '${JOB_ID}' needs generous timeout-minutes (>=30), got: ${TIMEOUT}"
fi
pass "job '${JOB_ID}' timeout-minutes=${TIMEOUT}"

# Find the step that runs coverage:race and assert its env.
STEP_ENV_CI=""
STEP_ENV_CGO=""
STEP_ENV_MIN=""
FOUND_RACE_CMD=0
STEP_COUNT="$(yq eval ".jobs.${JOB_ID}.steps | length" "${WORKFLOW}")"
for ((i = 0; i < STEP_COUNT; i++)); do
  RUN="$(yq eval ".jobs.${JOB_ID}.steps[${i}].run // \"\"" "${WORKFLOW}")"
  if [[ "${RUN}" == *coverage:race* ]]; then
    FOUND_RACE_CMD=1
    STEP_ENV_CI="$(yq eval ".jobs.${JOB_ID}.steps[${i}].env.CI // \"\"" "${WORKFLOW}")"
    STEP_ENV_CGO="$(yq eval ".jobs.${JOB_ID}.steps[${i}].env.CGO_ENABLED // \"\"" "${WORKFLOW}")"
    STEP_ENV_MIN="$(yq eval ".jobs.${JOB_ID}.steps[${i}].env.COVERAGE_MIN // \"\"" "${WORKFLOW}")"
    # Also accept job-level env if step-level unset.
    if [[ -z "${STEP_ENV_CI}" || "${STEP_ENV_CI}" == "null" ]]; then
      STEP_ENV_CI="$(yq eval ".jobs.${JOB_ID}.env.CI // \"\"" "${WORKFLOW}")"
    fi
    if [[ -z "${STEP_ENV_CGO}" || "${STEP_ENV_CGO}" == "null" ]]; then
      STEP_ENV_CGO="$(yq eval ".jobs.${JOB_ID}.env.CGO_ENABLED // \"\"" "${WORKFLOW}")"
    fi
    if [[ -z "${STEP_ENV_MIN}" || "${STEP_ENV_MIN}" == "null" ]]; then
      STEP_ENV_MIN="$(yq eval ".jobs.${JOB_ID}.env.COVERAGE_MIN // \"\"" "${WORKFLOW}")"
    fi
    break
  fi
done

[[ "${FOUND_RACE_CMD}" -eq 1 ]] || fail "job '${JOB_ID}' must run task coverage:race"
pass "job '${JOB_ID}' invokes task coverage:race"

[[ "${STEP_ENV_CI}" == "true" ]] || fail "race step must set CI=true (got: ${STEP_ENV_CI})"
[[ "${STEP_ENV_CGO}" == "1" ]] || fail "race step must set CGO_ENABLED=1 (got: ${STEP_ENV_CGO})"
[[ "${STEP_ENV_MIN}" == "0" ]] || fail "race step must set COVERAGE_MIN=0 (racing is the signal; got: ${STEP_ENV_MIN})"
pass "race step env: CI=true CGO_ENABLED=1 COVERAGE_MIN=0"

# Failure summary must point maintainers at race/flake labels.
# Workflow YAML escapes backticks as \`…\`, so match that form (or plain race/flake).
# shellcheck disable=SC2016 # intentional literal backticks in fixed-string match
if ! grep -Fq '\`race\`/\`flake\`' "${WORKFLOW}" && ! grep -Fq 'race/flake' "${WORKFLOW}"; then
  fail "workflow must instruct filing an issue labeled race/flake on DATA RACE failure"
fi
pass "failure summary mentions race/flake filing"

# go-cache with envtest + setup-task present (shape check via uses:).
USES="$(yq eval ".jobs.${JOB_ID}.steps[].uses // \"\"" "${WORKFLOW}")"
echo "${USES}" | grep -q 'go-cache' || fail "job '${JOB_ID}' must use go-cache action"
echo "${USES}" | grep -q 'setup-task' || fail "job '${JOB_ID}' must use setup-task"
ENVTEST="$(yq eval ".jobs.${JOB_ID}.steps[] | select(.uses == \"./.github/actions/go-cache\") | .with.envtest" "${WORKFLOW}")"
[[ "${ENVTEST}" == "true" ]] || fail "go-cache must set envtest: true (got: ${ENVTEST})"
pass "checkout path uses go-cache(envtest) + setup-task"

echo "All HY-07 nightly race advisory contract tests passed."
