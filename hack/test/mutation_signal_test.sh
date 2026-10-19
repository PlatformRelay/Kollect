#!/usr/bin/env bash
# The nightly mutation measurement reports and never gates, and cannot go blind (MUT-1 to MUT-4,
# openspec change mutation-testing-signal).
#
# hack/ci/mutation.sh (`task mutation`, nightly in mutation.yaml) runs gremlins on a fixture
# module (hack/test/testdata/mutation/, its control mutant must be LIVED), then once per package on
# internal/aggregate, internal/redact and internal/pathvalidate, and hands the reports to
# hack/ci/mutation-summary.sh, which writes five counts per package to the job summary. This test
# never installs or runs gremlins: the runner cases pass a fake gremlins that records each call and
# serves the reports recorded by task 1.1 (hack/test/testdata/mutation/reports/, provenance in the
# README there); everything else reads files.
#
# What it locks (tasks.md 2.1-2.6):
#   M1-M9  the formatter: exact counts per status and package; survivors stay green; a missing,
#          empty, unreadable, shapeless, mutation-less, non-measurement, measured-nothing (no
#          mutant killed or lived) or misattributed report,
#          and a missing, repeated or unknown package, fail with an ::error::mutation: line and
#          no counts table;
#   R1-R9  the runner: four calls in order with the exact argv (--timeout-coefficient=10 in it),
#          a fresh report directory and GOFLAGS=-count=1; a gremlins failure or a missing or
#          stale report fails; the fixture control must be LIVED exactly once;
#          MUTATION_FORCE_FAILURE can only fail the run; any GREMLINS_ variable is refused; a tool
#          failure never reads like a night with survivors; the report directory reaches
#          $GITHUB_OUTPUT before the first call and is kept; the pinned binary's mutator flags
#          are exactly the ones passed;
#   W1-W3  mutation.yaml's shape, the Taskfile install chain, and no install or run of gremlins
#          (or reach of the mutation task) anywhere else in the scanned tree;
#   T1-T3  no threshold on a command line, in a config file or in the environment;
#   C1     ci-failure-report.yaml lists `Mutation testing`, the workflow's name:;
#   C3     a file that does not parse, cannot be read, or a tree git cannot list is a scan error,
#          never "nothing found".
# Every failure is collected before the exit (FAIL: <case>: lines), so one defect never hides
# another's message.
#
# Usage:
#   hack/test/mutation_signal_test.sh              # check the repository tree
#   hack/test/mutation_signal_test.sh --self-test  # mutate copies, assert each mutant reds
#   hack/test/mutation_signal_test.sh <tree-root>  # check another tree root
# MUTATION_SIGNAL_ONLY (set by --self-test only, never in CI): an anchored regex over case ids;
# cases it does not name are skipped, so one mutant runs only the cases that judge it.
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

for tool in yq jq git awk; do
  command -v "${tool}" >/dev/null 2>&1 || fail "${tool} not found; install it to run this test"
done

ONLY="${MUTATION_SIGNAL_ONLY:-}"
want() { [[ -z "${ONLY}" || "$1" =~ ^(${ONLY})$ ]]; }

SCRATCH=""
trap 'rm -rf ${SCRATCH} 2>/dev/null' EXIT

# Called directly (never as $(new_tmp)), so SCRATCH survives in this shell for the EXIT trap.
TMP=""
new_tmp() {
  TMP="$(mktemp -d)"
  SCRATCH="${SCRATCH} ${TMP}"
}

SUMMARY_SH="${ROOT}/hack/ci/mutation-summary.sh"
RUNNER="${ROOT}/hack/ci/mutation.sh"
WF_REL=.github/workflows/mutation.yaml
WF="${ROOT}/${WF_REL}"
TF="${ROOT}/Taskfile.yml"
CFR_REL=.github/workflows/ci-failure-report.yaml
FX="${ROOT}/hack/test/testdata/mutation"
REP="${FX}/reports"

# The eleven mutator flags of gremlins v0.6.0 (`gremlins unleash --help`, task 1.1), in help order.
FLAGS=(--arithmetic-base --conditionals-boundary --conditionals-negation --invert-assignments
  --invert-bitwise --invert-bwassign --increment-decrement --invert-logical --invert-loopctrl
  --invert-negatives --remove-self-assignments)
# Stage-B review B1: at gremlins' default coefficient (3) a loaded runner times mutants out
# before their tests finish (4 of 6 local runs had TIMED OUT mutants, 0 of 6 at 10).
TIMEOUT_FLAG=--timeout-coefficient=10
PKGS=(internal/aggregate internal/redact internal/pathvalidate)
# The control mutant (task 1.1): control.go:9:11 ARITHMETIC_BASE (the fixture's only file).
CTRL='.line == 9 and .column == 11 and .type == "ARITHMETIC_BASE"'
# The Taskfile lines W2 pins.
INSTALL_LINE='GOBIN={{.ROOT_DIR}}/bin/gremlins-{{.GREMLINS_VERSION}} go install github.com/go-gremlins/gremlins/cmd/gremlins@{{.GREMLINS_VERSION}}'
RUN_LINE='bash hack/ci/mutation.sh {{.ROOT_DIR}}/bin/gremlins-{{.GREMLINS_VERSION}}/gremlins'
ANNOTATION='  # renovate: datasource=go depName=github.com/go-gremlins/gremlins'

if [[ "${MODE}" == "check" ]]; then
  BAD=0
  CASE_BAD=0
  SELECTED=0
  case_fail() {
    echo "FAIL: $1: $2" >&2
    BAD=$((BAD + 1))
  }
  start_case() {
    CASE_BAD="${BAD}"
    SELECTED=$((SELECTED + 1))
  }
  done_case() { # <label>: ok only when the case added no failure since it started
    [[ "${BAD}" -ne "${CASE_BAD}" ]] || pass "$1"
  }

  # oracle <report>: the five counts of a report, counted here from its statuses.
  oracle() {
    jq -r '[.files[].mutations[].status] as $s
      | ["KILLED", "LIVED", "NOT COVERED", "TIMED OUT", "NOT VIABLE"]
      | map(. as $k | $s | map(select(. == $k)) | length | tostring) | join(" | ")' "$1"
  }

  # ===========================================================================================
  # Formatter (M1-M9): hack/ci/mutation-summary.sh <package>=<report>...
  # ===========================================================================================
  # run_fmt <summary:0|1> <arg>...: FRC, FD (out, err, summary). With <summary> 1 the table goes
  # to $GITHUB_STEP_SUMMARY, with 0 to stdout; TABLE is both.
  run_fmt() {
    local to_summary="$1"
    shift
    new_tmp
    FD="${TMP}"
    : >"${FD}/summary"
    local -a envs=(env -i "PATH=${PATH}" "HOME=${HOME:-/tmp}")
    ((to_summary)) && envs+=("GITHUB_STEP_SUMMARY=${FD}/summary")
    if [[ -f "${SUMMARY_SH}" ]]; then
      (cd "${FD}" && "${envs[@]}" bash "${SUMMARY_SH}" "$@") >"${FD}/out" 2>"${FD}/err" && FRC=0 || FRC=$?
    else
      echo "absence: hack/ci/mutation-summary.sh is missing" >"${FD}/err"
      : >"${FD}/out"
      FRC=127
    fi
    TABLE="$(cat "${FD}/out" "${FD}/summary")"
  }
  # row_ok <case> <package> <report>: exactly one table row for <package>, holding the oracle's
  # five counts of <report>.
  row_ok() {
    local want_row n
    want_row="| $2 | $(oracle "$3") |"
    n="$(grep -cFx -- "${want_row}" <<<"${TABLE}" || true)"
    [[ "${n}" -eq 1 ]] ||
      case_fail "$1" "want exactly one row '${want_row}', found ${n}; rows: $(grep -F "| $2 |" <<<"${TABLE}" | tr '\n' ' ' || true)"
  }
  rows_n() { grep -cE '^\| internal/' <<<"${TABLE}" || true; }
  # failed_shape <case> <package> [<text>]: non-zero, an ::error::mutation: line naming the
  # package (and <text>), and no counts table.
  failed_shape() {
    local c="$1" p="$2" t="${3:-}"
    [[ "${FRC}" -ne 0 ]] || case_fail "${c}" "the formatter exited 0, want non-zero"
    grep '^::error::mutation:' "${FD}/err" | grep -F -- "${p}" | grep -F -- "${t}" >/dev/null ||
      case_fail "${c}" "no '::error::mutation:' line naming '${p}'${t:+ and '${t}'}; stderr: $(head -c 300 "${FD}/err" | tr '\n' '|')"
    ! grep -E '^\| (Package|internal/)' <<<"${TABLE}" >/dev/null ||
      case_fail "${c}" "a counts table was printed although a report is invalid: $(grep -E '^\| ' <<<"${TABLE}" | head -2 | tr '\n' ' ')"
  }

  # Synthetic reports, each file_name real in its package directory.
  new_tmp
  SYN="${TMP}"
  # Each status a different number of times, so a column swap or a fold shows as a wrong count.
  jq -n '{go_module: "github.com/platformrelay/kollect", files: [{file_name: "redact.go", mutations: (
      [range(1)  | {type: "ARITHMETIC_BASE", status: "KILLED", line: 1, column: 1}] +
      [range(2)  | {type: "ARITHMETIC_BASE", status: "LIVED", line: 2, column: 1}] +
      [range(3)  | {type: "ARITHMETIC_BASE", status: "NOT COVERED", line: 3, column: 1}] +
      [range(4)  | {type: "ARITHMETIC_BASE", status: "TIMED OUT", line: 4, column: 1}] +
      [range(5)  | {type: "ARITHMETIC_BASE", status: "NOT VIABLE", line: 5, column: 1}])}]}' >"${SYN}/all-statuses.json"
  : >"${SYN}/empty.json"
  printf '{"go_module": "x", "files": [' >"${SYN}/invalid.json"
  printf '{"go_module": "github.com/platformrelay/kollect"}\n' >"${SYN}/no-files.json"
  printf '{"go_module": "github.com/platformrelay/kollect", "files": []}\n' >"${SYN}/zero.json"
  jq '.files[0].mutations[0].status = "SKIPPED"' "${REP}/pathvalidate.json" >"${SYN}/skipped.json"
  jq '.files[0].mutations[0].status = "FROZEN"' "${REP}/pathvalidate.json" >"${SYN}/unknown.json"
  jq '.files[].mutations[].status = "NOT COVERED"' "${REP}/pathvalidate.json" >"${SYN}/not-covered.json"
  jq '.files[].mutations |= [range(length) as $i | .[$i] | .status = (["TIMED OUT", "NOT VIABLE", "NOT COVERED"][$i % 3])]' \
    "${REP}/pathvalidate.json" >"${SYN}/mixed.json"
  AG="internal/aggregate=${REP}/aggregate.json"
  RE="internal/redact=${REP}/redact.json"
  PV="internal/pathvalidate=${REP}/pathvalidate.json"

  if want M1; then
    start_case
    run_fmt 1 "${AG}" "${RE}" "${PV}"
    [[ "${FRC}" -eq 0 ]] || case_fail M1 "the formatter exited ${FRC} on the three recorded reports; stderr: $(head -c 300 "${FD}/err" | tr '\n' '|')"
    row_ok M1 internal/aggregate "${REP}/aggregate.json"
    row_ok M1 internal/redact "${REP}/redact.json"
    row_ok M1 internal/pathvalidate "${REP}/pathvalidate.json"
    [[ "$(rows_n)" -eq 3 ]] || case_fail M1 "want 3 package rows, found $(rows_n)"
    [[ ! -s "${FD}/out" ]] || ! grep -E '^\| internal/' "${FD}/out" >/dev/null ||
      case_fail M1 "with GITHUB_STEP_SUMMARY set the table went to stdout"
    run_fmt 0 "${AG}" "internal/redact=${SYN}/all-statuses.json" "${PV}"
    [[ "${FRC}" -eq 0 ]] || case_fail M1 "the formatter exited ${FRC} on a report holding every status; stderr: $(head -c 300 "${FD}/err" | tr '\n' '|')"
    row_ok M1 internal/redact "${SYN}/all-statuses.json"
    grep -E '^\| internal/redact \|' "${FD}/out" >/dev/null ||
      case_fail M1 "without GITHUB_STEP_SUMMARY the table is not on stdout"
    done_case "M1 counts per package and status equal the reports' statuses"
  fi

  if want M2; then
    start_case
    run_fmt 1 "${AG}" "${RE}" "${PV}"
    [[ "${FRC}" -eq 0 ]] || case_fail M2 "surviving mutants failed the formatter (exit ${FRC}): a score gate"
    [[ "$(jq '[.files[].mutations[] | select(.status == "LIVED")] | length' "${REP}/aggregate.json")" -gt 0 ]] ||
      case_fail M2 "the recorded aggregate report has no LIVED mutant to show"
    row_ok M2 internal/aggregate "${REP}/aggregate.json"
    done_case "M2 survivors are shown and stay green"
  fi

  if want M3; then
    for bad in missing empty invalid no-files; do
      start_case
      run_fmt 1 "${AG}" "${RE}" "internal/pathvalidate=${SYN}/${bad}.json"
      failed_shape M3 internal/pathvalidate
      done_case "M3 a ${bad} report (last of three) fails with no partial table"
    done
  fi

  if want M4; then
    start_case
    run_fmt 1 "${AG}" "${RE}" "internal/pathvalidate=${SYN}/zero.json"
    failed_shape M4 internal/pathvalidate "no mutation: nothing was measured"
    done_case "M4 a report with zero mutations fails"
  fi

  if want M5; then
    for st in RUNNABLE SKIPPED FROZEN; do
      start_case
      case "${st}" in
        RUNNABLE) r="${REP}/dryrun-pathvalidate.json" ;;
        SKIPPED) r="${SYN}/skipped.json" ;;
        *) r="${SYN}/unknown.json" ;;
      esac
      run_fmt 1 "${AG}" "${RE}" "internal/pathvalidate=${r}"
      failed_shape M5 internal/pathvalidate "${st}"
      done_case "M5 status ${st} is not a measurement"
    done
  fi

  if want M6; then
    start_case
    run_fmt 1 "${AG}" "${RE}" "internal/pathvalidate=${REP}/redact.json"
    failed_shape M6 internal/pathvalidate redact.go
    done_case "M6 a report whose files are not in its package fails"
  fi

  if want M8; then
    start_case
    run_fmt 1 "${AG}" "${RE}"
    failed_shape M8 internal/pathvalidate
    done_case "M8 a missing package fails"
    start_case
    run_fmt 1 "${AG}" "${AG}" "${RE}" "${PV}"
    failed_shape M8 internal/aggregate
    done_case "M8 a package given twice fails"
    start_case
    run_fmt 1 "${AG}" "${RE}" "${PV}" "internal/collect=${REP}/redact.json"
    failed_shape M8 internal/collect
    done_case "M8 an unknown package fails"
  fi

  # M9 (stage-B review, B1): a package none of whose mutants was killed or lived measured
  # nothing. The recorded warm-cache report (every mutant TIMED OUT, task 1.1 F1) is the real
  # shape; a loaded runner produced the same for internal/aggregate in 2 of 7 local runs.
  if want M9; then
    for bad in warm-cache not-covered mixed; do
      start_case
      case "${bad}" in
        warm-cache) r="${REP}/warm-cache-pathvalidate.json" ;;
        *) r="${SYN}/${bad}.json" ;;
      esac
      run_fmt 1 "${AG}" "${RE}" "internal/pathvalidate=${r}"
      failed_shape M9 internal/pathvalidate "no mutant was killed or lived"
      done_case "M9 a ${bad} report with no killed or lived mutant fails"
    done
  fi

  # ===========================================================================================
  # Runner (R1-R9): hack/ci/mutation.sh <gremlins-bin>, with a fake gremlins.
  # ===========================================================================================
  # Globals a case sets before run_runner: SRC (directory of <short>.json reports the fake
  # serves), FAIL_CALL / NOWRITE_CALL (call number that exits 1 / writes nothing; with FAIL_WRITE 1
