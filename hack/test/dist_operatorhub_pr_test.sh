#!/usr/bin/env bash
# DIST-OLM-02: operatorhub-pr.sh supports kollect dual-upstream submission with DRY_RUN.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"
SCRIPT="${ROOT}/hack/operatorhub-pr.sh"
WORKFLOW="${ROOT}/.github/workflows/release.yaml"

fail() {
  printf 'dist operatorhub pr: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -x "${SCRIPT}" || -f "${SCRIPT}" ]] || fail "${SCRIPT} is missing"
[[ -f "${WORKFLOW}" ]] || fail "${WORKFLOW} is missing"

grep -Fq 'k8s-operatorhub/community-operators' "${SCRIPT}" ||
  fail "operatorhub-pr.sh must submit to k8s-operatorhub/community-operators"
grep -Fq 'redhat-openshift-ecosystem/community-operators-prod' "${SCRIPT}" ||
  fail "operatorhub-pr.sh must submit to community-operators-prod"
grep -Fq 'v4.19' "${SCRIPT}" ||
  fail "operatorhub-pr.sh must annotate OpenShift v4.19 for prod catalog"

# The three checks below inspect EXECUTABLE lines only — the comments in operatorhub-pr.sh
# legitimately name the very anti-patterns being banned, and a naive grep matches its own docs.
CODE="$(grep -v '^[[:space:]]*#' "${SCRIPT}")"

# GNU-only `sed -i <script>` works on the CI runner and fails on BSD/macOS sed, where -i
# consumes the next argument as a backup suffix. This script is documented as manually
# re-runnable, so in-place edits must stay portable (awk, or an explicit -i.bak suffix).
if printf '%s\n' "${CODE}" | grep -Fq "sed -i '"; then
  fail "operatorhub-pr.sh must not use GNU-only 'sed -i <script>' (breaks on BSD/macOS sed)"
fi

# The EXISTING-PR LOOKUP must match the head owner case-insensitively. GitHub stores the
# canonical org casing ("PlatformRelay"), so a `gh pr list --head "<owner>:<branch>"` filter
# built from a lower-cased FORK_OWNER matches nothing and the script then tries to open a
# duplicate PR and dies. (`gh pr create --head <owner>:<branch>` is fine — creation resolves
# the owner case-insensitively; only the list filter is a literal string match.)
printf '%s\n' "${CODE}" | grep -Fq 'headRepositoryOwner' ||
  fail "operatorhub-pr.sh must resolve an existing PR via headRepositoryOwner (case-insensitive), not a literal \${FORK_OWNER}:\${BRANCH} list filter"
printf '%s\n' "${CODE}" | grep -Fq 'ascii_downcase' ||
  fail "operatorhub-pr.sh must compare the PR head owner case-insensitively (ascii_downcase)"

# `gh pr edit` resolves assignees/labels/reviewers over GraphQL and requires `read:org`,
# forcing a broader PAT than this cross-repo submission needs. PATCH via `gh api` instead.
if printf '%s\n' "${CODE}" | grep -Fq 'gh pr edit'; then
  fail "operatorhub-pr.sh must not use 'gh pr edit' (requires read:org); PATCH the PR via gh api"
fi
grep -Fq 'FORK_OWNER="${FORK_OWNER:-platformrelay}"' "${SCRIPT}" ||
  fail "operatorhub-pr.sh default FORK_OWNER must be platformrelay"
grep -Fq 'DRY_RUN' "${SCRIPT}" ||
  fail "operatorhub-pr.sh must support DRY_RUN"
grep -Fq 'operators/kollect' "${SCRIPT}" ||
  fail "operatorhub-pr.sh operator dir must be operators/kollect"

# DIST-OH-02: the structural file-presence checks in operatorhub-pr.sh cannot see a
# non-standard category, a missing alm-examples entry or a malformed minKubeVersion. The
# submission path must run the modern validator set before pushing to a third-party repo.
printf '%s\n' "${CODE}" | grep -Fq 'make validate-olm-bundle' ||
  fail "operatorhub-pr.sh must run 'make validate-olm-bundle' before submitting the bundle upstream"

# GATE-SCOPE-01: every assertion about release.yaml below is STRUCTURAL and scoped to the
# operatorhub-pr job's own steps. It used to be a set of file-global `grep`s, which is the same
# defect class GATE-HARDEN-01 fixed in the sibling gate (hack/test/dist_ci_wiring_test.sh) by
# anchoring to `.jobs.lint.steps[]`. Measured against the real release.yaml, three mutations left
# this gate at rc=0 printing "All dist OperatorHub PR tests passed":
#
#   - Deleting `chmod +x hack/operatorhub-pr.sh` and `hack/operatorhub-pr.sh` from the submission
#     step. The workflow then submits nothing, but the literal survives on the REPORTING step's
#     `::warning` and job-summary echoes. Comment-stripping (GATE-COMMENT-01) cannot see this:
#     those echoes are executable lines. Only the LOAD-BEARING occurrence counts, so the
#     invocation is now matched inside the submission step, as an invocation rather than as text
#     anywhere in the file -- `chmod +x hack/operatorhub-pr.sh` alone is not an invocation.
#   - Repointing `GH_TOKEN` at a different secret. The step's own
#     `echo "OPERATORHUB_PAT not configured; skipping..."` satisfied a file-global grep while the
#     cross-repo submission had no credential. The token is now read off `.env.GH_TOKEN`, where
#     no echo can stand in for it.
#   - Moving `bash hack/install-operator-sdk.sh ./bin` out of operatorhub-pr into the release
#     job. The gate stayed green while its failure message claimed "operatorhub-pr.sh hard-fails
#     without it" -- exactly the GATE-HARDEN-01 shape. The install step is now counted within
#     this job only, and exactly once.
#
# The old self-test could not have caught the first two: it stripped EVERY executable line
# carrying a literal, so a mutation that removes only the load-bearing one is invisible to it.
# `operatorhub-pr:` and `continue-on-error: true` are gone as literals too -- the job type guard
# and the per-step `continue-on-error` assertions below already say that structurally.
command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the operatorhub-pr job"

JOB='.jobs["operatorhub-pr"]'
SUBMIT_STEP='Generate OLM bundle and create OperatorHub PRs'
REPORT_STEP='Report OperatorHub submission outcome'
SDK_STEP='Install operator-sdk (OLM bundle validators)'

# GATE-COMMENT-01 discipline, one trimmed line per element: every line of a step's `run:` body
# whose first non-blank character is `#` is dropped before matching, so a commented-out command
# is not a command. `.` is the step map.
RUN_CODE_LINES='((.run // "") | split("\n") | map(select(test("^[[:space:]]*#") | not) | sub("^[[:space:]]*"; "") | sub("[[:space:]]*$"; "")))'

# The executable lines of the one step in this job with the given name.
job_step_code() {
  local workflow="$1" name="$2"
  yq eval "${JOB}.steps[] | select(.name == \"${name}\") | ${RUN_CODE_LINES} | .[]" "${workflow}"
}

job_step_field() {
  local workflow="$1" name="$2" field="$3"
  yq eval "${JOB}.steps[] | select(.name == \"${name}\") | ${field}" "${workflow}"
}

check_operatorhub_job() {
  local workflow="$1"
  local job_kind steps_kind operatorhub_env operatorhub_needs operatorhub_ref
  local step_name step_kind step_coe submit_code token sdk_count report_code outcome

  [[ -f "${workflow}" ]] || fail "expected ${workflow}"

  # Without these two guards a renamed job or a stepless job makes every assertion below select
  # over an empty list, and the gate passes vacuously.
  job_kind="$(yq eval "${JOB} | type" "${workflow}")"
  [[ "${job_kind}" == "!!map" ]] ||
    fail "release workflow has no operatorhub-pr job (assertions below would pass vacuously)"
  steps_kind="$(yq eval "${JOB}.steps | type" "${workflow}")"
  [[ "${steps_kind}" == "!!seq" ]] ||
    fail "the operatorhub-pr job declares no steps -- the assertions below would pass vacuously"

  # The job carries secrets.OPERATORHUB_PAT — a cross-repo write credential for two
  # third-party repositories. It must be gated by the same protected environment as
  # the release job, or the token is reachable without the eligibility gate.
  operatorhub_env="$(yq eval "${JOB}.environment" "${workflow}")"
  [[ "${operatorhub_env}" != "null" && -n "${operatorhub_env}" ]] ||
    fail "operatorhub-pr job must declare an 'environment:' — it holds secrets.OPERATORHUB_PAT"

  # The release job deliberately checks out the immutable SHA proven by eligibility,
  # "never the mutable tag ref alone". operatorhub-pr must do the same.
  operatorhub_needs="$(yq eval "${JOB}.needs | join(\",\")" "${workflow}")"
  [[ ",${operatorhub_needs}," == *",eligibility,"* ]] ||
    fail "operatorhub-pr must need the eligibility job to consume its proven SHA (needs: ${operatorhub_needs})"

  operatorhub_ref="$(yq eval "${JOB}.steps[0].with.ref" "${workflow}")"
  [[ "${operatorhub_ref}" == *'needs.eligibility.outputs.sha'* ]] ||
    fail "operatorhub-pr must check out needs.eligibility.outputs.sha, not the mutable tag (got: ${operatorhub_ref})"

  for step_name in "${SUBMIT_STEP}" "${REPORT_STEP}"; do
    step_kind="$(job_step_field "${workflow}" "${step_name}" 'type')"
    [[ "${step_kind}" == "!!map" ]] ||
      fail "operatorhub-pr job has no step named '${step_name}' (assertion would pass vacuously)"
    step_coe="$(job_step_field "${workflow}" "${step_name}" '.["continue-on-error"]')"
    [[ "${step_coe}" == "true" ]] ||
      fail "'${step_name}' must declare continue-on-error: true — OperatorHub submission is discoverability only and must never fail the pipeline"
  done

  # The LOAD-BEARING invocation, inside the submission step. A `chmod +x` on the same script is
  # not an invocation, and the reporting step's echoes of the same path are not either.
  submit_code="$(job_step_code "${workflow}" "${SUBMIT_STEP}")"
  grep -Eq '^(bash[[:space:]]+)?(\./)?hack/operatorhub-pr\.sh([[:space:]]|$)' <<<"${submit_code}" ||
    fail "the '${SUBMIT_STEP}' step must actually invoke hack/operatorhub-pr.sh — no executable line of its run: body does, so the job submits nothing (a 'chmod +x' on the script, or an echo of its path from the reporting step, is not an invocation)"

  # Read the credential off the step's env, not off the file: the step's own
  # "OPERATORHUB_PAT not configured" echo is executable text and satisfies any text search.
  token="$(job_step_field "${workflow}" "${SUBMIT_STEP}" '.env.GH_TOKEN')"
  [[ "${token}" == *'secrets.OPERATORHUB_PAT'* ]] ||
    fail "the '${SUBMIT_STEP}' step must pass GH_TOKEN from secrets.OPERATORHUB_PAT (got: ${token}) — the cross-repo submission has no other credential, and the step's own 'OPERATORHUB_PAT not configured' echo is not one"

  # DIST-OH-02: operatorhub-pr.sh runs the modern OperatorHub validator set and hard-fails
  # without operator-sdk, so THIS job must install it. A step in another job does not count.
  sdk_count="$(yq eval "[${JOB}.steps[] | select(${RUN_CODE_LINES} | join(\"\n\") | contains(\"hack/install-operator-sdk.sh\"))] | length" "${workflow}")"
  case "${sdk_count}" in
  1) ;;
  0) fail "the operatorhub-pr job has no hack/install-operator-sdk.sh step — operatorhub-pr.sh hard-fails without it, and an install step in another job does not put the binary on this runner" ;;
  *) fail "the operatorhub-pr job installs operator-sdk ${sdk_count} times — keep exactly one so this assertion cannot be satisfied by a stray duplicate" ;;
  esac

  # Soft-fail must not be silent: continue-on-error pins .conclusion to "success",
  # so the reporting step has to read .outcome — and say so where a human sees it.
  outcome="$(job_step_field "${workflow}" "${REPORT_STEP}" '.env.OPERATORHUB_OUTCOME')"
  [[ "${outcome}" == *'steps.operatorhub.outcome'* ]] ||
    fail "'${REPORT_STEP}' must read steps.operatorhub.outcome (got: ${outcome}) — conclusion is always 'success' under continue-on-error"
  report_code="$(job_step_code "${workflow}" "${REPORT_STEP}")"
  grep -Fq '::warning title=OperatorHub submission did not complete' <<<"${report_code}" ||
    fail "a failed OperatorHub submission must emit a ::warning:: annotation from '${REPORT_STEP}'"
  grep -Fq 'OperatorHub PRs were NOT created' <<<"${report_code}" ||
    fail "a failed OperatorHub submission must write a GITHUB_STEP_SUMMARY line from '${REPORT_STEP}'"
}

