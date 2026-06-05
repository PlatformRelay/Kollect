#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# tenantMode Helm install: namespaced Role/RoleBinding for manager SA (no ClusterRole).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly TENANT_NS="${TENANT_MODE_NAMESPACE:-kollect-tenant-ops}"
readonly TENANT_RELEASE="${TENANT_MODE_RELEASE:-kollect-tenant}"
readonly TENANT_VALUES="${TENANT_MODE_VALUES:-${REPO_ROOT}/charts/kollect/ci/e2e-tenant-mode-values.yaml}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-120s}"

_log() { echo "[tenant-mode] $*"; }

assert_tenant_rbac() {
  local role_name="${TENANT_RELEASE}-manager"

  if ! kubectl get role "${role_name}" -n "${TENANT_NS}" >/dev/null 2>&1; then
    echo "expected Role ${role_name} in ${TENANT_NS}" >&2
    kubectl get role,rolebinding -n "${TENANT_NS}" >&2 || true
    return 1
  fi

  if kubectl get clusterrole "${role_name}" >/dev/null 2>&1; then
    echo "unexpected ClusterRole ${role_name} for tenantMode install" >&2
    kubectl get clusterrole "${role_name}" -o yaml >&2
    return 1
  fi

  if kubectl get clusterrolebinding "${role_name}" >/dev/null 2>&1; then
    echo "unexpected ClusterRoleBinding ${role_name} for tenantMode install" >&2
    return 1
  fi

  _log "Role ${role_name} present; ClusterRole/ClusterRoleBinding absent"
}

main() {
  REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"

  _kind_require_tools
  kind_use_context "${CLUSTER_NAME}"

  _log "Creating tenant operator namespace ${TENANT_NS}"
  kubectl create namespace "${TENANT_NS}" --dry-run=client -o yaml | kubectl apply -f -

  _log "Installing ${TENANT_RELEASE} with tenantMode=true (values: ${TENANT_VALUES})"
  helm upgrade --install "${TENANT_RELEASE}" "${KOLLECT_HELM_CHART}" \
    --namespace "${TENANT_NS}" \
    -f "${TENANT_VALUES}" \
    --set "image.repository=${KOLLECT_IMAGE%%:*}" \
    --set "image.tag=${KOLLECT_IMAGE##*:}" \
    --set image.pullPolicy=IfNotPresent \
    --wait --timeout 120s

  assert_tenant_rbac

  _log "Waiting for tenant manager pod Ready..."
  kubectl wait --for=condition=Ready pod \
    -l app.kubernetes.io/name=kollect \
    -n "${TENANT_NS}" \
    --timeout="${WAIT_TIMEOUT}"

  _log "tenantMode RBAC smoke OK"
}

main "$@"