# the failing call writes its report first, as a gated gremlins does), XENV (array of
  # extra NAME=VALUE), GHOUT (1: set GITHUB_OUTPUT), KEEP_TMPDIR (a TMPDIR to reuse), HELP (the
  # text `unleash --help` prints; the recorded v0.6.0 help by default), HELP_FAIL (1: it exits 1).
  # Results: RC, RD (case dir: out, err, summary, ghout, help, calls/N.{argv,cwd,goflags,ghout}),
  # NCALL (unleash calls; the help query is not one), RTMP (the TMPDIR the runner saw).
  reset_runner() {
    SRC="${REP}" FAIL_CALL=0 FAIL_WRITE=0 NOWRITE_CALL=0 GHOUT=0 KEEP_TMPDIR=""
    HELP="${FX}/unleash-help.txt" HELP_FAIL=0
    XENV=()
  }
  reset_runner
  run_runner() {
    new_tmp
    RD="${TMP}"
    mkdir -p "${RD}/bin" "${RD}/calls"
    RTMP="${KEEP_TMPDIR:-${RD}/tmp}"
    mkdir -p "${RTMP}"
    echo 0 >"${RD}/n"
    : >"${RD}/summary"
    cat >"${RD}/bin/gremlins" <<'FAKE'
#!/usr/bin/env bash
# Fake gremlins: records argv, working directory, GOFLAGS and $GITHUB_OUTPUT per call; fails or
# writes nothing on a named call; otherwise copies the served report named like --output's
# basename to --output, and like the real one writes nothing when its directory is missing.
# `unleash --help` prints the served help (or fails) and records how many unleash calls came
# before it; it is not counted as a call.
d="${FAKE_DIR:?}"
if [[ "$*" == "unleash --help" ]]; then
  cat "${d}/n" >>"${d}/help"
  if [[ "${FAKE_HELP_FAIL:-0}" == 1 ]]; then
    echo "Error: unknown command" >&2
    exit 1
  fi
  cat "${FAKE_HELP:?}"
  exit 0
fi
n=$(($(cat "${d}/n") + 1))
echo "${n}" >"${d}/n"
printf '%s\n' "$@" >"${d}/calls/${n}.argv"
pwd -P >"${d}/calls/${n}.cwd"
printf '%s' "${GOFLAGS-<unset>}" >"${d}/calls/${n}.goflags"
env | grep '^GREMLINS_' >"${d}/calls/${n}.genv" || true
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then cat "${GITHUB_OUTPUT}" >"${d}/calls/${n}.ghout" 2>/dev/null || true; fi
out=""
for a in "$@"; do
  case "${a}" in --output=*) out="${a#--output=}" ;; esac
done
if [[ "${FAKE_FAIL_CALL:-0}" == "${n}" ]]; then
  if [[ "${FAKE_FAIL_WRITE:-0}" == 1 && -n "${out}" ]]; then
    cp "${FAKE_SRC:?}/$(basename "${out}")" "${out}" 2>/dev/null || true
  fi
  echo "# example.com/broken [example.com/broken.test]" >&2
  echo "./broken.go:3:1: cannot use \"x\" (untyped string constant) as int value in return statement" >&2
  echo "ERROR: failed to gather coverage: impossible to executeCoverage coverage: exit status 1" >&2
  exit 1
