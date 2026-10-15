#!/usr/bin/env bash
# CFC-5 (openspec change cross-file-consistency-gates): falling behind the latest Go patch is a
# red signal.
#
# hack/ci/go-patch-lag.sh compares go.mod's `go` directive with the newest STABLE release of the
# same minor in the go.dev release listing, and .github/workflows/go-patch-lag.yaml runs it on a
# weekly schedule. A Go patch carries stdlib security fixes that govulncheck reports as red once
# they exist, so the sensor makes the lag itself visible. This test pins the verdicts and the
# workflow's shape, offline (the listing is a fixture generated from the live go.dev output; the
# generating command is in evidence/G4.md of the change).
#
# What it locks:
#   * behind (go.mod names patch N, the minor's latest stable is N+1) -> exit 1, says so;
#   * current -> exit 0;
#   * lookup failure (unreachable, unparsable, not an array, empty, minor absent, go.mod ahead
#     of the listing, unusable go directive) -> exit 2 -- the sensor fails closed, never green;
#   * a newer minor existing is not a failure on its own (but a lag inside go.mod's own minor
#     still is), and unstable (rc/beta) releases never count as "latest";
#   * the workflow runs on a weekly schedule and workflow_dispatch only, with
#     `permissions: contents: read` and nothing wider, no secrets, runs the script as one plain
#     step, and has no `if:` or `continue-on-error` that could skip or soften it;
#   * curl's worst case in the script (--retry-max-time + --max-time) ends inside that step's own
#     `timeout-minutes`, and the job's timeout is above the step's, so a stalled go.dev fails
#     with the script's message instead of a killed step.
#
# Usage:
#   hack/test/go_patch_lag_test.sh              # check the repository tree
#   hack/test/go_patch_lag_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/go_patch_lag_test.sh <tree-root>  # check an arbitrary tree root
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi
ROOT="${ROOT%/}"

command -v jq >/dev/null 2>&1 || fail "jq not found; install jq to run this test"
command -v yq >/dev/null 2>&1 || fail "yq not found; install yq (mikefarah/yq v4) to run this test"

SCRIPT="${ROOT}/hack/ci/go-patch-lag.sh"
WORKFLOW="${ROOT}/.github/workflows/go-patch-lag.yaml"
LISTING="${ROOT}/hack/test/testdata/go-patch-lag/listing-go1.26-go1.27.json"

# One scratch root for the whole run, created in THIS shell so the EXIT trap sees it.
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK:?}"' EXIT

