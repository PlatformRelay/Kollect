#!/usr/bin/env bash
# Shared helpers for kollect kind dev/e2e clusters. Source this file; do not execute directly.
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${KIND_DIR}/../.." && pwd)"

# Pin kind CLI version (matches .github/workflows/e2e-nightly.yaml).
readonly KIND_VERSION="${KIND_VERSION:-0.27.0}"

# Kubernetes node image version — derived from go.mod k8s.io/api (kept in sync with envtest/CI).
k8s_version_from_gomod() {
  local api_ver patch
  api_ver="$(grep -E '^\s*k8s\.io/api ' "${REPO_ROOT}/go.mod" | awk '{print $2}' | sed 's/^v//')"
  patch="${api_ver#0.}"
  printf '1.%s' "$patch"
}

readonly K8S_VERSION="${K8S_VERSION:-$(k8s_version_from_gomod)}"
readonly KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v${K8S_VERSION}}"

readonly KOLLECT_NAMESPACE="${KOLLECT_NAMESPACE:-kollect-system}"
readonly KOLLECT_RELEASE="${KOLLECT_RELEASE:-kollect}"
readonly KOLLECT_IMAGE="${KOLLECT_IMAGE:-kollect-controller-manager:dev}"
readonly KOLLECT_HELM_CHART="${KOLLECT_HELM_CHART:-${REPO_ROOT}/charts/kollect}"

# Dev ingress NodePorts (must match hack/kind/dev/cluster.yaml extraPortMappings).
readonly KIND_HOST_HTTP_PORT="${KIND_HOST_HTTP_PORT:-30080}"
readonly KIND_HOST_HTTPS_PORT="${KIND_HOST_HTTPS_PORT:-30443}"

_kind_log() { echo "[kind] $*"; }

_kind_require() {
  local cmd="$1" hint="${2:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "${cmd} is required.${hint:+ $hint}" >&2
    return 1
  fi
}

_kind_require_tools() {
  _kind_require kind "https://kind.sigs.k8s.io/"
  _kind_require kubectl "https://kubernetes.io/docs/tasks/tools/"
  _kind_require helm "https://helm.sh/"
}

_kind_detect_provider() {
  if [[ -n "${KIND_EXPERIMENTAL_PROVIDER:-}" ]]; then
    return 0
  fi
  if command -v docker >/dev/null 2>&1; then
    return 0
  fi
  if command -v nerdctl >/dev/null 2>&1; then
    export KIND_EXPERIMENTAL_PROVIDER="nerdctl"
  elif command -v podman >/dev/null 2>&1; then
    export KIND_EXPERIMENTAL_PROVIDER="podman"
  else
    echo "A container runtime is required (docker, nerdctl, or podman)." >&2
    return 1
  fi
}

kind_cluster_exists() {
  local name="$1"
  kind get clusters 2>/dev/null | grep -qx "$name"
}

kind_use_context() {
  local name="$1"
  kubectl config use-context "kind-${name}" >/dev/null
}

kind_create_cluster() {
  local name="$1" config="$2"
  if kind_cluster_exists "$name"; then
    _kind_log "Cluster ${name} already exists; refreshing kubeconfig context."
    kind export kubeconfig --name "$name"
    kind_use_context "$name"
    return 0
  fi

  _kind_log "Creating kind cluster ${name} (k8s ${K8S_VERSION}, image ${KIND_NODE_IMAGE})..."
  kind create cluster \
    --name "$name" \
    --config "$config" \
    --image "$KIND_NODE_IMAGE" \
    --wait 120s
  kind_use_context "$name"
}

kind_delete_cluster() {
  local name="$1"
  if kind_cluster_exists "$name"; then
    _kind_log "Deleting kind cluster ${name}..."
    kind delete cluster --name "$name"
  else
    _kind_log "Cluster ${name} does not exist; nothing to delete."
  fi
}

kollect_build_image() {
  _kind_log "Building controller image ${KOLLECT_IMAGE}..."
  if command -v task >/dev/null 2>&1; then
    (cd "$REPO_ROOT" && task docker:build)
  else
    (cd "$REPO_ROOT" && docker build -t "$KOLLECT_IMAGE" .)
  fi
}

kollect_load_image() {
  local cluster="$1"
  _kind_log "Loading ${KOLLECT_IMAGE} into kind cluster ${cluster}..."
  kind load docker-image "$KOLLECT_IMAGE" --name "$cluster"
}

kollect_helm_install() {
  local values_file="$1"
  shift || true

  _kind_log "Installing kollect via Helm (values: ${values_file})..."
  helm upgrade --install "$KOLLECT_RELEASE" "$KOLLECT_HELM_CHART" \
    --namespace "$KOLLECT_NAMESPACE" \
    --create-namespace \
    -f "$values_file" \
    --set "image.repository=${KOLLECT_IMAGE%%:*}" \
    --set "image.tag=${KOLLECT_IMAGE##*:}" \
    --set image.pullPolicy=IfNotPresent \
    "$@" \
    --wait --timeout 120s
}

kollect_wait_manager_ready() {
  local timeout="${1:-120s}"
  _kind_log "Waiting for manager pod Ready (timeout ${timeout})..."
  kubectl wait --for=condition=Ready pod \
    -l app.kubernetes.io/name=kollect \
    -n "$KOLLECT_NAMESPACE" \
    --timeout="$timeout"
}

kollect_install_base() {
  local cluster="$1" values_file="$2"
  shift 2 || true

  kollect_build_image
  kollect_load_image "$cluster"
  kollect_helm_install "$values_file" "$@"
  kollect_wait_manager_ready
}

# --- CLI entrypoints (when executed, not sourced) ---

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
    load-image)
      _kind_require_tools
      _kind_detect_provider
      kollect_build_image
      kollect_load_image "${2:?cluster name required}"
      ;;
    delete)
      _kind_require kind
      kind_delete_cluster "${2:?cluster name required}"
      ;;
    k8s-version)
      echo "$K8S_VERSION"
      ;;
    *)
      echo "Usage: $0 {load-image CLUSTER|delete CLUSTER|k8s-version}" >&2
      exit 1
      ;;
  esac
fi
