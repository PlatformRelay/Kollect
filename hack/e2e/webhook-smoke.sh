#!/usr/bin/env bash
# Tier 1 webhook e2e: assert serving cert + validating webhook rejects invalid family sink CRs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-300s}"

_kind_require kubectl
kind_use_context "$CLUSTER_NAME"

_log() { echo "[webhook-smoke] $*"; }

_log "Waiting for webhook serving Certificate Ready..."
kubectl wait --for=condition=Ready "certificate/${KOLLECT_RELEASE}-serving-cert" \
  -n "$KOLLECT_NAMESPACE" \
  --timeout="$WAIT_TIMEOUT"

_log "Asserting ValidatingWebhookConfiguration registered..."
if ! kubectl get validatingwebhookconfiguration "${KOLLECT_RELEASE}-validating-webhook-configuration" \
  >/dev/null 2>&1; then
  kubectl get validatingwebhookconfiguration
  exit 1
fi

_log "Expect validating webhook to reject git snapshot sink without git block..."
set +e
reject_out="$(kubectl apply --dry-run=server -f - 2>&1 <<'EOF'
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: webhook-reject-test
  namespace: default
spec:
  type: git
  endpoint: https://example.com/repo.git
EOF
)"
set -e
if ! echo "$reject_out" | grep -Eiq 'denied|invalid|failed|Forbidden'; then
  echo "expected webhook rejection for invalid KollectSnapshotSink; got: ${reject_out}" >&2
  exit 1
fi

_log "Applying valid minimal snapshot sink via webhook..."
kubectl apply -f "${REPO_ROOT}/config/samples/e2e/snapshot-sink.yaml"

_log "Webhook smoke checks passed."
