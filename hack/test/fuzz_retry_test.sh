#!/usr/bin/env bash
# The fuzz runner retries only the known deadline flake and is never green on nothing (TDS-1,
# openspec change test-depth-signals).
#
# hack/ci/fuzz-retry.sh <target> <pkg> runs one `go test -fuzz` per matrix leg of ci.yaml's `fuzz`
# job. This test drives it with a fake `go` first on PATH that replays one fixture per attempt
# (hack/test/testdata/fuzz-retry/, captured from runs 37911142180 and 37288739312) and records
# each call's argv, inside a throwaway git repository so the corpus check reads a real index.
#
# What it locks (cases F0-F17 of tasks.md 1.1-1.2, 1.6 and 5.2-5.5):
#   F0      a malformed target, a missing or an empty pkg: usage error, go never called;
#   F1      the deadline flake then a pass: exit 0, two calls, exact argv, a warning naming it;
#   F2-F4   a deny marker (panic:, signal: killed, fatal error:) next to the deadline: no retry;
#           a deny marker in an attempt that exited 0 is red too;
#   F5a-c   a crash corpus in an untracked testdata/, Failing input written without a corpus,
#           a failing git status: no retry;
#   F6      the deadline twice: red after exactly two calls;
#   F7-F9   empty or unknown output, zero fuzz targets, a pass with no `fuzz: elapsed`: red;
#   F10     control: a plain pass is green after one call with the exact argv;
#   F11     ci.yaml's fuzz job calls the runner with the matrix values and keeps no inline loop,
#           the runner is executable (the step runs it directly), the job's if: is exactly its
#           changes gate, and neither the job nor a step sets continue-on-error: or another if:;
#   F12-F15 the flake is the exact, ordered, complete line sequence for the matrix target;
#   F17     the flake text under an exit status other than 1 (2, 137) is not the flake: no retry;
#   F18     the real crash output (crash-local.txt) is rejected on its deny marker, no retry;
#   F16     ci.yaml's fuzz job runs `go mod download` in its own step between go-cache and the
#           runner, with no if: or continue-on-error:, so cold-cache `go: downloading` lines
#           never reach the flake match (task 1.6).
# "The diagnostic" is the runner's own `::error::fuzz-retry:` line; a case asserts the full set
# of those lines, never the replayed go output.
#
# Usage:
#   hack/test/fuzz_retry_test.sh              # check the repository tree
#   hack/test/fuzz_retry_test.sh --self-test  # mutate copies of the runner, assert each case reds
#   hack/test/fuzz_retry_test.sh <tree-root>  # check a tree holding hack/ci/fuzz-retry.sh and ci.yaml
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
FIX="$(dirname "${SELF}")/testdata/fuzz-retry"

MODE="check"
if [[ "${1:-}" == "--self-test" ]]; then
  MODE="self-test"
  shift
fi

ROOT="${1:-}"
if [[ -z "${ROOT}" ]]; then
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fi
ROOT="$(cd "${ROOT}" && pwd -P)"
RUNNER="${ROOT}/hack/ci/fuzz-retry.sh"
CI="${ROOT}/.github/workflows/ci.yaml"

for tool in git yq; do
  command -v "${tool}" >/dev/null 2>&1 || fail "${tool} not found; install it to run this test"
