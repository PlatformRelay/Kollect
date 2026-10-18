#!/usr/bin/env bash
# The nightly shuffle runs every unit package three times under one printed seed (TDS-2, openspec
# change test-depth-signals).
#
# hack/ci/test-shuffle.sh (`task test:shuffle`, nightly in test-shuffle.yaml) runs each package of
# `go list ./...` minus an e2e-only exclusion list three times with `-shuffle=<seed>`: a RunSpecs
# (Ginkgo) package as three `-count=1` invocations, every other package once at `-count=3`. This
# test drives it with a fake `make` first on PATH (records its calls; `echo-kubebuilder-assets`
# prints a temp directory) and a fake `go` whose `list` prints the case's packages and whose
# `test` records argv and KUBEBUILDER_ASSETS, or forwards to the real go for the order-dependent
# fixture in hack/test/testdata/shuffle/ (which `go list ./...` never selects).
#
# What it locks (tasks.md 2.2-2.3):
#   S0   a SHUFFLE_SEED that is not a decimal integer fails before any test;
#   S1   the fixture under seed 2 (TestDependent first, task 2.1) fails and the failure names it;
#   S2   control: the fixture is green with -shuffle=off -count=3 (and red under seed 2 directly);
#   S3   a failing run's printed seed reproduces with go test -shuffle=<seed> -count=3, the seed
#        go used is the printed one, and runs choose different seeds;
#   S4   the seed line comes before the first test output;
#   S5   an exclusion without a reason fails (the real list, and the script on a bad entry);
#   S6   an exclusion of a non-e2e package fails and names it (same two ways);
#   S7   an empty package list fails;
#   S8   a RunSpecs package runs three times at -count=1, a plain one once at -count=3;
#   S9a  make setup-envtest runs before make -s echo-kubebuilder-assets, and every go test call
#        sees KUBEBUILDER_ASSETS set to what it printed;
#   S9b  empty assets fail before any go test call;
#   S10  the packages tested are exactly go list ./... minus the exclusion list;
#   S11  test-shuffle.yaml: name, cron, permissions, pinned checkout without credentials,
#        go-cache with envtest, pinned setup-task, timeout, task test:shuffle, no seed set, and
#        no if: or continue-on-error: on the job or a step;
#   S12  Taskfile.test.yml `shuffle:` runs the script; ci-failure-report.yaml lists Test shuffle.
#
# Usage:
#   hack/test/test_shuffle_test.sh              # check the repository tree
#   hack/test/test_shuffle_test.sh --self-test  # mutate copies, assert each case reds
#   hack/test/test_shuffle_test.sh <tree-root>  # check a tree holding the script and the three YAMLs
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
# The real repository: its go.mod and fixture serve every case, whatever tree is under test.
REPO="$(cd "$(dirname "${SELF}")/../.." && pwd -P)"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-${REPO}}"
ROOT="$(cd "${ROOT}" && pwd -P)"
SCRIPT="${ROOT}/hack/ci/test-shuffle.sh"
WF="${ROOT}/.github/workflows/test-shuffle.yaml"
TF="${ROOT}/hack/task/Taskfile.test.yml"
CFR="${ROOT}/.github/workflows/ci-failure-report.yaml"

for tool in go yq; do
  command -v "${tool}" >/dev/null 2>&1 || fail "${tool} not found; install it to run this test"
done
REAL_GO="$(command -v go)"
for f in "${SCRIPT}" "${WF}" "${TF}" "${CFR}"; do
  [[ -f "${f}" ]] || fail "${f#"${ROOT}"/} is missing"
