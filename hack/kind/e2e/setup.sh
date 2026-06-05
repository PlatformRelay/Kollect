#!/usr/bin/env bash
# Minimal kollect-e2e kind cluster: create + helm install only (CI / nightly smoke).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly CLUSTER_CONFIG="${CLUSTER_CONFIG:-${SCRIPT_DIR}/cluster.yaml}"
readonly E2E_VALUES="${E2E_VALUES:-${REPO_ROOT}/charts/kollect/ci/e2e-tenant-values.yaml}"

_kind_require_tools
_kind_detect_provider

kind_create_cluster "$CLUSTER_NAME" "$CLUSTER_CONFIG"
kollect_install_base "$CLUSTER_NAME" "$E2E_VALUES"

_kind_log "E2E cluster ${CLUSTER_NAME} ready (operator installed)."
