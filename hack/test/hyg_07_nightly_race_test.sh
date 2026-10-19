#!/usr/bin/env bash
# HY-07 / TEST-02: e2e-nightly must carry a blocking race job.
# Asserts that neither the job nor its race step sets continue-on-error (SINK-BREAKER-RACE-01:
# the CI failure reporter judges the run conclusion only, so a soft-failed race job is never
# reported), the coverage:race invocation, CGO/CI/COVERAGE_MIN=0, and race/flake filing guidance
# in the failure summary.
#
# Usage: hyg_07_nightly_race_test.sh [--self-test] [ROOT]
#   --self-test  mutate copies of the workflow and require each mutant to fail its own clause,
#                plus a no-op control that must pass.
set -euo pipefail

SELF="${BASH_SOURCE[0]}"
if [[ "${1:-}" == "--self-test" ]]; then
  SRC_ROOT="${2:-$(cd "$(dirname "${SELF}")/../.." && pwd)}"
  command -v yq >/dev/null 2>&1 || {
    echo "FAIL: yq not found; install yq (mikefarah/yq v4) to run this self-test" >&2
    exit 1
  }
  WORK="$(mktemp -d)"
  trap 'rm -rf "${WORK:?}"' EXIT
  st_fail=0
  # mutant <label> <expected FAIL text> <yq expression | "">: copy the workflow, apply the edit,
  # refuse an edit that changed nothing, and require exit non-zero with <expected> on a FAIL line.
  # An empty <expected> is the no-op control: no edit, must pass.
  mutant() {
    local label="$1" want="$2" expr="$3" copy out rc=0
    copy="${WORK}/case"
    rm -rf "${copy:?}"
    mkdir -p "${copy}/.github/workflows"
    cp "${SRC_ROOT}/.github/workflows/e2e-nightly.yaml" "${copy}/.github/workflows/e2e-nightly.yaml"
    if [[ -n "${expr}" ]]; then
      yq eval -i "${expr}" "${copy}/.github/workflows/e2e-nightly.yaml"
      # Compare parsed content, not bytes: yq may reformat a file whose content it did not change.
      if [[ "$(yq eval -o=json -I=0 . "${SRC_ROOT}/.github/workflows/e2e-nightly.yaml")" == \
        "$(yq eval -o=json -I=0 . "${copy}/.github/workflows/e2e-nightly.yaml")" ]]; then
        echo "FAIL: self-test ${label}: the mutation changed nothing, nothing was tested" >&2
        st_fail=1
        rm -rf "${copy:?}"
        return 0
      fi
    fi
    out="$(bash "${SELF}" "${copy}" 2>&1)" || rc=$?
    if [[ -z "${want}" ]]; then
      if [[ "${rc}" -eq 0 ]]; then
        echo "ok - self-test ${label}: green"
      else
        echo "FAIL: self-test ${label}: expected green, got exit ${rc}: $(grep '^FAIL' <<<"${out}" | head -n 1)" >&2
        st_fail=1
      fi
    elif [[ "${rc}" -ne 0 ]] && grep '^FAIL: ' <<<"${out}" | grep -F -- "${want}" >/dev/null; then
      echo "ok - self-test ${label}: red with '${want}'"
    else
      echo "FAIL: self-test ${label}: expected a non-zero exit and a FAIL line with '${want}', got exit ${rc}: $(grep '^FAIL' <<<"${out}" | head -n 1)" >&2
      st_fail=1
    fi
    rm -rf "${copy:?}"
  }
  mutant "no-op control" "" ""
  mutant "job continue-on-error true" "job 'race' must not set continue-on-error" '.jobs.race."continue-on-error" = true'
  # shellcheck disable=SC2016 # a literal GitHub expression is the mutant value
  mutant "job continue-on-error expression" "job 'race' must not set continue-on-error" '.jobs.race."continue-on-error" = "${{ always() }}"'
  mutant "race step continue-on-error true" "race step must not set continue-on-error" \
    '(.jobs.race.steps[] | select(.run // "" | test("coverage:race")))."continue-on-error" = true'
  mutant "race step swallows the race exit" "race step must exit non-zero on a DATA RACE" \
    '(.jobs.race.steps[] | select(.run // "" | test("coverage:race")) | .run) |= sub("exit 1"; "exit 0")'
  # shellcheck disable=SC2016 # the literal $status is the workflow text being mutated
  mutant "race step swallows a non-race failure" "race step must exit with the task's status on a non-race failure" \
    '(.jobs.race.steps[] | select(.run // "" | test("coverage:race")) | .run) |= sub("exit \"\\$status\""; "exit 0")'
  # shellcheck disable=SC2016 # the literal $status is the workflow text being mutated
  mutant "race step drops the non-race failure branch" "race step must exit with the task's status on a non-race failure" \
    '(.jobs.race.steps[] | select(.run // "" | test("coverage:race")) | .run) |= sub("if \\[ \"\\$status\" -ne 0 \\]"; "if false")'
  mutant "COVERAGE_MIN raised" "COVERAGE_MIN=0" \
    '(.jobs.race.steps[] | select(.run // "" | test("coverage:race")) | .env.COVERAGE_MIN) = "80"'
  mutant "race job deleted (absence)" "missing race / race-detector job" 'del(.jobs.race)'
  if [[ "${st_fail}" -ne 0 ]]; then
    echo "FAIL: HY-07 self-test" >&2
    exit 1
  fi
  echo "All HY-07 self-test cases passed."
  exit 0
