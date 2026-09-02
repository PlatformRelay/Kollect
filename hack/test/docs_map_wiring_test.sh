#!/usr/bin/env bash
# Locks hack/test/docs_map_contract_test.sh into a job that actually runs.
#
# DOCS-MAPGATE-02 R2-04: the docs-map gate shipped with no wiring self-lock, unlike its siblings
# repo_root_links_wiring_test.sh and dist_ci_wiring_test.sh. Its `ci.yaml` step could therefore
# be deleted in the same commit that weakened the map, and nothing in the repo would notice --
# the LAB-DEKIND / CI-CONTRIBFILTER-01 / CI-TIPGAP-01 shape this repo keeps re-filing. The gate
# is the ONLY thing that can see a Documentation map label that resolves and lies, so "green
# because it never ran" is the exact failure it must not have.
#
# Two reachability facts have to hold at once, and both are asserted here:
#   1. `ci.yaml`'s `lint` job runs the gate unconditionally. `hack/**` is in neither of ci.yaml's
#      `paths-ignore` blocks, so this is what covers a change to the gate SCRIPT.
#   2. `hack/docs/verify.sh` -- what `task docs:verify` runs, and what the path-filtered Docs
#      workflow reaches -- composes the gate. docs.yaml's `paths:` filter lists `docs/**`, so
#      this is what covers a change to the MAP itself in a docs-only PR.
# Dropping either one narrows the gate to half its change classes, so neither may go quietly.
#
# WHY THE STEP MATCHING IS DELIBERATELY NARROW. `.github/workflows/ci.yaml` is under active
# restructuring (a `changes` classifier job, a `test` -> `test-suite` rename, `if:` gating on
# many jobs; `lint` is deliberately left ungated). A wiring lock that asserted line numbers, job
# ordering, neighbouring steps or the overall job set would red the moment that lands, and a
# wiring lock that reds for an unrelated reason gets deleted. So this file asserts exactly one
# thing about ci.yaml's shape: that SOMEWHERE in the `lint` job there is a bare, uncommented,
# unguarded `bash hack/test/docs_map_contract_test.sh` step that can fail the build. Adding jobs,
# renaming other jobs, and gating other jobs on a classifier are all invisible to it.
#
# Matching follows the GATE-COMMENT-01 / GATE-SCOPE-01 lessons from dist_ci_wiring_test.sh: it is
# LINE-EXACT against a COMMENT-STRIPPED view of the `run:` body, so `# bash ...`, `bash ... ||
# true`, `bash ... &` and a narrowed lookalike are all rejected rather than counted as wiring.
#
# KNOWN GAP, deliberate and recorded. This file is composed into `hack/docs/verify.sh` rather
# than given its own `ci.yaml` step, because the lane that added it was forbidden from touching
# `.github/workflows/**` while the restructure above was in flight. Consequence: a PR that
# deletes ONLY the ci.yaml gate step triggers ci.yaml but not this file. The follow-up is one
# step in the `lint` job, next to the gate's own:
#
#     - name: Verify the docs map gate stays wired into a job that runs
#       run: bash hack/test/docs_map_wiring_test.sh
#
# The self-test at the bottom mutates the real files every run and proves each assertion rejects
# the shape it exists to catch -- including a ci.yaml already carrying the restructure above, to
# prove this lock stays green through it. A gate that has never been watched failing is not a gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"
VERIFY_SCRIPT="${ROOT}/hack/docs/verify.sh"
GATE_SCRIPT="hack/test/docs_map_contract_test.sh"
WIRING_SCRIPT="hack/test/docs_map_wiring_test.sh"
readonly CI_WORKFLOW VERIFY_SCRIPT GATE_SCRIPT WIRING_SCRIPT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the workflow job and trigger definitions"

# The executable half of a step's `run:` body, one trimmed line per element. Every line whose
# first non-blank character is `#` is dropped first, so a commented-out command is not a command.
# `.value` is a `to_entries` entry.
RUN_CODE_LINES='((.value.run // "") | split("\n")
  | map(select(test("^[[:space:]]*#") | not))
  | map(sub("^[[:space:]]*"; "") | sub("[[:space:]]*$"; "")))'
readonly RUN_CODE_LINES

