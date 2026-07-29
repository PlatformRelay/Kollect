#!/usr/bin/env bash
# Unit test for SEC-04d (kubernetes:S6865, KO-04): the kollect-ui chart Pod must not
# auto-mount a default ServiceAccount token it never uses. kollect-ui is a static
# nginx SPA (see ui/Dockerfile) — the browser-side k8s API stub in
# ui/src/api/k8s-status.ts runs client-side, not from the Pod, so the Pod itself has
# no legitimate need for a mounted k8s API credential.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART="${ROOT}/charts/kollect-ui"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

command -v helm >/dev/null 2>&1 || fail "helm binary is required to render the chart"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

RENDERED="${TMP}/rendered.yaml"
helm template test-release "${CHART}" >"${RENDERED}" 2>"${TMP}/err.txt" \
  || fail "helm template failed: $(cat "${TMP}/err.txt")"

# --- Deployment Pod spec must explicitly disable SA token automount ---
DEPLOYMENT_DOC="${TMP}/deployment.yaml"
awk '/^# Source: kollect-ui\/templates\/deployment.yaml$/{flag=1; next} /^# Source:/{flag=0} flag' \
  "${RENDERED}" >"${DEPLOYMENT_DOC}"
[[ -s "${DEPLOYMENT_DOC}" ]] || fail "could not isolate rendered Deployment document"

grep -q '^\s*automountServiceAccountToken: false\s*$' "${DEPLOYMENT_DOC}" \
  || fail "Deployment Pod spec must set automountServiceAccountToken: false"
pass "Deployment Pod spec sets automountServiceAccountToken: false"

# --- Sanity: the flag must live under the Pod template spec, not just anywhere ---
awk '/^spec:$/{s++} s==1 && /^  template:$/{t=1} t && /^    spec:$/{p=1} p' "${DEPLOYMENT_DOC}" \
  | grep -q 'automountServiceAccountToken: false' \
  || fail "automountServiceAccountToken: false must be set on the Pod template spec"
pass "flag is set on the Pod template spec"

echo "All sonar_ko_04 (kollect-ui automount) tests passed."
