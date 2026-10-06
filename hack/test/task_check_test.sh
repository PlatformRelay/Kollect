#!/usr/bin/env bash
# TCE-1..TCE-4 -- meta-test for openspec change task-check-entrypoint.
#
# What it locks:
#   TCE-1  task check runs every required CI gate that can run locally, every hack/test guard
#          in every mode its code declares (a new guard is picked up by the glob with no edit),
#          and reports a failing gate and exits non-zero instead of letting the last gate pass.
#          A guard whose header declares a Docker requirement is skipped with a printed reason
#          -- sweeping it unconditionally reds the no-Docker machine TCE-1 promises to serve.
#   TCE-2  task verify keeps its one meaning: it still runs exactly `bash hack/verify.sh` and
#          is not redefined as a subset or a superset of check.
#   TCE-3  every required gate in verify-eligibility.sh's required_checks is either run by
#          hack/check.sh or listed in its exclusion table WITH a reason; coverage mappings
#          (comment lines `coverage: <context>=<gate>,<gate>`) hold only while every gate they
#          name is still declared.
#   TCE-4  --self-test mutates a copy of the tree: each of TCE-1..TCE-3 is broken by one
#          mutant, each check reds with the message of the assertion the mutation was built
#          to trip, and one unmutated no-op copy passes.
#
# Usage:
#   hack/test/task_check_test.sh              # check the repo tree (CI plain mode)
#   hack/test/task_check_test.sh --self-test  # mutate copies, assert each check fails
#   hack/test/task_check_test.sh <tree-root>  # check an arbitrary tree root
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

if ! command -v yq >/dev/null 2>&1; then
  echo "yq not found; install yq (mikefarah/yq v4) to run this test" >&2
  exit 1
fi

TASKFILE="${ROOT}/Taskfile.yml"
CHECKSH="${ROOT}/hack/check.sh"
ELIG="${ROOT}/hack/release/verify-eligibility.sh"
CI="${ROOT}/.github/workflows/ci.yaml"

[[ -f "${TASKFILE}" ]] || fail "TCE-1: Taskfile not found: ${TASKFILE}"
[[ -f "${CHECKSH}" ]] || fail "TCE-1: ${CHECKSH} not found"
[[ -f "${ELIG}" ]] || fail "TCE-3: required-checks source not found: ${ELIG}"

# Run bodies and command lists are matched against a COMMENT-STRIPPED view: a commented-out
# command is not a command (the convention from ci_docs_gate_test.sh).
strip_comments() {
  sed -e 's/[[:space:]]*#.*$//' <<<"$1" | grep -v '^$' || true
}

# The gates the local check runs (the required contexts plus the local-only lints), each with
# its EXACT command: a name-only pin would let `run_gate verify true` pass, so the command is
# part of the contract.
declare -A GATE_COMMAND=(
  [verify]="task verify"
  [lint]="task lint"
  [vulncheck]="task vulncheck"
  [test]="task test"
  [build]="task build"
  [audit-rbac]="task audit:rbac"
  [helm]="task helm-test"
  [gitleaks]="bash hack/install-gitleaks.sh ./bin"
  [workflow-security]="bash hack/install-zizmor.sh ./bin"
  [scrub]="task scrub"
  [spec:validate]="task spec:validate"
  [lint:shell]="task lint:shell"
  [lint:markdown]="task lint:markdown"
  [go-mod]="bash -c 'go mod tidy -diff && go mod verify'"
  [format:check]="task format:check"
)

# The Taskfile declares a check task and it runs the orchestrator, not a local reimplementation.
c_tce1_task() {
  local has_check cmds
  has_check="$(yq eval '.tasks | has("check")' "${TASKFILE}")"
  [[ "${has_check}" == "true" ]] ||
    fail "TCE-1: Taskfile.yml has no check task -- there is no single local command that equals CI; add one that runs hack/check.sh"
  cmds="$(yq eval '.tasks.check.cmds // [] | join("\n")' "${TASKFILE}")"
  strip_comments "${cmds}" | grep -q 'bash hack/check.sh' ||
    fail "TCE-1: the check task must run hack/check.sh (got: ${cmds}) -- the gate's logic lives in one reviewable script that this guard pins; a Taskfile-native reimplementation is a second definition that can drift"
  pass "TCE-1: task check exists and runs hack/check.sh"
}

