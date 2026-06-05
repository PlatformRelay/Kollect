#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# One-shot wide-scope kind demo bootstrap: cluster, operator, Kollect CRs, workloads.
# Usage:
#   export GITHUB_TOKEN="$(gh auth token)"
#   bash hack/demo/kind-wide-scope/bootstrap.sh
#   bash hack/demo/kind-wide-scope/bootstrap.sh --churn   # also run churn in background
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-dev}"
readonly KUBECTL="${KUBECTL:-kubectl}"
RUN_CHURN=0

for arg in "$@"; do
  case "$arg" in
    --churn) RUN_CHURN=1 ;;
    -h|--help)
      echo "Usage: $0 [--churn]"
      exit 0
      ;;
    *) echo "Unknown arg: $arg" >&2; exit 1 ;;
  esac
done

_log() { echo "[demo-bootstrap] $*"; }

_require() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

_log "Checking prerequisites..."
for c in kind kubectl helm task docker gh; do
  _require "$c"
done

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  _log "GITHUB_TOKEN unset — attempting gh auth token"
  GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)"
  export GITHUB_TOKEN
fi
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  echo "Set GITHUB_TOKEN (repo scope) before bootstrap — git export will not push." >&2
fi

_log "Starting kind cluster (KOLLECT_DEV_MINIMAL=1 — operator only)..."
cd "${REPO_ROOT}"
KOLLECT_DEV_MINIMAL=1 task kind-dev-up

_log "Selecting kube context kind-${CLUSTER_NAME}"
"${KUBECTL}" config use-context "kind-${CLUSTER_NAME}"

_log "Applying demo namespaces..."
"${KUBECTL}" apply -f "${SCRIPT_DIR}/namespace.yaml"

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  _log "Creating git-push-credentials secret (default namespace)..."
  "${KUBECTL}" create secret generic git-push-credentials -n default \
    --from-literal=token="${GITHUB_TOKEN}" \
    --dry-run=client -o yaml | "${KUBECTL}" apply -f -
else
  _log "Skipping secret — apply manually from kollect/secret.example.yaml"
fi

_log "Applying Kollect CR stack (order: scope → profiles → targets → sink → inventory)..."
"${KUBECTL}" apply -f "${SCRIPT_DIR}/kollect/scope.yaml"
"${KUBECTL}" apply -f "${SCRIPT_DIR}/kollect/profiles.yaml"
"${KUBECTL}" apply -f "${SCRIPT_DIR}/kollect/targets.yaml"
"${KUBECTL}" apply -f "${SCRIPT_DIR}/kollect/sink.yaml"
"${KUBECTL}" apply -f "${SCRIPT_DIR}/kollect/inventory.yaml"

_log "Applying demo workloads (26 resources across 5 GVK types)..."
"${KUBECTL}" apply -f "${SCRIPT_DIR}/workloads/"

_log "Waiting for sink connection test..."
if ! "${KUBECTL}" wait --for=condition=ConnectionVerified \
  kollectsink/git-inventory-demo -n default --timeout=120s 2>/dev/null; then
  _log "ConnectionVerified not set — check secretRef and egress (see playbook troubleshooting)"
  "${KUBECTL}" describe kollectsink git-inventory-demo -n default || true
fi

_log "Waiting for inventory Ready (up to 180s)..."
if ! "${KUBECTL}" wait --for=condition=Ready \
  kollectinventory/team-inventory -n default --timeout=180s 2>/dev/null; then
  _log "Inventory not Ready yet — describe for conditions"
  "${KUBECTL}" describe kollectinventory team-inventory -n default || true
fi

_log "Inventory status:"
"${KUBECTL}" get kollectinventory team-inventory -n default \
  -o custom-columns=NAME:.metadata.name,ITEMS:.status.itemCount,EXPORT:.status.lastExportTime

if [[ "${RUN_CHURN}" -eq 1 ]]; then
  _log "Starting churn.sh in background (PID logged)..."
  nohup bash "${SCRIPT_DIR}/churn.sh" >"${SCRIPT_DIR}/churn.log" 2>&1 &
  echo $! >"${SCRIPT_DIR}/churn.pid"
  _log "churn PID=$(cat "${SCRIPT_DIR}/churn.pid") — tail -f ${SCRIPT_DIR}/churn.log"
fi

_log "Bootstrap complete."
_log "  Logs:    bash ${SCRIPT_DIR}/logs.sh"
_log "  Verify:  curl -sf http://127.0.0.1:8082/inventory (after port-forward — see playbook)"
_log "  Churn:   bash ${SCRIPT_DIR}/churn.sh"
