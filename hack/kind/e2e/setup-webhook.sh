#!/usr/bin/env bash
# Webhook-enabled kind e2e bootstrap: cert-manager controller + Helm with webhooks on.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly CLUSTER_CONFIG="${CLUSTER_CONFIG:-${SCRIPT_DIR}/cluster.yaml}"
readonly E2E_VALUES="${E2E_VALUES:-${REPO_ROOT}/charts/kollect/ci/e2e-webhook-values.yaml}"

_kind_require_tools
_kind_detect_provider

bash "${SCRIPT_DIR}/preflight.sh"

kind_create_cluster "$CLUSTER_NAME" "$CLUSTER_CONFIG"
bash "${REPO_ROOT}/hack/e2e/install-cert-manager-controller.sh"
kollect_install_base "$CLUSTER_NAME" "$E2E_VALUES"

_kind_log "Webhook e2e cluster ${CLUSTER_NAME} ready."
