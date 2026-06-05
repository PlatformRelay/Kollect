#!/usr/bin/env bash
# Post-install smoke checks shared by e2e-nightly (samples, nginx seed, bounded waits, HTTP probe).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-180s}"

_kind_require kubectl
kind_use_context "$CLUSTER_NAME"

_log() { echo "[e2e-smoke] $*"; }

_log "Ensuring multitenant sample namespace team-a..."
kubectl create namespace team-a --dry-run=client -o yaml | kubectl apply -f -

_log "Applying sample CRs..."
kubectl apply -k "${REPO_ROOT}/config/samples/"

_log "Seeding nginx Deployment for target collection..."
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels:
    app.kubernetes.io/name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: nginx
  template:
    metadata:
      labels:
        app.kubernetes.io/name: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
EOF

_log "Waiting for KollectTarget Ready..."
kubectl wait --for=condition=Ready kollecttarget/nginx-deployments \
  -n default --timeout="$WAIT_TIMEOUT"

_log "Waiting for KollectInventory reconciled..."
for i in $(seq 1 24); do
  gen="$(kubectl get kollectinventory team-inventory -n default -o jsonpath='{.metadata.generation}')"
  obs="$(kubectl get kollectinventory team-inventory -n default -o jsonpath='{.status.observedGeneration}')"
  if [[ -n "$obs" && "$obs" == "$gen" ]]; then
    kubectl get kollectinventory team-inventory -n default -o yaml | grep -E 'type:|reason:|message:' || true
    break
  fi
  if [[ "$i" -eq 24 ]]; then
    echo "inventory not reconciled within timeout" >&2
    kubectl describe kollectinventory team-inventory -n default
    exit 1
  fi
  sleep 5
done

_log "Asserting CRDs established..."
kubectl wait --for=condition=Established crd/kollectprofiles.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollecttargets.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectinventories.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectsinks.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollecthubs.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl wait --for=condition=Established crd/kollectscopes.kollect.dev --timeout="$WAIT_TIMEOUT"
kubectl get kollectprofiles,kollecttargets,kollectinventories,kollectsinks -A

_log "Probing inventory HTTP..."
kubectl port-forward -n "$KOLLECT_NAMESPACE" svc/kollect-controller-manager 18082:8082 &
PF_PID=$!
trap 'kill "$PF_PID" 2>/dev/null || true' EXIT
sleep 3
curl -sf http://127.0.0.1:18082/inventory | grep -q itemCount

_log "Generic CRD collection (cert-manager Certificate)..."
chmod +x "${REPO_ROOT}/hack/e2e/cert-manager.sh"
REPO_ROOT="${REPO_ROOT}" CLUSTER_NAME="${CLUSTER_NAME}" \
  bash "${REPO_ROOT}/hack/e2e/cert-manager.sh"

_log "Smoke checks passed."
