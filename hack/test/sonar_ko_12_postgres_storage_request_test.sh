#!/usr/bin/env bash
# Unit test for kubernetes:S6897 on the throwaway sample Postgres Deployment.
#
# SonarCloud flags the container when it has no storage request. The sample
# already bounds writable-layer/emptyDir usage with an ephemeral-storage
# *limit* (KO-05 / S6870). A matching *request* is required so the scheduler
# accounts for that disk, not only so a runaway container is killed.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SAMPLE="${ROOT}/config/samples/dev/postgres.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

command -v yq >/dev/null 2>&1 || fail "yq is required to run this test"
[[ -f "${SAMPLE}" ]] || fail "expected ${SAMPLE} to exist"

REQ="$(yq eval 'select(.kind == "Deployment") | .spec.template.spec.containers[0].resources.requests.ephemeral-storage' "${SAMPLE}")"
if [[ -z "${REQ}" || "${REQ}" == "null" ]]; then
  fail "sample Postgres container has no resources.requests.ephemeral-storage (kubernetes:S6897)"
fi
pass "sample Postgres container requests ephemeral-storage: ${REQ}"

LIMIT="$(yq eval 'select(.kind == "Deployment") | .spec.template.spec.containers[0].resources.limits.ephemeral-storage' "${SAMPLE}")"
if [[ -z "${LIMIT}" || "${LIMIT}" == "null" ]]; then
  fail "sample Postgres container lost resources.limits.ephemeral-storage (kubernetes:S6870)"
fi
pass "sample Postgres container still limits ephemeral-storage: ${LIMIT}"

echo "All sonar_ko_12 postgres storage-request tests passed."