if [[ "${MODE}" == "check" ]]; then
  [[ -f "${SCRIPT}" ]] || fail "hack/ci/go-patch-lag.sh is missing"
  [[ -f "${WORKFLOW}" ]] || fail ".github/workflows/go-patch-lag.yaml is missing"
  [[ -f "${LISTING}" ]] || fail "fixture ${LISTING#"${ROOT}"/} is missing"

  # gomod <name> <go-line-or-empty>: a minimal go.mod under WORK; prints nothing.
  gomod() {
    mkdir -p "${WORK}/$1"
    if [[ -n "$2" ]]; then
      printf 'module example.com/fixture\n\n%s\n' "$2" >"${WORK}/$1/go.mod"
    else
      printf 'module example.com/fixture\n' >"${WORK}/$1/go.mod"
    fi
  }

  # run_case <label> <want-exit> <sentinel> <script args...>
  run_case() {
    local label="$1" want="$2" sentinel="$3" out rc
    shift 3
    out="$(bash "${SCRIPT}" "$@" 2>&1)" && rc=0 || rc=$?
    [[ "${rc}" == "${want}" ]] ||
      fail "case ${label}: exit ${rc}, want ${want}: $(head -3 <<<"${out}" | tr '\n' ' ')"
    grep -Fq -- "${sentinel}" <<<"${out}" ||
      fail "case ${label}: output lacks '${sentinel}': $(head -3 <<<"${out}" | tr '\n' ' ')"
    pass "case ${label}: exit ${rc}, '${sentinel}'"
  }

  # Listing variants, all derived from the generated fixture or plainly not JSON.
  printf '<!DOCTYPE html><html><body>Downloads</body></html>\n' >"${WORK}/html.json"
  printf '[{"version": "go1.27.2", "stable": tr' >"${WORK}/truncated.json"
  jq '.[0]' "${LISTING}" >"${WORK}/object.json"
  printf '[]\n' >"${WORK}/empty.json"
  # A non-canonical patch number (leading zero) is not a release; with only such an entry the
  # minor is absent. Bash would read 09 as bad octal and silently skip both comparisons.
  printf '[{"version": "go1.27.09", "stable": true}]\n' >"${WORK}/leading-zero.json"
  # 2^64 + 2 wraps to 2 in bash arithmetic: a false "current" unless the number is bounded.
  printf '[{"version": "go1.27.18446744073709551618", "stable": true}]\n' >"${WORK}/overflow.json"
  jq 'map(select(.version | startswith("go1.26")))' "${LISTING}" >"${WORK}/only-1.26.json"
  # go1.27.2 demoted to unstable: an unstable release must not count as the latest.
  jq 'map(if .version == "go1.27.2" then .stable = false else . end)' "${LISTING}" >"${WORK}/unstable.json"

  gomod current 'go 1.27.2'
  gomod behind 'go 1.27.1'
  gomod older-minor-current 'go 1.26.9'
  gomod older-minor-behind 'go 1.26.8'
  gomod ahead 'go 1.27.3'
  gomod nopatch 'go 1.27'
  gomod leading-zero 'go 1.27.02'
  gomod overflow 'go 1.27.18446744073709551618'
  gomod nodirective ''

  run_case current 0 "go.mod's go 1.27.2 is the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${LISTING}"
  run_case behind 1 "go.mod's go 1.27.1 is behind go1.27.2, the latest go1.27 patch" \
    --gomod "${WORK}/behind/go.mod" --listing "${LISTING}"
  run_case newer-minor-current 0 "go.mod's go 1.26.9 is the latest go1.26 patch" \
    --gomod "${WORK}/older-minor-current/go.mod" --listing "${LISTING}"
  run_case newer-minor-note 0 "a newer minor exists (go1.27.2)" \
    --gomod "${WORK}/older-minor-current/go.mod" --listing "${LISTING}"
  run_case newer-minor-behind 1 "go.mod's go 1.26.8 is behind go1.26.9, the latest go1.26 patch" \
    --gomod "${WORK}/older-minor-behind/go.mod" --listing "${LISTING}"
  run_case unstable-ignored 0 "go.mod's go 1.27.1 is the latest go1.27 patch" \
    --gomod "${WORK}/behind/go.mod" --listing "${WORK}/unstable.json"
  # 127.0.0.1:9 (discard) refuses the connection at once on any CI runner or laptop.
  run_case unreachable 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --retries 0 \
    --url "https://127.0.0.1:9/dl/?mode=json&include=all"
  run_case unparsable-html 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/html.json"
  run_case unparsable-truncated 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/truncated.json"
  run_case unparsable-object 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/object.json"
  run_case unparsable-empty 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/empty.json"
  run_case minor-absent 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/only-1.26.json"
  run_case gomod-ahead 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/ahead/go.mod" --listing "${LISTING}"
  run_case leading-zero-listing 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/leading-zero.json"
  run_case overflow-listing 2 "cannot determine the latest go1.27 patch" \
    --gomod "${WORK}/current/go.mod" --listing "${WORK}/overflow.json"
  run_case gomod-overflow 2 "cannot read a go <major>.<minor>.<patch> directive" \
    --gomod "${WORK}/overflow/go.mod" --listing "${LISTING}"
  run_case gomod-leading-zero 2 "cannot read a go <major>.<minor>.<patch> directive" \
    --gomod "${WORK}/leading-zero/go.mod" --listing "${LISTING}"
  run_case gomod-no-patch 2 "cannot read a go <major>.<minor>.<patch> directive" \
    --gomod "${WORK}/nopatch/go.mod" --listing "${LISTING}"
  run_case gomod-no-directive 2 "cannot read a go <major>.<minor>.<patch> directive" \
    --gomod "${WORK}/nodirective/go.mod" --listing "${LISTING}"

  # --- the workflow ---------------------------------------------------------------------
  wf_fail() { fail "workflow: $*"; }

  triggers="$(yq -o=json '."on" | keys' "${WORKFLOW}" | jq -c 'sort')"
  [[ "${triggers}" == '["schedule","workflow_dispatch"]' ]] ||
    wf_fail "triggers are ${triggers}, want exactly schedule and workflow_dispatch"

  [[ "$(yq '."on".schedule | length' "${WORKFLOW}")" == "1" ]] ||
    wf_fail "want exactly one schedule entry"
  cron="$(yq '."on".schedule[0].cron' "${WORKFLOW}")"
  # Weekly: fixed minute and hour, every day-of-month and month, ONE day-of-week.
  [[ "${cron}" =~ ^[0-5]?[0-9]\ (1?[0-9]|2[0-3])\ \*\ \*\ [0-6]$ ]] ||
    wf_fail "cron '${cron}' is not weekly (want 'M H * * D' with one day of week)"

  perms="$(yq -o=json '.permissions' "${WORKFLOW}" | jq -c .)"
  [[ "${perms}" == '{"contents":"read"}' ]] ||
    wf_fail "top-level permissions are ${perms}, want {\"contents\":\"read\"}"
  jobperms="$(yq -o=json '[.jobs[] | select(has("permissions")) | .permissions]' "${WORKFLOW}" | jq -c .)"
  [[ "${jobperms}" == '[]' ]] ||
    wf_fail "a job declares its own permissions (${jobperms}); the workflow-level contents: read is the only grant"

  if grep -v '^[[:space:]]*#' "${WORKFLOW}" | grep -q 'secrets'; then
    wf_fail "references secrets; the sensor reads a public listing and needs none"
  fi

  skip_keys="$(yq -o=json '[.. | select(tag == "!!map") | keys | .[] | select(. == "if" or . == "continue-on-error")]' "${WORKFLOW}" | jq -c .)"
  [[ "${skip_keys}" == '[]' ]] ||
    wf_fail "declares ${skip_keys}; nothing may skip or soften the sensor step"

  runs="$(yq -o=json '[.jobs[].steps[] | select(has("run")) | .run]' "${WORKFLOW}" | jq -c '[.[] | sub("\\s+$"; "")]')"
  [[ "${runs}" == '["bash hack/ci/go-patch-lag.sh"]' ]] ||
    wf_fail "run steps are ${runs}, want exactly one: bash hack/ci/go-patch-lag.sh"

  # curl's worst case (the last retry may start at --retry-max-time and run --max-time) must end
  # inside the sensor step's own timeout, and the job's timeout must exceed the step's (checkout
  # and runner setup count against the job, not the step), so the step's timeout fires first.
  # Otherwise a stalled go.dev kills the run before the script prints why it failed. (The
  # setup headroom is job minus step, at least a minute; nothing here measures setup time.)
  # Canonical, bounded numbers only before any arithmetic (an error inside `if` reads as false).
  bounded='^[1-9][0-9]{0,3}$'
  step_timeout="$(yq '[.jobs[].steps[] | select(has("run"))][0]."timeout-minutes" // ""' "${WORKFLOW}")"
  [[ "${step_timeout}" =~ ${bounded} ]] ||
    wf_fail "the sensor step has no usable timeout-minutes (got '${step_timeout}'); curl's retry budget must be bounded by the step's own timeout"
  job_timeout="$(yq '[.jobs[] | select([.steps[] | has("run")] | any)][0]."timeout-minutes" // ""' "${WORKFLOW}")"
  [[ "${job_timeout}" =~ ${bounded} ]] ||
    wf_fail "the sensor job has no usable timeout-minutes (got '${job_timeout}')"
  ((10#${job_timeout} > 10#${step_timeout})) ||
    wf_fail "the job's timeout-minutes (${job_timeout}) is not above the sensor step's (${step_timeout}); checkout counts against the job, so the job could end before the step"
  # Code lines only: the script's comments name the flags too.
  code="$(grep -v '^[[:space:]]*#' "${SCRIPT}")"
  # One fetch, one budget: a second curl or a second flag would go unchecked by the parse below.
  # Occurrences, not lines: two calls on one line count twice.
  occurrences() { grep -oE -- "$1" <<<"${code}" | wc -l | tr -d '[:space:]'; }
  [[ "$(occurrences '(^|[^A-Za-z0-9_./-])curl([[:space:]]|$)')" == "1" &&
    "$(occurrences '[[:space:]]--max-time[[:space:]]')" == "1" &&
    "$(occurrences '[[:space:]]--retry-max-time[[:space:]]')" == "1" ]] ||
    fail "hack/ci/go-patch-lag.sh must make exactly one curl call with one --max-time and one --retry-max-time; the retry budget check reads only that one"
  max_time="$(sed -nE 's/.*[[:space:]]--max-time[[:space:]]+([^[:space:]]+).*/\1/p' <<<"${code}")"
  retry_max="$(sed -nE 's/.*[[:space:]]--retry-max-time[[:space:]]+([^[:space:]]+).*/\1/p' <<<"${code}")"
  [[ "${max_time}" =~ ${bounded} && "${retry_max}" =~ ${bounded} ]] ||
    fail "cannot read curl's --max-time ('${max_time}') and --retry-max-time ('${retry_max}') from hack/ci/go-patch-lag.sh; the retry budget must be bounded"
  ((10#${retry_max} + 10#${max_time} < 10#${step_timeout} * 60)) ||
    wf_fail "curl's worst case (--retry-max-time ${retry_max} + --max-time ${max_time} s) does not end inside the sensor step's timeout-minutes ${step_timeout}"

  pass "workflow: weekly schedule + workflow_dispatch, contents: read only, no secrets, one plain sensor step"
  pass "workflow: curl's retry budget ($((10#${retry_max} + 10#${max_time})) s) ends inside the sensor step's ${step_timeout}-minute timeout"
  echo "All go-patch-lag checks passed."
  exit 0
fi

# ---------------------------------------------------------------------------
# Self-test (CFC-6): copy the files the test reads, break one invariant per copy, and assert the
# test reds WITH THE MESSAGE of the assertion the mutant targets -- a non-zero exit alone is not
# evidence. One unmutated copy must pass.
# ---------------------------------------------------------------------------
NCOPY=0
new_copy() {
  NCOPY=$((NCOPY + 1))
  COPY="${WORK}/copy-${NCOPY}"
  mkdir -p "${COPY}/hack/ci" "${COPY}/hack/test/testdata" "${COPY}/.github/workflows"
  cp "${SCRIPT}" "${COPY}/hack/ci/"
  cp -R "${ROOT}/hack/test/testdata/go-patch-lag" "${COPY}/hack/test/testdata/"
  cp "${WORKFLOW}" "${COPY}/.github/workflows/"
}

# mutant_rejected <label> <sentinel> <probe-regex> <probe-file> <command...>
# The command runs inside the copy; the probe proves the mutation landed before the test runs.
mutant_rejected() {
  local label="$1" sentinel="$2" probe="$3" probe_file="$4" out rc
  shift 4
  new_copy
  (cd "${COPY}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- nothing was tested"
  grep -Eq -- "${probe}" "${COPY}/${probe_file}" ||
    fail "self-test: the '${label}' mutant did not land (no '${probe}' in ${probe_file}) -- nothing was tested"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the test accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  grep -Fq -- "${sentinel}" <<<"${out}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected '${sentinel}', got: $(head -3 <<<"${out}" | tr '\n' ' ')"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

new_copy
if ! out="$(bash "${SELF}" "${COPY}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree (or the test) first: $(head -3 <<<"${out}" | tr '\n' ' ')"
fi
pass "self-test: no-op control passes on an unmutated copy"

S=hack/ci/go-patch-lag.sh
W=.github/workflows/go-patch-lag.yaml

# --- script mutants: each breaks one verdict ------------------------------------------------
mutant_rejected "lookup failure passes (fail-open)" \
  "case unreachable: exit 0, want 2" '^undetermined\(\) \{ echo .*; exit 0; \}$' "${S}" \
  perl -0pi -e 's/^undetermined\(\) \{(.*?); exit 2; \}$/undetermined() {$1; exit 0; }/m' "${S}"

mutant_rejected "behind reported as current" \
  "case behind: exit 0, want 1" 'if \(\(10#\$\{patch\} < -1\)\)' "${S}" \
  perl -0pi -e 's/if \(\(10#\$\{patch\} < 10#\$\{latest\}\)\)/if ((10#\$\{patch\} < -1))/' "${S}"

mutant_rejected "newer minor treated as behind" \
  "case newer-minor-current: exit 1, want 0" 'MUTANT newest' "${S}" \
  perl -0pi -e 's/^(if \(\(10#\$\{patch\} < 10#\$\{latest\}\)\); then)$/# MUTANT newest\nif [[ "\$\{newest\}" != "go\$\{minor_tag\}.\$\{patch\}" ]]; then behind; fi\n$1/m' "${S}"

mutant_rejected "unstable releases counted" \
  "case unstable-ignored: exit 1, want 0" 'select\(true\)' "${S}" \
  perl -0pi -e 's/select\(\.stable == true\)/select(true)/g' "${S}"

mutant_rejected "non-array listing accepted" \
  "case unparsable-object: exit 0, want 2" 'if type == "object" then \[\.\] else' "${S}" \
  perl -0pi -e 's/if type != "array" then error\("not an array"\) else \. end/if type == "object" then [.] else . end/' "${S}"

mutant_rejected "leading-zero listing entry accepted" \
  "case leading-zero-listing: exit 1, want 2" 'select\(test\("\^\[0-9\]\+\$"\)\)' "${S}" \
  perl -0pi -e 's/select\(test\("\^\(0\|\[1-9\]\[0-9\]\{0,3\}\)\$"\)\)/select(test("^[0-9]+\$"))/' "${S}"

mutant_rejected "unbounded listing number accepted" \
  "case overflow-listing: exit 0, want 2" 'select\(test\("\^\(0\|\[1-9\]\[0-9\]\*\)\$"\)\)' "${S}" \
  perl -0pi -e 's/select\(test\("\^\(0\|\[1-9\]\[0-9\]\{0,3\}\)\$"\)\)/select(test("^(0|[1-9][0-9]*)\$"))/' "${S}"

mutant_rejected "unbounded go directive accepted" \
  "case gomod-overflow: exit 0, want 2" "^num='\(0\|\[1-9\]\[0-9\]\*\)'" "${S}" \
  perl -0pi -e "s/^num='.*'\$/num='(0|[1-9][0-9]*)'/m" "${S}"

mutant_rejected "leading-zero go directive accepted" \
  "case gomod-leading-zero: exit 0, want 2" "^num='\(\[0-9\]\{1,4\}\)'" "${S}" \
  perl -0pi -e "s/^num='.*'\$/num='([0-9]{1,4})'/m" "${S}"

# --- workflow mutants -------------------------------------------------------------------------
mutant_rejected "permissions widened" \
  "workflow: top-level permissions" '^permissions: write-all' "${W}" \
  yq -i '.permissions = "write-all"' "${W}"

mutant_rejected "job-level permissions added" \
  "workflow: a job declares its own permissions" 'contents: write' "${W}" \
  yq -i '.jobs[].permissions = {"contents": "write"}' "${W}"

mutant_rejected "schedule removed" \
  "workflow: triggers are" 'workflow_dispatch' "${W}" \
  yq -i 'del(."on".schedule)' "${W}"

mutant_rejected "push trigger added" \
  "workflow: triggers are" 'push:' "${W}" \
  yq -i '."on".push = {"branches": ["main"]}' "${W}"

mutant_rejected "daily instead of weekly" \
  "is not weekly" "cron: ['\"]?17 6 \* \* \*['\"]?" "${W}" \
  yq -i '."on".schedule[0].cron = "17 6 * * *"' "${W}"

mutant_rejected "secrets referenced" \
  "workflow: references secrets" 'secrets\.GITHUB_TOKEN' "${W}" \
  yq -i '.jobs[].env = {"GH_TOKEN": "${{ secrets.GITHUB_TOKEN }}"}' "${W}"

mutant_rejected "sensor step soft-fails" \
  "workflow: declares" 'continue-on-error: true' "${W}" \
  yq -i '(.jobs[].steps[] | select(has("run"))).continue-on-error = true' "${W}"

mutant_rejected "sensor step skipped by if" \
  "workflow: declares" 'if: false' "${W}" \
  yq -i '.jobs[].if = false' "${W}"

mutant_rejected "sensor run masked with || true" \
  "workflow: run steps are" 'go-patch-lag\.sh \|\| true' "${W}" \
  yq -i '(.jobs[].steps[] | select(has("run"))).run = "bash hack/ci/go-patch-lag.sh || true"' "${W}"

# --- retry budget vs. timeout mutants ---------------------------------------------------------
mutant_rejected "sensor step timeout removed" \
  "workflow: the sensor step has no usable timeout-minutes" 'MUTANT-NO-STEP-TIMEOUT' "${W}" \
  perl -0pi -e 's/^        timeout-minutes: 5\n/        # MUTANT-NO-STEP-TIMEOUT\n/m' "${W}"

mutant_rejected "sensor step timeout below curl's budget" \
  "does not end inside the sensor step's timeout-minutes 3" 'timeout-minutes: 3$' "${W}" \
  yq -i '(.jobs[].steps[] | select(has("run")))."timeout-minutes" = 3' "${W}"

mutant_rejected "curl retry budget grown past the step timeout" \
  "does not end inside the sensor step's timeout-minutes" '--retry-max-time 400 ' "${S}" \
  perl -pi -e 's/--retry-max-time 200 /--retry-max-time 400 /' "${S}"

mutant_rejected "curl retry budget unbounded" \
  "exactly one curl call with one --max-time and one --retry-max-time" '--MUTANT-NO-RETRY-MAX ' "${S}" \
  perl -pi -e 's/ --retry-max-time 200 / --MUTANT-NO-RETRY-MAX /' "${S}"

mutant_rejected "second curl call with its own budget" \
  "exactly one curl call" 'MUTANT-SECOND-CURL' "${S}" \
  perl -pi -e 's/^(# --- the listing.*)$/$1\nunused_fetch() { curl -fsS --max-time 900 --retry-max-time 900 https:\/\/127.0.0.1:9\/; } # MUTANT-SECOND-CURL/' "${S}"


mutant_rejected "second curl call on the same line" \
  "exactly one curl call" 'MUTANT-SAME-LINE ' "${S}" \
  perl -0pi -e 's/^(  )(curl -fsSL --proto .*?) \\\n\s+(--retry .*? \|\|)$/$1curl -fsS --max-time 900 --retry-max-time 900 https:\/\/127.0.0.1:9\/MUTANT-SAME-LINE >\/dev\/null 2>&1 || true; $2 $3/m' "${S}"

mutant_rejected "job timeout below the step's" \
  "is not above the sensor step's" '^    timeout-minutes: 4$' "${W}" \
  yq -i '.jobs[]."timeout-minutes" = 4' "${W}"

mutant_rejected "job timeout equal to the step's" \
  "is not above the sensor step's" '^    timeout-minutes: 5$' "${W}" \
  yq -i '.jobs[]."timeout-minutes" = 5' "${W}"

echo "All go-patch-lag self-tests passed."