done
MODULE="$(sed -n 's/^module //p' "${REPO}/go.mod")"
FIXTURE_DIR="${REPO}/hack/test/testdata/shuffle"
FIXTURE_PKG="${MODULE}/hack/test/testdata/shuffle"
[[ -f "${FIXTURE_DIR}/orderdep_test.go" ]] || fail "fixture ${FIXTURE_DIR#"${REPO}"/}/orderdep_test.go is missing"
SEED=2 # task 2.1: seed 2 runs the fixture's TestDependent first

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
  CASE_BAD=0
  case_fail() {
    echo "FAIL: $1: $2" >&2
    BAD=$((BAD + 1))
  }
  done_case() { # <label>: ok only when the case added no failure since it started
    [[ "${BAD}" -ne "${CASE_BAD}" ]] || pass "$1"
  }

  # --- one run of the script ------------------------------------------------------------------
  # Globals a case sets before run_script: LIST (lines "<import path> <dir>" the fake go list
  # prints), FORWARD (1: go test runs the real go), ASSETS_EMPTY (1: echo-kubebuilder-assets
  # prints nothing), SEED_ENV (SHUFFLE_SEED: unset when empty, set to "" when "<empty>"), UNDER
  # (the script to run).
  # Results: RC, D (case dir: log, make.log, test.N argv files, kba.N), NTEST (go test calls).
  run_script() {
    new_tmp
    D="${TMP}"
    mkdir -p "${D}/bin" "${D}/assets"
    printf '%s' "${LIST}" >"${D}/list"
    echo 0 >"${D}/ntest"
    cat >"${D}/bin/go" <<'FAKE'
#!/usr/bin/env bash
# Fake go: `list` prints the case's packages; `test` records argv and KUBEBUILDER_ASSETS, then
# prints ok per package or forwards to the real go.
d="${FAKE_DIR:?}"
case "${1:-}" in
  list)
    printf '%s\n' "$*" >>"${d}/list.calls"
    [[ -s "${d}/list" ]] && cat "${d}/list"
    exit 0
    ;;
  test)
    n=$(($(cat "${d}/ntest") + 1))
    echo "${n}" >"${d}/ntest"
    printf '%s\n' "${@:2}" >"${d}/test.${n}"
    printf '%s' "${KUBEBUILDER_ASSETS-<unset>}" >"${d}/kba.${n}"
    if [[ "${FAKE_FORWARD:-0}" == 1 ]]; then
      exec "${REAL_GO:?}" "$@"
    fi
    for a in "${@:2}"; do
      [[ "${a}" == -* ]] || printf 'ok  \t%s\t0.001s\n' "${a}"
    done
    exit 0
    ;;
esac
echo "fake go: unexpected call: $*" >&2
exit 97
FAKE
    cat >"${D}/bin/make" <<'FAKE'
#!/usr/bin/env bash
# Fake make: records each call; setup-envtest prints its path with no newline, as the real one.
d="${FAKE_DIR:?}"
printf '%s\n' "$*" >>"${d}/make.log"
case "$*" in
  setup-envtest) printf '%s' "${d}/assets" >&2 ;;
  "-s echo-kubebuilder-assets") [[ "${FAKE_ASSETS_EMPTY:-0}" == 1 ]] || printf '%s\n' "${d}/assets" ;;
  *)
    echo "fake make: unexpected target: $*" >&2
    exit 97
    ;;
