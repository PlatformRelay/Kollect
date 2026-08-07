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
grep -Fq 'FORK_OWNER="${FORK_OWNER:-platformrelay}"' "${SCRIPT}" ||
  fail "operatorhub-pr.sh default FORK_OWNER must be platformrelay"
grep -Fq 'DRY_RUN' "${SCRIPT}" ||
  fail "operatorhub-pr.sh must support DRY_RUN"
grep -Fq 'operators/kollect' "${SCRIPT}" ||
  fail "operatorhub-pr.sh operator dir must be operators/kollect"

grep -Fq 'operatorhub-pr:' "${WORKFLOW}" ||
  fail "release workflow must define operatorhub-pr job"
grep -Fq 'OPERATORHUB_PAT' "${WORKFLOW}" ||
  fail "release workflow must reference OPERATORHUB_PAT"
grep -Fq 'continue-on-error: true' "${WORKFLOW}" ||
  fail "release workflow operatorhub step must soft-fail"
grep -Fq 'hack/operatorhub-pr.sh' "${WORKFLOW}" ||
  fail "release workflow must invoke hack/operatorhub-pr.sh"

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
