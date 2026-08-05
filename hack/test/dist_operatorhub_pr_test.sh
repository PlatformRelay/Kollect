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

DRY_RUN=1 VERSION=9.9.9-test IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  bash "${SCRIPT}" >/tmp/kollect-operatorhub-dry-run.out 2>&1 ||
  fail "DRY_RUN operatorhub-pr.sh failed"

grep -Fq 'DRY_RUN' /tmp/kollect-operatorhub-dry-run.out ||
  fail "DRY_RUN invocation did not report dry-run behavior"

pass "operatorhub-pr.sh and release workflow wiring look correct"

echo "All dist OperatorHub PR tests passed."