# Every gate appears with its EXACT command; the sweep is the glob; the --self-test mode is
# run where a guard's code parses it; the gitleaks/zizmor invocations match CI's.
c_tce1_gates() {
  local body gate
  body="$(strip_comments "$(cat "${CHECKSH}")")"
  for gate in "${!GATE_COMMAND[@]}"; do
    if ! grep -qF "run_gate ${gate} ${GATE_COMMAND[${gate}]}" <<<"${body}"; then
      fail "TCE-1: hack/check.sh does not run the '${gate}' gate with its exact command ('run_gate ${gate} ${GATE_COMMAND[${gate}]}') -- the command is part of the contract, so a name-only stub or a swapped invocation reds here"
    fi
  done
  grep -qF 'gitleaks detect --source . --config .github/gitleaks.toml' <<<"${body}" ||
    fail "TCE-1: hack/check.sh must run the same gitleaks detect invocation as CI's gitleaks job -- a local gate that skips secret scanning is not the full gate"
  grep -qF 'zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml .github/' <<<"${body}" ||
    fail "TCE-1: hack/check.sh must run the pinned offline zizmor audit over .github/ exactly as ci.yaml's workflow-security job does -- a local gate that never audits the workflows is not the full gate"
  grep -qF 'hack/test/*_test.sh' <<<"${body}" ||
    fail "TCE-1: hack/check.sh must sweep every hack/test guard through the glob (a new guard then runs with no edit to the script); a hardcoded guard list rots the moment someone adds a guard and forgets it"
  grep -qF -- '--self-test' <<<"${body}" ||
    fail "TCE-1: hack/check.sh must run the guards' --self-test mode where the script's code declares it -- a mode CI runs that task check does not is a mode that can rot locally"
  # A guard whose header declares a Docker requirement must NOT be swept: on the no-Docker
  # machine this gate promises to serve it would fail for the wrong reason. It must be
  # skipped with a printed reason instead.
  if ! grep -qF 'requires docker' <<<"${body}"; then
    fail "TCE-1: hack/check.sh must skip Docker-requiring guards (their header declares it) with a printed reason -- sweeping them unconditionally makes task check red on exactly the no-Docker machine TCE-1 promises it serves"
  fi
  pass "TCE-1: the local gate runs the required runnable gates and the full guard sweep"
}

# A failed gate must be reported and must make the final exit non-zero; the last gate passing
# never masks an earlier failure.
c_tce1_aggregation() {
  local body
  body="$(strip_comments "$(cat "${CHECKSH}")")"
  grep -qF 'failures+=' <<<"${body}" ||
    fail "TCE-1: hack/check.sh does not aggregate failures -- a gate that fails must be collected, reported and turn the exit non-zero"
  grep -qE 'exit 1$' <<<"${body}" ||
    fail "TCE-1: hack/check.sh has no failing exit -- a failed gate must turn the exit non-zero so a caller cannot mistake a red gate for a green one"
  grep -qF 'set -uo pipefail' <<<"${body}" ||
    fail "TCE-1: hack/check.sh must run with the failure-collecting shell mode (set -uo pipefail, deliberately NOT set -e) -- set -e would stop at the first gate and hide the ones after it"
  # A duplicated run_gate line runs the gate twice per check and silently doubles the runtime;
  # each gate label must appear exactly once.
  for gate in "${!GATE_COMMAND[@]}"; do
    if [[ $(grep -cF "run_gate ${gate} " <<<"${body}") -ne 1 ]]; then
      fail "TCE-1: the '${gate}' gate is declared $(grep -cF "run_gate ${gate} " <<<"${body}") times in hack/check.sh, expected exactly once -- a duplicated gate line runs the gate twice per task check and two copies drift apart"
    fi
  done
  pass "TCE-1: a failed gate is reported and turns the exit non-zero (the last gate cannot mask the rest)"
}

# TCE-2: verify keeps its one meaning -- exactly bash hack/verify.sh, nothing added (a
# superset redefinition, verify.sh plus extra commands, is still a redefinition).
c_tce2_verify() {
  local ncmds desc
  ncmds="$(yq eval '.tasks.verify.cmds | length' "${TASKFILE}")"
  [[ "${ncmds}" == "1" ]] ||
    fail "TCE-2: the verify task declares ${ncmds} command(s), expected exactly 1 (bash hack/verify.sh) -- a superset redefinition (verify plus extra commands) is still a redefinition of the one task name whose meaning must not drift"
  local cmds
  cmds="$(strip_comments "$(yq eval '.tasks.verify.cmds // [] | join("\n")' "${TASKFILE}")")"
  grep -qF 'bash hack/verify.sh' <<<"${cmds}" ||
    fail "TCE-2: the verify task no longer runs bash hack/verify.sh -- task verify is the generated-artifact drift check and must not be redefined as a subset or superset of check"
  desc="$(yq eval '.tasks.verify.desc // ""' "${TASKFILE}")"
  grep -qF 'generated artifacts' <<<"${desc}" ||
    fail "TCE-2: the verify task's desc is '${desc}', expected to still promise the generated-artifact drift check -- the one task name with a meaning that must not drift"
  pass "TCE-2: task verify still means the generated-artifact drift check"
}

