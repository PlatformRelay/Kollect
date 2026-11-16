#!/usr/bin/env bash
# SVS-1 to SVS-3 (openspec change scheduled-vulncheck): govulncheck runs on main every day, the
# scan cannot be softened, and each advisory reaches the CI failure report on a FAIL line.
#
# .github/workflows/vulncheck-scheduled.yaml (`Vulnerability scan`) runs
# hack/ci/vulncheck-scheduled.sh daily; the script runs `task vulncheck`, prints its output, then
# one `FAIL: <GO id> <title>` line per advisory (or one "no advisory" line on a tool failure), and
# exits with task's status. ci-failure-report.yaml lists the workflow by name, so a failed
# scheduled run opens or updates a ci-failure issue whose excerpt (the lines holding FAIL) names
# the advisories. This test never runs govulncheck: the script cases put a fake `task` first on
# PATH that prints a fixture and exits with a given status.
#
# Fixtures (hack/test/testdata/vulncheck-scheduled/), each real `task vulncheck` output:
#   advisories.txt   CI run 43406013666 (2026-11-13, go1.27.3), job vulncheck, task exit 201: two
#                    module advisories and the stdlib GO-2026-6859; timestamps, colour codes and
#                    Actions annotations stripped, example traces cut to two per advisory;
#   clean.txt        Task 3.54.0 `task vulncheck` on this branch, go1.27.4, exit 0;
#   no-advisory.txt  the same with GOTOOLCHAIN=local, an empty GOMODCACHE and GOPROXY=off, exit 201.
# The generating commands are in openspec/changes/scheduled-vulncheck/evidence/G1.md.
#
# What it locks:
#   V-1  the workflow and the script exist;
#   V-2  the workflow's shape: name, exactly schedule (one daily cron) and input-less
#        workflow_dispatch, exactly contents: read, one job with only runs-on, timeout-minutes (a
#        positive integer), steps and name, the four steps (checkout at a SHA with only
#        persist-credentials: false, go-cache with no inputs, setup-task at a SHA with only
#        version:, the scan step running exactly the script), no continue-on-error, if, shell,
#        defaults, concurrency or env anywhere, no secret or token expression, and every action
#        SHA and the setup-task version already used by another workflow;
#   V-3  the script's verdicts: (a) advisories -> the whole report, one trimmed FAIL line per
#        advisory, task's exit status; (b) a failure with no advisory -> one "no advisory" FAIL
#        line naming the status, which passes through (201 and 7); (c) clean -> no FAIL line,
#        exit 0; (d) task is called with exactly `vulncheck`; (e) the title rule's edges (a
#        whitespace-only line skipped, trailing blanks cut, an entry with no title line).
# Every failure is collected before the exit (FAIL: <case>: lines), so one defect never hides
# another's message.
#
# Usage:
#   hack/test/vulncheck_schedule_test.sh              # check the repository tree
#   hack/test/vulncheck_schedule_test.sh --self-test  # mutate copies, assert each mutant reds
#   hack/test/vulncheck_schedule_test.sh <tree-root>  # check another tree root
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
REPO="$(cd "$(dirname "${SELF}")/../.." && pwd -P)"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-${REPO}}"
ROOT="$(cd "${ROOT}" && pwd -P)"

for tool in yq jq; do
  command -v "${tool}" >/dev/null 2>&1 || fail "${tool} not found; install it to run this test"
done

WF_REL=.github/workflows/vulncheck-scheduled.yaml
WF="${ROOT}/${WF_REL}"
SCRIPT_REL=hack/ci/vulncheck-scheduled.sh
SCRIPT="${ROOT}/${SCRIPT_REL}"
FX_REL=hack/test/testdata/vulncheck-scheduled
FX="${ROOT}/${FX_REL}"
RUN_LINE="bash ${SCRIPT_REL}"

SCRATCH=""
trap 'rm -rf ${SCRATCH} 2>/dev/null' EXIT

# Called directly (never as $(new_tmp)), so SCRATCH survives in this shell for the EXIT trap.
TMP=""
new_tmp() {
  TMP="$(mktemp -d)"
  SCRATCH="${SCRATCH} ${TMP}"
}