fi
[[ "${FAKE_NOWRITE_CALL:-0}" != "${n}" ]] || exit 0
[[ -n "${out}" ]] || exit 0
cp "${FAKE_SRC:?}/$(basename "${out}")" "${out}" 2>/dev/null || true
exit 0
FAKE
    chmod +x "${RD}/bin/gremlins"
    local -a envs=(env -i "PATH=${PATH}" "HOME=${HOME:-/tmp}" "TMPDIR=${RTMP}" "FAKE_DIR=${RD}"
      "FAKE_SRC=${SRC}" "FAKE_FAIL_CALL=${FAIL_CALL}" "FAKE_FAIL_WRITE=${FAIL_WRITE}" "FAKE_NOWRITE_CALL=${NOWRITE_CALL}"
      "FAKE_HELP=${HELP}" "FAKE_HELP_FAIL=${HELP_FAIL}" "GITHUB_STEP_SUMMARY=${RD}/summary")
    ((GHOUT)) && {
      : >"${RD}/ghout"
      envs+=("GITHUB_OUTPUT=${RD}/ghout")
    }
    ((${#XENV[@]} == 0)) || envs+=("${XENV[@]}")
    if [[ -f "${RUNNER}" ]]; then
      (cd "${RD}" && "${envs[@]}" bash "${RUNNER}" "${RD}/bin/gremlins") >"${RD}/out" 2>"${RD}/err" && RC=0 || RC=$?
    else
      echo "absence: hack/ci/mutation.sh is missing" >"${RD}/err"
      : >"${RD}/out"
      RC=127
    fi
    NCALL="$(cat "${RD}/n")"
    reset_runner
  }
  r_err_has() { # <case> <text>: an ::error::mutation: line of the runner holds <text>
    grep '^::error::mutation:' "${RD}/err" | grep -F -- "$2" >/dev/null ||
      case_fail "$1" "no '::error::mutation:' line holding '$2'; stderr: $(head -c 300 "${RD}/err" | tr '\n' '|')"
  }
  r_no_table() { # <case>
    ! grep -hE '^\| (Package|internal/)' "${RD}/out" "${RD}/summary" >/dev/null ||
      case_fail "$1" "a counts table was printed on a failed run"
  }
  r_table() { # <case>: the green run's summary has a row per package
    local p
    for p in "${PKGS[@]}"; do
      grep -E "^\| ${p} \|" "${RD}/summary" >/dev/null ||
        case_fail "$1" "\$GITHUB_STEP_SUMMARY has no row for ${p}: $(head -c 200 "${RD}/summary" | tr '\n' '|')"
    done
  }
  # out_dir_of <call>: the directory of that call's --output.
  out_dir_of() {
    local o
    o="$(sed -n 's/^--output=//p' "${RD}/calls/$1.argv" 2>/dev/null | head -1 || true)"
    [[ -n "${o}" ]] && dirname "${o}"
    return 0
  }

  if want R1; then
    start_case
    XENV=("GOFLAGS=-tags=narrowed")
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R1 "the runner exited ${RC} on the recorded reports; stderr: $(head -c 300 "${RD}/err" | tr '\n' '|')"
    [[ "${NCALL}" -eq 4 ]] || case_fail R1 "gremlins was called ${NCALL} times, want 4 (fixture, then three packages)"
    dir="$(out_dir_of 1)"
    if [[ -z "${dir}" ]]; then
      case_fail R1 "call 1 has no --output=<file>"
    elif [[ "${dir}" != "${RTMP}"/* ]]; then
      case_fail R1 "the report directory ${dir} is not created fresh under the run's TMPDIR ${RTMP}"
    fi
    i=0
    for spec in "fixture ${FX} ." "aggregate ${ROOT} ./internal/aggregate" \
      "redact ${ROOT} ./internal/redact" "pathvalidate ${ROOT} ./internal/pathvalidate"; do
      i=$((i + 1))
      read -r short cwd pkg <<<"${spec}"
      want_argv="$(printf '%s\n' unleash "--config=${ROOT}/hack/ci/gremlins.yaml" \
        "--output=${dir}/${short}.json" "${TIMEOUT_FLAG}" "${FLAGS[@]}" "${pkg}")"
      got_argv="$(cat "${RD}/calls/${i}.argv" 2>/dev/null || echo '<no call>')"
      [[ "${got_argv}" == "${want_argv}" ]] ||
        case_fail R1 "call ${i} (${short}) argv is '$(tr '\n' ' ' <<<"${got_argv}")', want '$(tr '\n' ' ' <<<"${want_argv}")'"
      got_cwd="$(cat "${RD}/calls/${i}.cwd" 2>/dev/null || echo '<no call>')"
      [[ "${got_cwd}" == "${cwd}" ]] || case_fail R1 "call ${i} (${short}) ran in ${got_cwd}, want ${cwd}"
      [[ -f "${RD}/calls/${i}.genv" && ! -s "${RD}/calls/${i}.genv" ]] ||
        case_fail R1 "call ${i} (${short}) saw GREMLINS_ variables '$(tr '\n' ' ' <"${RD}/calls/${i}.genv" 2>/dev/null || echo '<no call>')', want none (gremlins reads them as configuration)"
      got_gf="$(cat "${RD}/calls/${i}.goflags" 2>/dev/null || echo '<no call>')"
      [[ "${got_gf}" == "-count=1" ]] ||
        case_fail R1 "call ${i} (${short}) saw GOFLAGS='${got_gf}', want exactly -count=1 (a cached coverage run times every mutant out, task 1.1 F1)"
    done
    r_table R1
    first_dir="${dir}"
    run_runner
    [[ "$(out_dir_of 1)" != "${first_dir}" ]] || case_fail R1 "two runs used the same report directory ${first_dir}"
    done_case "R1 four calls in order, exact argv, fresh report directory, GOFLAGS=-count=1"
  fi

  if want R2; then
    start_case
    # The failing call writes its report, so only the exit status tells the runner it failed.
    FAIL_CALL=3 FAIL_WRITE=1
    run_runner
    [[ "${RC}" -ne 0 ]] || case_fail R2 "gremlins exited 1 on ./internal/redact but the runner exited 0"
    [[ "${NCALL}" -eq 3 ]] || case_fail R2 "gremlins was called ${NCALL} times, want 3 (no call after the failure)"
    r_err_has R2 "gremlins failed on ./internal/redact"
    r_no_table R2
    done_case "R2 a failing gremlins call fails the run and stops it"
  fi

  if want R3; then
    start_case
    NOWRITE_CALL=2
    run_runner
    [[ "${RC}" -ne 0 ]] || case_fail R3 "gremlins wrote no report for ./internal/aggregate but the runner exited 0"
    r_err_has R3 "no report for ./internal/aggregate"
    [[ "${NCALL}" -eq 2 ]] || case_fail R3 "gremlins was called ${NCALL} times, want 2 (no call after the missing report)"
    r_no_table R3
    done_case "R3 a missing report fails the run"
    # Stale: a green run leaves its reports; the next run, whose call 2 writes nothing, must not
    # find the old one.
    start_case
    new_tmp
    shared="${TMP}"
    KEEP_TMPDIR="${shared}"
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R3 "the first (green) run exited ${RC}"
    KEEP_TMPDIR="${shared}" NOWRITE_CALL=2
    run_runner
    [[ "${RC}" -ne 0 ]] || case_fail R3 "a stale report from the previous run stood in for a missing one (exit 0)"
    r_err_has R3 "no report for ./internal/aggregate"
    done_case "R3 a stale report from a previous run does not stand in"
  fi

  if want R4; then
    new_tmp
    VAR="${TMP}"
    for v in lived not-covered killed twice absent; do
      mkdir -p "${VAR}/${v}"
      cp "${REP}"/{aggregate,redact,pathvalidate}.json "${VAR}/${v}/"
    done
    cp "${REP}/fixture.json" "${VAR}/lived/fixture.json"
    jq "(.files[].mutations[] | select(${CTRL}) | .status) = \"NOT COVERED\"" \
      "${REP}/fixture.json" >"${VAR}/not-covered/fixture.json"
    jq "(.files[].mutations[] | select(${CTRL}) | .status) = \"KILLED\"" \
      "${REP}/fixture.json" >"${VAR}/killed/fixture.json"
    jq "(.files[0].mutations) += [.files[0].mutations[] | select(${CTRL})]" \
      "${REP}/fixture.json" >"${VAR}/twice/fixture.json"
    # Absent: the control gone, while another mutant (Unused's) is LIVED.
    jq "(.files[0].mutations) |= (map(select((${CTRL}) | not)) | .[0].status = \"LIVED\")" \
      "${REP}/fixture.json" >"${VAR}/absent/fixture.json"
    start_case
    SRC="${VAR}/lived"
    run_runner
    [[ "${RC}" -eq 0 && "${NCALL}" -eq 4 ]] ||
      case_fail R4 "with the control LIVED the run did not continue to the packages (exit ${RC}, ${NCALL} calls)"
    done_case "R4 control LIVED: the run continues"
    for v in not-covered killed twice absent; do
      start_case
      case "${v}" in
        not-covered) t="NOT COVERED" ;;
        killed) t="KILLED" ;;
        twice) t="2 times" ;;
        *) t="absent" ;;
      esac
      SRC="${VAR}/${v}"
      run_runner
      [[ "${RC}" -ne 0 ]] || case_fail R4 "control ${v}: the run exited 0"
      [[ "${NCALL}" -eq 1 ]] || case_fail R4 "control ${v}: gremlins was called ${NCALL} times, want 1 (no real package)"
      r_err_has R4 "${t}"
      r_no_table R4
      done_case "R4 control ${v} fails before the packages"
    done
  fi

  if want R5; then
    start_case
    XENV=("MUTATION_FORCE_FAILURE=true")
    run_runner
    [[ "${RC}" -ne 0 ]] || case_fail R5 "MUTATION_FORCE_FAILURE=true did not fail the run"
    [[ "${NCALL}" -eq 0 ]] || case_fail R5 "MUTATION_FORCE_FAILURE=true still called gremlins ${NCALL} times"
    r_err_has R5 "forced failure"
    done_case "R5 MUTATION_FORCE_FAILURE=true fails the run, no gremlins call"
    for v in unset empty false; do
      start_case
      case "${v}" in
        empty) XENV=("MUTATION_FORCE_FAILURE=") ;;
        false) XENV=("MUTATION_FORCE_FAILURE=false") ;;
      esac
      run_runner
      [[ "${RC}" -eq 0 && "${NCALL}" -eq 4 ]] ||
        case_fail R5 "MUTATION_FORCE_FAILURE ${v}: the run changed (exit ${RC}, ${NCALL} calls, want 0 and 4)"
      r_table R5
      done_case "R5 MUTATION_FORCE_FAILURE ${v}: the run is unchanged"
    done
  fi

  if want R6; then
    for kv in GREMLINS_UNLEASH_DIFF=HEAD GREMLINS_UNLEASH_TAGS=x GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED=true \
      GREMLINS_UNLEASH_THRESHOLD_EFFICACY=100 GREMLINS_KOLLECT_PROBE=1 GREMLINS_UNLEASH_TAGS=; do
      start_case
      XENV=("${kv}")
      run_runner
      [[ "${RC}" -ne 0 ]] || case_fail R6 "${kv}: the runner started (exit 0)"
      [[ "${NCALL}" -eq 0 ]] || case_fail R6 "${kv}: gremlins was called ${NCALL} times, want 0"
      r_err_has R6 "${kv%%=*}"
      done_case "R6 ${kv} is refused before any gremlins call"
    done
  fi

  if want R7; then
    new_tmp
    N3="${TMP}"
    cp "${REP}"/{fixture,aggregate,pathvalidate}.json "${N3}/"
    jq '.files[0].mutations[0].status = "LIVED"' "${REP}/redact.json" >"${N3}/redact.json"
    start_case
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R7 "night 1 exited ${RC}, want 0"
    grep -Fx -- "| internal/redact | $(oracle "${REP}/redact.json") |" "${RD}/summary" >/dev/null ||
      case_fail R7 "night 1's summary does not hold night 1's redact counts"
    FAIL_CALL=2
    run_runner
    [[ "${RC}" -ne 0 ]] || case_fail R7 "night 2 (gremlins build failure) exited 0"
    r_err_has R7 "gremlins failed"
    r_no_table R7
    SRC="${N3}"
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R7 "night 3 exited ${RC}, want 0"
    grep -Fx -- "| internal/redact | $(oracle "${N3}/redact.json") |" "${RD}/summary" >/dev/null ||
      case_fail R7 "night 3's summary does not hold night 3's own redact counts: $(grep -F 'internal/redact' "${RD}/summary" || true)"
    done_case "R7 night 1 ok, night 2 tool failure (red, no table), night 3 ok with its own counts"
  fi

  # R9 (stage-B review, B2): the runner reads the pinned binary's mutator flags before the first
  # unleash call and fails unless they are exactly the ones it passes, so a bump that adds a
  # mutator (off by default) cannot leave it silently disabled, and one that drops a flag names it.
  if want R9; then
    start_case
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R9 "the runner exited ${RC} on the recorded v0.6.0 help; stderr: $(head -c 300 "${RD}/err" | tr '\n' '|')"
    [[ "$(cat "${RD}/help" 2>/dev/null)" == 0 ]] ||
      case_fail R9 "'unleash --help' was queried after [$(tr '\n' ' ' 2>/dev/null <"${RD}/help" || true)] unleash calls, want exactly once, before the first"
    done_case "R9 the pinned gremlins' mutator flags are read before the first call"
    for v in extra missing none fail; do
      start_case
      new_tmp
      h="${TMP}/help"
      case "${v}" in
        extra)
          sed 's/^  -h, --help /      --invert-everything           enable "INVERT_EVERYTHING" mutants\n&/' \
            "${FX}/unleash-help.txt" >"${h}"
          want_msg=--invert-everything
          ;;
        missing)
          grep -v -- '--remove-self-assignments' "${FX}/unleash-help.txt" >"${h}" || true
          want_msg=--remove-self-assignments
          ;;
        none)
          grep -v 'enable "' "${FX}/unleash-help.txt" >"${h}" || true
          want_msg="lists no mutator flag"
          ;;
        fail)
          cp "${FX}/unleash-help.txt" "${h}"
          HELP_FAIL=1
          want_msg="unleash --help' failed"
          ;;
      esac
      [[ "${v}" != extra ]] || grep -F -- '--invert-everything' "${h}" >/dev/null ||
        case_fail R9 "the extra-mutator help was not built"
      HELP="${h}"
      run_runner
      [[ "${RC}" -ne 0 ]] || case_fail R9 "help ${v}: the run exited 0"
      [[ "${NCALL}" -eq 0 ]] || case_fail R9 "help ${v}: gremlins unleash was called ${NCALL} times, want 0 (fail before measuring)"
      r_err_has R9 "${want_msg}"
      r_no_table R9
      done_case "R9 help ${v}: the run fails before measuring, naming it"
    done
  fi

  if want R8; then
    start_case
    GHOUT=1
    run_runner
    [[ "${RC}" -eq 0 ]] || case_fail R8 "the run exited ${RC}"
    dir="$(out_dir_of 1)"
    first_out="$(cat "${RD}/calls/1.ghout" 2>/dev/null || echo '<no call or no GITHUB_OUTPUT>')"
    [[ -n "${dir}" && "${first_out}" == "reports=${dir}" ]] ||
      case_fail R8 "before the first gremlins call \$GITHUB_OUTPUT held '${first_out}', want 'reports=${dir}'"
    for s in fixture aggregate redact pathvalidate; do
      [[ -n "${dir}" && -s "${dir}/${s}.json" ]] ||
        case_fail R8 "after the green run ${dir:-<no dir>}/${s}.json is gone: the upload step would find nothing"
    done
    done_case "R8 reports=<dir> reaches \$GITHUB_OUTPUT first, and the four reports are kept"
  fi

  # ===========================================================================================
  # Static (W1-W3, T1-T3, C1, C3). YAML is read once per file through yq with anchors expanded
  # (explode), then queried as JSON; a file that does not parse is a scan error (C3).
  # ===========================================================================================
  STATIC=0
  for c in W1 W2 W3 T1 T2 T3 C1 C3; do want "${c}" && STATIC=1; done
  if ((STATIC)); then
    new_tmp
    SJ="${TMP}"
    SCAN_ERR=0
    scan_error() {
      SCAN_ERR=1
      case_fail C3 "scan error: $*"
    }
    # to_json <rel> <out>: 0 parsed, 1 missing, 2 scan error.
    to_json() {
      [[ -f "${ROOT}/$1" ]] || return 1
      if [[ ! -r "${ROOT}/$1" ]] || ! yq eval -o=json -I=0 'explode(.)' "${ROOT}/$1" >"$2" 2>"${SJ}/yq.err"; then
        scan_error "$1 does not parse as YAML ($(head -c 200 "${SJ}/yq.err" | tr '\n' ' '))"
        return 2
      fi
      return 0
    }
    WF_OK=0 TF_OK=0 CFR_OK=0 GC_OK=0
    to_json "${WF_REL}" "${SJ}/wf.json" && WF_OK=1 || [[ $? -eq 2 ]] || WF_OK=-1
    to_json Taskfile.yml "${SJ}/tf.json" && TF_OK=1 || [[ $? -eq 2 ]] || TF_OK=-1
    to_json "${CFR_REL}" "${SJ}/cfr.json" && CFR_OK=1 || [[ $? -eq 2 ]] || CFR_OK=-1
    to_json hack/ci/gremlins.yaml "${SJ}/gc.json" && GC_OK=1 || [[ $? -eq 2 ]] || GC_OK=-1
    wj() { jq -r "$1" "${SJ}/wf.json"; }
    tj() { jq -r "$1" "${SJ}/tf.json"; }

    # Tracked and untracked-but-not-ignored files: a local .gremlins that git does not yet track
    # would still be read by gremlins.
    top="$(git -C "${ROOT}" rev-parse --show-toplevel 2>/dev/null || true)"
    if [[ -n "${top}" && "$(cd "${top}" && pwd -P)" == "${ROOT}" ]] &&
      git -C "${ROOT}" ls-files -z --cached --others --exclude-standard >"${SJ}/files.z" 2>"${SJ}/git.err"; then
      tr '\0' '\n' <"${SJ}/files.z" | sort -u >"${SJ}/files"
    else
      scan_error "git ls-files failed in ${ROOT} (not the top of a git work tree?): $(head -c 200 "${SJ}/git.err" 2>/dev/null | tr '\n' ' ')"
      : >"${SJ}/files"
    fi
  fi

  # --- W1: mutation.yaml ---------------------------------------------------------------------
  if want W1; then
    start_case
    if [[ "${WF_OK}" -eq -1 ]]; then
      case_fail W1 "absence: ${WF_REL} is missing"
    elif [[ "${WF_OK}" -eq 1 ]]; then
      [[ "$(wj '.name')" == "Mutation testing" ]] || case_fail W1 "name: is '$(wj '.name')', want 'Mutation testing'"
      [[ "$(wj 'keys | join(",")')" == "jobs,name,on,permissions" ]] ||
        case_fail W1 "top-level keys are [$(wj 'keys | join(",")')], want exactly name, on, permissions, jobs (no env:, defaults: or other key)"
      [[ "$(wj '.on | keys | join(",")')" == "schedule,workflow_dispatch" ]] ||
        case_fail W1 "on: is [$(wj '.on | keys? | join(",")')], want exactly schedule and workflow_dispatch"
      [[ "$(wj '.on.schedule | tojson')" == '[{"cron":"30 1 * * *"}]' ]] ||
        case_fail W1 "on.schedule is $(wj '.on.schedule | tojson'), want exactly one cron \"30 1 * * *\""
      [[ "$(wj '.on.workflow_dispatch | (. == null or . == {})')" == true ]] ||
        case_fail W1 "on.workflow_dispatch takes inputs or options: $(wj '.on.workflow_dispatch | tojson')"
      [[ "$(wj '.permissions | tojson')" == '{"contents":"read"}' ]] ||
        case_fail W1 "permissions: is $(wj '.permissions | tojson'), want exactly contents: read"
      [[ "$(wj '.jobs | length')" == 1 ]] || case_fail W1 "want exactly one job, found $(wj '.jobs | length')"
      jk="$(wj '.jobs | to_entries | .[0].value | keys | join(",")')"
      [[ "${jk}" == "name,runs-on,steps,timeout-minutes" || "${jk}" == "runs-on,steps,timeout-minutes" ]] ||
        case_fail W1 "the job's keys are [${jk}], want runs-on, timeout-minutes, steps (and name): no if:, continue-on-error:, permissions:, env: or defaults:"
      [[ "$(wj '.jobs | to_entries | .[0].value."runs-on"')" == ubuntu-latest ]] ||
        case_fail W1 "runs-on is '$(wj '.jobs | to_entries | .[0].value."runs-on"')', want ubuntu-latest"
      [[ "$(wj '.jobs | to_entries | .[0].value."timeout-minutes"')" == 15 ]] ||
        case_fail W1 "timeout-minutes is '$(wj '.jobs | to_entries | .[0].value."timeout-minutes"')', want 15 (task 1.1: gremlins about 9 s, setup and install a few minutes)"
      st() { wj ".jobs | to_entries | .[0].value.steps[$1] | $2"; }
      [[ "$(wj '.jobs | to_entries | .[0].value.steps | length')" == 5 ]] ||
        case_fail W1 "want exactly 5 steps (checkout, go-cache, setup-task, task mutation, upload-artifact), found $(wj '.jobs | to_entries | .[0].value.steps | length')"
      sk() { st "$1" 'keys - ["name"] | join(",")'; }
      [[ "$(sk 0)" == "uses,with" && "$(st 0 '.uses')" =~ ^actions/checkout@[0-9a-f]{40}$ &&
        "$(st 0 '.with | tojson')" == '{"persist-credentials":false}' ]] ||
        case_fail W1 "step 1 must be actions/checkout@<sha> with exactly persist-credentials: false, found $(st 0 'tojson')"
      [[ "$(sk 1)" == "uses" && "$(st 1 '.uses')" == ./.github/actions/go-cache ]] ||
        case_fail W1 "step 2 must be ./.github/actions/go-cache with no inputs, found $(st 1 'tojson')"
      [[ "$(sk 2)" == "uses,with" && "$(st 2 '.uses')" =~ ^go-task/setup-task@[0-9a-f]{40}$ &&
        "$(st 2 '.with | keys | join(",")')" == version ]] ||
        case_fail W1 "step 3 must be go-task/setup-task@<sha> with only version:, found $(st 2 'tojson')"
      # shellcheck disable=SC2016 # the GitHub expressions are expected text
      want_env='{"MUTATION_FORCE_FAILURE":"${{ vars.MUTATION_FORCE_FAILURE }}"}'
      [[ "$(sk 3)" == "env,id,run" && "$(st 3 '.id')" == mutation && "$(st 3 '.run | sub("\\s+$"; "")')" == "task mutation" &&
        "$(st 3 '.env | tojson')" == "${want_env}" ]] ||
        case_fail W1 "step 4 must be id: mutation, run: exactly 'task mutation', env: exactly ${want_env}, nothing else; found $(st 3 'tojson')"
      # shellcheck disable=SC2016
      want_with='{"name":"mutation-reports","path":"${{ steps.mutation.outputs.reports }}","if-no-files-found":"error"}'
      [[ "$(sk 4)" == "if,uses,with" && "$(st 4 '.uses')" =~ ^actions/upload-artifact@[0-9a-f]{40}$ &&
        "$(st 4 '.if')" == "always()" && "$(st 4 '.with | tojson')" == "${want_with}" ]] ||
        case_fail W1 "step 5 must be actions/upload-artifact@<sha>, if: always(), with: exactly ${want_with}; found $(st 4 'tojson')"
      [[ "$(wj '[.. | objects | select(has("continue-on-error") or has("shell") or has("defaults"))] | length')" == 0 ]] ||
        case_fail W1 "continue-on-error:, shell: or defaults: appears in the workflow"
      # Each action at a SHA already used by another workflow, so dependency-review saw it, and
      # setup-task at a version another workflow uses.
      others=()
      for f in "${ROOT}"/.github/workflows/*.yaml "${ROOT}"/.github/workflows/*.yml; do
        [[ -f "${f}" && "${f}" != "${WF}" ]] && others+=("${f}")
      done
      for i in 0 2 4; do
        ref="$(st "${i}" '.uses')"
        ((${#others[@]} > 0)) && grep -hF -- "uses: ${ref}" "${others[@]}" >/dev/null ||
          case_fail W1 "step $((i + 1)) uses ${ref}, which no other workflow under .github/workflows/ uses: a new action version dependency-review never saw"
      done
      ver="$(st 2 '.with.version | tostring')"
      seen=0
      for f in "${others[@]}"; do
        if ! vs="$(yq eval -o=json -I=0 '[explode(.) | .jobs[]?.steps[]? | select((.uses // "") | test("^go-task/setup-task@")) | .with.version | tostring]' "${f}" 2>/dev/null)"; then
          scan_error "${f#"${ROOT}"/} does not parse as YAML"
          continue
        fi
        jq -e --arg v "${ver}" 'index($v) != null' <<<"${vs}" >/dev/null && seen=1
      done
      ((seen)) || case_fail W1 "setup-task version: ${ver} is used by no other workflow"
    fi
    done_case "W1 mutation.yaml: name, triggers, permissions, five pinned steps, nothing that can skip or soften"
  fi

  # --- W2: the Taskfile install chain ------------------------------------------------------------
  if want W2; then
    start_case
    if [[ "${TF_OK}" -eq -1 ]]; then
      case_fail W2 "absence: Taskfile.yml is missing"
    elif [[ "${TF_OK}" -eq 1 ]]; then
      mapfile -t vlines < <(grep -nE '^  GREMLINS_VERSION:' "${TF}" || true)
      if [[ "${#vlines[@]}" -ne 1 ]]; then
        case_fail W2 "Taskfile.yml must have exactly one two-space 'GREMLINS_VERSION:' line, found ${#vlines[@]}"
      else
        n="${vlines[0]%%:*}"
        v="$(sed -E 's/^  GREMLINS_VERSION:[[:space:]]*//; s/[[:space:]]+$//' <<<"${vlines[0]#*:}")"
        [[ "${v}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
          case_fail W2 "GREMLINS_VERSION is '${v}', want a full semver like v1.2.3 (a partial version floats)"
        above="$(sed -n "$((n - 1))p" "${TF}")"
        [[ "${above}" == "${ANNOTATION}" ]] ||
          case_fail W2 "the line above GREMLINS_VERSION is '${above}', want '${ANNOTATION}' (Renovate's Taskfile manager)"
        [[ "$(tj '.vars.GREMLINS_VERSION')" == "${v}" ]] ||
          case_fail W2 "GREMLINS_VERSION is not a top-level vars: entry (vars.GREMLINS_VERSION is '$(tj '.vars.GREMLINS_VERSION')')"
      fi
      [[ "$(tj '.tasks."tools:gremlins" | keys? - ["desc"] | join(",")')" == cmds &&
        "$(tj '.tasks."tools:gremlins".cmds | tojson')" == "$(jq -cn --arg l "${INSTALL_LINE}" '[$l]')" ]] ||
        case_fail W2 "task tools:gremlins must be exactly desc: and cmds: ['${INSTALL_LINE}'], found $(tj '.tasks."tools:gremlins" | tojson')"
      [[ "$(tj '.tasks.mutation | keys? - ["desc"] | join(",")')" == "cmds,deps" &&
        "$(tj '.tasks.mutation.deps | tojson')" == '["tools:gremlins"]' &&
        "$(tj '.tasks.mutation.cmds | tojson')" == "$(jq -cn --arg l "${RUN_LINE}" '[$l]')" ]] ||
        case_fail W2 "task mutation must be exactly desc:, deps: [tools:gremlins] and cmds: ['${RUN_LINE}'], found $(tj '.tasks.mutation | tojson')"
      [[ "$(tj 'has("dotenv")')" == false ]] || case_fail W2 "Taskfile.yml has a top-level dotenv:"
    fi
    done_case "W2 the pin, the install task and the mutation task"
  fi

  # --- W3: nothing else installs or runs gremlins, or reaches the mutation chain --------------------
  if want W3; then
    start_case
    # The scan set: Makefile, Taskfile.yml and every Taskfile it includes (recursively), every
    # file under .github/, every file under hack/ except hack/test/.
    : >"${SJ}/scan"
    awk '/^(Makefile|Taskfile\.yml|\.github\/.+|hack\/.+)$/ && !/^hack\/test\//' "${SJ}/files" >"${SJ}/scan" ||
      scan_error "could not filter the file list into the W3 scan set"
    # Included Taskfiles, recursively.
    queue=()
    [[ -f "${TF}" ]] && queue=(Taskfile.yml)
    seen_tf=" "
    tfs=()
    while ((${#queue[@]} > 0)); do
      cur="${queue[0]}"
      queue=("${queue[@]:1}")
      [[ "${seen_tf}" != *" ${cur} "* ]] || continue
      seen_tf+="${cur} "
      tfs+=("${cur}")
      if ! inc="$(yq eval -o=json -I=0 '[explode(.) | .includes // {} | to_entries[] | .value | (select(type == "!!str") // .taskfile)]' "${ROOT}/${cur}" 2>/dev/null)"; then
        scan_error "${cur}: the includes: query failed (does not parse, or includes: of an unexpected shape)"
        continue
      fi
      while IFS= read -r p; do
        [[ -n "${p}" ]] || continue
        p="$(dirname "${cur}")/${p}"
        p="$(realpath -m --relative-to="${ROOT}" "${ROOT}/${p}")"
        [[ -d "${ROOT}/${p}" ]] && p="${p}/Taskfile.yml"
        if [[ -f "${ROOT}/${p}" ]]; then
          queue+=("${p}")
        else
          scan_error "${cur} includes ${p}, which does not exist"
        fi
      done < <(jq -r '.[]' <<<"${inc}")
    done
    printf '%s\n' "${tfs[@]}" >>"${SJ}/scan"
    sort -u -o "${SJ}/scan" "${SJ}/scan"
    # Every non-comment line of the scan set as "<file>\t<line>\t<text>".
    mapfile -t scan_files <"${SJ}/scan"
    readable=()
    for f in "${scan_files[@]}"; do
      [[ -n "${f}" ]] || continue
      if [[ -f "${ROOT}/${f}" && -r "${ROOT}/${f}" ]]; then
        readable+=("${f}")
      elif [[ -e "${ROOT}/${f}" || -L "${ROOT}/${f}" ]]; then
        [[ ! -f "${ROOT}/${f}" ]] || scan_error "cannot read ${f}"
      fi
    done
    : >"${SJ}/lines"
    if ((${#readable[@]} > 0)); then
      # A line ending in a backslash is joined with the next (shell and make continuations), so
      # `task \` / `mutation` is one line, reported at its first line number (stage-B B4).
      (cd "${ROOT}" && awk '
        function flush() { if (pend != "") print pf "\t" pl "\t" pend; pend = "" }
        FNR == 1 { flush() }
        pend == "" && /^[[:space:]]*#/ { next }
        {
          t = $0; sub(/^[[:space:]]+/, "", t); sub(/^-[[:space:]]+/, "", t); sub(/[[:space:]]+$/, "", t)
          if (pend != "") t = pend " " t
          else { pf = FILENAME; pl = FNR }
          if (t ~ /\\$/) { sub(/[[:space:]]*\\$/, "", t); pend = t; next }
          pend = ""
          print pf "\t" pl "\t" t
        }
        END { flush() }' "${readable[@]}") >"${SJ}/lines" ||
        scan_error "awk could not read the scan set"
    fi
    # The parsed text of every block scalar (`|` or `>`) in the scan set's YAML files, line by
    # line with continuations joined, as "<file>\tblock\t<text>": a folded `>-` holding `task` and
    # `mutation` on two source lines is one command (stage-B B4). Plain scalars are already one
    # source line each.
    for f in "${readable[@]}"; do
      [[ "${f}" == *.yml || "${f}" == *.yaml ]] || continue
      if ! yq eval -o=json -I=0 '[explode(.) | .. | select(tag == "!!str" and (style == "literal" or style == "folded"))]' "${ROOT}/${f}" >"${SJ}/blocks.json" 2>/dev/null; then
        scan_error "${f}: the block-scalar query failed (does not parse as YAML)"
        continue
      fi
      jq -r --arg f "${f}" '.[] | gsub("\\\\\\n\\s*"; " ") | split("\n")[]
        | sub("^\\s+"; "") | sub("^-\\s+"; "") | sub("\\s+$"; "")
        | select(. != "" and (startswith("#") | not)) | $f + "\tblock\t" + .' "${SJ}/blocks.json" >>"${SJ}/lines" ||
        scan_error "${f}: the block-scalar lines could not be listed"
    done
    # One awk pass over the scan lines: "<rule>\t<file>\t<line>\t<text>" per hit that is not the
    # W1/W2 chain itself. Its exit status is checked (no `|| true`): an awk that fails is a scan
    # error, never "nothing found". The hits are read from a file in this shell, so case_fail
    # counts.
    if ! awk -F'\t' -v il="${INSTALL_LINE}" -v rl="${RUN_LINE}" -v w="${WF_REL}" '
      function hit(rule) { print rule "\t" $1 "\t" $2 "\t" $3 }
      {
        chain = ($1 == "Taskfile.yml" && ($3 == il || $3 == rl))
        if ($3 ~ /go-gremlins/ && !($1 == "Taskfile.yml" && $3 == il)) hit("go-gremlins outside the W2 install line")
        if ($1 != "hack/ci/mutation.sh" && !chain) {
          if ($3 ~ /(^|[^A-Za-z0-9_.-])gremlins["\047]?([[:space:]]+(unleash|run|r|help|version|completion)([^A-Za-z0-9_-]|$)|[[:space:]]+-|[[:space:]]*$)/)
            hit("gremlins invoked outside hack/ci/mutation.sh")
          if ($3 ~ /(^|[^A-Za-z0-9_-])unleash([^A-Za-z0-9_-]|$)/) hit("a gremlins unleash outside hack/ci/mutation.sh")
          if ($3 ~ /hack\/ci\/mutation\.sh/) hit("hack/ci/mutation.sh run outside the mutation task")
        }
        if ($3 ~ /tools:gremlins/ && !($1 == "Taskfile.yml" && ($3 == "tools:gremlins:" || $3 == "deps: [tools:gremlins]")))
          hit("the gremlins install task reached outside the mutation task")
        # Any task command line naming mutation as a word, quoted or not, whatever flags or other
        # tasks come first; and any task command line whose argument is a template, which could
        # name it.
        if ($3 ~ /(^|[^A-Za-z0-9_.:-])(go-)?task["\047]?([[:space:]]+[^[:space:]]+)*[[:space:]]+["\047]?:?mutation([[:space:]"\047;&|)]|$)/ && !($1 == w && $3 == "run: task mutation"))
          hit("a task command running mutation outside the mutation.yaml run step")
        if ($3 ~ /(^|[^A-Za-z0-9_.:-])(go-)?task["\047]?([[:space:]]+[^[:space:]]+)*[[:space:]]+[^[:space:]]*[{][{]/)
          hit("a templated task call the scan cannot resolve (it could name the mutation chain)")
      }' "${SJ}/lines" >"${SJ}/hits"; then
      scan_error "the W3 line scan failed"
    fi
    while IFS=$'\t' read -r what f l t; do
      case_fail W3 "${f}:${l}: ${what}: ${t}"
    done <"${SJ}/hits"
    # A dependency or task call that reaches the chain: any task of any scanned Taskfile but the
    # root mutation task naming mutation or tools:gremlins in deps: or a cmds: task: entry. A
    # templated reference ({{...}}) cannot be resolved here, so it fails too.
    for t in "${tfs[@]}"; do
      if ! reach="$(yq eval -o=json -I=0 '[explode(.) | .tasks // {} | to_entries[] | .key as $k
          | ((.value.deps // []) + ((.value.cmds // []) | map(select(type == "!!map")))) | .[]
          | (select(type == "!!str") // .task) | select(. != null)
          | select(test("^:?(mutation|tools:gremlins)$") or test("[{][{]")) | $k + " -> " + .]' "${ROOT}/${t}" 2>/dev/null)"; then
        scan_error "${t}: the task dependency query failed (does not parse, or a deps:/cmds: of an unexpected shape)"
        continue
      fi
      while IFS= read -r r; do
        [[ -n "${r}" ]] || continue
        [[ "${t}" == Taskfile.yml && "${r}" == "mutation -> tools:gremlins" ]] && continue
        if [[ "${r}" == *"{{"* ]]; then
          case_fail W3 "${t}: task ${r}: a templated task reference the scan cannot resolve (it could name the mutation chain)"
        else
          case_fail W3 "${t}: task ${r}: only the root mutation task may depend on the mutation chain"
        fi
      done < <(jq -r '.[]' <<<"${reach}")
    done
    done_case "W3 gremlins installed and run only through the W1/W2 chain"
  fi

  # --- T1: no threshold option on a command line --------------------------------------------------
  # T1/T3 text files: mutation.yaml, Taskfile.yml, hack/ci/mutation*.sh.
  t_files() {
    local f
    for f in "${WF_REL}" Taskfile.yml "${ROOT}"/hack/ci/mutation*.sh; do
      f="${f#"${ROOT}"/}"
      [[ -f "${ROOT}/${f}" ]] && printf '%s\n' "${f}"
    done
    return 0
  }
  # t_grep <case> <ERE> <what>: each matching line of a T file (comments included) fails.
  t_grep() {
    local f rc out
    while IFS= read -r f; do
      rc=0
      out="$(grep -inE -- "$2" "${ROOT}/${f}" 2>&1)" || rc=$?
      if [[ "${rc}" -gt 1 ]]; then
        scan_error "cannot read ${f}: ${out}"
      elif [[ "${rc}" -eq 0 ]]; then
        while IFS= read -r l; do
          case_fail "$1" "${f}:${l%%:*}: $3: ${l#*:}"
        done <<<"${out}"
      fi
    done < <(t_files)
  }
  if want T1; then
    start_case
    t_grep T1 '--threshold[-_.]' "a gremlins threshold option (report only: no gate)"
    done_case "T1 no threshold option in the workflow, the Taskfile or the scripts"
  fi

  # --- T2: no gremlins config file but the empty one ----------------------------------------------
  if want T2; then
    start_case
    awk -F/ '$NF == ".gremlins" || $NF ~ /^\.gremlins\./' "${SJ}/files" >"${SJ}/t2" ||
      scan_error "the T2 config-file scan failed"
    while IFS= read -r p; do
      [[ -n "${p}" ]] || continue
      case_fail T2 "${p}: a gremlins config file (gremlins reads .gremlins under any extension from a module root)"
    done <"${SJ}/t2"
    if [[ "${GC_OK}" -eq -1 ]]; then
      case_fail T2 "absence: hack/ci/gremlins.yaml is missing"
    elif [[ "${GC_OK}" -eq 1 && "$(jq -c '.' "${SJ}/gc.json")" != '{}' ]]; then
      case_fail T2 "hack/ci/gremlins.yaml must hold an empty map {} and comments only, found $(jq -c '.' "${SJ}/gc.json")"
    fi
    done_case "T2 no tracked .gremlins*, and hack/ci/gremlins.yaml is {}"
  fi

  # --- T3: no threshold through the environment ----------------------------------------------------
  if want T3; then
    start_case
    t_grep T3 'GREMLINS_[A-Z0-9_]*THRESHOLD' "a gremlins threshold variable"
    # Semantic: every key and string of the parsed files, so a quoted, escaped, flow or aliased
    # form is one form.
    for pair in "${WF_REL}:wf" "Taskfile.yml:tf"; do
      f="${pair%%:*}" j="${SJ}/${pair##*:}.json"
      [[ -s "${j}" ]] || continue
      jq -r '[.. | objects | keys[]] + [.. | strings] | .[] | select(test("GREMLINS_[A-Z0-9_]*THRESHOLD"; "i"))' "${j}" >"${SJ}/t3" ||
        scan_error "the T3 parsed scan of ${f} failed"
      while IFS= read -r k; do
        [[ -n "${k}" ]] || continue
        case_fail T3 "${f}: parsed key or value names a gremlins threshold variable: ${k}"
      done <"${SJ}/t3"
    done
    done_case "T3 no gremlins threshold variable in the workflow, the Taskfile or the scripts"
  fi

  # --- C1: the reporter lists the workflow ----------------------------------------------------------
  if want C1; then
    start_case
    if [[ "${CFR_OK}" -eq -1 ]]; then
      case_fail C1 "absence: ${CFR_REL} is missing"
    elif [[ "${CFR_OK}" -eq 1 ]]; then
      [[ "$(jq -r '.on.workflow_run.workflows // [] | map(select(. == "Mutation testing")) | length' "${SJ}/cfr.json")" == 1 ]] ||
        case_fail C1 "${CFR_REL} on.workflow_run.workflows does not list 'Mutation testing' exactly once: $(jq -c '.on.workflow_run.workflows' "${SJ}/cfr.json")"
      [[ "${WF_OK}" -ne 1 || "$(wj '.name')" == "Mutation testing" ]] ||
        case_fail C1 "the reporter's 'Mutation testing' does not equal ${WF_REL}'s name: '$(wj '.name')'"
      [[ "${WF_OK}" -ne -1 ]] || case_fail C1 "absence: ${WF_REL} is missing, so no name: matches the reporter's entry"
    fi
    done_case "C1 ci-failure-report.yaml lists Mutation testing, mutation.yaml's name:"
  fi

  # C3 has no case of its own: every scan error above is a FAIL: C3: line wherever it is found.
  if want C3 && ((STATIC)) && ((SCAN_ERR == 0)); then
    SELECTED=$((SELECTED + 1))
    pass "C3 every file read, parsed and listed"
  fi

  if [[ -n "${ONLY}" && "${SELECTED}" -eq 0 ]]; then
    case_fail filter "MUTATION_SIGNAL_ONLY='${ONLY}' names no case; nothing was checked"
  fi
  [[ "${BAD}" -eq 0 ]] || fail "the mutation signal breaks MUT-1 to MUT-4 (${BAD} failure(s) above)"
  pass "the nightly mutation measurement reports, cannot gate, and cannot go blind (M1-M9, R1-R9, W1-W3, T1-T3, C1, C3)"
  exit 0
fi

# =============================================================================================
# Self-test (tasks.md 2.8): each mutant edits a throwaway copy of the files this test reads (a git
# work tree, so W3 and T2 list it as they list the real one), then this test runs on the copy with
# MUTATION_SIGNAL_ONLY naming the cases that judge it, and must exit non-zero with a FAIL line
# holding the case's expected message; a non-zero exit alone is no evidence. The unmutated, the
# prose and the bot-bump copies must pass. One copy at a time, freed after each mutant. Every edit
# is an exactly-once literal replace (or an edit that is checked to have changed the copy), so a
# vanished anchor fails the self-test instead of testing nothing.
# =============================================================================================
SBAD=0
sbad() {
  echo "FAIL: self-test: $*" >&2
  SBAD=$((SBAD + 1))
}
GIT=(git -c core.hooksPath=/dev/null -c init.defaultBranch=main)

new_tmp
BASE="${TMP}/base"
COPY="${TMP}/copy"
mkdir -p "${BASE}"
# What this test reads: the scripts, the fixture and its reports, the three package directories
# (M6 checks file names there), the Taskfiles and Makefile, .github/ and hack/ outside hack/test/.
(cd "${ROOT}" && git ls-files --cached --others --exclude-standard) |
  grep -E '^(\.github/|hack/|Makefile$|Taskfile\.yml$|internal/(aggregate|redact|pathvalidate)/)' |
  grep -vE '^hack/test/' >"${TMP}/list" || true
(cd "${ROOT}" && git ls-files --cached --others --exclude-standard -- hack/test/testdata/mutation) >>"${TMP}/list"
while IFS= read -r f; do
  [[ -f "${ROOT}/${f}" ]] || continue
  mkdir -p "${BASE}/$(dirname "${f}")"
  cp -p "${ROOT}/${f}" "${BASE}/${f}"
done <"${TMP}/list"
"${GIT[@]}" -C "${BASE}" init -q
"${GIT[@]}" -C "${BASE}" add -A

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
rx() { sed -E -i "$2" "$1"; }
# cut_lines <file> <first> <last>: delete the lines from the one holding <first> through the one
# holding <last> (fixed strings, each on exactly one line).
cut_lines() {
  local a b
  [[ "$(grep -cF -- "$2" "$1")" == 1 && "$(grep -cF -- "$3" "$1")" == 1 ]] || return 1
  a="$(grep -nF -- "$2" "$1" | cut -d: -f1)"
  b="$(grep -nF -- "$3" "$1" | cut -d: -f1)"
  ((a <= b)) || return 1
  sed -i "${a},${b}d" "$1"
}
# add <path> <content>: a new file, added to the copy's git index.
add() {
  mkdir -p "$(dirname "$1")"
  printf '%s\n' "$2" >"$1"
  "${GIT[@]}" add -f -- "$1"
}

# judge <only> <label> <message> <edit...>
judge() {
  local only="$1" label="$2" msg="$3" out rc=0
  shift 3
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
  out="$(MUTATION_SIGNAL_ONLY="${only}" bash "${SELF}" "${COPY}" 2>&1)" || rc=$?
  if [[ "${rc}" -ne 0 ]] && grep '^FAIL: ' <<<"${out}" | grep -F -- "${msg}" >/dev/null; then
    pass "self-test: ${label}: red (exit ${rc}) with '${msg}'"
  else
    sbad "${label}: expected a non-zero exit and a FAIL line with '${msg}', got exit ${rc}: $(grep '^FAIL: ' <<<"${out}" | head -n 3 | tr '\n' ' ')"
  fi
  chmod -R u+rwX "${COPY}" 2>/dev/null || true
  rm -rf "${COPY:?}"
}

# control <label> [edit...]: as judge over every case, but the copy must pass.
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
  out="$(env -u MUTATION_SIGNAL_ONLY bash "${SELF}" "${COPY}" 2>&1)" || rc=$?
  if [[ "${rc}" -eq 0 ]]; then
    pass "self-test: control ${label}: passes"
  else
    sbad "control ${label}: expected a pass, got exit ${rc}: $(grep '^FAIL: ' <<<"${out}" | head -n 3 | tr '\n' ' ')"
  fi
  rm -rf "${COPY:?}"
}

S=hack/ci/mutation-summary.sh
R=hack/ci/mutation.sh
W=.github/workflows/mutation.yaml
T=Taskfile.yml
G=hack/ci/gremlins.yaml
C=.github/workflows/ci-failure-report.yaml
nl=$'\n'

# --- controls (C2) ----------------------------------------------------------------------------
control "unmutated copy"
prose() {
  lit "${W}" "name: Mutation testing${nl}" "# No threshold here: the run measures and never gates.${nl}name: Mutation testing${nl}" &&
    lit "${T}" "  tools:gremlins:${nl}" "  # No threshold for gremlins: the nightly measures, it does not gate.${nl}  tools:gremlins:${nl}" &&
    lit "${R}" "set -euo pipefail${nl}" "# There is no threshold: a low score is a number, never a failure.${nl}set -euo pipefail${nl}"
}
control "a comment holding the word threshold in prose (C2)" prose
bump() { # what a grouped bot PR would do: every pin the guard reads, at every site
  rx "${T}" 's/^  GREMLINS_VERSION: v[0-9.]+$/  GREMLINS_VERSION: v99.0.0/' &&
    sed -E -i 's/(actions\/checkout@)[0-9a-f]{40} # v[0-9.]+/\1ffffffffffffffffffffffffffffffffffffffff # v99.0.0/' .github/workflows/*.yaml &&
    sed -E -i 's/(version: )3\.[0-9]+\.[0-9]+$/\199.0.0/' .github/workflows/*.yaml
}
control "every pin bumped by a bot (implausible versions and SHAs)" bump
# The case filter itself: a filter naming no case must red, or a mistyped one passes silently.
fout="$(MUTATION_SIGNAL_ONLY='no-such-case' bash "${SELF}" "${BASE}" 2>&1)" && frc=0 || frc=$?
if [[ "${frc}" -ne 0 ]] && grep '^FAIL: ' <<<"${fout}" | grep -F 'names no case' >/dev/null; then
  pass "self-test: a case filter naming no case reds (exit ${frc})"
else
  sbad "a case filter naming no case must red with 'names no case', got exit ${frc}"
fi

# shellcheck disable=SC2016 # every ${...} below is file text to find or write, never expanded
{
  # --- formatter (2.8) -------------------------------------------------------------------------
  judge M1 "timed out counted as killed (M1)" "want exactly one row '| internal/redact | 1 | 2 | 3 | 4 | 5 |'" \
    lit "${S}" '[.files[].mutations[].status] as $all' '[.files[].mutations[].status | if . == "TIMED OUT" then "KILLED" else . end] as $all'
  judge M2 "exit 1 on any LIVED (M2)" "M2: surviving mutants failed the formatter" \
    lit "${S}" "((errors == 0)) || exit 1${nl}" "((errors == 0)) || exit 1${nl}for r in \"\${report[@]}\"; do ! grep -F '\"LIVED\"' \"\${r}\" >/dev/null || exit 1; done${nl}"
  judge M3 "rows printed before the last report is validated (M3)" "M3: a counts table was printed" \
    lit "${S}" '  row[${p}]="$(jq' '  echo "| Package | ${p} |"; echo "| ${p} | partial |"'"${nl}"'  row[${p}]="$(jq'
  # M9's check also rejects an empty mutation list, so this mutant reds on M4's own message.
  judge M4 "zero-mutation check dropped (M4)" "M4: no '::error::mutation:' line naming 'internal/pathvalidate' and 'no mutation: nothing was measured'" \
    lit "${S}" "          elif ([.files[].mutations[]] | length) == 0 then \"no mutation: nothing was measured\"${nl}" ''
  judge M5 "unknown statuses accepted (M5)" "M5: the formatter exited 0" \
    lit "${S}" 'select(. as $st | $known | index($st) | not)' 'select(false)'
  judge M6 "package-file check dropped (M6)" "M6: the formatter exited 0" \
    lit "${S}" 'if [[ "${name}" == */* || ! -f "${ROOT}/${p}/${name}" ]]; then' 'if false; then'
  judge M8 "a duplicate package accepted (M8)" "M8: the formatter exited 0" \
    lit "${S}" 'if [[ -n "${report[${pkg}]+set}" ]]; then' 'if false; then'
  judge M8 "a missing package accepted (M8)" "M8: a counts table was printed" \
    lit "${S}" '[[ -n "${report[${p}]+set}" ]] || err "${p}: no report given' '[[ -n "${report[${p}]+set}" ]] || : "${p}: no report given'
  judge M8 "an unknown package accepted (M8)" "M8: no '::error::mutation:' line naming 'internal/collect'" \
    lit "${S}" 'if ((known == 0)); then' 'if false; then'
  judge M9 "measured-nothing check dropped (M9, stage-B B1)" "M9: the formatter exited 0" \
    lit "${S}" 'elif ([.files[].mutations[] | select(.status == "KILLED" or .status == "LIVED")] | length) == 0 then' 'elif false then'
  judge M9 "only TIMED OUT counted as measuring nothing (M9, stage-B B1)" "M9: the formatter exited 0" \
    lit "${S}" 'select(.status == "KILLED" or .status == "LIVED")' 'select(.status != "TIMED OUT")'

  # --- runner (2.8) ----------------------------------------------------------------------------
  judge R2 "|| true on the gremlins call (R2)" "R2: no '::error::mutation:' line holding 'gremlins failed on ./internal/redact'" \
    lit "${R}" '"$3") || rc=$?' '"$3") || true'
  judge R7 "|| true on the gremlins call (R7)" "R7: no '::error::mutation:' line holding 'gremlins failed'" \
    lit "${R}" '"$3") || rc=$?' '"$3") || true'
  judge R3 "report-exists check dropped (R3)" "R3: no '::error::mutation:' line holding 'no report for ./internal/aggregate'" \
    lit "${R}" '  [[ -s "${out}" ]] || die "no report for $4: gremlins exited 0 but wrote nothing to ${out}"'"${nl}" ''
  judge R3 "a fixed output directory reused (R3)" "R3: a stale report from the previous run stood in" \
    lit "${R}" 'REPORTS="$(mktemp -d)"' 'REPORTS="${TMPDIR:-/tmp}/kollect-mutation-reports"; mkdir -p "${REPORTS}"'
  judge R1 "one mutator flag dropped (R1)" "R1: call 1 (fixture) argv is" \
    lit "${R}" '  --invert-negatives --remove-self-assignments)' '  --invert-negatives)'
  judge R1 "--config dropped (R1)" "R1: call 1 (fixture) argv is" \
    lit "${R}" '"--config=${CONFIG}" "--output=${out}"' '"--output=${out}"'
  judge R1 "--config made relative (R1)" "R1: call 1 (fixture) argv is" \
    lit "${R}" '"--config=${CONFIG}" "--output=${out}"' '"--config=hack/ci/gremlins.yaml" "--output=${out}"'
  judge R1 "a GREMLINS_ variable set inline on the call (R1, round-1 review)" "R1: call 1 (fixture) saw GREMLINS_ variables 'GREMLINS_UNLEASH_TAGS=x '" \
    lit "${R}" '(cd "$2" && "${GREMLINS}" unleash' '(cd "$2" && GREMLINS_UNLEASH_TAGS=x "${GREMLINS}" unleash'
  judge R1 "the timeout coefficient dropped (R1, stage-B B1)" "R1: call 1 (fixture) argv is" \
    lit "${R}" '"--output=${out}" "${TIMEOUT_FLAG}" "${FLAGS[@]}"' '"--output=${out}" "${FLAGS[@]}"'
  judge R1 "the timeout coefficient back at the default 3 (R1, stage-B B1)" "R1: call 1 (fixture) argv is" \
    lit "${R}" 'TIMEOUT_FLAG=--timeout-coefficient=10' 'TIMEOUT_FLAG=--timeout-coefficient=3'
  judge R1 "GOFLAGS=-count=1 dropped (R1, task 1.1 F1)" "R1: call 1 (fixture) saw GOFLAGS='-tags=narrowed'" \
    lit "${R}" 'export GOFLAGS=-count=1' ':'
  judge R1 "one merged run instead of one per package (R1)" "R1: gremlins was called 2 times, want 4" \
    lit "${R}" 'for short in aggregate redact pathvalidate; do
  measure "${short}" "${ROOT}" "./internal/${short}" "./internal/${short}"
done' 'measure aggregate "${ROOT}" ./internal/... ./internal/...'
  judge 'R1|R4' "the fixture step dropped (R1)" "R1: gremlins was called 3 times, want 4" \
    cut_lines "${R}" 'measure fixture "${FIXTURE}" .' 'echo "mutation: ${CONTROL} is LIVED, as it must be"'
  judge R4 "the fixture step dropped (R4)" "R4: control not-covered: the run exited 0" \
    cut_lines "${R}" 'measure fixture "${FIXTURE}" .' 'echo "mutation: ${CONTROL} is LIVED, as it must be"'
  judge R4 "NOT COVERED accepted as the control (R4)" "R4: control not-covered: the run exited 0" \
    lit "${R}" 'elif [[ "${status}" != LIVED ]]; then' 'elif [[ "${status}" != LIVED && "${status}" != "NOT COVERED" ]]; then'
  judge R4 "any LIVED mutant matched as the control (R4)" "R4: control absent: the run exited 0" \
    lit "${R}" '| select(.line == 9 and .column == 11 and .type == "ARITHMETIC_BASE") | .status]' '| select(.status == "LIVED") | .status]'
  judge R4 "a duplicated control accepted (R4)" "R4: control twice: the run exited 0" \
    lit "${R}" 'elif [[ "${n}" != 1 ]]; then' 'elif false; then'
  judge R5 "MUTATION_FORCE_FAILURE skips gremlins and exits 0 (R5)" "R5: MUTATION_FORCE_FAILURE=true did not fail the run" \
    lit "${R}" '  true) die "forced failure:' '  true) echo "forced"; exit 0; die "forced failure:'
  judge R5 "MUTATION_FORCE_FAILURE=false fails the run (R5)" "R5: MUTATION_FORCE_FAILURE false: the run changed" \
    lit "${R}" '  "" | false) ;;' '  "") ;;'
  judge R6 "the GREMLINS_ refusal dropped (R6)" "R6: GREMLINS_UNLEASH_DIFF=HEAD: the runner started" \
    lit "${R}" '  die "refusing to run with' '  echo "refusing to run with'
  judge R6 "the refusal is a fixed list of names (R6)" "R6: GREMLINS_KOLLECT_PROBE=1: the runner started" \
    lit "${R}" 'if [[ "${name}" == GREMLINS_* ]]; then' 'if [[ "${name}" == GREMLINS_UNLEASH_THRESHOLD_EFFICACY || "${name}" == GREMLINS_UNLEASH_DIFF || "${name}" == GREMLINS_UNLEASH_TAGS || "${name}" == GREMLINS_MUTANTS_ARITHMETIC_BASE_ENABLED ]]; then'
  judge R6 "empty GREMLINS_ values ignored (R6)" "R6: GREMLINS_UNLEASH_TAGS=: the runner started" \
    lit "${R}" 'if [[ "${name}" == GREMLINS_* ]]; then' 'if [[ "${name}" == GREMLINS_* && -n "${!name}" ]]; then'
  judge R9 "the mutator-flag comparison dropped (R9, stage-B B2)" "R9: help extra: the run exited 0" \
    lit "${R}" 'if [[ "${offered}" != "${passed}" ]]; then' 'if false; then'
  judge R9 "only a dropped flag checked, an added one accepted (R9, stage-B B2)" "R9: help extra: the run exited 0" \
    lit "${R}" 'if [[ "${offered}" != "${passed}" ]]; then' 'if [[ -n "$(LC_ALL=C comm -13 <(echo "${offered}") <(echo "${passed}"))" ]]; then'
  judge R9 "an empty mutator list not singled out (R9, stage-B B2)" "R9: no '::error::mutation:' line holding 'lists no mutator flag'" \
    lit "${R}" '[[ -n "${offered}" ]] ||' 'true ||'
  judge R9 "a failing --help ignored (R9, stage-B B2)" "R9: no '::error::mutation:' line holding 'unleash --help' failed'" \
    lit "${R}" 'help="$("${GREMLINS}" unleash --help 2>&1)" ||' 'help="$("${GREMLINS}" unleash --help 2>&1)" || true ||'
  judge R8 "the \$GITHUB_OUTPUT line dropped (R8)" "R8: before the first gremlins call" \
    lit "${R}" '  echo "reports=${REPORTS}" >>"${GITHUB_OUTPUT}"' '  :'
  judge R8 "the report directory removed on exit (R8)" "R8: after the green run" \
    lit "${R}" 'REPORTS="$(mktemp -d)"' 'REPORTS="$(mktemp -d)"'"${nl}"'trap '"'"'rm -rf "${REPORTS}"'"'"' EXIT'
  judge R1 "the runner deleted (absence)" "absence: hack/ci/mutation.sh is missing" rm -f "${R}"
  judge M1 "the formatter deleted (absence)" "absence: hack/ci/mutation-summary.sh is missing" rm -f "${S}"

  # --- workflow (2.4) --------------------------------------------------------------------------
  judge W1 "run: task mutation || true (W1)" "W1: step 4 must be" lit "${W}" 'run: task mutation' 'run: task mutation || true'
  judge W1 "an extra run: step (W1)" "W1: want exactly 5 steps" \
    lit "${W}" '      - name: Upload the gremlins reports' "      - run: echo extra${nl}${nl}      - name: Upload the gremlins reports"
  judge W1 "persist-credentials dropped (W1)" "W1: step 1 must be" \
    lit "${W}" "        with:${nl}          persist-credentials: false${nl}" ''
  judge W1 "continue-on-error: true on the job (W1)" "W1: the job's keys are" \
    lit "${W}" '    timeout-minutes: 15' "    timeout-minutes: 15${nl}    continue-on-error: true"
  judge W1 "if: on the run step (W1)" "W1: step 4 must be" \
    lit "${W}" '        id: mutation' "        id: mutation${nl}        if: github.event_name == 'schedule'"
  judge W1 "upload-artifact at another SHA (W1)" "W1: step 5 uses actions/upload-artifact@ffffffffffffffffffffffffffffffffffffffff, which no other workflow" \
    rx "${W}" 's/(upload-artifact@)[0-9a-f]{40}/\1ffffffffffffffffffffffffffffffffffffffff/'
  judge W1 "the upload path: changed (W1)" "W1: step 5 must be" \
    lit "${W}" 'path: ${{ steps.mutation.outputs.reports }}' 'path: /tmp'
  judge W1 "if-no-files-found: warn (W1)" "W1: step 5 must be" lit "${W}" 'if-no-files-found: error' 'if-no-files-found: warn'
  judge W1 "the cron changed (W1)" "W1: on.schedule is" lit "${W}" '"30 1 * * *"' '"30 1 * * 1"'
  judge W1 "timeout-minutes dropped (W1)" "W1: timeout-minutes is" lit "${W}" "    timeout-minutes: 15${nl}" ''
  judge W1 "a job-level permissions: (W1)" "W1: the job's keys are" \
    lit "${W}" '    timeout-minutes: 15' "    timeout-minutes: 15${nl}    permissions:${nl}      contents: write"
  judge W1 "a second key in the run step's env: (W1)" "W1: step 4 must be" \
    lit "${W}" '          MUTATION_FORCE_FAILURE: ${{ vars.MUTATION_FORCE_FAILURE }}' "          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}${nl}          GOFLAGS: -count=2"
  judge W1 "a workflow env: (W1)" "W1: top-level keys are" \
    lit "${W}" "permissions:${nl}  contents: read${nl}" "permissions:${nl}  contents: read${nl}${nl}env:${nl}  GOFLAGS: -tags=x${nl}"
  judge W1 "a workflow defaults.run.shell (W1)" "W1: top-level keys are" \
    lit "${W}" "permissions:${nl}  contents: read${nl}" "permissions:${nl}  contents: read${nl}${nl}defaults:${nl}  run:${nl}    shell: bash -c true {0}${nl}"
  judge W1 "a step shell: (W1)" "W1: step 4 must be" \
    lit "${W}" '        run: task mutation' "        run: task mutation${nl}        shell: bash -c true {0}"
  judge W1 "a pull_request trigger (W1)" "W1: on: is" lit "${W}" '  workflow_dispatch:' "  workflow_dispatch:${nl}  pull_request:"
  judge W1 "permissions widened (W1)" "W1: permissions: is" lit "${W}" '  contents: read' '  contents: write'
  judge W1 "setup-task at a version no other workflow uses (W1)" "W1: setup-task version: 99.0.0 is used by no other workflow" \
    lit "${W}" '          version: 3.52.0' '          version: 99.0.0'
  judge 'W1|C1' "the workflow deleted (absence)" "W1: absence: .github/workflows/mutation.yaml is missing" rm -f "${W}"
  judge C1 "the workflow renamed (C1)" "C1: the reporter's 'Mutation testing' does not equal" \
    lit "${W}" 'name: Mutation testing' 'name: Mutation testing (nightly)'

  # --- install chain (2.5, W2) -----------------------------------------------------------------
  judge W2 "the renovate annotation dropped (W2)" "W2: the line above GREMLINS_VERSION is" \
    lit "${T}" "${ANNOTATION}${nl}" ''
  judge W2 "a partial version v0.6 (W2)" "W2: GREMLINS_VERSION is 'v0.6'" \
    rx "${T}" 's/^  GREMLINS_VERSION: v([0-9]+)\.([0-9]+)\.[0-9]+$/  GREMLINS_VERSION: v\1.\2/'
  judge W2 "a literal version on the install line (W2)" "W2: task tools:gremlins must be" \
    rx "${T}" 's#(gremlins/cmd/gremlins@)\{\{\.GREMLINS_VERSION\}\}#\1v99.0.0#'
  judge W2 "sources: and generates: on mutation (W2)" "W2: task mutation must be" \
    lit "${T}" '    deps: [tools:gremlins]' "    deps: [tools:gremlins]${nl}    sources: [go.mod]${nl}    generates: [bin/mutation.stamp]"
  judge W2 "status: [true] on the install task (W2)" "W2: task tools:gremlins must be" \
    lit "${T}" '    desc: Install the pinned gremlins under bin/ (nightly mutation run only)' "    desc: Install the pinned gremlins under bin/ (nightly mutation run only)${nl}    status: [\"true\"]"
  judge W2 "a command-level ignore_error: true (W2)" "W2: task mutation must be" \
    lit "${T}" "      - ${RUN_LINE}" "      - cmd: ${RUN_LINE}${nl}        ignore_error: true"
  judge W2 "the runner pointed at gremlins on PATH (W2)" "W2: task mutation must be" \
    lit "${T}" "${RUN_LINE}" 'bash hack/ci/mutation.sh gremlins'
  judge W2 "a task-level vars: override of the pin (W2)" "W2: task mutation must be" \
    lit "${T}" '    deps: [tools:gremlins]' "    deps: [tools:gremlins]${nl}    vars:${nl}      GREMLINS_VERSION: v0.5.1"
  judge W2 "a top-level dotenv: (W2)" "W2: Taskfile.yml has a top-level dotenv:" \
    lit "${T}" "run: once${nl}" "run: once${nl}${nl}dotenv: [\".env\"]${nl}"
  judge W2 "the GREMLINS_VERSION line duplicated (W2)" "W2: Taskfile.yml must have exactly one two-space 'GREMLINS_VERSION:' line, found 2" \
    lit "${T}" '  GREMLINS_VERSION: v0.6.0' "  GREMLINS_VERSION: v0.6.0${nl}  GREMLINS_VERSION: v0.6.0"

  # --- scope (2.5, W3) -------------------------------------------------------------------------
  judge W3 "a Makefile go install of gremlins@v0.5.1 (W3)" "W3: Makefile:" \
    eval 'printf "\ngremlins-old:\n\tgo install github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.1\n" >>Makefile'
  judge W3 "a hack/install-gremlins.sh that downloads a release (W3)" "W3: hack/install-gremlins.sh:" \
    add hack/install-gremlins.sh "#!/usr/bin/env bash${nl}curl -fsSL https://github.com/go-gremlins/gremlins/releases/download/v0.6.0/gremlins_0.6.0_linux_amd64.tar.gz | tar -xz -C bin"
  judge W3 "a workflow step run: gremlins unleash ./... (W3)" "W3: .github/workflows/test-shuffle.yaml:" \
    lit .github/workflows/test-shuffle.yaml '        run: task test:shuffle' "        run: task test:shuffle${nl}${nl}      - run: gremlins unleash ./..."
  judge W3 "a run: task mutation step in the ci.yaml lint job (W3)" "W3: .github/workflows/ci.yaml:" \
    lit .github/workflows/ci.yaml "      - name: Lint${nl}        run: task lint${nl}" "      - run: task mutation${nl}      - name: Lint${nl}        run: task lint${nl}"
  judge W3 "deps: [mutation] on another task (W3)" "W3: Taskfile.yml: task vulncheck -> mutation" \
    lit "${T}" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    deps: [install]" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    deps: [install, mutation]"
  judge W3 "a task: :mutation call in an included Taskfile (W3)" "W3: hack/task/Taskfile.test.yml: task shuffle -> :mutation" \
    lit hack/task/Taskfile.test.yml '      - bash hack/ci/test-shuffle.sh' "      - bash hack/ci/test-shuffle.sh${nl}      - task: :mutation"
  extra_include() {
    add extra/Taskfile.extra.yml "version: \"3\"${nl}tasks:${nl}  nightly:${nl}    cmds:${nl}      - bash hack/ci/mutation.sh bin/gremlins" &&
      lit "${T}" "    taskfile: ./hack/task/Taskfile.test.yml${nl}" "    taskfile: ./hack/task/Taskfile.test.yml${nl}  extra:${nl}    taskfile: ./extra/Taskfile.extra.yml${nl}"
  }
  judge W3 "an included Taskfile outside hack/task/ runs hack/ci/mutation.sh (W3)" "W3: extra/Taskfile.extra.yml:" extra_include
  judge W3 "the install task reached from another Taskfile line (W3)" "W3: hack/task/Taskfile.test.yml:" \
    lit hack/task/Taskfile.test.yml '      - bash hack/ci/test-shuffle.sh' "      - bash hack/ci/test-shuffle.sh${nl}      - task tools:gremlins"
  # Round-1 review (G2): spellings the first scan did not model.
  for tc in 'task -s mutation' 'task --silent mutation' 'task lint mutation' 'task -t Taskfile.yml :mutation'; do
    judge W3 "run: ${tc} in the ci.yaml lint job (W3)" "W3: .github/workflows/ci.yaml:" \
      lit .github/workflows/ci.yaml "      - name: Lint${nl}        run: task lint${nl}" "      - run: ${tc}${nl}      - name: Lint${nl}        run: task lint${nl}"
  done
  judge W3 "a templated dependency that resolves to mutation (W3)" "W3: Taskfile.yml: task vulncheck -> {{.MEASUREMENT}}: a templated task reference" \
    lit "${T}" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    deps: [install]" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    vars:${nl}      MEASUREMENT: mutation${nl}    deps: [install, \"{{.MEASUREMENT}}\"]"
  judge W3 "a quoted \"gremlins\" unleash in a workflow step (W3)" "W3: .github/workflows/test-shuffle.yaml:" \
    lit .github/workflows/test-shuffle.yaml '        run: task test:shuffle' "        run: task test:shuffle${nl}${nl}      - run: '\"gremlins\" unleash ./...'"
  # Round-2 review (G2): the quote after gremlins without the unleash rule's help, quoted task
  # names, go-task, and a templated shell-form task call.
  judge W3 "a quoted \"gremlins\" version in a workflow step (W3)" "W3: .github/workflows/test-shuffle.yaml:44: gremlins invoked outside hack/ci/mutation.sh" \
    lit .github/workflows/test-shuffle.yaml '        run: task test:shuffle' "        run: task test:shuffle${nl}${nl}      - run: '\"gremlins\" version'"
  for tc in "task 'mutation'" 'task "mutation"' "'\"task\" mutation'" 'go-task mutation'; do
    judge W3 "run: ${tc} in the ci.yaml lint job (W3)" "W3: .github/workflows/ci.yaml:" \
      lit .github/workflows/ci.yaml "      - name: Lint${nl}        run: task lint${nl}" "      - run: ${tc}${nl}      - name: Lint${nl}        run: task lint${nl}"
  done
  judge W3 "a templated shell-form task call (W3)" "W3: hack/task/Taskfile.test.yml:" \
    lit hack/task/Taskfile.test.yml '      - bash hack/ci/test-shuffle.sh' "      - bash hack/ci/test-shuffle.sh${nl}      - task {{.MEASUREMENT}}"
  judge W3 "a binary in a variable running unleash in a hack script (W3)" "W3: hack/run-mutants.sh:2: a gremlins unleash outside hack/ci/mutation.sh" \
    add hack/run-mutants.sh "#!/usr/bin/env bash${nl}\"\${G:?}\" unleash ./..."
  # Stage-B review (B4): a task call split by a shell continuation, or folded by YAML.
  judge W3 "run: | with task \\ continued onto mutation in ci.yaml (W3, stage-B B4)" "W3: .github/workflows/ci.yaml:" \
    lit .github/workflows/ci.yaml "      - name: Lint${nl}        run: task lint${nl}" "      - run: |${nl}          task \\${nl}            mutation${nl}      - name: Lint${nl}        run: task lint${nl}"
  judge W3 "run: >- folding task and mutation into one command in ci.yaml (W3, stage-B B4)" "W3: .github/workflows/ci.yaml:" \
    lit .github/workflows/ci.yaml "      - name: Lint${nl}        run: task lint${nl}" "      - run: >-${nl}          task${nl}          mutation${nl}      - name: Lint${nl}        run: task lint${nl}"
  judge W3 "a hack script continuing task onto mutation (W3, stage-B B4)" "W3: hack/run-nightly.sh:2:" \
    add hack/run-nightly.sh "#!/usr/bin/env bash${nl}task \\${nl}  mutation"
  judge 'W3|C3' "a deps: of an unexpected shape (C3)" "C3: scan error: Taskfile.yml: the task dependency query failed" \
    lit "${T}" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    deps: [install]" "    desc: Scan for known Go vulnerabilities (govulncheck)${nl}    deps: mutation"

  # --- threshold sources (2.6) -----------------------------------------------------------------
  for sp in '--threshold-efficacy 100' '--threshold_efficacy=100' '--threshold-mcover 50' '--threshold.mcover=50'; do
    judge T1 "${sp} in the runner (T1)" "T1: hack/ci/mutation.sh:" \
      lit "${R}" '"${FLAGS[@]}" "$3")' "\"\${FLAGS[@]}\" ${sp} \"\$3\")"
    judge T1 "${sp} in the mutation task (T1)" "T1: Taskfile.yml:" lit "${T}" "      - ${RUN_LINE}" "      - ${RUN_LINE} ${sp}"
    judge T1 "${sp} in a workflow step (T1)" "T1: ${W}:" lit "${W}" 'run: task mutation' "run: task mutation -- ${sp}"
  done
  judge T1 "--THRESHOLD-EFFICACY in capitals (T1)" "T1: hack/ci/mutation.sh:" \
    lit "${R}" '"${FLAGS[@]}" "$3")' '"${FLAGS[@]}" --THRESHOLD-EFFICACY=100 "$3")'
  for p in .gremlins .gremlins.yaml .gremlins.json hack/test/testdata/mutation/.gremlins.yml; do
    judge T2 "a tracked ${p} (T2)" "T2: ${p}: a gremlins config file" add "${p}" "unleash:${nl}  threshold:${nl}    efficacy: 100"
  done
  judge T2 "hack/ci/gremlins.yaml given an unleash: key (T2)" "T2: hack/ci/gremlins.yaml must hold an empty map" \
    lit "${G}" "{}${nl}" "unleash:${nl}  threshold:${nl}    efficacy: 100${nl}"
  judge T2 "hack/ci/gremlins.yaml deleted (absence)" "T2: absence: hack/ci/gremlins.yaml is missing" rm -f "${G}"
  TV='GREMLINS_UNLEASH_THRESHOLD_EFFICACY'
  judge T3 "the variable at workflow env: (T3)" "T3: ${W}:" \
    lit "${W}" "permissions:${nl}  contents: read${nl}" "permissions:${nl}  contents: read${nl}${nl}env:${nl}  ${TV}: \"100\"${nl}"
  judge T3 "the variable at job env: (T3)" "T3: ${W}:" \
    lit "${W}" '    timeout-minutes: 15' "    timeout-minutes: 15${nl}    env:${nl}      ${TV}: \"100\""
  judge T3 "the variable at step env: (T3)" "T3: ${W}:" \
    lit "${W}" '          MUTATION_FORCE_FAILURE: ${{ vars.MUTATION_FORCE_FAILURE }}' "          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}${nl}          ${TV}: \"100\""
  judge T3 "the variable in a flow map env: (T3)" "T3: ${W}:" \
    lit "${W}" "        env:${nl}          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}" "        env: {MUTATION_FORCE_FAILURE: \"\${{ vars.MUTATION_FORCE_FAILURE }}\", ${TV}: \"100\"}"
  judge T3 "the variable as a quoted key (T3)" "T3: ${W}:" \
    lit "${W}" '          MUTATION_FORCE_FAILURE: ${{ vars.MUTATION_FORCE_FAILURE }}' "          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}${nl}          \"${TV}\": \"100\""
  judge T3 "the variable through an anchor and alias (T3)" "T3: ${W}:" \
    lit "${W}" "        env:${nl}          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}" "        env: &gate${nl}          ${TV}: \"100\"${nl}        with: *gate"
  judge T3 "the variable as an escaped key only a YAML parser decodes (T3)" "T3: ${W}: parsed key or value names a gremlins threshold variable" \
    lit "${W}" '          MUTATION_FORCE_FAILURE: ${{ vars.MUTATION_FORCE_FAILURE }}' "          MUTATION_FORCE_FAILURE: \${{ vars.MUTATION_FORCE_FAILURE }}${nl}          \"GREMLINS_UNLEASH_\\x54HRESHOLD_EFFICACY\": \"100\""
  judge T3 "the variable written to \$GITHUB_ENV (T3)" "T3: ${W}:" \
    lit "${W}" '        run: task mutation' "        run: |${nl}          echo GREMLINS_UNLEASH_THRESHOLD_MUTANT_COVERAGE=100 >> \"\$GITHUB_ENV\"${nl}          task mutation"
  judge T3 "the variable in Taskfile.yml env: (T3)" "T3: Taskfile.yml:" \
    lit "${T}" "env:${nl}  CGO_ENABLED: \"0\"" "env:${nl}  CGO_ENABLED: \"0\"${nl}  ${TV}: \"100\""
  judge T3 "the variable inline in the runner (T3)" "T3: hack/ci/mutation.sh:" \
    lit "${R}" '(cd "$2" && "${GREMLINS}" unleash' "(cd \"\$2\" && ${TV}=100 \"\${GREMLINS}\" unleash"

  # --- reporter (2.6, C1) and scan errors (C3) ----------------------------------------------------
  judge C1 "Mutation testing dropped from the reporter (C1)" "C1: ${C} on.workflow_run.workflows does not list 'Mutation testing'" \
    rx "${C}" '/^ +- Mutation testing$/d'
  judge C1 "the reporter lists a near-miss name (C1)" "C1: ${C} on.workflow_run.workflows does not list 'Mutation testing'" \
    lit "${C}" '      - Mutation testing' '      - Mutation testing (old)'
  for f in "${W}" "${T}" "${G}" "${C}"; do
    judge C3 "${f} does not parse (C3)" "C3: scan error: ${f} does not parse as YAML" \
      eval "printf 'key: [unclosed\n' >>'${f}'"
  done
  judge 'W3|C3' "an unreadable file in the scan set (C3)" "C3: scan error: cannot read hack/install-zizmor.sh" \
    chmod 000 hack/install-zizmor.sh
  judge C3 "a copy git cannot list (C3)" "C3: scan error: git ls-files failed" rm -rf .git
}

[[ "${SBAD}" -eq 0 ]] || fail "self-test: ${SBAD} mutant(s) or control(s) above did not behave"
echo "All mutation signal self-tests passed."
