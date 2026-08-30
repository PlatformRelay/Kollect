#!/usr/bin/env bash
# Locks hack/test/repo_root_links_test.sh into a job that actually runs.
#
# CI-CONTRIBFILTER-01: the repo-root link gate landed composed ONLY into `task docs:verify`,
# which only the path-filtered Docs workflow runs -- and neither `CONTRIBUTING.md` nor the gate
# script itself was in that workflow's `paths:` filter (push or pull_request). `ci.yaml` cannot
# cover the `CONTRIBUTING.md` case either: it lists `CONTRIBUTING.md` in BOTH `paths-ignore`
# blocks. So the one file the gate exists to protect could be changed, and the gate weakened or
# deleted outright, with no job running it at all. A gate that is green because it never ran is
# the shape this repo keeps re-filing (LAB-DEKIND, CI-TIPGAP-01, CI-DOCSFILTER-01).
#
# Two wirings therefore have to hold at once, and both are asserted here:
#   1. `ci.yaml`'s `lint` job runs the gate unconditionally (the LAB-DEKIND precedent). This is
#      what covers a change to the gate SCRIPT -- `hack/**` is not in `paths-ignore`.
#   2. `docs.yaml`'s `paths:` filters list `CONTRIBUTING.md`, so a CONTRIBUTING-only PR triggers
#      the Docs workflow, whose `task docs:verify` composes the gate at hack/docs/verify.sh.
#      `ci.yaml` structurally cannot do this job; the filter entry is not redundant.
# Dropping either one re-opens the hole, so neither may be quietly removed.
#
# The step anchors follow the GATE-COMMENT-01 / GATE-SCOPE-01 lessons from
# dist_ci_wiring_test.sh: matching is LINE-EXACT against a COMMENT-STRIPPED view of the `run:`
# body, so `# bash ...`, `bash ... || true`, `bash ... &` and a narrowed lookalike are all
# rejected rather than counted as wiring. Path filters are matched LITERALLY -- re-implementing
# GitHub's glob semantics in bash would get `**` vs `*` wrong in the direction that passes.
#
# The self-test at the bottom mutates the real workflows every run and proves each assertion
# rejects the shape it exists to catch. A gate that has never been watched failing is not a gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"
DOCS_WORKFLOW="${ROOT}/.github/workflows/docs.yaml"
VERIFY_SCRIPT="${ROOT}/hack/docs/verify.sh"
GATE_SCRIPT="hack/test/repo_root_links_test.sh"
WIRING_SCRIPT="hack/test/repo_root_links_wiring_test.sh"
readonly CI_WORKFLOW DOCS_WORKFLOW VERIFY_SCRIPT GATE_SCRIPT WIRING_SCRIPT

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
    fail "the ${label} step must not declare 'continue-on-error: ${step_coe}' -- a soft-failed step reports a dead repo-root link as green"
}