if [[ "${MODE}" == "check" ]]; then
  BAD=0
  case_fail() {
    echo "FAIL: $1: $2" >&2
    BAD=$((BAD + 1))
  }
  one_line() { tr '\n' '|' <"$1" | sed 's/|$//'; }

  # ===========================================================================================
  # V-3: the script's verdicts, against a fake `task` first on PATH
  # ===========================================================================================
  new_tmp
  ST="${TMP}"
  mkdir -p "${ST}/bin"
  cat >"${ST}/bin/task" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$@" >"${FAKE_TASK_ARGS}"
cat "${FAKE_TASK_FIXTURE}"
exit "${FAKE_TASK_EXIT}"
FAKE
  chmod +x "${ST}/bin/task"

  # run_script <case> <fixture> <exit>: runs the script; RC, ${ST}/out (stdout and stderr),
  # ${ST}/fail (its FAIL lines). Checks V-3d (the argv) and that the output begins with the
  # whole report.
  run_script() {
    local id="$1" fixture="${FX}/$2"
    [[ "$2" != /* ]] || fixture="$2"
    : >"${ST}/args"
    (cd "${ST}" && env PATH="${ST}/bin:${PATH}" FAKE_TASK_ARGS="${ST}/args" FAKE_TASK_FIXTURE="${fixture}" \
      FAKE_TASK_EXIT="$3" bash "${SCRIPT}") >"${ST}/out" 2>&1 && RC=0 || RC=$?
    { grep '^FAIL:' "${ST}/out" || true; } >"${ST}/fail"
    [[ "$(cat "${ST}/args")" == vulncheck ]] ||
      case_fail V-3d "${id}: task was called with [$(one_line "${ST}/args")], want exactly [vulncheck]"
    head -n "$(wc -l <"${fixture}")" "${ST}/out" | cmp -s - "${fixture}" ||
      case_fail "${id}" "the output does not begin with the whole task vulncheck report ($2): the report was swallowed or altered"
  }
  # want_fail <case> <expected FAIL lines, one per line>
  want_fail() {
    printf '%s' "$2" >"${ST}/want"
    [[ -z "$2" ]] || printf '\n' >>"${ST}/want"
    cmp -s "${ST}/fail" "${ST}/want" ||
      case_fail "$1" "FAIL lines are [$(one_line "${ST}/fail")], want [$(one_line "${ST}/want")]"
  }
  want_rc() {
    [[ "${RC}" == "$2" ]] || case_fail "$1" "the script exited ${RC}, want $2 (task's status)"
  }

  fixtures_ok=1
  for f in advisories.txt clean.txt no-advisory.txt; do
    [[ -s "${FX}/${f}" ]] || {
      case_fail V-3 "absence: fixture ${FX_REL}/${f} is missing or empty"
      fixtures_ok=0
    }
  done
  if [[ ! -f "${SCRIPT}" ]]; then
    case_fail V-1 "absence: ${SCRIPT_REL} is missing"
  elif ((fixtures_ok)); then
    run_script V-3a advisories.txt 201
    want_rc V-3a 201
    want_fail V-3a "FAIL: GO-2026-6888 Excessive memory allocation when decompressing crafted streams in
FAIL: GO-2026-6865 Panic on two-block frames in github.com/klauspost/compress
FAIL: GO-2026-6859 HTTP/1 server crash due to MaxBytesReader header race in net/http"

    run_script V-3b no-advisory.txt 201
    want_rc V-3b 201
    want_fail V-3b "FAIL: task vulncheck exited 201; no advisory in its output (tool, network or build failure)"
    run_script V-3b no-advisory.txt 7
    want_rc V-3b 7
    want_fail V-3b "FAIL: task vulncheck exited 7; no advisory in its output (tool, network or build failure)"

    run_script V-3c clean.txt 0
    want_rc V-3c 0
    want_fail V-3c ""

    # V-3e: the SVS-1.2 title rule's edges, which real output does not show: a whitespace-only line
    # before the title (skipped), trailing blanks (cut), an entry followed directly by the next one
    # and an entry at the end of the output (each a FAIL line with its id and no title). Built here,
    # not committed, so no whitespace hook can strip it; the GO ids are implausible on purpose.
    printf '%s\n' '=== Symbol Results ===' '' 'Vulnerability #1: GO-2099-0001' $'   \t' \
      $'    Title after a whitespace-only line   \t' '  More info: https://pkg.go.dev/vuln/GO-2099-0001' '' \
      'Vulnerability #2: GO-2099-0002' 'Vulnerability #3: GO-2099-0003' >"${ST}/edge.txt"
    run_script V-3e "${ST}/edge.txt" 201
    want_rc V-3e 201
    want_fail V-3e "FAIL: GO-2099-0001 Title after a whitespace-only line
FAIL: GO-2099-0002
FAIL: GO-2099-0003"
  fi

  # ===========================================================================================
  # V-1, V-2: the workflow's shape (read once through yq, anchors expanded, then queried as JSON)
  # ===========================================================================================
  if [[ ! -f "${WF}" ]]; then
    case_fail V-1 "absence: ${WF_REL} is missing"
  elif ! yq eval -o=json -I=0 'explode(.)' "${WF}" >"${ST}/wf.json" 2>"${ST}/yq.err"; then
    case_fail V-2 "${WF_REL} does not parse as YAML: $(head -c 200 "${ST}/yq.err" | tr '\n' ' ')"
  else
    wj() { jq -r "$1" "${ST}/wf.json"; }
    c() { case_fail V-2 "$1"; }
    [[ "$(wj '.name')" == "Vulnerability scan" ]] || c "name: is '$(wj '.name')', want 'Vulnerability scan' (ci-failure-report.yaml matches it by name)"
    [[ "$(wj 'keys | join(",")')" == "jobs,name,on,permissions" ]] ||
      c "top-level keys are [$(wj 'keys | join(",")')], want exactly name, on, permissions, jobs (no concurrency:, env:, defaults: or other key)"
    [[ "$(wj '.on | keys? | join(",")')" == "schedule,workflow_dispatch" ]] ||
      c "on: is [$(wj '.on | keys? | join(",")')], want exactly schedule and workflow_dispatch"
    [[ "$(wj '.on.schedule | (type == "array" and length == 1 and (.[0] | type == "object" and keys == ["cron"]))')" == true ]] ||
      c "on.schedule is $(wj '.on.schedule | tojson'), want exactly one cron entry"
    cron="$(wj '.on.schedule[0].cron? // ""')"
    [[ "${cron}" =~ ^([0-9]|[1-5][0-9])\ ([0-9]|1[0-9]|2[0-3])\ \*\ \*\ \*$ ]] ||
      c "cron '${cron}' is not daily (want '<minute> <hour> * * *', minute and hour one integer each)"
    [[ "$(wj '.on.workflow_dispatch | (. == null or . == {})')" == true ]] ||
      c "on.workflow_dispatch takes inputs or options: $(wj '.on.workflow_dispatch | tojson')"
    [[ "$(wj '.permissions | tojson')" == '{"contents":"read"}' ]] ||
      c "permissions: is $(wj '.permissions | tojson'), want exactly contents: read"
    [[ "$(wj '.jobs | length')" == 1 ]] || c "want exactly one job, found $(wj '.jobs | length')"
    job='.jobs | to_entries | .[0].value'
    jk="$(wj "${job} | keys | join(\",\")")"
    [[ "${jk}" == "name,runs-on,steps,timeout-minutes" || "${jk}" == "runs-on,steps,timeout-minutes" ]] ||
      c "the job's keys are [${jk}], want runs-on, timeout-minutes, steps (and name): no if:, continue-on-error:, permissions:, concurrency:, env: or defaults:"
    [[ "$(wj "${job}.\"timeout-minutes\" | (type == \"number\" and . == floor and . > 0)")" == true ]] ||
      c "timeout-minutes is '$(wj "${job}.\"timeout-minutes\"")', want a positive integer"
    st() { wj "${job}.steps[$1] | $2"; }
    sk() { st "$1" 'keys - ["name"] | join(",")'; }
    [[ "$(wj "${job}.steps | length")" == 4 ]] ||
      c "want exactly 4 steps (checkout, go-cache, setup-task, the scan), found $(wj "${job}.steps | length")"
    [[ "$(sk 0)" == "uses,with" && "$(st 0 '.uses')" =~ ^actions/checkout@[0-9a-f]{40}$ &&
      "$(st 0 '.with | tojson')" == '{"persist-credentials":false}' ]] ||
      c "step 1 must be actions/checkout@<sha> with exactly persist-credentials: false, found $(st 0 'tojson')"
    [[ "$(sk 1)" == "uses" && "$(st 1 '.uses')" == ./.github/actions/go-cache ]] ||
      c "step 2 must be ./.github/actions/go-cache with no inputs, found $(st 1 'tojson')"
    [[ "$(sk 2)" == "uses,with" && "$(st 2 '.uses')" =~ ^go-task/setup-task@[0-9a-f]{40}$ &&
      "$(st 2 '.with | keys | join(",")')" == version ]] ||
      c "step 3 must be go-task/setup-task@<sha> with only version:, found $(st 2 'tojson')"
    [[ "$(sk 3)" == "run" && "$(st 3 '.run | sub("\\s+$"; "")')" == "${RUN_LINE}" ]] ||
      c "step 4 must be run: exactly '${RUN_LINE}' (and name), no env:, found $(st 3 'tojson')"
    soft="$(wj '[.. | objects | keys[] | select(. == "continue-on-error" or . == "if" or . == "shell" or . == "defaults" or . == "concurrency" or . == "env")] | unique | join(",")')"
    [[ -z "${soft}" ]] || c "[${soft}] appears in the workflow: nothing may skip, soften, cancel or re-environment the scan"
    # shellcheck disable=SC2016 # the GitHub expression syntax is the text searched for
    tok="$(wj '[.. | strings | select(test("\\$\\{\\{[^}]*(secrets|github\\.token)"))] | join(" ; ")')"
    [[ -z "${tok}" ]] || c "a secret or token expression appears (${tok}); the scan reads public data and needs none"
    # Each action at a SHA already used by another workflow, so dependency-review saw it, and
    # setup-task at a version another workflow uses.
    others=()
    for f in "${ROOT}"/.github/workflows/*.yaml "${ROOT}"/.github/workflows/*.yml; do
      [[ -f "${f}" && "${f}" != "${WF}" ]] && others+=("${f}")
    done
    for i in 0 2; do
      ref="$(st "${i}" '.uses')"
      [[ "${ref}" == *@* ]] || continue
      if ((${#others[@]} == 0)) || ! grep -hF -- "uses: ${ref}" "${others[@]}" >/dev/null; then
        c "step $((i + 1)) uses ${ref}, which no other workflow under .github/workflows/ uses: a new action version dependency-review never saw"
      fi
    done
    ver="$(st 2 '.with.version | tostring')"
    seen=0
    for f in "${others[@]}"; do
      if ! vs="$(yq eval -o=json -I=0 '[explode(.) | .jobs[]?.steps[]? | select((.uses // "") | test("^go-task/setup-task@")) | .with.version | tostring]' "${f}" 2>/dev/null)"; then
        c "${f#"${ROOT}"/} does not parse as YAML"
        continue
      fi
      jq -e --arg v "${ver}" 'index($v) != null' <<<"${vs}" >/dev/null && seen=1
    done
    ((seen)) || c "setup-task version: ${ver} is used by no other workflow"
  fi

  if ((BAD > 0)); then
    echo "${BAD} check(s) failed" >&2
    exit 1
  fi
  pass "V-1 to V-3: the daily Vulnerability scan workflow has its pinned shape, and the script names each advisory and keeps task's status"
  exit 0
fi

# =============================================================================================
# Self-test (SVS-3): each mutant edits a throwaway copy of the files this test reads, the test
# runs on the copy and must exit non-zero with a FAIL line holding the mutant's expected message;
# a non-zero exit alone is no evidence. The unmutated copy and the controls must pass. One copy
# at a time, freed after each mutant. Every literal edit is an exactly-once replace, and a copy
# the edit left unchanged fails the self-test, so a vanished anchor never tests nothing.
# =============================================================================================
SBAD=0
sbad() {
  echo "FAIL: self-test: $*" >&2
  SBAD=$((SBAD + 1))
}

new_tmp
BASE="${TMP}/base"
COPY="${TMP}/copy"
mkdir -p "${BASE}/hack/ci" "${BASE}/hack/test/testdata"
cp -R "${ROOT}/.github" "${BASE}/"
cp -p "${SCRIPT}" "${BASE}/hack/ci/" 2>/dev/null || true
cp -R "${FX}" "${BASE}/hack/test/testdata/" 2>/dev/null || true

# lit <file> <old> <new>: replace the one literal occurrence of <old>; fails when it is absent or
# occurs more than once.
lit() {
  local f="$1" old="$2" new="$3" text rest
  [[ -f "${f}" ]] || {
    echo "lit: ${f} is missing" >&2
    return 1
  }
  text="$(
    cat "${f}"
    printf x
  )"
  text="${text%x}"
  [[ "${text}" == *"${old}"* ]] || {
    echo "lit: text not found in ${f}: ${old}" >&2
    return 1
  }
  rest="${text#*"${old}"}"
  [[ "${rest}" != *"${old}"* ]] || {
    echo "lit: text occurs more than once in ${f}: ${old}" >&2
    return 1
  }
  printf '%s' "${text/"${old}"/"${new}"}" >"${f}"
}
yqe() { yq eval -i "$2" "$1"; }

# judge <label> <message> <edit...>
judge() {
  local label="$1" msg="$2" out rc=0
  shift 2
  rm -rf "${COPY:?}"
  cp -a "${BASE}" "${COPY}"
  if ! (cd "${COPY}" && "$@") >"${TMP}/edit.log" 2>&1; then
    sbad "${label}: the mutation step failed, nothing was tested: $(head -c 300 "${TMP}/edit.log" | tr '\n' ' ')"
    rm -rf "${COPY:?}"
    return 0
  fi
  if diff -r "${BASE}" "${COPY}" >/dev/null 2>&1; then
    sbad "${label}: the mutation changed nothing, nothing was tested"
    rm -rf "${COPY:?}"
    return 0
  fi
  out="$(bash "${SELF}" "${COPY}" 2>&1)" || rc=$?
  if [[ "${rc}" -ne 0 ]] && grep '^FAIL: ' <<<"${out}" | grep -F -- "${msg}" >/dev/null; then
    pass "self-test: ${label}: red (exit ${rc}) with '${msg}'"
  else
    sbad "${label}: expected a non-zero exit and a FAIL line with '${msg}', got exit ${rc}: $(grep '^FAIL: ' <<<"${out}" | head -n 3 | tr '\n' ' ')"
  fi
  rm -rf "${COPY:?}"
}

# control <label> [edit...]: as judge, but the copy must pass.
control() {
  local label="$1" out rc=0
  shift
  rm -rf "${COPY:?}"
  cp -a "${BASE}" "${COPY}"
  if [[ "$#" -gt 0 ]]; then
    if ! (cd "${COPY}" && "$@") >"${TMP}/edit.log" 2>&1; then
      sbad "control ${label}: the edit failed: $(head -c 300 "${TMP}/edit.log" | tr '\n' ' ')"
      rm -rf "${COPY:?}"
      return 0
    fi
    if diff -r "${BASE}" "${COPY}" >/dev/null 2>&1; then
      sbad "control ${label}: the edit changed nothing, nothing was tested"
      rm -rf "${COPY:?}"
      return 0
    fi
  fi
  out="$(bash "${SELF}" "${COPY}" 2>&1)" || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    pass "self-test: control ${label}: passes"
  else
    sbad "control ${label}: expected a pass, got exit ${rc}: $(grep '^FAIL: ' <<<"${out}" | head -n 3 | tr '\n' ' ')"
  fi
  rm -rf "${COPY:?}"
}

W="${WF_REL}"
S="${SCRIPT_REL}"
nl=$'\n'
# The pins read from the tree, never copied into this test (a bot bump must not red it).
CHECKOUT_REF="$(yq '.jobs[].steps[0].uses' "${WF}")"
SETUP_REF="$(yq '.jobs[].steps[2].uses' "${WF}")"
SETUP_VER="$(yq '.jobs[].steps[2].with.version' "${WF}")"

# --- controls (V-4) -----------------------------------------------------------------------------
control "unmutated copy"
control "a comment that mentions secrets and github.token" \
  lit "${W}" "permissions:${nl}" "# This scan reads no secrets and never github.token.${nl}permissions:${nl}"
bump() { # what a grouped bot PR would do: every pin this test reads, at every site
  sed -i "s|${CHECKOUT_REF%@*}@${CHECKOUT_REF#*@}|${CHECKOUT_REF%@*}@ffffffffffffffffffffffffffffffffffffffff|g; s|${SETUP_REF%@*}@${SETUP_REF#*@}|${SETUP_REF%@*}@eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee|g" .github/workflows/*.y*ml &&
    sed -E -i "s/(version: )${SETUP_VER//./\\.}\$/\\199.0.0/" .github/workflows/*.y*ml
}
control "every pin bumped by a bot (implausible versions and SHAs)" bump
control "the scan step written as a literal block" \
  lit "${W}" "        run: ${RUN_LINE}${nl}" "        run: |${nl}          ${RUN_LINE}${nl}"

# shellcheck disable=SC2016 # every ${...} below is file text to find or write, never expanded
{
  # --- V-1: absence -----------------------------------------------------------------------------
  judge "the workflow deleted (V-1)" "V-1: absence: ${W} is missing" rm -f "${W}"
  judge "the script deleted (V-1)" "V-1: absence: ${S} is missing" rm -f "${S}"
  judge "the advisory fixture deleted (V-3)" "V-3: absence: fixture ${FX_REL}/advisories.txt" rm -f "${FX_REL}/advisories.txt"

  # --- V-2: the workflow's shape ----------------------------------------------------------------
  judge "name changed (V-2)" "V-2: name: is 'Vulnerability scan (daily)'" \
    lit "${W}" "name: Vulnerability scan${nl}" "name: Vulnerability scan (daily)${nl}"
  judge "workflow concurrency: (V-2)" "V-2: top-level keys are [concurrency,jobs,name,on,permissions]" \
    yqe "${W}" '.concurrency = {"group": "vulncheck", "cancel-in-progress": true}'
  judge "workflow env: (V-2)" "V-2: top-level keys are [env,jobs,name,on,permissions]" \
    yqe "${W}" '.env = {"GOFLAGS": "-mod=mod"}'
  judge "workflow defaults: (V-2)" "V-2: top-level keys are [defaults,jobs,name,on,permissions]" \
    yqe "${W}" '.defaults = {"run": {"shell": "bash"}}'
  judge "job concurrency: (V-2)" "V-2: the job's keys are [concurrency," \
    yqe "${W}" '.jobs[].concurrency = {"group": "vulncheck"}'
  judge "job env: (V-2)" "V-2: the job's keys are [env," \
    yqe "${W}" '.jobs[].env = {"GOFLAGS": "-mod=mod"}'
  judge "job if: (V-2)" "V-2: the job's keys are [if," \
    yqe "${W}" '.jobs[].if = "github.ref == '"'"'refs/heads/main'"'"'"'
  judge "job permissions: (V-2)" "V-2: the job's keys are [name,permissions," \
    yqe "${W}" '.jobs[].permissions = {"contents": "read"}'
  judge "schedule removed (V-2)" "V-2: on: is [workflow_dispatch]" yqe "${W}" 'del(.on.schedule)'
  judge "workflow_dispatch removed (V-2)" "V-2: on: is [schedule]" yqe "${W}" 'del(.on.workflow_dispatch)'
  judge "push trigger added (V-2)" "V-2: on: is [push,schedule,workflow_dispatch]" yqe "${W}" '.on.push = {"branches": ["main"]}'
  judge "a second cron (V-2)" "V-2: on.schedule is" yqe "${W}" '.on.schedule += [{"cron": "41 16 * * *"}]'
  judge "workflow_dispatch inputs (V-2)" "V-2: on.workflow_dispatch takes inputs" \
    yqe "${W}" '.on.workflow_dispatch = {"inputs": {"db": {"type": "string"}}}'
  judge "cron weekly (V-2)" "V-2: cron '0 5 * * 1' is not daily" yqe "${W}" '.on.schedule[0].cron = "0 5 * * 1"'
  judge "cron monthly (V-2)" "V-2: cron '0 5 1 * *' is not daily" yqe "${W}" '.on.schedule[0].cron = "0 5 1 * *"'
  judge "cron hourly (V-2)" "V-2: cron '17 * * * *' is not daily" yqe "${W}" '.on.schedule[0].cron = "17 * * * *"'
  judge "cron every five minutes (V-2)" "V-2: cron '*/5 * * * *' is not daily" yqe "${W}" '.on.schedule[0].cron = "*/5 * * * *"'
  judge "cron with a list of hours (V-2)" "V-2: cron '41 4,16 * * *' is not daily" yqe "${W}" '.on.schedule[0].cron = "41 4,16 * * *"'
  judge "permissions widened (V-2)" "V-2: permissions: is" lit "${W}" "  contents: read${nl}" "  contents: write${nl}"
  judge "a permission added (V-2)" "V-2: permissions: is" yqe "${W}" '.permissions.issues = "write"'
  judge "permissions write-all (V-2)" "V-2: permissions: is" yqe "${W}" '.permissions = "write-all"'
  judge "a second job (V-2)" "V-2: want exactly one job, found 2" \
    yqe "${W}" '.jobs.extra = {"runs-on": "ubuntu-latest", "timeout-minutes": 5, "steps": [{"run": "true"}]}'
  judge "continue-on-error on the job (V-2)" "V-2: [continue-on-error] appears in the workflow" \
    yqe "${W}" '.jobs[]."continue-on-error" = true'
  judge "continue-on-error on the scan step (V-2)" "V-2: [continue-on-error] appears in the workflow" \
    yqe "${W}" '.jobs[].steps[3]."continue-on-error" = true'
  judge "if: on the scan step (V-2)" "V-2: [if] appears in the workflow" \
    yqe "${W}" '.jobs[].steps[3].if = "false"'
  judge "shell: on the scan step (V-2)" "V-2: [shell] appears in the workflow" \
    yqe "${W}" '.jobs[].steps[3].shell = "bash {0} || true"'
  judge "job defaults: (V-2)" "V-2: [defaults] appears in the workflow" \
    yqe "${W}" '.jobs[].defaults = {"run": {"working-directory": "hack"}}'
  judge "timeout-minutes missing (V-2)" "V-2: timeout-minutes is 'null', want a positive integer" \
    yqe "${W}" 'del(.jobs[]."timeout-minutes")'
  judge "timeout-minutes 0 (V-2)" "V-2: timeout-minutes is '0', want a positive integer" \
    yqe "${W}" '.jobs[]."timeout-minutes" = 0'
  judge "timeout-minutes not an integer (V-2)" "V-2: timeout-minutes is '7.5', want a positive integer" \
    yqe "${W}" '.jobs[]."timeout-minutes" = 7.5'
  judge "timeout-minutes a string (V-2)" "V-2: timeout-minutes is '15m', want a positive integer" \
    yqe "${W}" '.jobs[]."timeout-minutes" = "15m"'
  judge "a fifth step (V-2)" "V-2: want exactly 4 steps" yqe "${W}" '.jobs[].steps += [{"run": "echo extra"}]'
  judge "checkout at a tag (V-2)" "V-2: step 1 must be actions/checkout@<sha>" \
    yqe "${W}" '.jobs[].steps[0].uses = "actions/checkout@v7"'
  judge "checkout keeps credentials (V-2)" "V-2: step 1 must be actions/checkout@<sha>" \
    yqe "${W}" 'del(.jobs[].steps[0].with."persist-credentials")'
  judge "checkout ref: input (V-2)" "V-2: step 1 must be actions/checkout@<sha>" \
    yqe "${W}" '.jobs[].steps[0].with.ref = "release"'
  judge "checkout repository: input (V-2)" "V-2: step 1 must be actions/checkout@<sha>" \
    yqe "${W}" '.jobs[].steps[0].with.repository = "someone/fork"'
  judge "go-cache given inputs (V-2)" "V-2: step 2 must be ./.github/actions/go-cache with no inputs" \
    yqe "${W}" '.jobs[].steps[1].with = {"cache-suffix": "vuln"}'
  judge "go-cache replaced by another local action (V-2)" "V-2: step 2 must be ./.github/actions/go-cache" \
    yqe "${W}" '.jobs[].steps[1].uses = "./.github/actions/other"'
  judge "setup-task at a tag (V-2)" "V-2: step 3 must be go-task/setup-task@<sha>" \
    yqe "${W}" '.jobs[].steps[2].uses = "go-task/setup-task@v2"'
  judge "setup-task given another input (V-2)" "V-2: step 3 must be go-task/setup-task@<sha> with only version:" \
    yqe "${W}" '.jobs[].steps[2].with."repo-token" = "x"'
  judge "setup-task at a version no other workflow uses (V-2)" "V-2: setup-task version: 99.0.0 is used by no other workflow" \
    yqe "${W}" '.jobs[].steps[2].with.version = "99.0.0"'
  judge "checkout at a SHA no other workflow uses (V-2)" "V-2: step 1 uses actions/checkout@ffffffffffffffffffffffffffffffffffffffff, which no other workflow" \
    yqe "${W}" '.jobs[].steps[0].uses = "actions/checkout@ffffffffffffffffffffffffffffffffffffffff"'
  # Decoy: a SHA another workflow uses, but for a different action.
  judge "setup-task at checkout's SHA (V-2 decoy)" "V-2: step 3 uses go-task/setup-task@${CHECKOUT_REF#*@}, which no other workflow" \
    yqe "${W}" ".jobs[].steps[2].uses = \"go-task/setup-task@${CHECKOUT_REF#*@}\""
  judge "the scan run masked with || true (V-2)" "V-2: step 4 must be run: exactly" \
    lit "${W}" "run: ${RUN_LINE}${nl}" "run: ${RUN_LINE} || true${nl}"
  judge "the scan runs task vulncheck directly (V-2)" "V-2: step 4 must be run: exactly" \
    lit "${W}" "run: ${RUN_LINE}${nl}" "run: task vulncheck${nl}"
  judge "the scan step given env: (V-2)" "V-2: step 4 must be run: exactly" \
    yqe "${W}" '.jobs[].steps[3].env = {"GOFLAGS": "-mod=mod"}'
  judge "a secrets expression (V-2)" "V-2: a secret or token expression appears" \
    yqe "${W}" '.jobs[].steps[2].with.version = "${{ secrets.TASK_VERSION }}"'
  judge "a github.token expression (V-2)" "V-2: a secret or token expression appears" \
    yqe "${W}" '.jobs[].steps[0].with.token = "${{ github.token }}"'

  # --- V-3: the script's verdicts ---------------------------------------------------------------
  judge "status not propagated: always exit 0 (V-3)" "V-3a: the script exited 0, want 201" \
    lit "${S}" 'exit "${status}"' 'exit 0'
  judge "status not propagated: always exit 1 (V-3)" "V-3a: the script exited 1, want 201" \
    lit "${S}" 'exit "${status}"' 'exit 1'
  judge "the tee's status kept instead of task's (V-3)" "V-3a: the script exited 0, want 201" \
    lit "${S}" 'status="${PIPESTATUS[0]}"' 'status="${PIPESTATUS[1]}"'
  judge "the status hard-coded as 201 (V-3)" "V-3b: the script exited 201, want 7" \
    lit "${S}" 'exit "${status}"' 'exit "$((status == 0 ? 0 : 201))"'
  judge "the FAIL lines dropped (V-3)" "V-3a: FAIL lines are [], want" \
    lit "${S}" '  found=$((found + 1))' '  found=$((found + 1)); return 0'
  judge "the title not trimmed (V-3)" "V-3a: FAIL lines are [FAIL: GO-2026-6888     Excessive" \
    lit "${S}" '      title="${title#"${title%%[![:space:]]*}"}"' '      :'
  judge "the output swallowed (V-3)" "V-3a: the output does not begin with the whole task vulncheck report" \
    lit "${S}" 'task vulncheck 2>&1 | tee "${out}"' 'task vulncheck >"${out}" 2>&1'
  judge "a FAIL line on a clean scan (V-3)" "V-3c: FAIL lines are [FAIL: task vulncheck exited 0" \
    lit "${S}" 'if [[ "${status}" -ne 0 ]]; then' 'if true; then'
  judge "a whitespace-only line taken as the title (V-3e)" "V-3e: FAIL lines are [FAIL: GO-2099-0001|FAIL: GO-2099-0002|" \
    lit "${S}" '"${id}" && "${line}" =~ [^[:space:]] ]]' '"${id}" && -n "${line}" ]]'
  judge "trailing blanks kept (V-3e)" "V-3e: FAIL lines are [FAIL: GO-2099-0001 Title after a whitespace-only line " \
    lit "${S}" '      title="${title%"${title##*[![:space:]]}"}"' '      :'
  judge "an entry followed by the next one dropped (V-3e)" "V-3e: FAIL lines are [FAIL: GO-2099-0001 Title after a whitespace-only line|FAIL: GO-2099-0003]" \
    lit "${S}" '      [[ -z "${id}" ]] || report "${id}" ""' '      :'
  judge "an entry at the end of the output dropped (V-3e)" "V-3e: FAIL lines are [FAIL: GO-2099-0001 Title after a whitespace-only line|FAIL: GO-2099-0002]" \
    lit "${S}" "${nl}"'  [[ -z "${id}" ]] || report "${id}" ""' "${nl}"'  :'
  judge "task called with --exit-code (V-3)" "V-3d: V-3a: task was called with [--exit-code|vulncheck]" \
    lit "${S}" 'task vulncheck 2>&1 | tee "${out}"' 'task --exit-code vulncheck 2>&1 | tee "${out}"'
}

if ((SBAD > 0)); then
  echo "${SBAD} self-test check(s) failed" >&2
  exit 1
fi
echo "All vulncheck-schedule self-tests passed."