esac
FAKE
    chmod +x "${D}/bin/go" "${D}/bin/make"
    local -a envs=(-u KUBEBUILDER_ASSETS -u SHUFFLE_SEED)
    if [[ "${SEED_ENV}" == "<empty>" ]]; then
      envs+=("SHUFFLE_SEED=")
    elif [[ -n "${SEED_ENV}" ]]; then
      envs+=("SHUFFLE_SEED=${SEED_ENV}")
    fi
    (cd "${REPO}" && env "${envs[@]}" PATH="${D}/bin:${PATH}" FAKE_DIR="${D}" REAL_GO="${REAL_GO}" \
      FAKE_FORWARD="${FORWARD}" FAKE_ASSETS_EMPTY="${ASSETS_EMPTY}" bash "${UNDER}") \
      >"${D}/log" 2>&1 && RC=0 || RC=$?
    NTEST="$(cat "${D}/ntest")"
    LIST="" FORWARD=0 ASSETS_EMPTY=0 SEED_ENV="" UNDER="${SCRIPT}"
  }
  LIST="" FORWARD=0 ASSETS_EMPTY=0 SEED_ENV="" UNDER="${SCRIPT}"

  # with_exclusion <id> <entry>: a copy of the script with one exclusion entry added; sets UNDER.
  with_exclusion() {
    new_tmp
    awk -v e="$2" '{print} /^EXCLUDE=\($/ {printf "  \"%s\"\n", e}' "${SCRIPT}" >"${TMP}/test-shuffle.sh"
    grep -F "\"$2\"" "${TMP}/test-shuffle.sh" >/dev/null ||
      case_fail "$1" "the script has no 'EXCLUDE=(' line to add the exclusion '$2' to"
    UNDER="${TMP}/test-shuffle.sh"
  }

  # pkgdirs: two package directories, one whose _test.go calls RunSpecs (Ginkgo) and one plain.
  new_tmp
  PK="${TMP}"
  mkdir -p "${PK}/gk" "${PK}/plain" "${PK}/e2e"
  printf 'package gk\n\nfunc TestSuite(t *testing.T) { RunSpecs(t, "gk") }\n' >"${PK}/gk/suite_test.go"
  printf 'package plain\n\nfunc TestPlain(t *testing.T) {}\n' >"${PK}/plain/plain_test.go"
  printf 'package e2e\n\nfunc TestE2E(t *testing.T) {}\n' >"${PK}/e2e/e2e_test.go"
  GK="example.test/x/internal/gk" PL="example.test/x/internal/plain" E2E="example.test/x/test/e2e"

  # Per go test call: its packages (non-flag args) and its -count / -shuffle values.
  call_pkgs() { grep -v '^-' "${D}/test.$1" || true; }
  call_flag() { sed -n "s/^-$2=//p" "${D}/test.$1"; }
  tested_set() { # the sorted set of packages over all go test calls
    local i
    for ((i = 1; i <= NTEST; i++)); do call_pkgs "${i}"; done | sort -u
  }
  diag_has() { # <id> <fixed text>: a line of the log contains it
    grep -F -- "$2" "${D}/log" >/dev/null || case_fail "$1" "the output has no line containing '$2'; log: $(tail -3 "${D}/log" | tr '\n' '|')"
  }

  # --- S2: control, and the fixture's seed precondition -----------------------------------------
  CASE_BAD="${BAD}"
  (cd "${REPO}" && "${REAL_GO}" test -shuffle=off -count=3 "${FIXTURE_PKG}") >/dev/null 2>&1 ||
    case_fail S2 "the fixture is red with -shuffle=off -count=3: it fails for a reason other than order"
  if (cd "${REPO}" && "${REAL_GO}" test -shuffle="${SEED}" -count=3 "${FIXTURE_PKG}") >/dev/null 2>&1; then
    case_fail S2 "the fixture is green under -shuffle=${SEED}: seed ${SEED} no longer runs TestDependent first (pick a new one as task 2.1 did)"
  fi
  done_case S2

  # --- S0: a seed that is set but not a decimal integer (an empty one included) ---------------------
  for bad in 12x "<empty>"; do
    CASE_BAD="${BAD}"
    LIST="${PL} ${PK}/plain" SEED_ENV="${bad}"
    run_script
    [[ "${RC}" -ne 0 ]] || case_fail S0 "SHUFFLE_SEED=${bad} was accepted (exit 0)"
    [[ "${NTEST}" -eq 0 ]] || case_fail S0 "go test ran ${NTEST} times on SHUFFLE_SEED=${bad}, want 0"
    diag_has S0 "SHUFFLE_SEED must be a decimal integer"
    done_case "S0 (${bad})"
  done

  # --- S1: the fixture under seed 2 fails, and the failure names the seed --------------------------
  CASE_BAD="${BAD}"
  LIST="${FIXTURE_PKG} ${FIXTURE_DIR}" FORWARD=1 SEED_ENV="${SEED}"
  run_script
  [[ "${RC}" -ne 0 ]] || case_fail S1 "the order-dependent fixture passed under SHUFFLE_SEED=${SEED} (exit 0): no -shuffle?"
  grep -- '--- FAIL: TestDependent' "${D}/log" >/dev/null ||
    case_fail S1 "no '--- FAIL: TestDependent' line: the fixture did not run dependent-first"
  grep "^::error::test-shuffle: FAILED with seed ${SEED};" "${D}/log" >/dev/null ||
    case_fail S1 "no '::error::test-shuffle: FAILED with seed ${SEED};' line naming the seed"
  done_case S1
  S1_LOG="${D}/log"

  # --- S4: the seed line precedes the first test output -------------------------------------------
  CASE_BAD="${BAD}"
  seed_at="$(grep -n "^test-shuffle: seed ${SEED} " "${S1_LOG}" | head -1 | cut -d: -f1 || true)"
  out_at="$(grep -nE '^(-test\.shuffle |ok |FAIL|--- |=== )' "${S1_LOG}" | head -1 | cut -d: -f1 || true)"
  if [[ -z "${seed_at}" || -z "${out_at}" ]]; then
    case_fail S4 "want a 'test-shuffle: seed ${SEED} ' line and test output, found line '${seed_at}' and '${out_at}'"
  elif ((seed_at >= out_at)); then
    case_fail S4 "the seed line is line ${seed_at}, after the first test output at line ${out_at}"
  fi
  done_case S4

  # --- S3: a failing run's printed seed reproduces, and is the seed go used -----------------------
  CASE_BAD="${BAD}"
  found="" seeds=""
  for ((try = 1; try <= 20; try++)); do
    LIST="${FIXTURE_PKG} ${FIXTURE_DIR}" FORWARD=1
    run_script
    s="$(sed -n 's/^test-shuffle: seed \([0-9][0-9]*\) .*/\1/p' "${D}/log" | head -1)"
    [[ -n "${s}" ]] || {
      case_fail S3 "run ${try} printed no 'test-shuffle: seed <n>' line"
      break
    }
    seeds="${seeds} ${s}"
    if [[ "${RC}" -ne 0 && -z "${found}" ]]; then
      found="${s}"
      used="$(sed -n 's/^-test\.shuffle //p' "${D}/log" | sort -u | tr '\n' ' ')"
      [[ "${used}" == "${s} " ]] ||
        case_fail S3 "the script printed seed ${s} but go test used '${used% }'"
    fi
    [[ -z "${found}" || "$(tr ' ' '\n' <<<"${seeds}" | sed '/^$/d' | sort -u | wc -l)" -lt 2 ]] || break
  done
  if [[ -z "${found}" ]]; then
    case_fail S3 "no failing run in ${try} runs without SHUFFLE_SEED (seeds:${seeds}): the seed never varies onto TestDependent first"
  else
    (cd "${REPO}" && "${REAL_GO}" test -shuffle="${found}" -count=3 "${FIXTURE_PKG}") >"${TMP}/repro" 2>&1 &&
      case_fail S3 "go test -shuffle=${found} -count=3 passed: the printed seed does not reproduce the failure"
    grep -- '--- FAIL: TestDependent' "${TMP}/repro" >/dev/null ||
      case_fail S3 "the reproduction with seed ${found} did not fail on TestDependent"
  fi
  [[ "$(tr ' ' '\n' <<<"${seeds}" | sed '/^$/d' | sort -u | wc -l)" -ge 2 ]] ||
    case_fail S3 "every run without SHUFFLE_SEED chose the same seed (${seeds# })"
  done_case S3

  # --- S5 / S6: the exclusion list -------------------------------------------------------------------
  # The real list: every entry "<import path>|<reason>", with a reason and an e2e path.
  CASE_BAD="${BAD}"
  grep '^EXCLUDE=($' "${SCRIPT}" >/dev/null || case_fail S5 "the script has no 'EXCLUDE=(' exclusion list"
  entries="$(awk '/^EXCLUDE=\($/ {on = 1; next} on && /^\)$/ {exit} on && !/^[[:space:]]*(#|$)/ {print}' "${SCRIPT}")"
  while IFS= read -r line; do
    [[ -n "${line}" ]] || continue
    e="$(sed -e 's/^[[:space:]]*"//' -e 's/"[[:space:]]*$//' <<<"${line}")"
    pkg="${e%%|*}" reason="${e#*|}"
    [[ "${e}" == *"|"* && -n "${reason// /}" ]] || case_fail S5 "exclusion entry '${e}' has no reason"
    [[ "${pkg}" =~ (^|/)e2e(/|$) ]] || case_fail S6 "exclusion entry '${pkg}' is not an e2e package"
  done <<<"${entries}"
  # The script itself: a bad entry fails before any test.
  with_exclusion S5 "${E2E}|"
  LIST="${PL} ${PK}/plain"$'\n'"${E2E} ${PK}/e2e"
  run_script
  [[ "${RC}" -ne 0 && "${NTEST}" -eq 0 ]] ||
    case_fail S5 "an exclusion without a reason was accepted (exit ${RC}, ${NTEST} go test calls)"
  diag_has S5 "exclusion '${E2E}|' has no reason"
  done_case S5
  CASE_BAD="${BAD}"
  with_exclusion S6 "${GK}|envtest is slow"
  LIST="${PL} ${PK}/plain"$'\n'"${GK} ${PK}/gk"
  run_script
  [[ "${RC}" -ne 0 && "${NTEST}" -eq 0 ]] ||
    case_fail S6 "a non-e2e exclusion was accepted (exit ${RC}, ${NTEST} go test calls)"
  diag_has S6 "exclusion '${GK}' is not an e2e package"
  done_case S6

  # --- S7: an empty package list -------------------------------------------------------------------
  CASE_BAD="${BAD}"
  LIST=""
  run_script
  [[ "${RC}" -ne 0 ]] || case_fail S7 "an empty go list ./... passed (exit 0): a vacuous green"
  [[ "${NTEST}" -eq 0 ]] || case_fail S7 "go test ran ${NTEST} times on an empty package list"
  diag_has S7 "no package to test"
  done_case S7

  # --- S8: a RunSpecs package three times at -count=1, a plain one once at -count=3 ----------------
  CASE_BAD="${BAD}"
  LIST="${GK} ${PK}/gk"$'\n'"${PL} ${PK}/plain" SEED_ENV=7
  run_script
  [[ "${RC}" -eq 0 ]] || case_fail S8 "the script exited ${RC}, want 0; log: $(tail -3 "${D}/log" | tr '\n' '|')"
  gk_calls=0 pl_calls=0
  for ((i = 1; i <= NTEST; i++)); do
    counts="$(call_flag "${i}" count | tr '\n' ' ')" shuffles="$(call_flag "${i}" shuffle | tr '\n' ' ')"
    [[ "${shuffles}" == "7 " ]] || case_fail S8 "go test call ${i} has -shuffle '${shuffles% }', want exactly 7"
    if call_pkgs "${i}" | grep -Fx "${GK}" >/dev/null; then
      gk_calls=$((gk_calls + 1))
      [[ "${counts}" == "1 " ]] || case_fail S8 "the Ginkgo package ran with -count '${counts% }', want exactly 1"
    fi
    if call_pkgs "${i}" | grep -Fx "${PL}" >/dev/null; then
      pl_calls=$((pl_calls + 1))
      [[ "${counts}" == "3 " ]] || case_fail S8 "the plain package ran with -count '${counts% }', want exactly 3"
    fi
  done
  [[ "${gk_calls}" -eq 3 ]] || case_fail S8 "the Ginkgo package was invoked ${gk_calls} times, want 3"
  [[ "${pl_calls}" -eq 1 ]] || case_fail S8 "the plain package was invoked ${pl_calls} times, want 1"
  done_case S8

  # --- S9a: envtest provisioned and exported -----------------------------------------------------------
  CASE_BAD="${BAD}"
  LIST="${GK} ${PK}/gk"$'\n'"${PL} ${PK}/plain"
  run_script
  setup_at="$(grep -nx 'setup-envtest' "${D}/make.log" 2>/dev/null | head -1 | cut -d: -f1 || true)"
  echo_at="$(grep -nx -- '-s echo-kubebuilder-assets' "${D}/make.log" 2>/dev/null | head -1 | cut -d: -f1 || true)"
  if [[ -z "${setup_at}" || -z "${echo_at}" ]]; then
    case_fail S9a "make calls are '$(tr '\n' '|' 2>/dev/null <"${D}/make.log" || true)', want setup-envtest then -s echo-kubebuilder-assets"
  elif ((setup_at >= echo_at)); then
    case_fail S9a "make setup-envtest ran after make -s echo-kubebuilder-assets"
  fi
  [[ "${NTEST}" -gt 0 ]] || case_fail S9a "go test never ran"
  for ((i = 1; i <= NTEST; i++)); do
    [[ "$(cat "${D}/kba.${i}")" == "${D}/assets" ]] ||
      case_fail S9a "go test call ${i} saw KUBEBUILDER_ASSETS='$(cat "${D}/kba.${i}")', want ${D}/assets"
  done
  done_case S9a

  # --- S9b: empty assets fail before any test ------------------------------------------------------------
  CASE_BAD="${BAD}"
  LIST="${PL} ${PK}/plain" ASSETS_EMPTY=1
  run_script
  [[ "${RC}" -ne 0 ]] || case_fail S9b "empty envtest assets were accepted (exit 0)"
  [[ "${NTEST}" -eq 0 ]] || case_fail S9b "go test ran ${NTEST} times with empty envtest assets, want 0"
  diag_has S9b "envtest assets missing"
  done_case S9b

  # --- S10: the packages tested are go list ./... minus the exclusion list -------------------------------
  CASE_BAD="${BAD}"
  LIST="${GK} ${PK}/gk"$'\n'"${PL} ${PK}/plain"$'\n'"${PL}2 ${PK}/plain"$'\n'"${E2E} ${PK}/e2e"
  run_script
  # Discovery is one `go list` over the whole module: a narrower pattern drops packages unseen.
  list_calls="$(cat "${D}/list.calls" 2>/dev/null || true)"
  [[ "$(wc -l <<<"${list_calls}")" -eq 1 && "${list_calls}" == list\ * && "${list_calls##* }" == "./..." ]] ||
    case_fail S10 "go list was called as '$(tr '\n' '|' <<<"${list_calls}")', want one 'go list ... ./...'"
  want="$(printf '%s\n' "${GK}" "${PL}" "${PL}2" "${E2E}" | sort -u)"
  [[ "$(tested_set)" == "${want}" ]] ||
    case_fail S10 "tested '$(tested_set | tr '\n' ' ')', want go list ./... '$(tr '\n' ' ' <<<"${want}")' (the exclusion list is empty)"
  with_exclusion S10 "${E2E}|kind cluster required"
  LIST="${GK} ${PK}/gk"$'\n'"${PL} ${PK}/plain"$'\n'"${PL}2 ${PK}/plain"$'\n'"${E2E} ${PK}/e2e"
  run_script
  want="$(printf '%s\n' "${GK}" "${PL}" "${PL}2" | sort -u)"
  [[ "$(tested_set)" == "${want}" ]] ||
    case_fail S10 "with ${E2E} excluded, tested '$(tested_set | tr '\n' ' ')', want '$(tr '\n' ' ' <<<"${want}")'"
  done_case S10

  # --- S11: test-shuffle.yaml ----------------------------------------------------------------------------
  CASE_BAD="${BAD}"
  wq() { yq eval -o=json -I=0 "$1" "${WF}" 2>/dev/null || echo "<parse error>"; }
  [[ "$(wq '.name')" == '"Test shuffle"' ]] || case_fail S11 "name: is $(wq '.name'), want \"Test shuffle\""
  [[ "$(wq '."on".schedule')" == '[{"cron":"30 2 * * *"}]' ]] ||
    case_fail S11 "on.schedule is $(wq '."on".schedule'), want exactly one cron \"30 2 * * *\""
  [[ "$(wq '(."on" // {}) | keys | . - ["schedule", "workflow_dispatch"]')" == '[]' ]] ||
    case_fail S11 "on: has triggers besides schedule and workflow_dispatch: $(wq '."on" | keys')"
  [[ "$(wq '.permissions')" == '{"contents":"read"}' ]] ||
    case_fail S11 "permissions: is $(wq '.permissions'), want exactly contents: read"
  [[ "$(wq '[.jobs[] | select(has("permissions"))] | length')" == 0 ]] ||
    case_fail S11 "a job sets its own permissions:"
  [[ "$(wq '.jobs | length')" == 1 ]] || case_fail S11 "want exactly one job, found $(wq '.jobs | length')"
  [[ "$(wq '.jobs[]."timeout-minutes"')" == 45 ]] ||
    case_fail S11 "the job's timeout-minutes is $(wq '.jobs[]."timeout-minutes"'), want 45 (task 2.1)"
  # step_at <var> <select> <what>: <var> is the index of the one step matching <select>, else -1
  # and a failure naming <what>. Called directly, never as $(...), so case_fail's count survives.
  step_at() {
    local got
    got="$(yq eval ".jobs[].steps | to_entries | .[] | select(.value | $2) | .key" "${WF}" 2>/dev/null)"
    if [[ "$(wc -w <<<"${got}")" -ne 1 ]]; then
      case_fail S11 "want exactly one step that $3, found $(wc -w <<<"${got}")"
      got=-1
    fi
    printf -v "$1" '%s' "${got}"
  }
  co=-1 gc=-1 st=-1 rn=-1
  step_at co '(.uses // "") | test("^actions/checkout@[0-9a-f]{40}$")' "uses a SHA-pinned actions/checkout"
  [[ "${co}" -lt 0 ]] || [[ "$(wq ".jobs[].steps[${co}].with.\"persist-credentials\"")" == false ]] ||
    case_fail S11 "actions/checkout does not set persist-credentials: false"
  step_at gc '(.uses // "") == "./.github/actions/go-cache"' "uses ./.github/actions/go-cache"
  [[ "${gc}" -lt 0 ]] || [[ "$(wq ".jobs[].steps[${gc}].with.envtest | tostring")" == '"true"' ]] ||
    case_fail S11 "go-cache does not set envtest: true"
  step_at st '(.uses // "") | test("^go-task/setup-task@[0-9a-f]{40}$")' "uses a SHA-pinned go-task/setup-task"
  step_at rn '(.run // "") | sub("\s+$"; "") == "task test:shuffle"' "runs exactly 'task test:shuffle'"
  if ((co >= 0 && gc >= 0 && st >= 0 && rn >= 0)) && ! ((co < gc && gc < st && st < rn)); then
    case_fail S11 "steps out of order: checkout ${co}, go-cache ${gc}, setup-task ${st}, task test:shuffle ${rn}"
  fi
  # An `if:` or `continue-on-error:` on the job or a step would skip the run or turn its failure
  # green (precedent: dist_operatorhub_pr_test.sh reads continue-on-error the same way).
  [[ "$(wq '[.jobs[] | select(has("if") or has("continue-on-error"))] | length')" == 0 ]] ||
    case_fail S11 "the job sets if: or continue-on-error:"
  soft="$(wq '[.jobs[].steps[] | select(has("if") or has("continue-on-error")) | (.name // .uses // .run)]')"
  [[ "${soft}" == '[]' ]] || case_fail S11 "steps set if: or continue-on-error: ${soft}"
  # The script picks a fresh seed when SHUFFLE_SEED is unset: the workflow must not set it, anywhere.
  ! grep -F 'SHUFFLE_SEED' "${WF}" >/dev/null ||
    case_fail S11 "test-shuffle.yaml mentions SHUFFLE_SEED: the nightly would rerun one seed"
  done_case S11

  # --- S12: the task and the reporter ---------------------------------------------------------------------
  CASE_BAD="${BAD}"
  got="$(yq eval -o=json -I=0 '.tasks.shuffle.cmds' "${TF}" 2>/dev/null || echo '<parse error>')"
  [[ "${got}" == '["bash hack/ci/test-shuffle.sh"]' ]] ||
    case_fail S12 "Taskfile.test.yml shuffle.cmds is ${got}, want [\"bash hack/ci/test-shuffle.sh\"]"
  ! grep -F 'SHUFFLE_SEED' "${TF}" >/dev/null ||
    case_fail S12 "Taskfile.test.yml mentions SHUFFLE_SEED: a task variable would pin the seed"
  [[ "$(yq eval '."on".workflow_run.workflows | any_c(. == "Test shuffle")' "${CFR}" 2>/dev/null)" == true ]] ||
    case_fail S12 "ci-failure-report.yaml does not list Test shuffle under on.workflow_run.workflows"
  done_case S12

  [[ "${BAD}" -eq 0 ]] || fail "the nightly shuffle breaks TDS-2 (cases above)"
  pass "the nightly shuffle runs every unit package three times under one printed seed (S0-S12)"
  exit 0
fi

# ---------------------------------------------------------------------------
# Self-test: copy the script and the three YAMLs into a fresh root, break one clause per copy, and
# assert the check reds on the case built to catch it (its `FAIL: <case>:` line, not just a
# non-zero exit). One unmutated copy must pass.
# ---------------------------------------------------------------------------
new_copy() {
  new_tmp
  COPY="${TMP}"
  mkdir -p "${COPY}/hack/ci" "${COPY}/hack/task" "${COPY}/.github/workflows"
  cp "${SCRIPT}" "${COPY}/hack/ci/test-shuffle.sh"
  cp "${TF}" "${COPY}/hack/task/Taskfile.test.yml"
  cp "${WF}" "${COPY}/.github/workflows/test-shuffle.yaml"
  cp "${CFR}" "${COPY}/.github/workflows/ci-failure-report.yaml"
}

# replace_once <file> <old> <new>: an exactly-once literal replace; a vanished or repeated anchor
# fails the self-test instead of testing nothing.
replace_once() {
  local file="$1" old="$2" new="$3" content rest
  content="$(cat "${file}")"
  [[ "${content}" == *"${old}"* ]] || return 1
  rest="${content#*"${old}"}"
  [[ "${rest}" != *"${old}"* ]] || return 1
  printf '%s\n' "${content%%"${old}"*}${new}${rest}" >"${file}"
}

# mutant <label> <file under the copy> <case id> <old> <new> [<old> <new>...]
mutant() {
  local label="$1" rel="$2" id="$3" out rc
  shift 3
  new_copy
  while [[ "$#" -ge 2 ]]; do
    replace_once "${COPY}/${rel}" "$1" "$2" ||
      fail "self-test: the '${label}' mutant did not apply (anchor missing or not unique in ${rel}) -- nothing was tested"
    shift 2
  done
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] || fail "self-test: the check accepted the '${label}' mutant (exit 0)"
  grep "^FAIL: ${id}:" <<<"${out}" >/dev/null ||
    fail "self-test: the '${label}' mutant did not red case ${id}; got: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
  rm -rf "${COPY:?}"
  pass "self-test: ${label} rejected by ${id} (red cases: $(sed -n 's/^FAIL: \(S[0-9a-z]*\):.*/\1/p' <<<"${out}" | sort -u | tr '\n' ' '))"
}

