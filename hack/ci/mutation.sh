#!/usr/bin/env bash
# Nightly mutation measurement, report only (MUT-1 to MUT-4, openspec change
# mutation-testing-signal).
#
# Usage: hack/ci/mutation.sh <gremlins binary>
#   `task mutation` installs the pinned gremlins and passes it; nothing else should run this.
#
# 1. Refuses to start when any GREMLINS_ variable is set, empty or not: gremlins reads every
#    GREMLINS_* name as configuration (a score gate, a diff or tag narrowing, a mutator switched
#    off), including ones no file scan can see. MUTATION_FORCE_FAILURE=true (a repository
#    variable, for proving the failure report) fails the run here and does nothing else.
# 2. Sets GOFLAGS=-count=1: gremlins times each mutant out at three times its coverage run, and a
#    coverage run served from the Go test cache takes milliseconds, so every mutant would time out
#    (task 1.1, finding F1). This also replaces any inherited GOFLAGS (a -tags would narrow).
# 3. Writes every report into a directory created fresh for this run, so a report from an earlier
#    run can never stand in for a missing one, and hands it to the upload step through
#    $GITHUB_OUTPUT (reports=<dir>) before the first gremlins call. The directory is kept.
# 4. Runs gremlins on the fixture module first (hack/test/testdata/mutation/): its control mutant
#    must be reported exactly once and LIVED, or the pipeline cannot see a survivor (MUT-3).
# 5. Runs gremlins once per package, each to its own report (the JSON names no package), with all
#    eleven mutator flags of the pinned version and the empty --config, so no other config file
#    and no environment value changes what is measured, and with --timeout-coefficient=10, so a
#    loaded runner does not turn measurements into timeouts.
# 6. Hands the three reports to hack/ci/mutation-summary.sh, which validates them all and writes
#    the counts to the job summary. A low score never fails the run; a tool failure, a missing
#    report or a broken control always does. Diagnostics are ::error::mutation: lines on stderr.
# Locked by hack/test/mutation_signal_test.sh (R1-R8).
set -euo pipefail

die() {
  printf '::error::mutation: %s\n' "$*" >&2
  exit 1
}

[[ "$#" -eq 1 && -n "$1" ]] || die "usage: mutation.sh <gremlins binary> (the Taskfile mutation task passes the pinned one)"
GREMLINS="$1"

refused=()
while IFS= read -r name; do
  if [[ "${name}" == GREMLINS_* ]]; then
    refused+=("${name}")
  fi
done < <(compgen -e)
((${#refused[@]} == 0)) ||
  die "refusing to run with ${refused[*]} set: gremlins reads GREMLINS_* as configuration; unset it"

case "${MUTATION_FORCE_FAILURE:-}" in
  "" | false) ;;
  true) die "forced failure: MUTATION_FORCE_FAILURE is true (repository variable); unset it to measure again" ;;
  *) die "MUTATION_FORCE_FAILURE must be true, false or empty, got '${MUTATION_FORCE_FAILURE}'" ;;
esac

export GOFLAGS=-count=1

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
CONFIG="${ROOT}/hack/ci/gremlins.yaml"
FIXTURE="${ROOT}/hack/test/testdata/mutation"
[[ -f "${CONFIG}" ]] || die "${CONFIG} is missing"
[[ -f "${FIXTURE}/go.mod" ]] || die "the fixture module ${FIXTURE} is missing"
# The eleven mutator flags of gremlins v0.6.0 (`gremlins unleash --help`), six of which default
# to off; a bump re-checks this list.
FLAGS=(--arithmetic-base --conditionals-boundary --conditionals-negation --invert-assignments
  --invert-bitwise --invert-bwassign --increment-decrement --invert-logical --invert-loopctrl
  --invert-negatives --remove-self-assignments)
# gremlins gives each mutant three times the coverage run's time by default, counted from before
# the mutant's build; on a loaded runner builds alone exceed that and whole packages come back
# TIMED OUT (stage-B review: 4 of 6 local runs at 3, 0 of 6 at 10). A hung mutant then costs about
# ten coverage runs, seconds here.
TIMEOUT_FLAG=--timeout-coefficient=10

REPORTS="$(mktemp -d)"
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "reports=${REPORTS}" >>"${GITHUB_OUTPUT}"
fi
echo "mutation: reports in ${REPORTS}"

# measure <short> <dir> <package> <label>: one gremlins call, run in <dir>, to <short>.json.
measure() {
  local out="${REPORTS}/$1.json" rc=0
  echo "mutation: gremlins on $4"
  (cd "$2" && "${GREMLINS}" unleash "--config=${CONFIG}" "--output=${out}" "${TIMEOUT_FLAG}" "${FLAGS[@]}" "$3") || rc=$?
  ((rc == 0)) || die "gremlins failed on $4 (exit ${rc}): a tool failure, not a measurement; its output is above"
  [[ -s "${out}" ]] || die "no report for $4: gremlins exited 0 but wrote nothing to ${out}"
}

measure fixture "${FIXTURE}" . "the fixture (hack/test/testdata/mutation)"
# The control (task 1.1): control.go:9:11 ARITHMETIC_BASE, executed by its test and never checked.
read -r n status < <(jq -r '[.files[] | select(.file_name == "control.go") | .mutations[]
  | select(.line == 9 and .column == 11 and .type == "ARITHMETIC_BASE") | .status]
  | "\(length) \(.[0] // "-")"' "${REPORTS}/fixture.json") ||
  die "the fixture report ${REPORTS}/fixture.json is not readable JSON"
CONTROL="fixture control control.go:9:11 ARITHMETIC_BASE"
if [[ "${n}" == 0 ]]; then
  die "${CONTROL} is absent from the fixture report (the fixture or the mutator changed): the run cannot prove it sees a survivor"
elif [[ "${n}" != 1 ]]; then
  die "${CONTROL} found ${n} times, want exactly once"
elif [[ "${status}" != LIVED ]]; then
  die "${CONTROL} is ${status}, want LIVED (NOT COVERED: the test no longer runs it; KILLED: the test now checks it)"
fi
echo "mutation: ${CONTROL} is LIVED, as it must be"

for short in aggregate redact pathvalidate; do
  measure "${short}" "${ROOT}" "./internal/${short}" "./internal/${short}"
done

bash "${ROOT}/hack/ci/mutation-summary.sh" \
  "internal/aggregate=${REPORTS}/aggregate.json" \
  "internal/redact=${REPORTS}/redact.json" \
  "internal/pathvalidate=${REPORTS}/pathvalidate.json"