# Wiring 1: ci.yaml's lint job runs the gate, and the gate's own reachability holds --
# `hack/**` must stay out of both `paths-ignore` lists or a PR touching only the gate script
# triggers no CI at all.
check_ci_wiring() {
  local workflow="$1"
  local job_kind steps_kind gate_idx wiring_idx job_if job_coe trigger entry

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  # Without these two guards a renamed job or a stepless lint job makes every selection below
  # run over an empty list, and the gate passes vacuously.
  job_kind="$(yq eval '.jobs.lint | type' "${workflow}")"
  [[ "${job_kind}" == "!!map" ]] ||
    fail "${workflow} declares no lint job -- the wiring assertions would pass vacuously"
  steps_kind="$(yq eval '.jobs.lint.steps | type' "${workflow}")"
  [[ "${steps_kind}" == "!!seq" ]] ||
    fail "${workflow} lint job declares no steps -- the wiring assertions would pass vacuously"

  job_if="$(yq eval '.jobs.lint.if' "${workflow}")"
  [[ "${job_if}" == "null" ]] ||
    fail "the lint job must run unconditionally, got 'if: ${job_if}' -- a skipped lint job never runs the repo-root link gate"
  job_coe="$(yq eval '.jobs.lint["continue-on-error"]' "${workflow}")"
  [[ "${job_coe}" == "null" || "${job_coe}" == "false" ]] ||
    fail "the lint job must not declare 'continue-on-error: ${job_coe}' -- a soft-failed lint job turns the repo-root link gate into an advisory notice"

  gate_idx="$(lint_step_index "${workflow}" "${GATE_SCRIPT}")" || exit 1
  assert_step_can_fail_build "${workflow}" "${gate_idx}" "repo-root link gate"

  # Self-lock: the assertions above are only load-bearing if THIS script also runs. Without it,
  # deleting the gate step and this file together is silent.
  wiring_idx="$(lint_step_index "${workflow}" "${WIRING_SCRIPT}")" || exit 1
  assert_step_can_fail_build "${workflow}" "${wiring_idx}" "repo-root link wiring"

  pass "ci.yaml lint job runs the repo-root link gate and this wiring test unconditionally, on lines that can fail the build"

  # Literal, not glob: GitHub's `*` does not cross `/` but bash's does, so a hand-rolled matcher
  # would be wrong in the direction that passes. Any `paths-ignore` entry naming `hack` at all is
  # a hard failure -- it is the only tree that makes the two steps above reachable.
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

# Wiring 2: ci.yaml ignores CONTRIBUTING.md by design, so the Docs workflow is the ONLY job that
# can run the gate for a CONTRIBUTING-only change. Both of its `paths:` lists must say so --
# push and pull_request drift apart easily, and only one of them protects a PR.
check_docs_filter() {
  local workflow="$1"
  local trigger required found entry ignored

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  # A filterless or renamed trigger would make the membership loop below vacuous.
  for trigger in push pull_request; do
    [[ "$(yq eval ".on.${trigger}.paths | type" "${workflow}")" == "!!seq" ]] ||
      fail "docs.yaml declares no ${trigger} paths filter -- the membership assertions would pass vacuously"
  done

  for trigger in push pull_request; do
    for required in README.md CONTRIBUTING.md; do
      found=0
      while IFS= read -r entry; do
        [[ "${entry}" == "${required}" ]] && found=1
      done < <(yq eval ".on.${trigger}.paths[] // \"\"" "${workflow}")
      [[ "${found}" -eq 1 ]] ||
        fail "docs.yaml's ${trigger} paths filter does not list '${required}' -- ci.yaml ignores it in both paths-ignore blocks, so a PR changing only that file would trigger no workflow that runs ${GATE_SCRIPT}"
    done
  done
  pass "docs.yaml triggers on README.md and CONTRIBUTING.md for both push and pull_request"

  # The filter entry buys nothing if ci.yaml stops ignoring the file and starts running full CI
  # on it -- but that is a deliberate, separate decision. What must NOT happen silently is the
  # reverse: this assertion documents the reason the docs.yaml entry exists.
  ignored=0
  while IFS= read -r entry; do
    [[ "${entry}" == "CONTRIBUTING.md" ]] && ignored=1
  done < <(yq eval '.on.pull_request.paths-ignore[] // ""' "${CI_WORKFLOW}")
  [[ "${ignored}" -eq 1 ]] ||
    pass "note: ci.yaml no longer ignores CONTRIBUTING.md; the docs.yaml filter entry is now belt-and-braces rather than the only coverage"
}

# Wiring 3: the composition point the Docs workflow reaches. `task docs:verify` runs
# hack/docs/verify.sh; if the gate is not invoked there, triggering the workflow changes nothing.
check_docs_composition() {
  local script="$1"

  [[ -f "${script}" ]] || fail "expected ${script}"
  grep -v '^[[:space:]]*#' "${script}" |
    sed 's/^[[:space:]]*//; s/[[:space:]]*$//' |
    grep -Fxq "bash ${GATE_SCRIPT}" ||
    fail "hack/docs/verify.sh does not invoke ${GATE_SCRIPT} on a bare 'bash ${GATE_SCRIPT}' line -- the Docs workflow would trigger on CONTRIBUTING.md and still run no repo-root link check"
  pass "task docs:verify composes the repo-root link gate on an unguarded line"
}

[[ -s "${ROOT}/${GATE_SCRIPT}" ]] ||
  fail "${GATE_SCRIPT} is missing or empty -- there is nothing for this wiring test to lock in"

check_ci_wiring "${CI_WORKFLOW}"
check_docs_filter "${DOCS_WORKFLOW}"
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
  'the repo-root link gate step is deleted from lint' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  .jobs.lint.steps += [.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\"))]
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-duplicated.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-duplicated.yaml" "${CI_WORKFLOW}" \
  'the repo-root link gate step appears twice in lint' \
  "steps invoking ${GATE_SCRIPT}"

yq eval "
  (.jobs.lint.steps[] | select((.run // \"\") | test(\"^bash ${GATE_SCRIPT//./\\\\.}\$\")) | .run) |= . + \" || true\"
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-or-true.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-or-true.yaml" "${CI_WORKFLOW}" \
  'the repo-root link gate step swallows failure with || true' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  (.jobs.lint.steps[] | select((.run // \"\") | test(\"^bash ${GATE_SCRIPT//./\\\\.}\$\")) | .run) |= \"# \" + .
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-commented-out.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-commented-out.yaml" "${CI_WORKFLOW}" \
  'the repo-root link gate step body is commented out' \
  "has no step invoking ${GATE_SCRIPT}"

yq eval "
  with(.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\")); .[\"continue-on-error\"] = true)
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the repo-root link gate step is soft-failed with continue-on-error' \
  'must not declare'

yq eval "
  with(.jobs.lint.steps[] | select((.run // \"\") | contains(\"${GATE_SCRIPT}\")); .if = \"false\")
" "${CI_WORKFLOW}" >"${MUTANTS}/gate-step-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/gate-step-disabled.yaml" "${CI_WORKFLOW}" \
  'the repo-root link gate step is disabled with if: false' \
  'must run unconditionally'

# --- ci.yaml: this wiring test's own step, and the lint job around it ---
yq eval "
  .jobs.lint.steps = [.jobs.lint.steps[] | select(((.run // \"\") | contains(\"${WIRING_SCRIPT}\")) | not)]
" "${CI_WORKFLOW}" >"${MUTANTS}/wiring-step-removed.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/wiring-step-removed.yaml" "${CI_WORKFLOW}" \
  'this wiring test is no longer run by lint' \
  "has no step invoking ${WIRING_SCRIPT}"

yq eval '.jobs.lint["continue-on-error"] = true' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-soft-fail.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-soft-fail.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is soft-failed with continue-on-error' \
  'the lint job must not declare'

yq eval '.jobs.lint.if = "false"' "${CI_WORKFLOW}" >"${MUTANTS}/lint-job-disabled.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/lint-job-disabled.yaml" "${CI_WORKFLOW}" \
  'the whole lint job is disabled with if: false' \
  'the lint job must run unconditionally'

# --- ci.yaml: the trigger that makes the lint steps reachable at all ---
yq eval '.on.pull_request.paths-ignore += ["hack/test/**"]' "${CI_WORKFLOW}" \
  >"${MUTANTS}/hack-path-ignored.yaml"
mutant_rejected check_ci_wiring "${MUTANTS}/hack-path-ignored.yaml" "${CI_WORKFLOW}" \
  'ci.yaml starts ignoring hack/test/** on pull_request' \
  'would then trigger no CI'

# --- docs.yaml: the only coverage a CONTRIBUTING-only PR has ---
yq eval '.on.pull_request.paths = [.on.pull_request.paths[] | select(. != "CONTRIBUTING.md")]' \
  "${DOCS_WORKFLOW}" >"${MUTANTS}/docs-contributing-dropped.yaml"
mutant_rejected check_docs_filter "${MUTANTS}/docs-contributing-dropped.yaml" "${DOCS_WORKFLOW}" \
  'docs.yaml drops CONTRIBUTING.md from the pull_request filter' \
  "does not list 'CONTRIBUTING.md'"

yq eval '.on.push.paths = [.on.push.paths[] | select(. != "README.md")]' \
  "${DOCS_WORKFLOW}" >"${MUTANTS}/docs-readme-dropped.yaml"
mutant_rejected check_docs_filter "${MUTANTS}/docs-readme-dropped.yaml" "${DOCS_WORKFLOW}" \
  'docs.yaml drops README.md from the push filter' \
  "does not list 'README.md'"

yq eval 'del(.on.pull_request.paths)' "${DOCS_WORKFLOW}" >"${MUTANTS}/docs-filter-deleted.yaml"
mutant_rejected check_docs_filter "${MUTANTS}/docs-filter-deleted.yaml" "${DOCS_WORKFLOW}" \
  'docs.yaml has no pull_request paths filter at all' \
  'would pass vacuously'

# --- hack/docs/verify.sh: the composition point the Docs workflow reaches ---
sed "s|^bash ${GATE_SCRIPT}\$|# bash ${GATE_SCRIPT}|" "${VERIFY_SCRIPT}" \
  >"${MUTANTS}/verify-composition-removed.sh"
mutant_rejected check_docs_composition "${MUTANTS}/verify-composition-removed.sh" "${VERIFY_SCRIPT}" \
  'task docs:verify no longer invokes the repo-root link gate' \
  'does not invoke'

# The self-test's own guard rails: the degenerate "rejections" that an any-nonzero-exit check
# would have accepted as proof. mutant_rejected must refuse every one of them.
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

echo "All repo-root link wiring tests passed."
