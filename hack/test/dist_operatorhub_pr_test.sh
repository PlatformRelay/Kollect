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

# GATE-COMMENT-01: same discipline as ${CODE} above, applied to the workflow. release.yaml
# documents this job's own contract in comments that repeat the very literals asserted here, so
# a raw grep over the raw file matches its own documentation. Measured against release.yaml:
#
#   - OPERATORHUB_PAT        — comments at :478 and :481. Removing every executable line that
#                              carries it left the raw grep GREEN. Vacuous before this change.
#   - hack/operatorhub-pr.sh — comment at :499. Same: GREEN with the real invocation gone.
#                              Vacuous before this change.
#   - operatorhub-pr:, continue-on-error: true, hack/install-operator-sdk.sh — no comment
#     carries these today, so the raw greps already redded on removal. Stripped anyway: "no
#     comment happens to carry it today" is not a property anyone maintains.
#
# (An earlier revision of this comment justified leaving the first four raw, on the claim that a
# raw install-operator-sdk grep "would keep passing if the `run:` line were ever commented out or
# deleted". The deletion half was false — and it was the other literals that were vacuous. The
# self-test below now measures this instead of asserting it in prose.)
workflow_code() { grep -v '^[[:space:]]*#' "$1"; }

check_workflow_code_refs() {
  local workflow="$1" code
  code="$(workflow_code "${workflow}")"
  printf '%s\n' "${code}" | grep -Fq 'operatorhub-pr:' ||
    fail "release workflow must define operatorhub-pr job"
  printf '%s\n' "${code}" | grep -Fq 'OPERATORHUB_PAT' ||
    fail "release workflow must reference OPERATORHUB_PAT"
  printf '%s\n' "${code}" | grep -Fq 'continue-on-error: true' ||
    fail "release workflow operatorhub step must soft-fail"
  printf '%s\n' "${code}" | grep -Fq 'hack/operatorhub-pr.sh' ||
    fail "release workflow must invoke hack/operatorhub-pr.sh"
  printf '%s\n' "${code}" | grep -Fq 'hack/install-operator-sdk.sh' ||
    fail "release workflow must install operator-sdk — operatorhub-pr.sh hard-fails without it"
}

check_workflow_code_refs "${WORKFLOW}"

# Self-test: prove the comment-stripping is load-bearing rather than decorative. For each
# literal, build a copy of release.yaml with every EXECUTABLE line carrying it removed and the
# comments left untouched, then assert the check above reds. The `cmp` guard is the other half:
# if no executable line carries the literal, the assertion above is asserting nothing and this
# self-test says so instead of quietly passing.
WORKFLOW_MUTANTS="$(mktemp -d)"
trap 'rm -rf "${WORKFLOW_MUTANTS}"' EXIT

strip_executable_occurrences() {
  local workflow="$1" literal="$2"
  awk -v lit="${literal}" '
    /^[[:space:]]*#/ { print; next }
    index($0, lit) == 0 { print }
  ' "${workflow}"
}

for literal in \
  'operatorhub-pr:' \
  'OPERATORHUB_PAT' \
  'continue-on-error: true' \
  'hack/operatorhub-pr.sh' \
  'hack/install-operator-sdk.sh'; do
  mutant="${WORKFLOW_MUTANTS}/stripped.yaml"
  strip_executable_occurrences "${WORKFLOW}" "${literal}" >"${mutant}"
  [[ -s "${mutant}" ]] ||
    fail "self-test: stripping '${literal}' produced an empty file — the awk mutation step failed"
  if cmp -s "${mutant}" "${WORKFLOW}"; then
    fail "self-test: no executable line of ${WORKFLOW} carries '${literal}' — the assertion for it is satisfied by comments alone and asserts nothing"
  fi
  # Subshell: fail's `exit 1` must not take the parent down — rejection is the expectation.
  if (check_workflow_code_refs "${mutant}") >/dev/null 2>&1; then
    fail "self-test: the workflow reference checks still pass with every executable '${literal}' line removed — the surviving comments satisfy them, so the check is vacuous"
  fi
  pass "self-test: workflow reference check reds when '${literal}' survives only in comments"