# TCE-3: every required check is runnable, mapped, or excluded with a reason.
c_tce3_coverage() {
  local names body name
  names="$(awk '/^required_checks=\(/{f=1;next} /^\)/{f=0} f' "${ELIG}" | tr -s ' \t' '\n' | grep -v '^$' || true)"
  [[ -n "${names}" ]] ||
    fail "TCE-3: no required_checks could be parsed from hack/release/verify-eligibility.sh -- the list of CI gates is what TCE-3 checks against"
  body="$(strip_comments "$(cat "${CHECKSH}")")"
  local gate command
  for gate in "${!GATE_COMMAND[@]}"; do
    command="${GATE_COMMAND[${gate}]}"
    grep -qF "run_gate ${gate} ${command}" <<<"${body}" ||
      fail "TCE-3: hack/check.sh no longer declares the exact local run for the '${gate}' gate ('${command}') -- a required gate must be reachable from task check or listed as an exclusion with a reason"
  done
  # Exclusion lines must carry a reason: `exclusion <name> "<reason>"` with a non-empty tail.
  while IFS= read -r xline; do
    [[ -z "${xline}" ]] && continue
    name="${xline#exclusion }"
    name="${name%% *}"
    local reason="${xline#exclusion ${name}}"
    reason="${reason# }"
    if [[ -z "${reason}" || "${reason}" == '""' ]]; then
      fail "TCE-3: the exclusion for '${name}' has no reason -- an exclusion without its reason is how a required gate goes silently missing from the local gate"
    fi
  done < <(grep -E '^exclusion ' <<<"${body}")
  # Coverage mapping lines are comments in hack/check.sh:
  #   `coverage: <context>=<gate>,<gate>` -- a mapped context is covered when EVERY gate on the
  # right-hand side is declared as a run_gate (guard-sweep = the guard sweep line).
  local coverage_raw cov cov_context cov_gates
  coverage_raw="$(grep -E '^[[:space:]]*#[[:space:]]+coverage: ' "${CHECKSH}" | sed -E 's/^[[:space:]]*#[[:space:]]+coverage:[[:space:]]+//' || true)"
  while IFS= read -r cov; do
    [[ -z "${cov}" ]] && continue
    cov_context="${cov%%=*}"
    cov_gates="${cov#*=}"
    local IFS=','
    read -ra cov_parts <<<"${cov_gates}"
    for gate in "${cov_parts[@]}"; do
      if [[ "${gate}" == "guard-sweep" ]]; then
        grep -qF 'hack/test/*_test.sh' <<<"${body}" ||
          fail "TCE-3: the mapped required check '${cov_context}' relies on the guard sweep, which hack/check.sh no longer declares"
      elif ! grep -qF "run_gate ${gate} " <<<"${body}"; then
        fail "TCE-3: the mapped required check '${cov_context}' relies on the '${gate}' gate, which hack/check.sh no longer declares"
      fi
    done
  done <<<"${coverage_raw}"
  # Coverage: every required context is run, mapped, or excluded.
  while IFS= read -r name; do
    [[ -z "${name}" ]] && continue
    if grep -qF "run_gate ${name} " <<<"${body}" || grep -qF "exclusion ${name} " <<<"${body}"; then
      continue
    fi
    if grep -qF "coverage: ${name}=" "${CHECKSH}"; then
      local rhs comps comp
      rhs="$(grep -oE "coverage: ${name}=[^ ]+" "${CHECKSH}" | head -1 | cut -d= -f2)"
      local IFS=','
      read -ra comps <<<"${rhs}"
      for comp in "${comps[@]}"; do
        if [[ "${comp}" == "guard-sweep" ]]; then
          grep -qF 'hack/test/*_test.sh' <<<"${body}" ||
            fail "TCE-3: the coverage mapping for '${name}' relies on the guard sweep, which hack/check.sh no longer declares"
        elif ! grep -qF "run_gate ${comp} " <<<"${body}"; then
          fail "TCE-3: the coverage mapping for '${name}' names the gate '${comp}', which hack/check.sh does not declare"
        fi
      done
      continue
    fi
    fail "TCE-3: required check '${name}' is neither run by hack/check.sh nor listed as an exclusion with a reason -- it must be reachable from task check or excluded with its reason"
  done <<<"${names}"
  pass "TCE-3: every required check is run locally or excluded with its reason"
}

