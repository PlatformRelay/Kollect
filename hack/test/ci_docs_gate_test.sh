#!/usr/bin/env bash
# CI-DOCSGATE-01 -- every required status context must report a conclusion on EVERY pull
# request, including a documentation-only one.
#
# The defect this gate locks down (observed on PR #350, 2026-09-01; worked around on #321,
# 2026-08-23): ci.yaml and e2e-smoke.yaml both carried a blanket `paths-ignore` on
# `pull_request`. A workflow skipped by `paths-ignore` does not report its contexts as
# "skipped" -- it does not report them AT ALL. The protect-main ruleset requires
# `preflight`, `test`, `kind-smoke` and `Analyze (Go)`, so a docs-only PR sat at
# mergeStateStatus BLOCKED with a SUCCESS rollup and an empty failure list, and the only way
# through was `gh pr merge --admin` or the web UI's bypass prompt. A bypass required on every
# docs PR is not an exception, it is the merge path -- and it trains the reflex that one day
# waves through a genuinely red required check.
#
# THE INVARIANT, in both directions:
#
#   (1) For every PR to main, whatever paths it touches, all four required contexts report a
#       conclusion. On a docs-only PR the `test` / `kind-smoke` contexts report success
#       WITHOUT burning a kind cluster or the full Go suite.
#   (2) The converse, which is the half that must not regress: on a PR touching watched code
#       paths, the no-op path must be STRUCTURALLY INCAPABLE of satisfying `test` or
#       `kind-smoke`. A skipped worker job must red the required context.
#
# Why a structural/executable gate rather than "watch it on a PR": this lane's own PR touches
# `.github/**`, which is a watched code path, so a real CI run can only ever exercise
# direction (2). Direction (1) is unobservable from any PR that changes the workflows. So the
# assertions below read the workflow YAML with yq, and -- for the two pieces of shell that
# carry the actual decisions -- EXECUTE the exact `run:` bodies the workflows will run, with
# the exact `env:` bindings the workflows declare, against a truth table and a scratch git
# repository.
#
# Follows the local conventions of hack/test/dist_ci_wiring_test.sh:
#   * every anchor is scoped to one named job, and must resolve to exactly one match;
#   * `run:` bodies are matched against a COMMENT-STRIPPED view (a commented-out command is
#     not a command);
#   * "wired in" is always paired with "can still fail the build" (`if:` / `continue-on-error`);
#   * a self-test at the bottom mutates the real workflows and requires each check to red with
#     the MESSAGE of the assertion the mutation was built to trip -- a non-zero exit alone is
#     not evidence (an empty or unparseable file exits non-zero too).
#
# METHOD-MUTHARNESS-01: the mutants below are produced with yq, structurally. No `perl -0pi -e`
# with a double-quoted replacement -- perl interpolates `$1`, `${script}`, `${#arr[@]}` out of
# the mutant, which reds the gate for the wrong reason and yields a false KILL.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CI_WORKFLOW="${ROOT}/.github/workflows/ci.yaml"
SMOKE_WORKFLOW="${ROOT}/.github/workflows/e2e-smoke.yaml"
PREFLIGHT_WORKFLOW="${ROOT}/.github/workflows/preflight.yaml"
CODEQL_WORKFLOW="${ROOT}/.github/workflows/codeql.yaml"
GATE_SCRIPT="hack/test/ci_docs_gate_test.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the workflow triggers and job graph"
command -v git >/dev/null 2>&1 ||
  fail "git is required: the changes-filter assertions run the real filter body over a scratch repository"

# The executable half of a `run:` body -- every line whose first non-blank character is `#`
# is dropped. `.value` is a `to_entries` entry, matching the dist_ci_wiring_test.sh idiom.
RUN_CODE='((.value.run // "") | split("\n") | map(select(test("^[[:space:]]*#") | not)) | join("\n"))'

# 0-based index of the ONE step in <job> satisfying a yq predicate. Zero matches and duplicates
# are both hard failures: a duplicate would let every later assertion lock onto the first hit.
job_step_index() {
  local workflow="$1" job="$2" predicate="$3" label="$4"
  local query count
  query="$(printf '[.jobs["%s"].steps | to_entries[] | select(%s) | .key]' "${job}" "${predicate}")"
  count="$(yq eval "${query} | length" "${workflow}")"
  case "${count}" in
  1) yq eval "${query} | .[0]" "${workflow}" ;;
  0) fail "job '${job}' in ${workflow} has no ${label} step -- a matching step in another job does not count, and a commented-out command is not a command" ;;
  *) fail "job '${job}' in ${workflow} has ${count} ${label} steps -- keep exactly one so the assertions cannot lock onto the first match" ;;
  esac
}

# A job that is conditional or soft-failed satisfies every structural assertion while never
# running, or while reporting its failure as success. yq's `//` is unusable here: its
# alternative operator treats a literal `false` as absent, and `false` is the value being
# guarded against.
assert_job_can_fail_build() {
  local workflow="$1" job="$2" why="$3"
  local job_if job_coe
  job_if="$(yq eval ".jobs[\"${job}\"].if" "${workflow}")"
  [[ "${job_if}" == "null" ]] ||
    fail "job '${job}' in ${workflow} must run unconditionally, got 'if: ${job_if}' -- ${why}"
  job_coe="$(yq eval ".jobs[\"${job}\"][\"continue-on-error\"]" "${workflow}")"
  [[ "${job_coe}" == "null" || "${job_coe}" == "false" ]] ||
    fail "job '${job}' in ${workflow} must not declare 'continue-on-error: ${job_coe}' -- ${why}"
}

