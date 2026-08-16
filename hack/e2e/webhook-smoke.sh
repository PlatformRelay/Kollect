#!/usr/bin/env bash
# Tier 1 webhook e2e: assert serving cert + validating webhook rejects invalid family sink CRs.
#
# Two modes (LAB-DEKIND / U-08) — "create a cluster" is separable from "assert against one":
#   default                        CI/Kind. Switches to the kind-${CLUSTER_NAME} context and
#                                  applies the valid sample for real. Unchanged behaviour.
#   KOLLECT_E2E_EXISTING_CLUSTER=1 Run the same assertions against the cluster the CURRENT
#                                  context points at — provided that context is on the lab
#                                  substrate allowlist (hack/lab/substrates.conf). Never
#                                  creates a cluster and never mutates: every apply is a
#                                  server-side dry run, so it is safe against a live release
#                                  that is holding evidence.
#
# Existing-cluster example (kumulus Talos lab, Helm release kollect-op1):
#   KUBECONFIG=... KOLLECT_E2E_EXISTING_CLUSTER=1 \
#     KOLLECT_RELEASE=kollect-op1 KOLLECT_NAMESPACE=kollect-op1 bash hack/e2e/webhook-smoke.sh
#
# Exit codes:
#   0  assertions passed
#   1  assertion failed
#   2  kube context refused (not on the lab substrate allowlist)
#   4  webhook stack not installed on the target cluster (precondition, not a product bug)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"
# shellcheck source=../lab/lib/substrate.sh
source "${SCRIPT_DIR}/../lab/lib/substrate.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-300s}"
readonly EXISTING_CLUSTER="${KOLLECT_E2E_EXISTING_CLUSTER:-0}"
readonly TEST_NAMESPACE="${KOLLECT_E2E_TEST_NAMESPACE:-default}"

_kind_require kubectl

_log() { echo "[webhook-smoke] $*"; }

# KOLLECT_E2E_TEST_NAMESPACE reaches a manifest that is piped to the API server. Validate it
# as a DNS-1123 label so it can never carry YAML — the manifest itself stays a quoted
# heredoc and the namespace is supplied via `kubectl -n`, so nothing is interpolated into it.
if [[ ! "${TEST_NAMESPACE}" =~ ^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$ ]]; then
  echo "invalid KOLLECT_E2E_TEST_NAMESPACE '${TEST_NAMESPACE}' (want a DNS-1123 label)" >&2
  exit 1
fi

# Mutating applies are the Kind-only half of this scenario. Against an existing lab cluster
# the same admission decision is observable with a server-side dry run, so the accept
# assertion is downgraded to --dry-run=server there (explicit --dry-run=none on Kind keeps
# the CI behaviour visible rather than implied).
APPLY_MODE="none"

if ! kollect_e2e_select_context "$CLUSTER_NAME"; then
  _log "existing-cluster mode requires an allowlisted lab context; refusing to assert"
  exit 2
fi
if [[ "${EXISTING_CLUSTER}" == "1" ]]; then
  APPLY_MODE="server"
  _log "existing-cluster mode: read-only assertions (release ${KOLLECT_RELEASE}, namespace ${KOLLECT_NAMESPACE})"
fi

_webhook_stack_missing() {
  _log "FAIL: the validating webhook stack is not installed on this cluster."
  _log "Release '${KOLLECT_RELEASE}' in namespace '${KOLLECT_NAMESPACE}' has no"
  _log "ValidatingWebhookConfiguration '${KOLLECT_RELEASE}-validating-webhook-configuration'."
  _log "Install/upgrade the release with webhooks enabled (and cert-manager present), then re-run."
  kubectl get validatingwebhookconfiguration || true
}

if [[ "${EXISTING_CLUSTER}" == "1" ]]; then
  # cert-manager is a Kind-stack assumption: an existing lab release may serve its webhook
  # cert another way. Only wait for the Certificate when cert-manager actually manages one.
  if kubectl get crd certificates.cert-manager.io >/dev/null 2>&1 \
    && kubectl get certificate "${KOLLECT_RELEASE}-serving-cert" -n "$KOLLECT_NAMESPACE" >/dev/null 2>&1; then
    _log "Waiting for webhook serving Certificate Ready..."
    kubectl wait --for=condition=Ready "certificate/${KOLLECT_RELEASE}-serving-cert" \
      -n "$KOLLECT_NAMESPACE" \
      --timeout="$WAIT_TIMEOUT"
  else
    _log "No cert-manager Certificate for this release; asserting the webhook itself instead."
  fi

  _log "Asserting ValidatingWebhookConfiguration registered..."
  if ! kubectl get validatingwebhookconfiguration "${KOLLECT_RELEASE}-validating-webhook-configuration" \
    >/dev/null 2>&1; then
    _webhook_stack_missing
    exit 4
  fi
else
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
fi

_log "Expect validating webhook to reject git snapshot sink without git block..."
set +e
reject_out="$(kubectl apply -n "${TEST_NAMESPACE}" --dry-run=server -f - 2>&1 <<'EOF'
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: webhook-reject-test
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

if [[ "${EXISTING_CLUSTER}" == "1" ]]; then
  _log "Admitting valid minimal snapshot sink through the webhook (server dry-run only)..."
else
  _log "Applying valid minimal snapshot sink via webhook..."
fi
kubectl apply "--dry-run=${APPLY_MODE}" -f "${REPO_ROOT}/config/samples/e2e/snapshot-sink.yaml"

_log "Webhook smoke checks passed."
