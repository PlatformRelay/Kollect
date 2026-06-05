#!/usr/bin/env bash
# Show kollect-dev cluster and addon status.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-dev}"

echo "=== kollect-dev status ==="
echo "K8s version pin: ${K8S_VERSION} (${KIND_NODE_IMAGE})"

if kind_cluster_exists "$CLUSTER_NAME"; then
  echo "Cluster: ${CLUSTER_NAME} (running)"
  kind_use_context "$CLUSTER_NAME"
  kubectl cluster-info --context "kind-${CLUSTER_NAME}" 2>/dev/null || true
  echo ""
  kubectl get nodes -o wide 2>/dev/null || true
  echo ""
  kubectl get pods -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect 2>/dev/null || true
  echo ""
  kubectl get pods -n ingress-nginx 2>/dev/null || echo "(ingress-nginx not installed)"
  echo ""
  kubectl get pods -n monitoring 2>/dev/null || echo "(monitoring addons not installed)"
else
  echo "Cluster: ${CLUSTER_NAME} (not found — run: task kind-dev-up)"
fi
