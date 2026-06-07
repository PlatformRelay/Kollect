#!/usr/bin/env bash
# Post-install smoke checks shared by e2e-nightly (samples, nginx seed, bounded waits, HTTP probe).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-180s}"
export CLUSTER_NAME WAIT_TIMEOUT REPO_ROOT

_kind_require kubectl
kind_use_context "$CLUSTER_NAME"

_log() { echo "[e2e-smoke] $*"; }

bash "${SCRIPT_DIR}/bootstrap-samples.sh"

_log "Asserting CRDs established..."
kubectl wait --for=condition=Established crd/kollectprofiles.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollecttargets.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectinventories.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectsnapshotsinks.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectdatabasesinks.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollecteventsinks.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectscopes.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl get kollectprofiles,kollecttargets,kollectinventories,kollectsnapshotsinks,kollectdatabasesinks -A

_log "Asserting family snapshot sink CR accepted..."
if ! kubectl get kollectsnapshotsink e2e-snapshot-sink -n default >/dev/null 2>&1; then
  echo "expected KollectSnapshotSink e2e-snapshot-sink in default namespace" >&2
  exit 1
fi

_log "Probing inventory HTTP..."
kubectl port-forward -n "$KOLLECT_NAMESPACE" svc/kollect-controller-manager 18082:8082 &
PF_PID=$!
trap 'kill "$PF_PID" 2>/dev/null || true' EXIT
for i in $(seq 1 30); do
  if curl -sf http://127.0.0.1:18082/inventory | grep -q '"itemCount"'; then
    break
  fi
  if [[ "$i" -eq 30 ]]; then
    echo "inventory HTTP probe failed within timeout" >&2
    curl -sv http://127.0.0.1:18082/inventory || true
    kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=120 || true
    exit 1
  fi
  sleep 2
done

_log "Generic CRD collection (cert-manager Certificate)..."
chmod +x "${REPO_ROOT}/hack/e2e/cert-manager.sh"
bash "${REPO_ROOT}/hack/e2e/cert-manager.sh"

_log "Smoke checks passed."