new_copy
if ! out="$(bash "${SELF}" "${COPY}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree first: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
fi
rm -rf "${COPY:?}"
pass "self-test: no-op control passes on an unmutated copy"

S=hack/ci/test-shuffle.sh
W=.github/workflows/test-shuffle.yaml
# shellcheck disable=SC2016 # every ${...} below is literal script or workflow text
{
  mutant "SHUFFLE_SEED not validated" "${S}" S0 \
    '[[ "${SHUFFLE_SEED}" =~ ^[0-9]{1,18}$ ]] || die' ': ||'
  mutant "-shuffle dropped" "${S}" S1 \
    'go test -shuffle="${seed}" -count=3 "${plain[@]}"' 'go test -count=3 "${plain[@]}"'
  mutant "constant seed" "${S}" S3 \
    'seed="$(od -An -N4 -tu4 /dev/urandom | tr -d '"' '"')"' 'seed=1'
  mutant "seed echoed after the run" "${S}" S4 \
    'say "seed ${seed} (reproduce' ': "seed ${seed} (reproduce' \
    'if ((rc)); then' 'say "seed ${seed} (reproduce)"; if ((rc)); then'
  mutant "reasonless exclusion" "${S}" S5 \
    'EXCLUDE=(' 'EXCLUDE=(
  "example.test/x/test/e2e|"'
  mutant "exclusion reason not checked" "${S}" S5 \
    '[[ "${entry}" == *"|"* && -n "${reason// /}" ]] || die' ': ||'
  mutant "non-e2e exclusion" "${S}" S6 \
    'EXCLUDE=(' 'EXCLUDE=(
  "example.test/x/internal/controller|envtest is slow"'
  mutant "exclusion not limited to e2e" "${S}" S6 \
    '[[ "${pkg}" =~ (^|/)e2e(/|$) ]] || die' ': ||'
  mutant "empty package list accepted" "${S}" S7 \
    '((${#plain[@]} + ${#ginkgo[@]} > 0)) || die' ': ||'
  mutant "-count=3 dropped" "${S}" S8 \
    'go test -shuffle="${seed}" -count=3 "${plain[@]}"' 'go test -shuffle="${seed}" "${plain[@]}"'
  mutant "-count=3 on the Ginkgo package" "${S}" S8 \
    'go test -shuffle="${seed}" -count=1 "${ginkgo[@]}"' 'go test -shuffle="${seed}" -count=3 "${ginkgo[@]}"'
  mutant "make setup-envtest dropped" "${S}" S9a 'make setup-envtest >&2' ':'
  mutant "KUBEBUILDER_ASSETS export dropped" "${S}" S9a 'export KUBEBUILDER_ASSETS' ':'
  mutant "empty-assets check dropped" "${S}" S9b '[[ -n "${KUBEBUILDER_ASSETS}" ]] || die' ': ||'
  mutant "one discovered package dropped" "${S}" S10 'done <<<"${listing}"' 'done < <(sed 1d <<<"${listing}")'
  mutant "discovery narrowed below ./..." "${S}" S10 "{{.Dir}}' ./...)\"" "{{.Dir}}' ./internal/...)\""
  mutant "an empty SHUFFLE_SEED picks a seed" "${S}" S0 '"${SHUFFLE_SEED+set}"' '"${SHUFFLE_SEED:-}"'
  mutant "persist-credentials: false dropped" "${W}" S11 'persist-credentials: false' 'persist-credentials: true'
  mutant "workflow renamed" "${W}" S11 'name: Test shuffle' 'name: Test shuffle (nightly)'
  mutant "cron changed" "${W}" S11 '- cron: "30 2 * * *"' '- cron: "30 2 * * 1"'
  mutant "permissions widened" "${W}" S11 'contents: read' 'contents: write'
  mutant "timeout-minutes dropped" "${W}" S11 'timeout-minutes: 45' ''
  mutant "checkout pinned to a tag" "${W}" S11 'uses: actions/checkout@' 'uses: actions/checkout@v7 # was '
  mutant "setup-task pinned to a tag" "${W}" S11 'uses: go-task/setup-task@' 'uses: go-task/setup-task@v2 # was '
  mutant "go-cache without envtest" "${W}" S11 'envtest: true' 'envtest: false'
  mutant "workflow runs another task" "${W}" S11 'run: task test:shuffle' 'run: task test:run'
  mutant "SHUFFLE_SEED set in the workflow" "${W}" S11 \
    'run: task test:shuffle' 'run: task test:shuffle
        env:
          SHUFFLE_SEED: "1"'
  mutant "run step continue-on-error" "${W}" S11 \
    '        run: task test:shuffle' $'        run: task test:shuffle\n        continue-on-error: true'
  mutant "run step if" "${W}" S11 '        run: task test:shuffle' $'        run: task test:shuffle\n        if: false'
  mutant "job continue-on-error" "${W}" S11 '    timeout-minutes: 45' $'    timeout-minutes: 45\n    continue-on-error: true'
  mutant "job if" "${W}" S11 '    runs-on: ubuntu-latest' $'    runs-on: ubuntu-latest\n    if: false'
  mutant "checkout step if" "${W}" S11 \
    '          persist-credentials: false' $'          persist-credentials: false\n        if: false'
  mutant "Test shuffle dropped from the reporter list" .github/workflows/ci-failure-report.yaml S12 \
    '      - Test shuffle
' ''
  mutant "reporter lists a near-miss name" .github/workflows/ci-failure-report.yaml S12 \
    '      - Test shuffle' '      - Test shuffle (old)'
  mutant "shuffle task runs something else" hack/task/Taskfile.test.yml S12 \
    'bash hack/ci/test-shuffle.sh' 'go test ./...'
}

echo "All nightly shuffle self-tests passed."
