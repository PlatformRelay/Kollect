#!/usr/bin/env bash
# Apply shared e2e sample CRs and seed nginx; wait for target + inventory reconciliation.
# Used by smoke.sh and matrix-isolated jobs (e.g. git-export) that no longer share one cluster.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-180s}"

_kind_require kubectl
kind_use_context "$CLUSTER_NAME"

_log() { echo "[e2e-bootstrap] $*"; }

_log "Ensuring multitenant sample namespace team-a..."
kubectl create namespace team-a --dry-run=client -o yaml | kubectl apply -f -

_log "Applying e2e sample CRs..."
readonly E2E_SAMPLE_DIR="${REPO_ROOT}/config/samples"
readonly E2E_SAMPLE_FILES=(
  kollect_v1alpha1_kollectprofile.yaml
  kollect_v1alpha1_kollecttarget.yaml
  kollect_v1alpha1_kollectscope_team-a.yaml
)
for sample in "${E2E_SAMPLE_FILES[@]}"; do
  kubectl apply -f "${E2E_SAMPLE_DIR}/${sample}"
done

_log "Applying family snapshot sink sample..."
kubectl apply -f "${E2E_SAMPLE_DIR}/e2e/snapshot-sink.yaml"
if ! kubectl get kollectsnapshotsink e2e-snapshot-sink -n default >/dev/null 2>&1; then
  echo "KollectSnapshotSink e2e-snapshot-sink not found after apply" >&2
  exit 1
fi

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
if ! kubectl wait --for=condition=Ready kollecttarget/nginx-deployments \
  -n default --timeout="$WAIT_TIMEOUT"; then
  kubectl describe kollecttarget nginx-deployments -n default
  kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=80 || true
  exit 1
fi


_log "Applying KollectInventory after target is Ready..."
kubectl apply -f "${E2E_SAMPLE_DIR}/e2e/team-inventory.yaml"

_log "Skipping KollectInventory status wait (smoke.sh validates collection via inventory HTTP)."

_log "Bootstrap samples ready."