# 0-based index of the ONE step in .jobs.lint.steps that invokes ${script} on a bare, unguarded
# line. Zero matches and duplicates are both hard failures: zero means the wiring is gone,
# neutered or commented out, and a duplicate would let the guards below lock onto the first hit.
#
# `test()` rather than `==`: yq's string equality GLOB-matches, so a literal is a pattern there.
lint_step_index() {
  local workflow="$1" script="$2"
  local escaped predicate query count
  escaped="${script//./\\\\.}"
  predicate="${RUN_CODE_LINES} | any_c(test(\"^bash ${escaped}\$\"))"
  query="$(printf '[.jobs.lint.steps | to_entries[] | select(%s) | .key]' "${predicate}")"
  count="$(yq eval "${query} | length" "${workflow}")"
  case "${count}" in
  1) yq eval "${query} | .[0]" "${workflow}" ;;
  0) fail "the lint job in ${workflow##*/} has no step invoking ${script} on a bare 'bash ${script}' line -- a step in another job does not count, a commented-out command is not a command, and a neutered invocation (trailing '|| true', '&', a redirect, or an 'echo' prefix) cannot fail the build" ;;
  *) fail "the lint job in ${workflow##*/} has ${count} steps invoking ${script} -- keep exactly one so the guards below cannot lock onto the first match" ;;
  esac
}

# A step that is skipped or soft-failed is wired in and still cannot fail the build -- the same
# defect one level down. yq's `//` is not usable here: it treats a literal `false` as absent,
# which is precisely the value being guarded against.
assert_step_can_fail_build() {
  local workflow="$1" idx="$2" label="$3"
  local step_if step_coe

  step_if="$(yq eval ".jobs.lint.steps[${idx}].if" "${workflow}")"
  [[ "${step_if}" == "null" ]] ||
    fail "the ${label} step must run unconditionally, got 'if: ${step_if}' -- a skipped step runs no gate"
  step_coe="$(yq eval ".jobs.lint.steps[${idx}][\"continue-on-error\"]" "${workflow}")"
  [[ "${step_coe}" == "null" || "${step_coe}" == "false" ]] ||
    fail "the ${label} step must not declare 'continue-on-error: ${step_coe}' -- a soft-failed step reports a lying Documentation map as green"
}

