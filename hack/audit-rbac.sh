#!/usr/bin/env bash
# Audit rendered operator RBAC for dangerous permissions (Q16 merge gate).
# Uses Polaris (RBAC-focused danger checks) and kubeaudit (manifest error severity).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RBAC_DIR="${RBAC_DIR:-${ROOT}/config/rbac}"
MANAGER_ROLE="${MANAGER_ROLE:-${RBAC_DIR}/role.yaml}"
POLARIS_VERSION="${POLARIS_VERSION:-9.6.0}"
KUBEAUDIT_VERSION="${KUBEAUDIT_VERSION:-0.22.2}"

if [[ ! -f "${MANAGER_ROLE}" ]]; then
  echo "manager ClusterRole not found: ${MANAGER_ROLE}" >&2
  echo "Run 'task verify' or 'make manifests' if RBAC is stale." >&2
  exit 1
fi

if ! command -v polaris >/dev/null 2>&1; then
  POLARIS_BIN_DIR="$(mktemp -d)"
  POLARIS_VERSION="${POLARIS_VERSION}" bash "${ROOT}/hack/install-polaris.sh" "${POLARIS_BIN_DIR}"
  PATH="${POLARIS_BIN_DIR}:${PATH}"
  export PATH
fi

if ! command -v kubeaudit >/dev/null 2>&1; then
  KUBEAUDIT_DIR="$(mktemp -d)"
  curl -fsSL "https://github.com/Shopify/kubeaudit/releases/download/v${KUBEAUDIT_VERSION}/kubeaudit_${KUBEAUDIT_VERSION}_linux_amd64.tar.gz" \
    | tar -xz -C "${KUBEAUDIT_DIR}" kubeaudit
  chmod +x "${KUBEAUDIT_DIR}/kubeaudit"
  PATH="${KUBEAUDIT_DIR}:${PATH}"
  export PATH
fi

echo "==> Polaris RBAC audit (${RBAC_DIR})"
polaris audit \
  --audit-path "${RBAC_DIR}" \
  --format pretty \
  --set-exit-code-on-danger

echo "==> kubeaudit manager ClusterRole (${MANAGER_ROLE})"
kubeaudit all \
  -f "${MANAGER_ROLE}" \
  --minseverity error \
  --exitcode 1 \
  --format pretty

echo "RBAC audit passed (no critical/danger findings)."
