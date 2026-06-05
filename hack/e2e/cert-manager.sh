#!/usr/bin/env bash
# cert-manager.io/Certificate generic CRD collection smoke (kind e2e / nightly).
# Installs Certificate CRDs, applies profile/target samples, seeds a Certificate, asserts Target Ready.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-180s}"
readonly CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.2}"
readonly CERT_TEST_NS="${CERT_TEST_NS:-cert-test}"

_kind_require kubectl
kind_use_context "$CLUSTER_NAME"

_log() { echo "[cert-manager-e2e] $*"; }

_log "Installing cert-manager CRDs (${CERT_MANAGER_VERSION})..."
kubectl apply -f \
  "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.crds.yaml"

_log "Applying Certificate profile and target samples..."
kubectl apply -f "${REPO_ROOT}/config/samples/kollect_v1alpha1_kollectprofile_certificate-summary.yaml"
kubectl apply -f "${REPO_ROOT}/config/samples/kollect_v1alpha1_kollecttarget_certificates.yaml"

_log "Creating namespace ${CERT_TEST_NS} with collection label..."
kubectl create namespace "$CERT_TEST_NS" --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace "$CERT_TEST_NS" \
  kollect.dev/collect-certificates=enabled --overwrite

_log "Seeding Certificate for generic CRD collection..."
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: smoke-selfsigned
  namespace: ${CERT_TEST_NS}
spec:
  secretName: smoke-selfsigned-tls
  issuerRef:
    name: smoke-issuer
    kind: ClusterIssuer
  commonName: smoke.example.com
  dnsNames:
    - smoke.example.com
EOF

_log "Waiting for KollectTarget team-certificates Ready..."
kubectl wait --for=condition=Ready kollecttarget/team-certificates \
  -n default --timeout="$WAIT_TIMEOUT"

_log "Waiting for Certificate in inventory HTTP payload..."
kubectl port-forward -n "$KOLLECT_NAMESPACE" svc/kollect-controller-manager 18084:8082 &
pf_pid=$!
trap 'kill "$pf_pid" 2>/dev/null || true' EXIT
sleep 2

deadline=$((SECONDS + 180))
while (( SECONDS < deadline )); do
  if curl -sf http://127.0.0.1:18084/inventory | grep -q smoke-selfsigned; then
    _log "Certificate collection smoke passed."
    exit 0
  fi
  sleep 5
done

echo "certificate smoke-selfsigned not found in inventory HTTP within timeout" >&2
kubectl describe kollecttarget team-certificates -n default
curl -sf http://127.0.0.1:18084/inventory || true
exit 1