# Reachability 1: ci.yaml's lint job runs the gate, and a change to the gate script itself
# triggers that job at all -- `hack/**` must stay out of both `paths-ignore` lists.
check_ci_wiring() {
  local workflow="$1"
  local job_kind steps_kind gate_idx job_if job_coe trigger entry

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  # Without these two guards a renamed job or a stepless lint job makes every selection below
  # run over an empty list, and the assertions pass vacuously.
  job_kind="$(yq eval '.jobs.lint | type' "${workflow}")"
  [[ "${job_kind}" == "!!map" ]] ||
    fail "${workflow} declares no lint job -- the wiring assertions would pass vacuously"
  steps_kind="$(yq eval '.jobs.lint.steps | type' "${workflow}")"
  [[ "${steps_kind}" == "!!seq" ]] ||
    fail "${workflow} lint job declares no steps -- the wiring assertions would pass vacuously"

  job_if="$(yq eval '.jobs.lint.if' "${workflow}")"
  [[ "${job_if}" == "null" ]] ||
    fail "the lint job must run unconditionally, got 'if: ${job_if}' -- a skipped lint job never runs the docs map gate"
  job_coe="$(yq eval '.jobs.lint["continue-on-error"]' "${workflow}")"
  [[ "${job_coe}" == "null" || "${job_coe}" == "false" ]] ||
    fail "the lint job must not declare 'continue-on-error: ${job_coe}' -- a soft-failed lint job turns the docs map gate into an advisory notice"

  gate_idx="$(lint_step_index "${workflow}" "${GATE_SCRIPT}")" || exit 1
  assert_step_can_fail_build "${workflow}" "${gate_idx}" "docs map gate"

  pass "ci.yaml lint job runs the docs map gate unconditionally, on a line that can fail the build"

  # Literal, not glob: GitHub's `*` does not cross `/` but bash's does, so a hand-rolled matcher
  # would be wrong in the direction that passes. Any `paths-ignore` entry naming `hack` at all is
  # a hard failure -- it is the only tree that makes the step above reachable for a change to the
  # gate script.
  for trigger in push pull_request; do
    while IFS= read -r entry; do
      [[ -z "${entry}" ]] && continue
      case "${entry}" in
      hack | hack/*)
        fail "ci.yaml's ${trigger} paths-ignore lists '${entry}' -- a PR that weakens or deletes ${GATE_SCRIPT} would then trigger no CI, and the lint step above would never run on the change it guards"
        ;;
      esac
    done < <(yq eval ".on.${trigger}.paths-ignore[] // \"\"" "${workflow}")
  done
  pass "ci.yaml keeps hack/** out of both paths-ignore lists, so a change to the gate itself reaches lint"
}

# Reachability 2: the composition point the Docs workflow reaches. ci.yaml ignores `docs/**` in
# both paths-ignore blocks, so for a docs-only PR -- a change to the MAP -- `task docs:verify` is
# the only thing that can run this gate at all.
#
# The self-lock lives here too: the assertions above are only load-bearing if THIS script runs.
check_docs_composition() {
  local script="$1"
  local -a code_lines=()
  local line found script_path

  [[ -f "${script}" ]] || fail "expected ${script}"

  # One comment-stripped, trimmed line per element, mirroring RUN_CODE_LINES above so that
  # `# bash ...` and `  bash ...  ` are judged the same way in both files.
  while IFS= read -r line; do
    code_lines+=("${line}")
  done < <(grep -v '^[[:space:]]*#' "${script}" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')

  [[ "${#code_lines[@]}" -gt 0 ]] ||
    fail "${script} has no executable lines -- the composition assertions would pass vacuously"

  for script_path in "${GATE_SCRIPT}" "${WIRING_SCRIPT}"; do
    found=0
    for line in "${code_lines[@]}"; do
      [[ "${line}" == "bash ${script_path}" ]] && found=1
    done
    [[ "${found}" -eq 1 ]] ||
      fail "hack/docs/verify.sh does not invoke ${script_path} on a bare 'bash ${script_path}' line -- the Docs workflow would trigger on a change to docs/index.md and still run no Documentation map check"
  done
  pass "task docs:verify composes the docs map gate and this wiring test on unguarded lines"
}

[[ -s "${ROOT}/${GATE_SCRIPT}" ]] ||
  fail "${GATE_SCRIPT} is missing or empty -- there is nothing for this wiring test to lock in"

check_ci_wiring "${CI_WORKFLOW}"
check_docs_composition "${VERIFY_SCRIPT}"

# ---------------------------------------------------------------------------
# Self-test: mutate the real files and prove each assertion rejects the shape it exists to
# catch. Structural yq mutations, so innocent reformatting cannot spuriously red this.
# ---------------------------------------------------------------------------
MUTANTS="$(mktemp -d)"
trap 'rm -rf "${MUTANTS}"' EXIT

# `${expect}` is the load-bearing half: any nonzero exit would otherwise count as proof, which
# an empty file, an unparseable file and a nonexistent path all produce for the wrong reason.
mutant_rejected() {
  local checker="$1" mutant="$2" original="$3" label="$4" expect="$5"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty -- the mutation step failed, so nothing was actually tested"
  if cmp -s "${mutant}" "${original}"; then
    fail "self-test: the mutant for '${label}' is byte-identical to ${original} -- the mutation was a no-op, so nothing was actually tested"
  fi

  # Subshell: fail's `exit 1` must not take the parent down -- rejection is the expectation.
  output="$( ("${checker}" "${mutant}") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a file where ${label} -- it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects a file where ${label}"
}

# --- ci.yaml: the gate step itself ---
yq eval "
  .jobs.lint.steps = [.jobs.lint.steps[] | select(((.run // \"\") | contains(\"${GATE_SCRIPT}\")) | not)]
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-removed.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-removed.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is deleted from lint' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  .jobs.lint.steps += [.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\"))]
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-duplicated.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-duplicated.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step appears twice in lint' \
  "steps invoking ${GATE_SCRIPT}"

yq eval "
  (.jobs.lint.steps[] | select((.run // \"\") | test(\"^bash ${GATE_SCRIPT//./\\\\.}\$\")) | .run) |= . + \" || true\"
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-or-true.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-or-true.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step swallows failure with || true' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  (.jobs.lint.steps[] | select((.run // \"\") | test(\"^bash ${GATE_SCRIPT//./\\\\.}\$\")) | .run) |= \"# \" + .
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-commented-out.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-commented-out.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step body is commented out' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  with(.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\")); .[\"continue-on-error\"] = true)
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is soft-failed with continue-on-error' \
  'must not declare'

yq eval "
  with(.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\")); .if = \"false\")
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-disabled.yaml" "${CI_WORKFLOW}" \
  'the docs map gate step is disabled with if: false' \
  'must run unconditionally'

# --- ci.yaml: the lint job around it ---
yq eval '.jobs.lint["continue-on-error"] = true' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is soft-failed with continue-on-error' \
  'the lint job must not declare'

yq eval '.jobs.lint.if = "false"' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-disabled.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is disabled with if: false' \
  'the lint job must run unconditionally'

yq eval 'del(.jobs.lint.steps)' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-stepless.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-stepless.yaml" "${CI_WORKFLOW}" \
  'the lint job declares no steps at all' \
  'would pass vacuously'

# --- ci.yaml: the trigger that makes the lint step reachable at all ---
yq eval '.on.pull_request.paths-ignore += ["hack/test/**"]' "${CI_WORKFLOW}" \
  >"${MUTANTS}/hack-path-ignored.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/hack-path-ignored.yaml" "${CI_WORKFLOW}" \
  'ci.yaml starts ignoring hack/test/** on pull_request' \
  'would then trigger no CI'

# --- hack/docs/verify.sh: the composition point, and this file's self-lock ---
sed "s|^bash ${GATE_SCRIPT}\$|# bash ${GATE_SCRIPT}|" "${VERIFY_SCRIPT}" \
  >"${MUTANTS}/verify-gate-removed.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-gate-removed.sh" "${VERIFY_SCRIPT}" \
  'task docs:verify no longer invokes the docs map gate' \
  "does not invoke ${GATE_SCRIPT}"

sed "s|^bash ${WIRING_SCRIPT}\$|# bash ${WIRING_SCRIPT}|" "${VERIFY_SCRIPT}" \
  >"${MUTANTS}/verify-wiring-removed.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-wiring-removed.sh" "${VERIFY_SCRIPT}" \
  'this wiring test is no longer run by task docs:verify' \
  "does not invoke ${WIRING_SCRIPT}"

# --- the GREEN direction that matters: the ci.yaml restructure in flight ---
# A `changes` classifier job, a `test` -> `test-suite` rename, and `if:`/`needs:` gating on
# sibling jobs. `lint` is left ungated, exactly as that lane leaves it. If this lock reddened on
# any of that it would be deleted rather than fixed, so the pass here is an assertion.
yq eval '
  .jobs.changes = {"name": "changes", "runs-on": "ubuntu-latest", "outputs": {"go": "steps.filter.outputs.go"}, "steps": [{"uses": "dorny/paths-filter@v3", "id": "filter"}]} |
  .jobs["test-suite"] = .jobs.test |
  del(.jobs.test) |
  .jobs["test-suite"].needs = ["changes"] |
  .jobs["test-suite"].if = "needs.changes.outputs.go == true" |
  .jobs.build.needs = ["changes"] |
  .jobs.build.if = "needs.changes.outputs.go == true" |
  .jobs.helm.needs = ["changes"] |
  .jobs.helm["continue-on-error"] = false
' "${CI_WORKFLOW}" >"${MUTANTS}/ci-restructured.yaml"
if cmp -s "${MUTANTS}/ci-restructured.yaml" "${CI_WORKFLOW}"; then
  fail "self-test: the simulated ci.yaml restructure changed nothing -- the resilience check below proves nothing"
fi
yq eval '.jobs | has("changes")' "${MUTANTS}/ci-restructured.yaml" | grep -Fxq true ||
  fail "self-test: the simulated ci.yaml restructure did not add the changes job -- it is not the shape being guarded against"
if ! (check_ci_wiring "${MUTANTS}/ci-restructured.yaml") >/dev/null 2>&1; then
  fail "self-test: this wiring lock reds on a ci.yaml carrying the changes-classifier restructure -- it is coupled to job structure it must not read"
fi
pass "self-test: gate stays green on a ci.yaml with an added changes job, a renamed test job and if:-gated siblings"

# --- guard rails ---
# The degenerate "rejections" that an any-nonzero-exit check would have accepted as proof.
self_test_guard_holds() {
  local checker="$1" mutant="$2" original="$3" label="$4"
  if (mutant_rejected "${checker}" "${mutant}" "${original}" "${label}" 'unreachable-expected-message') >/dev/null 2>&1; then
    fail "self-test: mutant_rejected accepted ${label} as a genuine rejection -- it is tautological again"
  fi
  pass "self-test: mutant_rejected refuses to count ${label} as a rejection"
}

: >"${MUTANTS}/empty.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/empty.yaml" "${CI_WORKFLOW}" 'an empty file'
printf 'jobs: [[[\n' >"${MUTANTS}/broken.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/broken.yaml" "${CI_WORKFLOW}" 'an unparseable file'
self_test_guard_holds check_ci_wiring "${MUTANTS}/does-not-exist.yaml" "${CI_WORKFLOW}" 'a nonexistent path'
cp "${CI_WORKFLOW}" "${MUTANTS}/unmutated.yaml"
self_test_guard_holds check_ci_wiring "${MUTANTS}/unmutated.yaml" "${CI_WORKFLOW}" \
  'an unmutated copy of the real workflow'

echo "All docs map wiring tests passed."
