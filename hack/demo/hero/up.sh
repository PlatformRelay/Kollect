#!/usr/bin/env bash
# Hero demo harness: kind + Forgejo + kollect + golden Git-only sample.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

VARIANT="${1:-git-only}"

_hero_require_tools
_hero_detect_provider

_hero_log "Bootstrapping hero demo (variant=${VARIANT}, cluster=${HERO_CLUSTER})..."

kind_create_cluster "$HERO_CLUSTER" "$HERO_CLUSTER_CONFIG"
kollect_install_base "$HERO_CLUSTER" "$HERO_DEV_VALUES"

_hero_log "Deploying in-cluster Forgejo..."
kubectl apply -f "${SCRIPT_DIR}/manifests/forgejo.yaml"
bash "${SCRIPT_DIR}/bootstrap-forgejo.sh"

_hero_log "Deploying demo workload (shop deployment)..."
kubectl apply -f "${SCRIPT_DIR}/manifests/demo-workloads.yaml"

case "$VARIANT" in
  git-only)
    _hero_log "Applying golden Git-only sample..."
    kubectl apply -k "${REPO_ROOT}/config/samples/demo/git-only/"
    ;;
  git-postgres)
    _hero_log "Applying Postgres backing store..."
    kubectl apply -f "${REPO_ROOT}/config/samples/dev/postgres.yaml"
    kubectl -n kollect-system rollout status deployment/postgres --timeout=120s
    kubectl create secret generic inventory-postgres-dsn -n kollect-system \
      --from-literal=dsn='postgres://kollect:example@postgres.kollect-system.svc:5432/inventory?sslmode=disable' \
      --dry-run=client -o yaml | kubectl apply -f -
    _hero_log "Applying golden Git+Postgres sample..."
    kubectl apply -k "${REPO_ROOT}/config/samples/demo/git-postgres/"
    ;;
  *)
    echo "Usage: $0 [git-only|git-postgres]" >&2
    exit 1
    ;;
esac

_hero_wait_inventory_ready demo-inventory

_hero_log "Waiting for Git snapshot sink connectivity..."
kubectl wait --for=condition=ConnectionVerified kollectsnapshotsink/hero-git-sink \
  -n default --timeout=120s

if [[ "$VARIANT" == "git-postgres" ]]; then
  kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/hero-postgres-sink \
    -n default --timeout=120s
fi

_hero_start_port_forward
_hero_clone_inventory_repo
_hero_wait_git_export

_hero_write_state
_hero_log "Hero demo ready."
_hero_log "  Inventory clone: ${HERO_INVENTORY_CLONE_DIR}"
_hero_log "  Forgejo UI:      http://127.0.0.1:${HERO_FORGEJO_PF_PORT}/"
_hero_log "  State file:      ${HERO_STATE_FILE}"
_hero_log "Next: cd ${HERO_INVENTORY_CLONE_DIR} && git log --oneline   (task demo-down when done)"
_hero_log "  Maintainers recording the demo GIF: bash hack/demo/hero/preflight.sh && task demo-hero-record-${VARIANT}"
