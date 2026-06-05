#!/usr/bin/env bash
# Shared helpers for kollect kind dev/e2e clusters. Source this file; do not execute directly.
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${KIND_DIR}/../.." && pwd)"

# Pin kind CLI version (matches .github/workflows/e2e-nightly.yaml).
readonly KIND_VERSION="${KIND_VERSION:-0.32.0}"

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

# Bounded waits for kind/Helm install (CI runners can exceed legacy 120s under load).
readonly KIND_CLUSTER_WAIT="${KIND_CLUSTER_WAIT:-300s}"
readonly KOLLECT_HELM_TIMEOUT="${KOLLECT_HELM_TIMEOUT:-300s}"
readonly KOLLECT_MANAGER_WAIT="${KOLLECT_MANAGER_WAIT:-300s}"

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
    _kind_log "Cluster ${name} already exists; verifying health."
    if kind export kubeconfig --name "$name" 2>/dev/null \
      && kind_use_context "$name" \
      && kubectl wait --for=condition=Ready node --all --timeout=60s >/dev/null 2>&1; then
      _kind_log "Reusing healthy cluster ${name}."
      return 0
    fi
    _kind_log "Cluster ${name} is missing or unhealthy; recreating."
    kind delete cluster --name "$name"
  fi

  _kind_log "Creating kind cluster ${name} (k8s ${K8S_VERSION}, image ${KIND_NODE_IMAGE})..."
  if ! kind create cluster \
    --name "$name" \
    --config "$config" \
    --image "$KIND_NODE_IMAGE" \
    --wait "$KIND_CLUSTER_WAIT"; then
    _kind_log "kind create failed; deleting orphaned cluster ${name} and retrying once..."
    kind delete cluster --name "$name" 2>/dev/null || true
    kind create cluster \
      --name "$name" \
      --config "$config" \
      --image "$KIND_NODE_IMAGE" \
      --wait "$KIND_CLUSTER_WAIT"
  fi
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

kollect_diagnose_install_failure() {
  _kind_log "Install diagnostics (namespace ${KOLLECT_NAMESPACE})..."
  kubectl get pods,deployments,events -n "$KOLLECT_NAMESPACE" --sort-by=.metadata.creationTimestamp 2>/dev/null || true
  local deploy
  deploy="$(kubectl get deployment -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -n "$deploy" ]]; then
    kubectl describe deployment "$deploy" -n "$KOLLECT_NAMESPACE" 2>/dev/null || true
  fi
  kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=80 2>/dev/null || true
}

kollect_helm_install() {
  local values_file="$1"
  shift || true

  _kind_log "Installing kollect via Helm (values: ${values_file}, timeout ${KOLLECT_HELM_TIMEOUT})..."
  if ! helm upgrade --install "$KOLLECT_RELEASE" "$KOLLECT_HELM_CHART" \
    --namespace "$KOLLECT_NAMESPACE" \
    --create-namespace \
    -f "$values_file" \
    --set "image.repository=${KOLLECT_IMAGE%%:*}" \
    --set "image.tag=${KOLLECT_IMAGE##*:}" \
    --set image.pullPolicy=IfNotPresent \
    "$@" \
    --wait --timeout "$KOLLECT_HELM_TIMEOUT"; then
    kollect_diagnose_install_failure
    return 1
  fi
}

kollect_wait_crds_established() {
  local timeout="${1:-$KOLLECT_MANAGER_WAIT}"
  _kind_log "Waiting for kollect CRDs Established (timeout ${timeout})..."
  local crd
  for crd in \
    kollectprofiles.kollect.dev \
    kollecttargets.kollect.dev \
    kollectinventories.kollect.dev \
    kollectsinks.kollect.dev \
    kollectscopes.kollect.dev \
    kollectclusterprofiles.kollect.dev \
    kollectclustertargets.kollect.dev \
    kollectclusterinventories.kollect.dev; do
    kubectl wait --for=condition=Established "crd/${crd}" --timeout="$timeout"
  done
}

kollect_wait_kube_system_ready() {
  local timeout="${1:-$KIND_CLUSTER_WAIT}"
  _kind_log "Waiting for kube-system pods Ready (timeout ${timeout})..."
  kubectl wait --for=condition=Ready pods --all -n kube-system --timeout="$timeout"
}

kollect_wait_controllers_started() {
  local timeout="${1:-180s}"
  _kind_log "Waiting for manager controllers to start (timeout ${timeout})..."
  local deadline=$((SECONDS + ${timeout%s}))
  while (( SECONDS < deadline )); do
    if kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=400 2>/dev/null \
      | grep -q 'Starting Controller.*kollecttarget'; then
      _kind_log "Manager controllers started."
      return 0
    fi
    sleep 5
  done
  kollect_diagnose_install_failure
  return 1
}

kollect_wait_manager_ready() {
  local timeout="${1:-$KOLLECT_MANAGER_WAIT}"
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
  kollect_wait_kube_system_ready
  kollect_helm_install "$values_file" "$@"
  kollect_wait_crds_established
  kollect_wait_manager_ready
  kollect_wait_controllers_started
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