check_operatorhub_job "${WORKFLOW}"

# Self-test: a gate that only passes on the happy path is not evidence. Mutate the real workflow
# with yq (structural, so innocent reformatting cannot spuriously red this) and assert the checks
# above reject each mutation — including the two that no amount of comment-stripping could catch.
# `${expect}` is the load-bearing half: `cmp` only rules out no-op mutations, it says nothing
# about WHICH check fired, and "some check failed" is not evidence that this one did.
WORKFLOW_MUTANTS="$(mktemp -d)"
trap 'rm -rf "${WORKFLOW_MUTANTS}"' EXIT

mutant_rejected() {
  local mutant="$1" label="$2" expect="$3"
  local output status=0

  [[ -s "${mutant}" ]] ||
    fail "self-test: the mutant for '${label}' is missing or empty — the yq mutation step failed, so nothing was actually tested"
  if cmp -s "${mutant}" "${WORKFLOW}"; then
    fail "self-test: the mutant for '${label}' is byte-identical to ${WORKFLOW} — the yq mutation was a no-op, so nothing was actually tested"
  fi

  # Subshell: fail's `exit 1` must not take the parent down — rejection is the expectation.
  output="$( (check_operatorhub_job "${mutant}") 2>&1 )" || status=$?
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate still passed on a workflow where ${label} — it is vacuous"
  [[ "${output}" == *"${expect}"* ]] ||
    fail "self-test: the gate rejected '${label}' but not for the intended reason — expected the failure to mention '${expect}', got: ${output}"
  pass "self-test: gate rejects a release workflow where ${label}"
}