# TCE-4: this guard must itself be run by ci.yaml in BOTH of its modes -- a gate nobody runs
# guards nothing (the CWS-6 self-wiring analogue; CWS-6's walk cannot see a guard that CI
# never invokes).
c_tce4_selfwiring() {
  local lint_body
  lint_body="$(yq eval '.jobs.lint.steps[] | .run // ""' "${CI}" 2>/dev/null || yq eval '.jobs["lint"].steps[] | .run // ""' "${ROOT}/.github/workflows/ci.yaml")"
  strip_comments "${lint_body}" | grep -q 'bash hack/test/task_check_test.sh$' ||
    fail "TCE-4: this meta-test is not itself run by ci.yaml in its plain mode -- wire it into the lint job (plain and --self-test as separate steps); a gate CI does not run guards nothing"
  strip_comments "${lint_body}" | grep -q 'bash hack/test/task_check_test.sh --self-test' ||
    fail "TCE-4: this meta-test is not itself run by ci.yaml in its --self-test mode -- wire it into the lint job, plain and --self-test as separate steps"
  pass "TCE-4: this guard is wired into the lint job in both modes"
}

if [[ "${MODE}" == "check" ]]; then
  c_tce1_task
  c_tce1_gates
  c_tce1_aggregation
  c_tce2_verify
  c_tce3_coverage
  c_tce4_selfwiring
  echo "All task-check meta-tests passed."
  exit 0
fi

# --- TCE-4: the self-test ----------------------------------------------------
#
# Mutants are applied to a copy of the tree; each one must make the gate RED WITH THE MESSAGE
# of the assertion it was built to trip (a non-zero exit alone is not evidence), plus one
# unmutated no-op copy that must pass.
REAL_ROOT="${ROOT}"

make_copy() {
  local dst="$1"
  rm -rf "${dst}"
  mkdir -p "${dst}"
  cp -R "${REAL_ROOT}/.github" "${dst}/.github"
  mkdir -p "${dst}/hack"
  cp "${REAL_ROOT}/hack/check.sh" "${dst}/hack/check.sh"
  cp "${REAL_ROOT}/Taskfile.yml" "${dst}/Taskfile.yml"
  cp -R "${REAL_ROOT}/hack/test" "${dst}/hack/test"
  cp -R "${REAL_ROOT}/hack/release" "${dst}/hack/release"
  cp -R "${REAL_ROOT}/hack/lib" "${dst}/hack/lib"
  cp -R "${REAL_ROOT}/hack/docs" "${dst}/hack/docs"
}

mutant_rejected() {
  local label="$1" sentinel="$2"
  shift 2
  local copy
  copy="$(mktemp -d)"
  make_copy "${copy}"
  (cd "${copy}" && "$@") ||
    fail "self-test: the mutation step for '${label}' failed -- the mutant was not applied, so nothing was tested"
  local out rc
  out="$(bash "${SELF}" "${copy}" 2>&1)" && rc=0 || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "self-test: the gate accepted the '${label}' mutant (exit 0) -- the assertion '${sentinel}' did not fire"
  echo "${out}" | grep -q -- "${sentinel}" ||
    fail "self-test: the '${label}' mutant red for the WRONG reason; expected the assertion '${sentinel}', got: $(echo "${out}" | head -2 | tr '\n' ' ')"
  rm -rf "${copy}"
  pass "self-test: ${label} rejected with '${sentinel}'"
}

# No-op control: an unmutated copy must pass -- proving the checks read the copy they are
# pointed at and not a hardcoded path.
CONTROL="$(mktemp -d)"
trap 'rm -rf "${CONTROL}"' EXIT
make_copy "${CONTROL}"
if out="$(bash "${SELF}" "${CONTROL}" 2>&1)"; then
  :
else
  fail "self-test: the no-op control failed on an unmutated copy -- the checks themselves are broken: $(echo "${out}" | head -3 | tr '\n' ' ')"
fi
rm -rf "${CONTROL}"
pass "self-test: no-op control passes on an unmutated copy"

# TCE-1: a gate removed from the local run list makes the gate-list check red.
mutant_rejected "TCE-1 gate removed" "does not run the 'vulncheck' gate" bash -c '
  set -euo pipefail
  sed -i.bak "/run_gate vulncheck task vulncheck/d" hack/check.sh && rm -f hack/check.sh.bak
  if grep -q "run_gate vulncheck" hack/check.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-1: a gate swapped for a no-op with the same name -- the EXACT command pin catches it.