fi

ROOT="${1:-$(cd "$(dirname "${SELF}")/../.." && pwd)}"
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

# Blocking: continue-on-error unset or literally false. Anything else (true, an expression)
# can leave the run concluded success while this job failed.
COO="$(yq eval ".jobs.${JOB_ID}.[\"continue-on-error\"]" "${WORKFLOW}")"
[[ "${COO}" == "null" || "${COO}" == "false" ]] ||
  fail "job '${JOB_ID}' must not set continue-on-error (got: ${COO}); a soft-failed race job never reaches the run conclusion or the CI failure reporter"
pass "job '${JOB_ID}' is blocking (no continue-on-error)"

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
    STEP_COE="$(yq eval ".jobs.${JOB_ID}.steps[${i}].[\"continue-on-error\"]" "${WORKFLOW}")"
    STEP_RUN="${RUN}"
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

[[ "${STEP_COE}" == "null" || "${STEP_COE}" == "false" ]] ||
  fail "race step must not set continue-on-error (got: ${STEP_COE})"
# Both failure branches must end the step non-zero: an `exit 1` inside the `if grep ... DATA RACE`
# block, and an `exit "$status"` inside the `if [ "$status" -ne 0 ]` block (a test failure, timeout
# or compile error without a race marker). Indentation is not part of the contract.
# shellcheck disable=SC2016 # awk program and the literal $status are not shell expansions
RACE_EXIT="$(awk '/if grep -q .WARNING: DATA RACE./ { inblk = 1; next } inblk && /^[[:space:]]*fi[[:space:]]*$/ { exit } inblk && /^[[:space:]]*exit [1-9]/ { print "yes"; exit }' <<<"${STEP_RUN}")"
[[ "${RACE_EXIT}" == "yes" ]] || fail "race step must exit non-zero on a DATA RACE (no non-zero exit in its DATA RACE branch)"
# shellcheck disable=SC2016 # awk program and the literal $status are not shell expansions
STATUS_EXIT="$(awk '/if \[ "\$status" -ne 0 \]/ { inblk = 1; next } inblk && /^[[:space:]]*fi[[:space:]]*$/ { exit } inblk && /^[[:space:]]*exit "\$status"[[:space:]]*$/ { print "yes"; exit }' <<<"${STEP_RUN}")"
[[ "${STATUS_EXIT}" == "yes" ]] || fail "race step must exit with the task's status on a non-race failure (no 'exit \"\$status\"' in its 'if [ \"\$status\" -ne 0 ]' branch)"
pass "race step: no continue-on-error, exits non-zero on a DATA RACE and on any other failure"

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

echo "All HY-07 nightly race contract tests passed."