done

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the operatorhub-pr job"

JOB='.jobs["operatorhub-pr"]'

job_kind="$(yq eval "${JOB} | type" "${WORKFLOW}")"
[[ "${job_kind}" == "!!map" ]] ||
  fail "release workflow has no operatorhub-pr job (assertions below would pass vacuously)"

# The job carries secrets.OPERATORHUB_PAT — a cross-repo write credential for two
# third-party repositories. It must be gated by the same protected environment as
# the release job, or the token is reachable without the eligibility gate.
OPERATORHUB_ENV="$(yq eval "${JOB}.environment" "${WORKFLOW}")"
[[ "${OPERATORHUB_ENV}" != "null" && -n "${OPERATORHUB_ENV}" ]] ||
  fail "operatorhub-pr job must declare an 'environment:' — it holds secrets.OPERATORHUB_PAT"

# The release job deliberately checks out the immutable SHA proven by eligibility,
# "never the mutable tag ref alone". operatorhub-pr must do the same.
OPERATORHUB_NEEDS="$(yq eval "${JOB}.needs | join(\",\")" "${WORKFLOW}")"
[[ ",${OPERATORHUB_NEEDS}," == *",eligibility,"* ]] ||
  fail "operatorhub-pr must need the eligibility job to consume its proven SHA (needs: ${OPERATORHUB_NEEDS})"

OPERATORHUB_REF="$(yq eval "${JOB}.steps[0].with.ref" "${WORKFLOW}")"
[[ "${OPERATORHUB_REF}" == *'needs.eligibility.outputs.sha'* ]] ||
  fail "operatorhub-pr must check out needs.eligibility.outputs.sha, not the mutable tag (got: ${OPERATORHUB_REF})"

for step_name in \
  "Generate OLM bundle and create OperatorHub PRs" \
  "Report OperatorHub submission outcome"; do
  step_kind="$(yq eval "${JOB}.steps[] | select(.name == \"${step_name}\") | type" "${WORKFLOW}")"
  [[ "${step_kind}" == "!!map" ]] ||
    fail "operatorhub-pr job has no step named '${step_name}' (assertion would pass vacuously)"
  step_coe="$(yq eval "${JOB}.steps[] | select(.name == \"${step_name}\") | .[\"continue-on-error\"]" "${WORKFLOW}")"
  [[ "${step_coe}" == "true" ]] ||
    fail "'${step_name}' must declare continue-on-error: true — OperatorHub submission is discoverability only and must never fail the pipeline"
done

# Soft-fail must not be silent: continue-on-error pins .conclusion to "success",
# so the reporting step has to read .outcome.
grep -Fq 'steps.operatorhub.outcome' "${WORKFLOW}" ||
  fail "release workflow must read steps.operatorhub.outcome (conclusion is always 'success' under continue-on-error)"
grep -Fq '::warning title=OperatorHub submission did not complete' "${WORKFLOW}" ||
  fail "a failed OperatorHub submission must emit a ::warning:: annotation"
grep -Fq 'OperatorHub PRs were NOT created' "${WORKFLOW}" ||
  fail "a failed OperatorHub submission must write a GITHUB_STEP_SUMMARY line"

pass "operatorhub-pr job is environment-gated, SHA-pinned and visibly soft-fail"

DRY_RUN=1 VERSION=9.9.9-test IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  bash "${SCRIPT}" >/tmp/kollect-operatorhub-dry-run.out 2>&1 ||
  fail "DRY_RUN operatorhub-pr.sh failed"

grep -Fq 'DRY_RUN' /tmp/kollect-operatorhub-dry-run.out ||
  fail "DRY_RUN invocation did not report dry-run behavior"

pass "operatorhub-pr.sh and release workflow wiring look correct"

echo "All dist OperatorHub PR tests passed."