mutant_rejected "TCE-1 gate command swapped for a no-op" "with its exact command" bash -c '
  set -euo pipefail
  sed -i.bak "s|run_gate verify task verify|run_gate verify true|" hack/check.sh && rm -f hack/check.sh.bak
  if grep -qF "run_gate verify task verify" hack/check.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-1: the guard sweep hard-coded instead of the glob -- a new guard would never be picked up.
mutant_rejected "TCE-1 guard glob hard-coded" "must sweep every hack/test guard through the glob" bash -c '
  set -euo pipefail
  sed -i.bak "s|for guard in hack/test/\*_test.sh hack/test/lab_harness_meta_suite.sh; do|for guard in hack/test/verify-sha256_test.sh; do|" hack/check.sh && rm -f hack/check.sh.bak
  if grep -q "hack/test/\*_test.sh" hack/check.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-1: a Docker-requiring guard swept unconditionally reds the no-Docker promise.
mutant_rejected "TCE-1 Docker guard swept" "Docker-requiring guards" bash -c '
  set -euo pipefail
  sed -i.bak "s|if grep -qi .*requires docker.*then|if false; then|" hack/check.sh && rm -f hack/check.sh.bak
  if grep -qF "requires docker" hack/check.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-2: verify redefined into something else.
mutant_rejected "TCE-2 verify redefined" "no longer runs bash hack/verify.sh" bash -c '
  set -euo pipefail
  yq eval ".tasks.verify.cmds = [\"task lint\"]" Taskfile.yml > m.yaml && mv m.yaml Taskfile.yml
  if grep -q "bash hack/verify.sh" Taskfile.yml; then echo "mutant did not apply"; exit 1; fi
'

# TCE-2: verify redefined as a SUPERSET (its own check plus an extra one).
mutant_rejected "TCE-2 verify redefined as a superset" "declares 2 command" bash -c '
  set -euo pipefail
  yq eval ".tasks.verify.cmds += [\"task lint\"]" Taskfile.yml > m.yaml && mv m.yaml Taskfile.yml
  n=$(yq eval ".tasks.verify.cmds | length" Taskfile.yml)
  [[ "$n" -ge 2 ]] || { echo "mutant did not apply"; exit 1; }
'

# TCE-3: an exclusion without its reason.
mutant_rejected "TCE-3 exclusion without reason" "has no reason" bash -c '
  set -euo pipefail
  sed -i.bak "s/exclusion test-integration .*/exclusion test-integration \"\"/" hack/check.sh && rm -f hack/check.sh.bak
  if ! grep -q "exclusion test-integration \"\"" hack/check.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-3: a required check with no run, mapping or exclusion — the silent-missing case.
mutant_rejected "TCE-3 new required gate without coverage" "neither run by hack/check.sh nor listed as an exclusion" bash -c '
  set -euo pipefail
  sed -i.bak "s/^	docker-build preflight kind-smoke pipeline-cli-smoke workflow-security$/& unregistered-gate/" hack/release/verify-eligibility.sh && rm -f hack/release/verify-eligibility.sh.bak
  if ! grep -q "unregistered-gate" hack/release/verify-eligibility.sh; then echo "mutant did not apply"; exit 1; fi
'

mutant_rejected "TCE-3 new required gate without coverage" "neither run by hack/check.sh nor listed as an exclusion" bash -c '
  set -euo pipefail
  sed -i.bak "s/^	docker-build preflight kind-smoke pipeline-cli-smoke workflow-security$/& unregistered-gate/" hack/release/verify-eligibility.sh && rm -f hack/release/verify-eligibility.sh.bak
  if ! grep -q "unregistered-gate" hack/release/verify-eligibility.sh; then echo "mutant did not apply"; exit 1; fi
'

# TCE-4: removing this guard from the lint job must red the self-wiring check.
mutant_rejected "TCE-4 guard unwired from lint" "not itself run by ci.yaml" bash -c '
  set -euo pipefail
  yq eval "(.jobs.lint.steps |= map(select(.run // \"\" | test(\"task_check_test.sh\") | not)))" .github/workflows/ci.yaml > m.yaml && mv m.yaml .github/workflows/ci.yaml
  if grep -q "task_check_test" .github/workflows/ci.yaml; then echo "mutant did not apply"; exit 1; fi
'

echo "All task-check self-tests passed."
