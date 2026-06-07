#!/usr/bin/env bash
# Install cert-manager controller for webhook-enabled e2e (Tier 1 / setup-webhook.sh).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

readonly CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.2}"

_kind_require kubectl
kind_use_context "${CLUSTER_NAME:-kollect-e2e}"

_log() { echo "[cert-manager-install] $*"; }

if kubectl get deployment cert-manager -n cert-manager >/dev/null 2>&1; then
  _log "cert-manager already installed; waiting for Available..."
else
  _log "Installing cert-manager ${CERT_MANAGER_VERSION}..."
  kubectl apply -f \
    "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
fi

kubectl wait --for=condition=Available deployment/cert-manager -n cert-manager --timeout=300s
kubectl wait --for=condition=Available deployment/cert-manager-webhook -n cert-manager --timeout=300s
kubectl wait --for=condition=Available deployment/cert-manager-cainjector -n cert-manager --timeout=300s

_log "cert-manager controller ready."