done
REAL_GIT="$(command -v git)"
[[ -f "${RUNNER}" ]] || fail "${RUNNER#"${ROOT}"/} is missing"
[[ -f "${CI}" ]] || fail "${CI#"${ROOT}"/} is missing"
[[ -d "${FIX}" ]] || fail "fixture directory ${FIX} is missing"

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

  # --- fixtures -------------------------------------------------------------------------------
  DEADLINE="${FIX}/deadline-FuzzContentHash.txt"
  PASSFX="${FIX}/pass-FuzzContentHash.txt"
  for f in "${DEADLINE}" "${PASSFX}" "${FIX}/deadline-FuzzValidateAttributePath.txt" \
    "${FIX}/pass-FuzzValidateAttributePath.txt" "${FIX}/zero-target.txt"; do
    [[ -s "${f}" ]] || fail "fixture ${f#"${FIX}"/} is missing or empty"
  done

  # A variant of the deadline fixture: <awk program> applied to it, written to stdout.
  variant() {
    awk "$1" "${DEADLINE}"
  }

  # --- one case ---------------------------------------------------------------------------------
  # Globals a case sets before run_case: ATTEMPTS (array of "<rc>:<file>"), CORPUS (a path the fake
  # go creates on every call, relative to the repo), GIT_STATUS_FAILS (1: `git status` exits 128),
  # ARGS (the runner's arguments, when they are not "<target> <pkg>").
  # run_case <id> <target> <pkg>; results in RC, CALLS, CASE_DIR (argv.N, log).
  run_case() {
    local id="$1" target="$2" pkg="$3" n=0 spec
    local -a args=("${target}" "${pkg}")
    [[ "${#ARGS[@]}" -eq 0 ]] || args=("${ARGS[@]}")
    new_tmp
    CASE_DIR="${TMP}"
    mkdir -p "${CASE_DIR}/bin" "${CASE_DIR}/go" "${CASE_DIR}/repo/internal/aggregate" \
      "${CASE_DIR}/repo/internal/collect" "${CASE_DIR}/repo/internal/sink/git"
    for spec in "${ATTEMPTS[@]}"; do
      n=$((n + 1))
      printf '%s\n' "${spec%%:*}" >"${CASE_DIR}/go/rc.${n}"
      cp "${spec#*:}" "${CASE_DIR}/go/out.${n}"
    done
    echo 0 >"${CASE_DIR}/go/calls"
    cat >"${CASE_DIR}/bin/go" <<'FAKE'
#!/usr/bin/env bash
# Fake go: replays out.N with exit rc.N on call N and records the call's argv, one arg per line.
d="${FAKE_GO_DIR:?}"
n=$(($(cat "${d}/calls") + 1))
echo "${n}" >"${d}/calls"
printf '%s\n' "$@" >"${d}/argv.${n}"
if [[ -n "${FAKE_GO_CORPUS:-}" ]]; then
  mkdir -p "$(dirname "${FAKE_GO_CORPUS}")" && printf 'go test fuzz v1\n' >"${FAKE_GO_CORPUS}"
fi
if [[ ! -f "${d}/out.${n}" ]]; then
  echo "fake go: no fixture for call ${n}"
  exit 97
fi
cat "${d}/out.${n}"
exit "$(cat "${d}/rc.${n}")"
FAKE
    if [[ "${GIT_STATUS_FAILS:-0}" == 1 ]]; then
      cat >"${CASE_DIR}/bin/git" <<'FAKE'
#!/usr/bin/env bash
# Fake git: `status` fails as in a broken work tree; everything else is the real git.
if [[ "${1:-}" == "status" ]]; then
  echo "fatal: not a git repository (simulated)" >&2
  exit 128
fi
exec "${REAL_GIT:?}" "$@"
FAKE
    fi
    chmod +x "${CASE_DIR}"/bin/*
    # The throwaway repository tracks a file in every matrix package and no testdata/ directory,
    # so an untracked corpus collapses to `?? <dir>/testdata/` under plain `git status --porcelain`.
    printf 'package aggregate\n' >"${CASE_DIR}/repo/internal/aggregate/a.go"
    printf 'package collect\n' >"${CASE_DIR}/repo/internal/collect/c.go"
    printf 'package git\n' >"${CASE_DIR}/repo/internal/sink/git/g.go"
    git -C "${CASE_DIR}/repo" init -q
    git -C "${CASE_DIR}/repo" add -A -f
    (cd "${CASE_DIR}/repo" && PATH="${CASE_DIR}/bin:${PATH}" FAKE_GO_DIR="${CASE_DIR}/go" \
      FAKE_GO_CORPUS="${CORPUS:-}" REAL_GIT="${REAL_GIT}" bash "${RUNNER}" "${args[@]}") \
      >"${CASE_DIR}/log" 2>&1 && RC=0 || RC=$?
    CALLS="$(cat "${CASE_DIR}/go/calls")"
    CASE_CORPUS="${CORPUS:-}"
    CASE_BAD="${BAD}"
    ATTEMPTS=()
    ARGS=()
    CORPUS=""
    GIT_STATUS_FAILS=0
    : "${id}"
  }

  # --- assertions -------------------------------------------------------------------------------
  expect_rc() { # <id> zero|nonzero
    if [[ "$2" == zero && "${RC}" -ne 0 ]]; then
      case_fail "$1" "runner exited ${RC}, want 0; log: $(tail -3 "${CASE_DIR}/log" | tr '\n' '|')"
    elif [[ "$2" == nonzero && "${RC}" -eq 0 ]]; then
      case_fail "$1" "runner exited 0, want non-zero; log: $(tail -3 "${CASE_DIR}/log" | tr '\n' '|')"
    fi
  }
  expect_calls() { # <id> <n>
    [[ "${CALLS}" == "$2" ]] || case_fail "$1" "go was called ${CALLS} times, want $2"
  }
  expect_argv() { # <id> <target> <pkg>: every recorded call's argv is exact
    local i want got
    want="$(printf '%s\n' test '-run=^$' "-fuzz=$2" -fuzztime=30s "$3")"
    for ((i = 1; i <= CALLS; i++)); do
      got="$(cat "${CASE_DIR}/go/argv.${i}")"
      [[ "${got}" == "${want}" ]] ||
        case_fail "$1" "go call ${i} argv is '$(tr '\n' ' ' <<<"${got}")', want '$(tr '\n' ' ' <<<"${want}")'"
    done
  }
  expect_diag() { # <id> <diagnostic or empty>: the runner's ::error::fuzz-retry: lines, exactly
    local got want=""
    got="$(grep '^::error::fuzz-retry:' "${CASE_DIR}/log" || true)"
    [[ -n "$2" ]] && want="::error::fuzz-retry: $2"
    [[ "${got}" == "${want}" ]] ||
      case_fail "$1" "diagnostic is '$(printf '%s' "${got}" | tr '\n' '|')', want '${want}'"
  }
  expect_flake_warning() { # <id>
    grep '^::warning::fuzz-retry:.*context deadline exceeded.*golang/go#75804' "${CASE_DIR}/log" >/dev/null ||
      case_fail "$1" "no ::warning::fuzz-retry: line naming the deadline flake (context deadline exceeded, golang/go#75804)"
  }
  done_case() { # <label>: ok only when the case added no failure since run_case
    [[ "${BAD}" -ne "${CASE_BAD}" ]] || pass "$1"
  }

  ARGS=()

  # --- F0: argument validation (the target is put into the flake regex) ---------------------------
  for bad in 'Fuzz.*|./internal/aggregate/...' 'FuzzContentHash'; do
    ATTEMPTS=("0:${PASSFX}")
    IFS='|' read -r -a ARGS <<<"${bad}"
    run_case F0 x x
    [[ "${RC}" -eq 2 ]] || case_fail F0 "runner exited ${RC} on arguments '${bad}', want 2"
    expect_calls F0 0
    expect_diag F0 "usage: fuzz-retry.sh <FuzzTarget> <pkg>"
    done_case "F0 (${bad})"
  done

  # F0: an empty pkg (the matrix value missing): usage error, go never called.
  ATTEMPTS=("0:${PASSFX}")
  ARGS=(FuzzContentHash "")
  run_case F0 x x
  [[ "${RC}" -eq 2 ]] || case_fail F0 "runner exited ${RC} on an empty pkg, want 2"
  expect_calls F0 0
  expect_diag F0 "usage: fuzz-retry.sh <FuzzTarget> <pkg>"
  done_case "F0 (empty pkg)"

  # --- F1: the deadline flake then a pass, for both captured runs --------------------------------
  for t in FuzzContentHash:./internal/aggregate/... FuzzValidateAttributePath:./internal/collect/...; do
    tgt="${t%%:*}" pkg="${t#*:}"
    ATTEMPTS=("1:${FIX}/deadline-${tgt}.txt" "0:${FIX}/pass-${tgt}.txt")
    run_case F1 "${tgt}" "${pkg}"
    expect_rc F1 zero
    expect_calls F1 2
    expect_argv F1 "${tgt}" "${pkg}"
    expect_diag F1 ""
    expect_flake_warning F1
    done_case "F1 (${tgt})"
  done

  # A deadline variant as attempt 1, the pass as attempt 2 (so a wrongful retry turns green).
  T=FuzzContentHash P=./internal/aggregate/...
  one_attempt_red() { # <id> <variant file> <diagnostic or empty>
    ATTEMPTS=("1:$2" "0:${PASSFX}")
    run_case "$1" "${T}" "${P}"
    expect_rc "$1" nonzero
    expect_calls "$1" 1
    expect_argv "$1" "${T}" "${P}"
    expect_diag "$1" "$3"
    done_case "$1"
  }
  new_tmp
  VAR="${TMP}"

  # --- F2-F4: deny markers ----------------------------------------------------------------------
  variant '{print} /context deadline exceeded/ {print "        panic: runtime error: index out of range [3] with length 3"}' >"${VAR}/f2"
  one_attempt_red F2 "${VAR}/f2" "deny marker panic:"
  awk '{print} /^PASS$/ {print "panic: logged by a passing attempt"}' "${PASSFX}" >"${VAR}/f2pass"
  ATTEMPTS=("0:${VAR}/f2pass" "0:${PASSFX}")
  run_case F2 "${T}" "${P}"
  expect_rc F2 nonzero
  expect_calls F2 1
  expect_diag F2 "deny marker panic:"
  done_case "F2 (exit 0)"
  variant '{print} END {print "signal: killed"}' >"${VAR}/f3"
  one_attempt_red F3 "${VAR}/f3" "deny marker signal: killed"
  variant '{print} /context deadline exceeded/ {print "fatal error: concurrent map writes"}' >"${VAR}/f4"
  one_attempt_red F4 "${VAR}/f4" "deny marker fatal error:"

  # --- F5: corpus ---------------------------------------------------------------------------------
  CORPUS="internal/aggregate/testdata/fuzz/${T}/abc"
  one_attempt_red F5a "${DEADLINE}" "crash corpus under testdata/fuzz/"
  [[ -f "${CASE_DIR}/repo/${CASE_CORPUS}" ]] || fail "F5a: the fake go did not write ${CASE_CORPUS}; nothing was tested"
  variant '{print} /context deadline exceeded/ {print "    Failing input written to testdata/fuzz/FuzzContentHash/abc"}' >"${VAR}/f5b"
  one_attempt_red F5b "${VAR}/f5b" "deny marker Failing input written"
  GIT_STATUS_FAILS=1
  one_attempt_red F5c "${DEADLINE}" "git status failed"

  # --- F18: the real crash output (crash-local.txt, a panicking target), never retried -----------
  [[ -s "${FIX}/crash-local.txt" ]] || fail "fixture crash-local.txt is missing or empty"
  one_attempt_red F18 "${FIX}/crash-local.txt" "deny marker panic:"

  # --- F6: the deadline twice -----------------------------------------------------------------------
  ATTEMPTS=("1:${DEADLINE}" "1:${DEADLINE}" "0:${PASSFX}")
  run_case F6 "${T}" "${P}"
  expect_rc F6 nonzero
  expect_calls F6 2
  expect_argv F6 "${T}" "${P}"
  expect_diag F6 "deadline flake on both attempts"
  done_case F6

  # --- F17: the exact flake text under an exit status other than 1 ----------------------------------
  # go test exits 1 for a failed test; 2 (build or usage), 137 (SIGKILL) and others are not the
  # flake, whatever the output says.
  for code in 2 137; do
    ATTEMPTS=("${code}:${DEADLINE}" "0:${PASSFX}")
    run_case F17 "${T}" "${P}"
    expect_rc F17 nonzero
    expect_calls F17 1
    expect_diag F17 "unrecognised failure text"
    done_case "F17 (exit ${code})"
  done

  # --- F7: empty and unknown output -----------------------------------------------------------------
  : >"${VAR}/empty"
  one_attempt_red F7 "${VAR}/empty" "unrecognised failure text"
  printf 'go: something unexpected happened\nFAIL\n' >"${VAR}/unknown"
  one_attempt_red F7 "${VAR}/unknown" "unrecognised failure text"

  # --- F8: zero fuzz targets (exit 0) -----------------------------------------------------------------
  ATTEMPTS=("0:${FIX}/zero-target.txt" "0:${PASSFX}")
  run_case F8 "${T}" "${P}"
  expect_rc F8 nonzero
  expect_calls F8 1
  expect_diag F8 "no fuzz target ran"
  done_case F8

  # --- F9: exit 0 without a fuzz: elapsed line and without the zero-target warning -------------------
  grep -v '^fuzz: ' "${PASSFX}" >"${VAR}/f9"
  ATTEMPTS=("0:${VAR}/f9" "0:${PASSFX}")
  run_case F9 "${T}" "${P}"
  expect_rc F9 nonzero
  expect_calls F9 1
  expect_diag F9 "unrecognised failure text"
  done_case F9

  # --- F10: control, a plain pass ---------------------------------------------------------------------
  ATTEMPTS=("0:${PASSFX}")
  run_case F10 "${T}" "${P}"
  expect_rc F10 zero
  expect_calls F10 1
  expect_argv F10 "${T}" "${P}"
  expect_diag F10 ""
  done_case F10

  # --- F11: ci.yaml's fuzz job calls the runner -------------------------------------------------------
  CASE_BAD="${BAD}"
  # shellcheck disable=SC2016 # the ${...} below are literal workflow text, not shell expansions
  want_run='hack/ci/fuzz-retry.sh "${FUZZ_TARGET}" "${FUZZ_PKG}"'
  runner_steps="$(yq eval '[.jobs.fuzz.steps[] | select((.run // "") | test("fuzz-retry\.sh"))] | length' "${CI}")"
  if [[ "${runner_steps}" != 1 ]]; then
    case_fail F11 "the fuzz job has ${runner_steps} steps calling hack/ci/fuzz-retry.sh, want 1"
  else
    step='.jobs.fuzz.steps[] | select((.run // "") | test("fuzz-retry\.sh"))'
    got_run="$(yq eval "${step} | .run" "${CI}" | sed -e 's/[[:space:]]*$//' -e '/^$/d')"
    [[ "${got_run}" == "${want_run}" ]] ||
      case_fail F11 "the fuzz step runs '${got_run}', want '${want_run}'"
    # shellcheck disable=SC2016 # literal workflow expressions
    [[ "$(yq eval "${step} | .env.FUZZ_TARGET" "${CI}")" == '${{ matrix.target }}' ]] ||
      case_fail F11 "the fuzz step's env.FUZZ_TARGET is not \${{ matrix.target }}"
    # shellcheck disable=SC2016 # literal workflow expressions
    [[ "$(yq eval "${step} | .env.FUZZ_PKG" "${CI}")" == '${{ matrix.pkg }}' ]] ||
      case_fail F11 "the fuzz step's env.FUZZ_PKG is not \${{ matrix.pkg }}"
  fi
  [[ -x "${RUNNER}" ]] ||
    case_fail F11 "${RUNNER#"${ROOT}"/} is not executable; the fuzz step runs it directly"
  # An `if:` or `continue-on-error:` would skip or soften the gate: the job keeps exactly its
  # changes gate, and no step but the download one (F16's) carries either key (precedent:
  # dist_operatorhub_pr_test.sh reads continue-on-error the same way).
  job_if="$(yq eval '.jobs.fuzz.if // ""' "${CI}")"
  [[ "${job_if}" == "needs.changes.outputs.code == 'true'" ]] ||
    case_fail F11 "the fuzz job's if: is '${job_if}', want exactly needs.changes.outputs.code == 'true'"
  [[ "$(yq eval '.jobs.fuzz | has("continue-on-error")' "${CI}")" == false ]] ||
    case_fail F11 "the fuzz job sets continue-on-error:"
  # Counted, not joined from names: a step named "" would join to an empty string.
  soft_sel='[.jobs.fuzz.steps[] | select((.run // "") | sub("\s+$"; "") != "go mod download") | select(has("if") or has("continue-on-error"))]'
  [[ "$(yq eval "${soft_sel} | length" "${CI}")" == 0 ]] ||
    case_fail F11 "fuzz job steps set if: or continue-on-error: ($(yq eval "${soft_sel} | map(.name // .uses // .run) | join(\", \")" "${CI}"))"
  inline="$(yq eval '[.jobs.fuzz.steps[] | select((.run // "") | test("for attempt|go test"))] | length' "${CI}")"
  [[ "${inline}" == 0 ]] ||
    case_fail F11 "the fuzz job still runs go test or a 'for attempt' loop inline (${inline} steps)"
  done_case F11

  # --- F16: modules download in their own step, after go-cache and before the runner --------------
  # A cold module cache prints `go: downloading` lines, which the runner's exact flake sequence
  # does not allow; a separate, uncaptured step keeps them out of the output it judges.
  CASE_BAD="${BAD}"
  idx() { # <yq select expression>: the indexes of the fuzz job's steps that match, one per line
    yq eval ".jobs.fuzz.steps | to_entries | .[] | select($1) | .key" "${CI}"
  }
  dl="$(idx '(.value.run // "") | sub("\s+$"; "") == "go mod download"')"
  cache="$(idx '(.value.uses // "") == "./.github/actions/go-cache"')"
  runner="$(idx '(.value.run // "") | test("fuzz-retry\.sh")')"
  if [[ "$(wc -w <<<"${dl}")" -ne 1 ]]; then
    case_fail F16 "the fuzz job has $(wc -w <<<"${dl}") steps running exactly 'go mod download', want 1"
  elif [[ "$(wc -w <<<"${cache}")" -ne 1 || "$(wc -w <<<"${runner}")" -ne 1 ]]; then
    case_fail F16 "the fuzz job needs one go-cache step and one runner step to order the download against"
  elif ((dl <= cache || dl >= runner)); then
    case_fail F16 "the 'go mod download' step is step ${dl}, want it after go-cache (${cache}) and before the runner (${runner})"
  elif [[ "$(yq eval ".jobs.fuzz.steps[${dl}] | (has(\"if\") or has(\"continue-on-error\"))" "${CI}")" != false ]]; then
    case_fail F16 "the 'go mod download' step sets if: or continue-on-error:; a skipped or failed download lets go test fetch modules inside the judged output"
  fi
  done_case F16

  # --- F12-F15: the exact, ordered, complete line sequence for the matrix target -----------------------
  variant '{print} /context deadline exceeded/ {print "    fuzzing process hung or terminated unexpectedly"}' >"${VAR}/f12"
  one_attempt_red F12 "${VAR}/f12" "unrecognised failure text"
  variant '{sub(/^--- FAIL: FuzzContentHash /, "--- FAIL: FuzzOther ")} {print}' >"${VAR}/f13"
  grep '^--- FAIL: FuzzOther ' "${VAR}/f13" >/dev/null || fail "F13 fixture did not rename the target"
  one_attempt_red F13 "${VAR}/f13" "unrecognised failure text"
  head -n -2 "${DEADLINE}" >"${VAR}/f14"
  [[ "$(tail -1 "${VAR}/f14")" == FAIL ]] || fail "F14 fixture is not cut before 'exit status 1'"
  one_attempt_red F14 "${VAR}/f14" "unrecognised failure text"
  variant '{print} /context deadline exceeded/ {print}' >"${VAR}/f15a"
  one_attempt_red F15 "${VAR}/f15a" "unrecognised failure text"
  variant '/context deadline exceeded/ {held = $0; next} held != "" && $0 == "FAIL" {print; print held; held = ""; next} {print}' >"${VAR}/f15b"
  [[ "$(grep -v '^fuzz: ' "${VAR}/f15b" | sed -n 2p)" == FAIL ]] || fail "F15 fixture did not swap the lines"
  one_attempt_red F15 "${VAR}/f15b" "unrecognised failure text"

  [[ "${BAD}" -eq 0 ]] || fail "the fuzz runner breaks TDS-1 (cases above)"
  pass "the fuzz runner retries only the deadline flake and is never green on nothing (F0-F18)"
  exit 0
fi

# ---------------------------------------------------------------------------
# Self-test: copy the runner and ci.yaml into a fresh root, break one clause per copy, and assert
# the check reds on the case built to catch it (its `FAIL: <case>:` line, not just a non-zero
# exit). One unmutated copy must pass.
# ---------------------------------------------------------------------------
new_copy() {
  new_tmp
  COPY="${TMP}"
  mkdir -p "${COPY}/hack/ci" "${COPY}/.github/workflows"
  cp "${RUNNER}" "${COPY}/hack/ci/fuzz-retry.sh"
  cp "${CI}" "${COPY}/.github/workflows/ci.yaml"
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

# mutant_rejected <label> <file under the copy> <old> <new> <case id...>
mutant_rejected() {
  local label="$1" rel="$2" old="$3" new="$4" out rc id
  shift 4
  new_copy
  replace_once "${COPY}/${rel}" "${old}" "${new}" ||
    fail "self-test: the '${label}' mutant did not apply (anchor missing or not unique in ${rel}) -- nothing was tested"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] || fail "self-test: the check accepted the '${label}' mutant (exit 0)"
  for id in "$@"; do
    grep "^FAIL: ${id}:" <<<"${out}" >/dev/null ||
      fail "self-test: the '${label}' mutant did not red case ${id}; got: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
  done
  rm -rf "${COPY:?}"
  pass "self-test: ${label} rejected by $*"
}

new_copy
if ! out="$(bash "${SELF}" "${COPY}" 2>&1)"; then
  fail "self-test: the no-op control failed on an unmutated copy -- fix the real tree first: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
fi
rm -rf "${COPY:?}"
pass "self-test: no-op control passes on an unmutated copy"

R=hack/ci/fuzz-retry.sh
# 1.4: retry-decision defects.
mutant_rejected "deny marker panic: dropped" "${R}" "'panic:' " "" F2
mutant_rejected "deny marker signal: killed dropped" "${R}" " 'signal: killed'" "" F3
mutant_rejected "deny marker fatal error: dropped" "${R}" " 'fatal error:'" "" F4
mutant_rejected "deny marker Failing input written dropped" "${R}" " 'Failing input written'" "" F5b
mutant_rejected "corpus check dropped" "${R}" \
  "grep -F 'testdata/fuzz/' <<<\"\${status}\" >/dev/null" "false" F5a
mutant_rejected "--untracked-files=all dropped" "${R}" " --untracked-files=all" "" F5a
mutant_rejected "failing git status ignored" "${R}" '|| { diag "git status failed"; exit 1; }' "|| true" F5c
mutant_rejected "retry cap raised" "${R}" "max_attempts=2" "max_attempts=3" F6
mutant_rejected "deny markers only on a failed attempt" "${R}" \
  'if grep -F -- "${marker}" "${out}" >/dev/null; then' \
  'if [[ "${rc}" -ne 0 ]] && grep -F -- "${marker}" "${out}" >/dev/null; then' F2
mutant_rejected "target not validated" "${R}" ' || ! "${1}" =~ ^Fuzz[A-Za-z0-9_]*$' "" F0
mutant_rejected "argument count not checked" "${R}" '"$#" -ne 2 || ' "" F0
mutant_rejected "empty pkg accepted" "${R}" ' || -z "${2}"' "" F0
mutant_rejected "deny markers matched at line start only" "${R}" \
  'if grep -F -- "${marker}" "${out}" >/dev/null; then' \
  'if grep -E -- "^[[:space:]]*${marker}" "${out}" >/dev/null; then' F18
# 1.5: output-shape defects.
mutant_rejected "zero-target check dropped" "${R}" \
  "grep -F 'no fuzz tests to fuzz' \"\${out}\" >/dev/null" "false" F8
mutant_rejected "fuzz: elapsed check dropped" "${R}" \
  "grep '^fuzz: elapsed' \"\${out}\" >/dev/null" "true" F9
mutant_rejected "unrecognised diagnostic dropped" "${R}" "diag 'unrecognised failure text'" ":" F7
mutant_rejected "-fuzztime=1s" "${R}" '-fuzztime=30s "${pkg}"' '-fuzztime=1s "${pkg}"' F1 F10
mutant_rejected "allow-list replaced by a substring match" "${R}" \
  'is_deadline_flake "${out}" ||' "grep -F 'context deadline exceeded' \"\${out}\" >/dev/null ||" F12
mutant_rejected "any non-zero exit retried" "${R}" '[[ "${rc}" -eq 1 ]] || unrecognised' ":" F17
mutant_rejected "target pin dropped" "${R}" '"^--- FAIL: ${target} ' '"^--- FAIL: Fuzz[A-Za-z0-9_]* ' F13
mutant_rejected "every line of the sequence no longer required" "${R}" \
  '[[ "${#got[@]}" -eq "${#want[@]}" ]] || return 1' ":" F14
mutant_rejected "membership instead of the ordered sequence" "${R}" \
  '[[ "${got[i]}" =~ ${want[i]} ]] || return 1' \
  '{ hit=0; for w in "${want[@]}"; do [[ "${got[i]}" =~ ${w} ]] && hit=1; done; [[ "${hit}" == 1 ]]; } || return 1' F15
# F11: the job going back to an inline go test.
# shellcheck disable=SC2016 # literal workflow text
mutant_rejected "fuzz job bypasses the runner" .github/workflows/ci.yaml \
  'hack/ci/fuzz-retry.sh "${FUZZ_TARGET}" "${FUZZ_PKG}"' \
  'go test -run='"'"'^$'"'"' -fuzz="${FUZZ_TARGET}" -fuzztime=30s "${FUZZ_PKG}"' F11

# F16: the module download step dropped, or moved out from between go-cache and the runner.
mutant_rejected "module download step dropped" .github/workflows/ci.yaml \
  "run: go mod download" "run: go version" F16
# shellcheck disable=SC2016 # literal workflow text
DL_STEP=$'      - name: Download Go modules\n        run: go mod download\n'
# shellcheck disable=SC2016 # literal workflow text
RUN_LINE='        run: hack/ci/fuzz-retry.sh "${FUZZ_TARGET}" "${FUZZ_PKG}"'
# shellcheck disable=SC2016 # literal workflow text
CACHE_STEP=$'      - uses: ./.github/actions/go-cache\n        with:\n          cache-suffix: fuzz-${{ matrix.target }}'
for where in after-runner before-cache; do
  new_copy
  replace_once "${COPY}/.github/workflows/ci.yaml" "${DL_STEP}" "" ||
    fail "self-test: the module download step is not the expected two lines -- nothing was tested"
  if [[ "${where}" == after-runner ]]; then
    replace_once "${COPY}/.github/workflows/ci.yaml" "${RUN_LINE}" "${RUN_LINE}"$'\n'"${DL_STEP%$'\n'}"
  else
    replace_once "${COPY}/.github/workflows/ci.yaml" "${CACHE_STEP}" "${DL_STEP}${CACHE_STEP}"
  fi || fail "self-test: the ${where} anchor is missing or not unique in ci.yaml -- nothing was tested"
  out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] || fail "self-test: the check accepted the module download step ${where} (exit 0)"
  grep '^FAIL: F16:.*want it after go-cache' <<<"${out}" >/dev/null ||
    fail "self-test: the download-${where} mutant did not red F16; got: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
  [[ "$(grep -c '^FAIL: F[0-9]*[a-z]*:' <<<"${out}")" -eq 1 ]] ||
    fail "self-test: the download-${where} mutant red more than F16: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
  rm -rf "${COPY:?}"
  pass "self-test: module download step ${where} rejected by F16"
done

# F11 / F16: an if: or continue-on-error: that skips or softens the gate (5.2).
mutant_rejected "runner step continue-on-error" .github/workflows/ci.yaml \
  "${RUN_LINE}" "${RUN_LINE}"$'\n        continue-on-error: true' F11
mutant_rejected "runner step if" .github/workflows/ci.yaml "${RUN_LINE}" "${RUN_LINE}"$'\n        if: false' F11
mutant_rejected "unnamed runner step continue-on-error" .github/workflows/ci.yaml \
  $'      - name: Native Go fuzz (30s)\n' $'      - name: ""\n        continue-on-error: true\n' F11
mutant_rejected "job continue-on-error" .github/workflows/ci.yaml \
  $'    name: fuzz\n' $'    name: fuzz\n    continue-on-error: true\n' F11
mutant_rejected "job if widened" .github/workflows/ci.yaml \
  $'    name: fuzz\n    needs: [changes]\n    if: needs.changes.outputs.code == \'true\'' \
  $'    name: fuzz\n    needs: [changes]\n    if: always()' F11
# shellcheck disable=SC2016 # literal workflow text
mutant_rejected "go-cache step if" .github/workflows/ci.yaml \
  'cache-suffix: fuzz-${{ matrix.target }}' 'cache-suffix: fuzz-${{ matrix.target }}'$'\n        if: false' F11
mutant_rejected "download step continue-on-error" .github/workflows/ci.yaml \
  '        run: go mod download' $'        run: go mod download\n        continue-on-error: true' F16
mutant_rejected "download step if" .github/workflows/ci.yaml \
  '        run: go mod download' $'        run: go mod download\n        if: false' F16

# F11: the runner loses its exec bit.
new_copy
chmod -x "${COPY}/hack/ci/fuzz-retry.sh"
out="$(bash "${SELF}" "${COPY}" 2>&1)" && rc=0 || rc=$?
[[ "${rc}" -ne 0 ]] || fail "self-test: the check accepted a runner without its exec bit (exit 0)"
grep '^FAIL: F11:.*is not executable' <<<"${out}" >/dev/null ||
  fail "self-test: the exec-bit mutant did not red F11; got: $(grep '^FAIL' <<<"${out}" | head -3 | tr '\n' ' ')"
rm -rf "${COPY:?}"
pass "self-test: runner without exec bit rejected by F11"

echo "All fuzz runner self-tests passed."