assert_step_can_fail_build() {
  local workflow="$1" job="$2" idx="$3" label="$4"
  local step_if step_coe
  step_if="$(yq eval ".jobs[\"${job}\"].steps[${idx}].if" "${workflow}")"
  [[ "${step_if}" == "null" ]] ||
    fail "the ${label} step of job '${job}' must run unconditionally, got 'if: ${step_if}' -- a skipped step decides nothing"
  step_coe="$(yq eval ".jobs[\"${job}\"].steps[${idx}][\"continue-on-error\"]" "${workflow}")"
  [[ "${step_coe}" == "null" || "${step_coe}" == "false" ]] ||
    fail "the ${label} step of job '${job}' must not declare 'continue-on-error: ${step_coe}' -- a soft-failed step reports its failure as success"
}

# ---------------------------------------------------------------------------
# (1) The four required contexts must be reachable on a docs-only PR.
#
# A `paths` or `paths-ignore` filter on the `pull_request` trigger of a workflow that owns a
# required context is the exact defect: the workflow is not skipped-with-a-conclusion, it is
# never dispatched, and the context never appears. Note `push` is deliberately NOT checked --
# required contexts gate PR merges, and keeping docs pushes to main off the expensive
# workflows is the whole point of the filter that remains there.
# ---------------------------------------------------------------------------
check_pr_trigger_unfiltered() {
  local workflow="$1" context="$2"
  local kind filt

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  kind="$(yq eval '.on.pull_request | has("paths")' "${workflow}")"
  # A workflow with no `pull_request` trigger at all cannot report on a PR either.
  [[ "$(yq eval '.on | has("pull_request")' "${workflow}")" == "true" ]] ||
    fail "${workflow} has no pull_request trigger, so the required context '${context}' can never report on a PR"
  [[ "${kind}" == "false" ]] ||
    fail "${workflow} filters its pull_request trigger with 'paths:' -- a workflow the filter excludes never reports the required context '${context}' at all (not even as skipped), so the PR stays BLOCKED forever"
  filt="$(yq eval '.on.pull_request | has("paths-ignore")' "${workflow}")"
  [[ "${filt}" == "false" ]] ||
    fail "${workflow} filters its pull_request trigger with 'paths-ignore:' -- a workflow the filter excludes never reports the required context '${context}' at all (not even as skipped), so a docs-only PR stays BLOCKED forever and needs an admin bypass"

  pass "${workflow} dispatches on every pull request, so '${context}' can report"
}

# The job that produces the context must exist, be named byte-identically to the ruleset's
# context string, and be able to fail the build. Renaming it renames the context and silently
# breaks protect-main, which lists contexts by exact string.
check_context_job() {
  local workflow="$1" job="$2" context="$3"
  local job_kind job_name

  job_kind="$(yq eval ".jobs[\"${job}\"] | type" "${workflow}")"
  [[ "${job_kind}" == "!!map" ]] ||
    fail "${workflow} declares no '${job}' job -- the required context '${context}' would have no producer"
  job_name="$(yq eval ".jobs[\"${job}\"].name" "${workflow}")"
  [[ "${job_name}" == "${context}" ]] ||
    fail "job '${job}' in ${workflow} is named '${job_name}', not '${context}' -- the status context is the job NAME, and protect-main lists required contexts by exact string, so this rename breaks the ruleset"
  local job_coe
  job_coe="$(yq eval ".jobs[\"${job}\"][\"continue-on-error\"]" "${workflow}")"
  [[ "${job_coe}" == "null" || "${job_coe}" == "false" ]] ||
    fail "job '${job}' in ${workflow} must not declare 'continue-on-error: ${job_coe}' -- a soft-failed required context reports red as green"
  pass "${workflow} job '${job}' produces the required context '${context}'"
}