yq eval "
  (${JOB}.steps[] | select(.name == \"${SUBMIT_STEP}\") | .run) |=
    (split(\"\n\") | map(select(test(\"^[[:space:]]*(chmod \\\\+x )?hack/operatorhub-pr\\\\.sh[[:space:]]*\$\") | not)) | join(\"\n\"))
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/submission-not-invoked.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/submission-not-invoked.yaml" \
  'the submission step no longer invokes hack/operatorhub-pr.sh, while the reporting step still echoes its path' \
  'must actually invoke hack/operatorhub-pr.sh'

yq eval "
  (${JOB}.steps[] | select(.name == \"${SUBMIT_STEP}\") | .env.GH_TOKEN) = \"\${{ secrets.SOME_OTHER_TOKEN }}\"
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/token-repointed.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/token-repointed.yaml" \
  'GH_TOKEN is repointed at another secret, while the step still echoes "OPERATORHUB_PAT not configured"' \
  'must pass GH_TOKEN from secrets.OPERATORHUB_PAT'

yq eval "
  .jobs.release.steps += [${JOB}.steps[] | select((.run // \"\") | contains(\"hack/install-operator-sdk.sh\"))] |
  ${JOB}.steps = [${JOB}.steps[] | select(((.run // \"\") | contains(\"hack/install-operator-sdk.sh\")) | not)]
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/sdk-moved-to-release.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/sdk-moved-to-release.yaml" \
  'the operator-sdk install moved out of operatorhub-pr into the release job' \
  'has no hack/install-operator-sdk.sh step'

yq eval "
  ${JOB}.steps += [${JOB}.steps[] | select((.run // \"\") | contains(\"hack/install-operator-sdk.sh\"))]
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/sdk-duplicated.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/sdk-duplicated.yaml" \
  'the operator-sdk install appears twice in operatorhub-pr' \
  'installs operator-sdk 2 times'

# GATE-COMMENT-01 kept in force: a commented-out command is not a command. This is the shape the
# old every-executable-line strip loop covered, restated against the new structural checks.
yq eval "
  (${JOB}.steps[] | select(.name == \"${SDK_STEP}\") | .run) |= \"# \" + .
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/sdk-commented-out.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/sdk-commented-out.yaml" \
  'the operator-sdk install body is commented out' \
  'has no hack/install-operator-sdk.sh step'

yq eval "
  (${JOB}.steps[] | select(.name == \"${SUBMIT_STEP}\") | .run) |=
    sub(\"(?m)^([[:space:]]*)hack/operatorhub-pr\\\\.sh\$\", \"\${1}# hack/operatorhub-pr.sh\")
" "${WORKFLOW}" >"${WORKFLOW_MUTANTS}/submission-commented-out.yaml"
mutant_rejected "${WORKFLOW_MUTANTS}/submission-commented-out.yaml" \
  'the submission step invocation is commented out' \
  'must actually invoke hack/operatorhub-pr.sh'

pass "operatorhub-pr job is environment-gated, SHA-pinned and visibly soft-fail"

DRY_RUN=1 VERSION=9.9.9-test IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  bash "${SCRIPT}" >/tmp/kollect-operatorhub-dry-run.out 2>&1 ||
  fail "DRY_RUN operatorhub-pr.sh failed"

grep -Fq 'DRY_RUN' /tmp/kollect-operatorhub-dry-run.out ||
  fail "DRY_RUN invocation did not report dry-run behavior"

pass "operatorhub-pr.sh and release workflow wiring look correct"

echo "All dist OperatorHub PR tests passed."
