#!/usr/bin/env bash
# Unit test for SEC-04e / KO-05: the kollect Helm chart's manager container must set
# resources.limits.ephemeral-storage, so a filled /tmp emptyDir (git mirror clones,
# git exports) or writable-layer growth cannot exhaust node disk and starve other
# pods on the same node (SonarCloud resource-exhaustion / noisy-neighbor finding).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART="${ROOT}/charts/kollect"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

need_yq() {
  command -v yq >/dev/null 2>&1 || fail "yq is required to run this test"
}

need_yq

# --- default resources profile: ephemeral-storage limit must be set and non-empty ---
DEFAULT_RENDER="$(helm template test-release "${CHART}" --show-only templates/deployment.yaml)"
DEFAULT_EPHEMERAL="$(echo "${DEFAULT_RENDER}" | yq eval '.spec.template.spec.containers[0].resources.limits.ephemeral-storage' -)"
if [[ -z "${DEFAULT_EPHEMERAL}" || "${DEFAULT_EPHEMERAL}" == "null" ]]; then
  fail "default-profile manager container has no resources.limits.ephemeral-storage set"
fi
pass "default-profile manager container sets ephemeral-storage limit: ${DEFAULT_EPHEMERAL}"

# --- large-cluster resources profile: ephemeral-storage limit must also be set ---
LARGE_RENDER="$(helm template test-release "${CHART}" --show-only templates/deployment.yaml --set resourcesProfile=large)"
LARGE_EPHEMERAL="$(echo "${LARGE_RENDER}" | yq eval '.spec.template.spec.containers[0].resources.limits.ephemeral-storage' -)"
if [[ -z "${LARGE_EPHEMERAL}" || "${LARGE_EPHEMERAL}" == "null" ]]; then
  fail "large-cluster-profile manager container has no resources.limits.ephemeral-storage set"
fi
pass "large-cluster-profile manager container sets ephemeral-storage limit: ${LARGE_EPHEMERAL}"

# --- override: operators can override the default value via values.yaml ---
OVERRIDE_RENDER="$(helm template test-release "${CHART}" --show-only templates/deployment.yaml --set resources.limits.ephemeral-storage=2Gi)"
OVERRIDE_EPHEMERAL="$(echo "${OVERRIDE_RENDER}" | yq eval '.spec.template.spec.containers[0].resources.limits.ephemeral-storage' -)"
[[ "${OVERRIDE_EPHEMERAL}" == "2Gi" ]] || fail "expected overridden ephemeral-storage limit '2Gi', got '${OVERRIDE_EPHEMERAL}'"
pass "ephemeral-storage limit is operator-overridable"

echo "All sonar_ko_05 ephemeral-storage tests passed."