# ---------------------------------------------------------------------------
# (2) The two reporting jobs (`test`, `kind-smoke`) and their workers.
#
# Shape: a `changes` job classifies the PR; a WORKER job does the real work under
# `if: needs.changes.outputs.code == 'true'`; and a REPORTER job carrying the required
# context name runs `if: always()` and converts (verdict, worker result) into a conclusion.
# The reporter is the no-op path. Direction (2) of the invariant is entirely about it, so it
# is not merely read -- its `run:` body is executed against a truth table below.
# ---------------------------------------------------------------------------
check_reporter() {
  local workflow="$1" reporter="$2" worker="$3" context="$4"
  local job_if steps_len step_env_code step_env_result needs body tmpdir

  [[ "$(yq eval ".jobs[\"${reporter}\"] | type" "${workflow}")" == "!!map" ]] ||
    fail "${workflow} declares no '${reporter}' job -- the required context '${context}' would have no producer"

  # `if: always()` is what makes the context report when the worker is SKIPPED. Without it a
  # skipped worker skips the reporter too, and the context vanishes exactly as before.
  job_if="$(yq eval ".jobs[\"${reporter}\"].if" "${workflow}")"
  # shellcheck disable=SC2016  # the ${{ }} form is GitHub's literal syntax, not a shell expansion
  [[ "${job_if}" == "always()" || "${job_if}" == '${{ always() }}' ]] ||
    fail "job '${reporter}' in ${workflow} must declare 'if: always()' so it still reports '${context}' when '${worker}' is skipped, got 'if: ${job_if}'"

  # It has to depend on both, or the two expressions it reads are empty and it degenerates
  # into an unconditional green.
  needs="$(yq eval ".jobs[\"${reporter}\"].needs | join(\",\")" "${workflow}")"
  [[ ",${needs}," == *",changes,"* ]] ||
    fail "job '${reporter}' in ${workflow} must 'needs: changes' -- without it needs.changes.outputs.code is empty and the reporter cannot tell a docs-only PR from a code one (needs: ${needs})"
  [[ ",${needs}," == *",${worker},"* ]] ||
    fail "job '${reporter}' in ${workflow} must 'needs: ${worker}' -- without it needs.${worker}.result is empty and the no-op path would satisfy '${context}' while the real job failed (needs: ${needs})"

  # Exactly one step: the reporter must be a report and nothing else. Any extra step is either
  # real work that belongs in the worker, or a way to smuggle a second exit path past the
  # truth table below.
  steps_len="$(yq eval ".jobs[\"${reporter}\"].steps | length" "${workflow}")"
  [[ "${steps_len}" == "1" ]] ||
    fail "job '${reporter}' in ${workflow} must have exactly one step (the report), got ${steps_len} -- the truth table below executes step 0 and would not see the others"
  assert_step_can_fail_build "${workflow}" "${reporter}" 0 "report"

  # The executed body is only evidence if it is fed by the real wiring. Both bindings are
  # asserted byte-exact against the expressions GitHub will substitute.
  step_env_code="$(yq eval ".jobs[\"${reporter}\"].steps[0].env.CODE" "${workflow}")"
  # shellcheck disable=SC2016
  [[ "${step_env_code}" == '${{ needs.changes.outputs.code }}' ]] ||
    fail "the report step of '${reporter}' in ${workflow} must bind CODE to '\${{ needs.changes.outputs.code }}', got '${step_env_code}' -- otherwise the truth table below tests a body wired to nothing"
  step_env_result="$(yq eval ".jobs[\"${reporter}\"].steps[0].env.RESULT" "${workflow}")"
  [[ "${step_env_result}" == "\${{ needs.${worker}.result }}" ]] ||
    fail "the report step of '${reporter}' in ${workflow} must bind RESULT to '\${{ needs.${worker}.result }}', got '${step_env_result}' -- otherwise it reports on some other job's outcome"

  body="$(yq eval ".jobs[\"${reporter}\"].steps[0].run" "${workflow}")"
  [[ -n "${body}" && "${body}" != "null" ]] ||
    fail "the report step of '${reporter}' in ${workflow} has an empty run: body -- an empty script exits 0 for every input, which is exactly the unconditional green this gate exists to prevent"

  tmpdir="$(mktemp -d)"
  printf '%s\n' "${body}" >"${tmpdir}/report.sh"
  bash -n "${tmpdir}/report.sh" ||
    fail "the report step of '${reporter}' in ${workflow} is not valid bash -- 'bash -n' rejected it"

  # --- the truth table ---------------------------------------------------
  # Rows marked "must red" are direction (2): the no-op path standing in for the real job.
  local row code result expect status out
  local rows=(
    # CODE      RESULT      expected-exit  why
    "true|success|0|a code-touching PR whose worker passed"
    "true|failure|1|a code-touching PR whose worker FAILED"
    "true|skipped|1|a code-touching PR whose worker was SKIPPED (the no-op standing in for the real job)"
    "true|cancelled|1|a code-touching PR whose worker was cancelled"
    "true||1|a code-touching PR with no worker result at all"
    "|skipped|1|an unclassifiable PR (the changes job produced no verdict) whose worker was skipped -- must fail safe"
    "banana|skipped|1|a corrupted verdict whose worker was skipped -- anything but a literal false must fail safe"
    "false|skipped|0|a documentation-only PR: the required context reports without running the worker"
    "false|success|0|a documentation-only PR whose worker ran anyway and passed"
    "false|failure|1|a documentation-only PR whose worker ran and FAILED"
    "false|cancelled|1|a documentation-only PR whose worker was cancelled"
  )
  for row in "${rows[@]}"; do
    IFS='|' read -r code result expect why <<<"${row}"
    status=0
    out="$(CODE="${code}" RESULT="${result}" bash "${tmpdir}/report.sh" 2>&1)" || status=$?
    if [[ "${expect}" == "0" && "${status}" -ne 0 ]]; then
      rm -rf "${tmpdir}"
      fail "the '${reporter}' report body FAILED (exit ${status}) on ${why} [CODE='${code}' RESULT='${result}'] -- '${context}' would red on a PR it must let through. Output: ${out}"
    fi
    if [[ "${expect}" == "1" && "${status}" -eq 0 ]]; then
      rm -rf "${tmpdir}"
      fail "the '${reporter}' report body PASSED on ${why} [CODE='${code}' RESULT='${result}'] -- the no-op path can satisfy the required context '${context}' when the real job did not succeed. Output: ${out}"
    fi
  done
  rm -rf "${tmpdir}"
  pass "job '${reporter}' in ${workflow} reports '${context}' on every PR, and its body rejects every no-op stand-in (${#rows[@]} truth-table rows)"
}

# The worker is the real job. It must be gated on the changes verdict (so a docs-only PR does
# not burn it), must NOT carry a required context name (that would make the ruleset depend on
# a job that legitimately skips), and must actually still do the work.
check_worker() {
  local workflow="$1" worker="$2" work_predicate="$3" work_label="$4"
  local job_if job_name job_coe needs idx

  [[ "$(yq eval ".jobs[\"${worker}\"] | type" "${workflow}")" == "!!map" ]] ||
    fail "${workflow} declares no '${worker}' job -- there would be nothing behind the reporter"

  job_if="$(yq eval ".jobs[\"${worker}\"].if" "${workflow}")"
  [[ "${job_if}" == "needs.changes.outputs.code == 'true'" ]] ||
    fail "job '${worker}' in ${workflow} must be gated on \"needs.changes.outputs.code == 'true'\", got 'if: ${job_if}' -- any other condition either burns the job on docs-only PRs or lets it skip on code PRs"

  needs="$(yq eval ".jobs[\"${worker}\"].needs | join(\",\")" "${workflow}")"
  [[ ",${needs}," == *",changes,"* ]] ||
    fail "job '${worker}' in ${workflow} must 'needs: changes' -- an if: referencing a job it does not need is always false, so the worker would NEVER run (needs: ${needs})"

  job_name="$(yq eval ".jobs[\"${worker}\"].name" "${workflow}")"
  case "${job_name}" in
  preflight | test | kind-smoke | "Analyze (Go)")
    fail "job '${worker}' in ${workflow} is named '${job_name}', a required context, while carrying an if: that skips it on docs-only PRs -- that is the original defect, moved one job over"
    ;;
  esac

  job_coe="$(yq eval ".jobs[\"${worker}\"][\"continue-on-error\"]" "${workflow}")"
  [[ "${job_coe}" == "null" || "${job_coe}" == "false" ]] ||
    fail "job '${worker}' in ${workflow} must not declare 'continue-on-error: ${job_coe}' -- a soft-failed worker always reports success, and the reporter would pass it through"

  idx="$(job_step_index "${workflow}" "${worker}" "${work_predicate}" "${work_label}")" || exit 1
  assert_step_can_fail_build "${workflow}" "${worker}" "${idx}" "${work_label}"
  pass "job '${worker}' in ${workflow} still runs ${work_label}, gated on the changes verdict, and can fail the build"
}

# ---------------------------------------------------------------------------
# (3) The changes filter itself. Everything above rests on this one shell body being right,
# so it is executed -- not read -- over a scratch git repository whose diffs are constructed
# to be exactly the change classes that matter.
# ---------------------------------------------------------------------------
check_changes_filter() {
  local workflow="$1"
  local idx body tmpdir repo out code

  [[ "$(yq eval '.jobs.changes | type' "${workflow}")" == "!!map" ]] ||
    fail "${workflow} declares no 'changes' job -- every if: in it would evaluate against an empty output and the workers would never run"
  assert_job_can_fail_build "${workflow}" changes \
    "a conditional or soft-failed classifier produces no verdict, and the reporter then treats every PR as code-touching (or worse, as docs-only)"

  # shellcheck disable=SC2016
  [[ "$(yq eval '.jobs.changes.outputs.code' "${workflow}")" == '${{ steps.filter.outputs.code }}' ]] ||
    fail "the 'changes' job in ${workflow} must publish outputs.code from '\${{ steps.filter.outputs.code }}' -- nothing else is what the reporters and workers read"

  idx="$(job_step_index "${workflow}" changes '.value.id == "filter"' 'id: filter classification')" || exit 1
  assert_step_can_fail_build "${workflow}" changes "${idx}" "classification"

  body="$(yq eval ".jobs.changes.steps[${idx}].run" "${workflow}")"
  [[ -n "${body}" && "${body}" != "null" ]] ||
    fail "the classification step of 'changes' in ${workflow} has an empty run: body -- it would publish no verdict at all"

  tmpdir="$(mktemp -d)"
  printf '%s\n' "${body}" >"${tmpdir}/filter.sh"
  bash -n "${tmpdir}/filter.sh" ||
    fail "the classification step of 'changes' in ${workflow} is not valid bash -- 'bash -n' rejected it"

  repo="${tmpdir}/repo"
  mkdir -p "${repo}/docs" "${repo}/internal" "${repo}/.github/ISSUE_TEMPLATE" "${repo}/hack/test" "${repo}/.github/workflows"
  (
    cd "${repo}"
    git init -q -b main .
    git config user.email ci-docsgate@example.invalid
    git config user.name ci-docsgate
    printf 'seed\n' >docs/index.md
    printf 'seed\n' >README.md
    printf 'seed\n' >LICENSE
    printf 'seed\n' >CHANGELOG.md
    printf 'seed\n' >CONTRIBUTING.md
    printf 'seed\n' >mkdocs.yml
    printf 'seed\n' >.github/ISSUE_TEMPLATE/bug.md
    printf 'seed\n' >internal/thing.go
    printf 'seed\n' >hack/test/some_test.sh
    printf 'seed\n' >.github/workflows/ci.yaml
    printf 'seed\n' >go.mod
    git add -A
    git commit -qm seed
  )

  # Runs the REAL body with the REAL env bindings, and returns whatever verdict it published.
  run_filter() {
    local event="$1" base="$2" head="$3"
    local outfile
    outfile="$(mktemp "${tmpdir}/out.XXXXXX")"
    (
      cd "${repo}"
      EVENT_NAME="${event}" BASE_SHA="${base}" HEAD_SHA="${head}" GITHUB_OUTPUT="${outfile}" \
        bash "${tmpdir}/filter.sh" >/dev/null 2>&1
    ) || {
      printf 'ERROR\n'
      return 0
    }
    # Last verdict wins, mirroring how GitHub reads GITHUB_OUTPUT.
    local n
    n="$(grep -c '^code=' "${outfile}" || true)"
    if [[ "${n}" -eq 0 ]]; then
      printf 'MISSING\n'
      return 0
    fi
    grep '^code=' "${outfile}" | tail -1 | cut -d= -f2
  }

  # A commit touching exactly the listed paths, returned as "<base> <head>".
  commit_touching() {
    local f
    (
      cd "${repo}"
      for f in "$@"; do
        mkdir -p "$(dirname "${f}")"
        printf 'change\n' >>"${f}"
      done
      git add -A
      git commit -qm "touch $*" >/dev/null
    )
  }

  local base head verdict
  local -a cases=(
    "false|docs/index.md|a docs/ page"
    "false|README.md|README.md"
    "false|LICENSE|LICENSE"
    "false|CHANGELOG.md|CHANGELOG.md"
    "false|CONTRIBUTING.md|CONTRIBUTING.md"
    "false|mkdocs.yml|mkdocs.yml"
    "false|.github/ISSUE_TEMPLATE/bug.md|an issue template"
    "false|docs/index.md README.md .github/ISSUE_TEMPLATE/bug.md|several documentation files at once"
    "true|internal/thing.go|a Go source file"
    "true|go.mod|go.mod"
    "true|hack/test/some_test.sh|a hack/test gate script"
    "true|.github/workflows/ci.yaml|the CI workflow itself"
    "true|docs/index.md internal/thing.go|a MIXED docs+code change"
    "true|README.md.go|a path that merely starts with a documentation file's name"
    "true|documentation/notes.md|a path that merely starts with the docs/ prefix's letters"
  )
  local row expect paths label
  for row in "${cases[@]}"; do
    IFS='|' read -r expect paths label <<<"${row}"
    base="$( (cd "${repo}" && git rev-parse HEAD) )"
    # shellcheck disable=SC2086  # deliberate word split: the case lists several paths
    commit_touching ${paths}
    head="$( (cd "${repo}" && git rev-parse HEAD) )"
    verdict="$(run_filter pull_request "${base}" "${head}")"
    if [[ "${verdict}" != "${expect}" ]]; then
      rm -rf "${tmpdir}"
      if [[ "${expect}" == "true" ]]; then
        fail "the 'changes' filter in ${workflow} classified ${label} as code='${verdict}' (expected true) -- a PR touching watched code paths would satisfy the required contexts through the no-op path without running anything"
      fi
      fail "the 'changes' filter in ${workflow} classified ${label} as code='${verdict}' (expected false) -- a documentation-only PR would still burn the full Go suite and a kind cluster"
    fi
  done

  # Fail-safe directions: anything the filter cannot classify must come out as code=true, so
  # the worst case is a wasted CI run rather than an unrun required check reported green.
  base="$( (cd "${repo}" && git rev-parse HEAD) )"
  commit_touching docs/index.md
  head="$( (cd "${repo}" && git rev-parse HEAD) )"

  local -a failsafe=(
    "push|${base}|${head}|a push event, where no PR base/head diff exists"
    "schedule|${base}|${head}|a scheduled event"
    "pull_request||${head}|a pull_request with an empty base SHA"
    "pull_request|${base}||a pull_request with an empty head SHA"
    "pull_request|0000000000000000000000000000000000000000|${head}|a pull_request whose base SHA is not in the repository"
  )
  local ev fb fh flabel
  for row in "${failsafe[@]}"; do
    IFS='|' read -r ev fb fh flabel <<<"${row}"
    verdict="$(run_filter "${ev}" "${fb}" "${fh}")"
    if [[ "${verdict}" != "true" ]]; then
      rm -rf "${tmpdir}"
      fail "the 'changes' filter in ${workflow} produced code='${verdict}' for ${flabel} -- it must fail safe to 'true' there, or an unclassifiable change silently reports the required contexts green without running them"
    fi
  done

  rm -rf "${tmpdir}"
  pass "the 'changes' filter in ${workflow} classifies ${#cases[@]} real diffs correctly and fails safe on ${#failsafe[@]} degenerate inputs"
}

# ci.yaml and e2e-smoke.yaml each carry their own copy of the classifier (a workflow cannot
# import a job). Two copies that drift are two different answers to "is this docs-only?", and
# the pair (test, kind-smoke) would then disagree about the same PR. Lock them byte-identical.
check_filters_agree() {
  local a="$1" b="$2"
  local ia ib ba bb
  ia="$(job_step_index "${a}" changes '.value.id == "filter"' 'id: filter classification')" || exit 1
  ib="$(job_step_index "${b}" changes '.value.id == "filter"' 'id: filter classification')" || exit 1
  ba="$(yq eval ".jobs.changes.steps[${ia}].run" "${a}")"
  bb="$(yq eval ".jobs.changes.steps[${ib}].run" "${b}")"
  [[ "${ba}" == "${bb}" ]] ||
    fail "the 'changes' classification bodies in ${a} and ${b} have drifted apart -- the required contexts 'test' and 'kind-smoke' would then disagree about whether the same PR is documentation-only. Keep the two bodies byte-identical."
  pass "the changes classifier is byte-identical in both workflows"
}

# ---------------------------------------------------------------------------
# (4) This gate must itself run in CI, in a job that runs on every PR and can fail the build.
# An unwired gate is not a gate; and the lint job is the one job here that is deliberately not
# gated on the changes verdict, so it also covers the docs-only direction.
# ---------------------------------------------------------------------------
check_wired() {
  local workflow="$1"
  local idx
  assert_job_can_fail_build "${workflow}" lint \
    "a skipped or soft-failed lint job never runs ${GATE_SCRIPT}, and this whole gate becomes decoration"
  idx="$(job_step_index "${workflow}" lint \
    "${RUN_CODE} | split(\"\n\") | map(sub(\"^[[:space:]]*\"; \"\") | sub(\"[[:space:]]*\$\"; \"\")) | any_c(. == \"bash ${GATE_SCRIPT}\")" \
    "bare 'bash ${GATE_SCRIPT}' invocation")" || exit 1
  assert_step_can_fail_build "${workflow}" lint "${idx}" "${GATE_SCRIPT}"
  pass "${GATE_SCRIPT} is wired into the lint job on an uncommented, unguarded line"
}

# ---------------------------------------------------------------------------
# Run the checks against the real workflows.
# ---------------------------------------------------------------------------
check_pr_trigger_unfiltered "${PREFLIGHT_WORKFLOW}" "preflight"
check_pr_trigger_unfiltered "${CODEQL_WORKFLOW}" "Analyze (Go)"
check_pr_trigger_unfiltered "${CI_WORKFLOW}" "test"
check_pr_trigger_unfiltered "${SMOKE_WORKFLOW}" "kind-smoke"

check_context_job "${PREFLIGHT_WORKFLOW}" preflight "preflight"
check_context_job "${CODEQL_WORKFLOW}" analyze "Analyze (Go)"
check_context_job "${CI_WORKFLOW}" test "test"
check_context_job "${SMOKE_WORKFLOW}" kind-smoke "kind-smoke"

# preflight and CodeQL do the real work unconditionally on every PR -- they are cheap enough
# that no no-op path exists for them, and an `if:` on either is the same starvation defect.
assert_job_can_fail_build "${PREFLIGHT_WORKFLOW}" preflight \
  "the required context 'preflight' must report a conclusion on every PR, including a documentation-only one"
assert_job_can_fail_build "${CODEQL_WORKFLOW}" analyze \
  "the required context 'Analyze (Go)' must report a conclusion on every PR, including a documentation-only one"
pass "preflight and Analyze (Go) run unconditionally on every PR"

check_changes_filter "${CI_WORKFLOW}"
check_changes_filter "${SMOKE_WORKFLOW}"
check_filters_agree "${CI_WORKFLOW}" "${SMOKE_WORKFLOW}"

# shellcheck disable=SC2016  # the yq predicate is a literal, not a shell expansion
check_worker "${CI_WORKFLOW}" test-suite "${RUN_CODE} | test(\"(^|\\n)[[:space:]]*task coverage[[:space:]]*(\\n|\$)\")" 'task coverage'
# shellcheck disable=SC2016
check_worker "${SMOKE_WORKFLOW}" kind-smoke-run '(.value.uses // "") == "./.github/actions/kind-e2e-setup"' 'the kind e2e setup action'

check_reporter "${CI_WORKFLOW}" test test-suite "test"
check_reporter "${SMOKE_WORKFLOW}" kind-smoke kind-smoke-run "kind-smoke"

check_wired "${CI_WORKFLOW}"

# ---------------------------------------------------------------------------
# Self-test. A gate that only passes on the happy path is not evidence.
#
# Each mutant is a structural yq edit of the REAL workflow, and each must be rejected with the
# MESSAGE of the assertion it was built to trip -- not merely with a non-zero exit, which an
# empty, broken or missing file also produces. The four degenerate "rejections" at the bottom
# are the guard rails on that rule.
# ---------------------------------------------------------------------------
MUTANTS="$(mktemp -d)"
trap 'rm -rf "${MUTANTS}"' EXIT

mutant_rejected() {
  local original="$1" mutant="$2" label="$3" expect="$4"
  shift 4
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty -- the yq mutation step failed, so nothing was actually tested"
  # Compared through yq, not byte-for-byte: every mutant is yq output, so a raw `cmp` against
  # the hand-formatted original differs even when the mutation matched nothing, and the no-op
  # guard would never fire. A `sub()` whose regex missed is the single most likely way to build
  # a mutant that tests nothing.
  if diff -q <(yq eval '.' "${mutant}" 2>/dev/null) <(yq eval '.' "${original}" 2>/dev/null) >/dev/null 2>&1; then
    fail "self-test: the mutant for '${label}' parses identically to ${original} -- the yq mutation was a no-op (a sub() regex that matched nothing?), so nothing was actually tested"
  fi

  # Subshell: fail's `exit 1` is the expectation here, it must not take the parent down.
  output="$( ("$@") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a workflow where ${label} -- it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason -- expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects a workflow where ${label}"
}

# --- the original defect, re-introduced ---
yq eval '.on.pull_request.paths-ignore = ["docs/**", "README.md"]' "${CI_WORKFLOW}" >"${MUTANTS}/ci-paths-ignore.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/ci-paths-ignore.yaml" \
  'ci.yaml re-adds a blanket paths-ignore on pull_request' \
  'never reports the required context' \
  check_pr_trigger_unfiltered "${MUTANTS}/ci-paths-ignore.yaml" "test"

yq eval '.on.pull_request.paths-ignore = ["docs/**"]' "${SMOKE_WORKFLOW}" >"${MUTANTS}/smoke-paths-ignore.yaml"
mutant_rejected "${SMOKE_WORKFLOW}" "${MUTANTS}/smoke-paths-ignore.yaml" \
  'e2e-smoke.yaml re-adds a blanket paths-ignore on pull_request' \
  'never reports the required context' \
  check_pr_trigger_unfiltered "${MUTANTS}/smoke-paths-ignore.yaml" "kind-smoke"

yq eval '.on.pull_request.paths-ignore = ["docs/**"]' "${CODEQL_WORKFLOW}" >"${MUTANTS}/codeql-paths-ignore.yaml"
mutant_rejected "${CODEQL_WORKFLOW}" "${MUTANTS}/codeql-paths-ignore.yaml" \
  'codeql.yaml starts path-filtering pull_request, starving Analyze (Go)' \
  'never reports the required context' \
  check_pr_trigger_unfiltered "${MUTANTS}/codeql-paths-ignore.yaml" "Analyze (Go)"

yq eval '.on.pull_request.paths = ["internal/**"]' "${PREFLIGHT_WORKFLOW}" >"${MUTANTS}/preflight-paths.yaml"
mutant_rejected "${PREFLIGHT_WORKFLOW}" "${MUTANTS}/preflight-paths.yaml" \
  'preflight.yaml starts path-filtering pull_request with an allowlist' \
  "filters its pull_request trigger with 'paths:'" \
  check_pr_trigger_unfiltered "${MUTANTS}/preflight-paths.yaml" "preflight"

# --- direction (2): the no-op path standing in for the real job ---
# The reporter stops looking at the worker's result: a skipped worker then reports green.
yq eval '(.jobs.test.steps[0].run) = "echo ok"' "${CI_WORKFLOW}" >"${MUTANTS}/test-reporter-always-green.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-always-green.yaml" \
  "the 'test' reporter always exits 0" \
  'the no-op path can satisfy the required context' \
  check_reporter "${MUTANTS}/test-reporter-always-green.yaml" test test-suite "test"

yq eval '(.jobs.kind-smoke.steps[0].run) = "echo ok"' "${SMOKE_WORKFLOW}" >"${MUTANTS}/smoke-reporter-always-green.yaml"
mutant_rejected "${SMOKE_WORKFLOW}" "${MUTANTS}/smoke-reporter-always-green.yaml" \
  "the 'kind-smoke' reporter always exits 0" \
  'the no-op path can satisfy the required context' \
  check_reporter "${MUTANTS}/smoke-reporter-always-green.yaml" kind-smoke kind-smoke-run "kind-smoke"

# The fail-safe default inverted: an unclassifiable PR would be treated as docs-only.
# shellcheck disable=SC2016  # a yq program, not a shell expansion
yq eval '(.jobs.test.steps[0].run) |= sub("\\$\\{CODE\\}\" != \"false\"", "${CODE}\" == \"true\"")' \
  "${CI_WORKFLOW}" >"${MUTANTS}/test-reporter-failopen.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-failopen.yaml" \
  "the 'test' reporter treats an unknown verdict as documentation-only instead of failing safe" \
  'the no-op path can satisfy the required context' \
  check_reporter "${MUTANTS}/test-reporter-failopen.yaml" test test-suite "test"

# The reporter stops depending on the worker: needs.<worker>.result is then always empty.
yq eval '.jobs.test.needs = ["changes"]' "${CI_WORKFLOW}" >"${MUTANTS}/test-reporter-no-needs.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-no-needs.yaml" \
  "the 'test' reporter no longer needs test-suite" \
  "must 'needs: test-suite'" \
  check_reporter "${MUTANTS}/test-reporter-no-needs.yaml" test test-suite "test"

# The reporter loses `if: always()`: a skipped worker skips it too, and the context vanishes
# exactly as it did before this lane.
yq eval 'del(.jobs.test.if)' "${CI_WORKFLOW}" >"${MUTANTS}/test-reporter-no-always.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-no-always.yaml" \
  "the 'test' reporter drops if: always()" \
  "must declare 'if: always()'" \
  check_reporter "${MUTANTS}/test-reporter-no-always.yaml" test test-suite "test"

# Real work smuggled into the reporter, which is the one job that runs on docs-only PRs.
yq eval '.jobs.test.steps += [{"name": "sneak", "run": "task coverage"}]' "${CI_WORKFLOW}" \
  >"${MUTANTS}/test-reporter-extra-step.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-extra-step.yaml" \
  "the 'test' reporter grows a second step" \
  'must have exactly one step' \
  check_reporter "${MUTANTS}/test-reporter-extra-step.yaml" test test-suite "test"

# The reporter reads some other job's result.
# shellcheck disable=SC2016
yq eval '.jobs.test.steps[0].env.RESULT = "${{ needs.changes.result }}"' "${CI_WORKFLOW}" \
  >"${MUTANTS}/test-reporter-wrong-binding.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/test-reporter-wrong-binding.yaml" \
  "the 'test' reporter binds RESULT to the wrong job" \
  'must bind RESULT to' \
  check_reporter "${MUTANTS}/test-reporter-wrong-binding.yaml" test test-suite "test"

# --- the classifier ---
# The docs path set widens to swallow a code tree: a Go change would then be called docs-only.
yq eval '(.jobs.changes.steps[] | select(.id == "filter") | .run) |= sub("docs/\\* \\|", "docs/* | internal/* |")' \
  "${CI_WORKFLOW}" >"${MUTANTS}/filter-swallows-internal.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/filter-swallows-internal.yaml" \
  'the changes filter classifies internal/** as documentation' \
  'would satisfy the required contexts through the no-op path' \
  check_changes_filter "${MUTANTS}/filter-swallows-internal.yaml"

# The classifier always answers "documentation-only".
# shellcheck disable=SC2016
yq eval '(.jobs.changes.steps[] | select(.id == "filter") | .run) = "echo \"code=false\" >> \"${GITHUB_OUTPUT}\""' \
  "${CI_WORKFLOW}" >"${MUTANTS}/filter-always-false.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/filter-always-false.yaml" \
  'the changes filter always answers documentation-only' \
  'would satisfy the required contexts through the no-op path' \
  check_changes_filter "${MUTANTS}/filter-always-false.yaml"

# The fail-safe on a degenerate input inverted.
yq eval '(.jobs.changes.steps[] | select(.id == "filter") | .run) |= sub("!= \"pull_request\" \]; then\n  emit true", "!= \"pull_request\" ]; then\n  emit false")' \
  "${CI_WORKFLOW}" >"${MUTANTS}/filter-failopen.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/filter-failopen.yaml" \
  'the changes filter fails OPEN on an event it cannot classify' \
  'it must fail safe' \
  check_changes_filter "${MUTANTS}/filter-failopen.yaml"

# The two copies drift apart.
yq eval '(.jobs.changes.steps[] | select(.id == "filter") | .run) |= . + "\n# drift\n"' \
  "${SMOKE_WORKFLOW}" >"${MUTANTS}/filter-drift.yaml"
mutant_rejected "${SMOKE_WORKFLOW}" "${MUTANTS}/filter-drift.yaml" \
  'the two changes classifiers drift apart' \
  'have drifted apart' \
  check_filters_agree "${CI_WORKFLOW}" "${MUTANTS}/filter-drift.yaml"

# --- the worker ---
# The worker stops being gated and burns on every docs PR (the cost half of the invariant).
yq eval 'del(.jobs.test-suite.if)' "${CI_WORKFLOW}" >"${MUTANTS}/worker-ungated.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/worker-ungated.yaml" \
  'the test-suite worker loses its changes gate' \
  'must be gated on' \
  check_worker "${MUTANTS}/worker-ungated.yaml" test-suite "${RUN_CODE} | test(\"(^|\\n)[[:space:]]*task coverage[[:space:]]*(\\n|\$)\")" 'task coverage'

# The worker keeps the gate but stops doing the work.
yq eval '.jobs.test-suite.steps = [{"name": "nothing", "run": "echo skipped"}]' "${CI_WORKFLOW}" \
  >"${MUTANTS}/worker-gutted.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/worker-gutted.yaml" \
  'the test-suite worker no longer runs task coverage' \
  'has no task coverage step' \
  check_worker "${MUTANTS}/worker-gutted.yaml" test-suite "${RUN_CODE} | test(\"(^|\\n)[[:space:]]*task coverage[[:space:]]*(\\n|\$)\")" 'task coverage'

# The worker is soft-failed: it always "succeeds", so the reporter passes it through.
yq eval '.jobs.kind-smoke-run["continue-on-error"] = true' "${SMOKE_WORKFLOW}" \
  >"${MUTANTS}/worker-soft-fail.yaml"
mutant_rejected "${SMOKE_WORKFLOW}" "${MUTANTS}/worker-soft-fail.yaml" \
  'the kind-smoke-run worker is soft-failed with continue-on-error' \
  'must not declare' \
  check_worker "${MUTANTS}/worker-soft-fail.yaml" kind-smoke-run '(.value.uses // "") == "./.github/actions/kind-e2e-setup"' 'the kind e2e setup action'

# The worker takes the required context name back while keeping the skipping if: -- the
# original defect, moved one job over.
yq eval '.jobs.test-suite.name = "test"' "${CI_WORKFLOW}" >"${MUTANTS}/worker-steals-context.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/worker-steals-context.yaml" \
  'the test-suite worker takes the required context name back' \
  'that is the original defect, moved one job over' \
  check_worker "${MUTANTS}/worker-steals-context.yaml" test-suite "${RUN_CODE} | test(\"(^|\\n)[[:space:]]*task coverage[[:space:]]*(\\n|\$)\")" 'task coverage'

# --- the context producers ---
yq eval '.jobs.test.name = "unit-test"' "${CI_WORKFLOW}" >"${MUTANTS}/context-renamed.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/context-renamed.yaml" \
  "the job producing the 'test' context is renamed" \
  'breaks the ruleset' \
  check_context_job "${MUTANTS}/context-renamed.yaml" test "test"

yq eval '.jobs.analyze.if = "false"' "${CODEQL_WORKFLOW}" >"${MUTANTS}/analyze-disabled.yaml"
mutant_rejected "${CODEQL_WORKFLOW}" "${MUTANTS}/analyze-disabled.yaml" \
  "the CodeQL analyze job becomes conditional" \
  'must run unconditionally' \
  assert_job_can_fail_build "${MUTANTS}/analyze-disabled.yaml" analyze \
  "the required context 'Analyze (Go)' must report a conclusion on every PR, including a documentation-only one"

# --- the wiring ---
yq eval '.jobs.lint.steps = [.jobs.lint.steps[] | select(((.run // "") | contains("'"${GATE_SCRIPT}"'")) | not)]' \
  "${CI_WORKFLOW}" >"${MUTANTS}/gate-unwired.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/gate-unwired.yaml" \
  'this gate is no longer invoked from the lint job' \
  "has no bare 'bash ${GATE_SCRIPT}' invocation step" \
  check_wired "${MUTANTS}/gate-unwired.yaml"

yq eval '(.jobs.lint.steps[] | select((.run // "") | contains("'"${GATE_SCRIPT}"'")) | .run) |= . + " || true"' \
  "${CI_WORKFLOW}" >"${MUTANTS}/gate-neutered.yaml"
mutant_rejected "${CI_WORKFLOW}" "${MUTANTS}/gate-neutered.yaml" \
  'the gate invocation is neutered with || true' \
  "has no bare 'bash ${GATE_SCRIPT}' invocation step" \
  check_wired "${MUTANTS}/gate-neutered.yaml"

# --- guard rails on the self-test itself ---
# These four are the degenerate "rejections" that an any-nonzero-exit check would accept as
# proof. mutant_rejected must refuse every one of them, or the self-test is tautological.
self_test_guard_holds() {
  local mutant="$1" label="$2"
  shift 2
  if (mutant_rejected "${CI_WORKFLOW}" "${mutant}" "${label}" 'unreachable-expected-message' "$@") >/dev/null 2>&1; then
    fail "self-test: mutant_rejected accepted ${label} as a genuine rejection -- it is tautological again"
  fi
  pass "self-test: mutant_rejected refuses to count ${label} as a rejection"
}

: >"${MUTANTS}/empty.yaml"
self_test_guard_holds "${MUTANTS}/empty.yaml" 'an empty file' \
  check_pr_trigger_unfiltered "${MUTANTS}/empty.yaml" "test"
printf 'jobs: [[[\n' >"${MUTANTS}/broken.yaml"
self_test_guard_holds "${MUTANTS}/broken.yaml" 'an unparseable file' \
  check_pr_trigger_unfiltered "${MUTANTS}/broken.yaml" "test"
self_test_guard_holds "${MUTANTS}/does-not-exist.yaml" 'a nonexistent path' \
  check_pr_trigger_unfiltered "${MUTANTS}/does-not-exist.yaml" "test"
cp "${CI_WORKFLOW}" "${MUTANTS}/unmutated.yaml"
self_test_guard_holds "${MUTANTS}/unmutated.yaml" 'an unmutated copy of the real workflow' \
  check_pr_trigger_unfiltered "${MUTANTS}/unmutated.yaml" "test"

echo "All CI docs-gate (CI-DOCSGATE-01) tests passed."
