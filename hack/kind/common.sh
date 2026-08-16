#!/usr/bin/env bash
# Shared helpers for kollect kind dev/e2e clusters. Source this file; do not execute directly.
set -euo pipefail

KIND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${KIND_DIR}/../.." && pwd)"

# Substrate allowlist + image-delivery policy (LAB-DEKIND). Sourced here so the install path
# has ONE auditable place deciding "which cluster" and "how images get there".
# shellcheck source=../lab/lib/substrate.sh
source "${KIND_DIR}/../lab/lib/substrate.sh"

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

# kindest/node tags often lag go.mod patch bumps; walk down patch versions until one exists.
kind_node_image_resolve() {
  local version="$1"
  if ! command -v docker >/dev/null 2>&1; then
    printf 'kindest/node:v%s' "$version"
    return 0
  fi

  local major="${version%%.*}"
  local rest="${version#*.}"
  local minor="${rest%%.*}"
  local patch="${rest#*.}"
  local p tag img

  p="$patch"
  while (( p >= 0 )); do
    tag="v${major}.${minor}.${p}"
    img="kindest/node:${tag}"
    if docker manifest inspect "$img" >/dev/null 2>&1; then
      if [[ "$p" != "$patch" ]]; then
        _kind_log "kindest/node:v${version} is not published; using ${img} for the kind cluster."
      fi
      printf '%s' "$img"
      return 0
    fi
    p=$((p - 1))
  done

  _kind_log "No published kindest/node image found for k8s ${version}; trying kindest/node:v${version}."
  printf 'kindest/node:v%s' "$version"
}

_kind_effective_node_image() {
  if [[ -n "${KIND_NODE_IMAGE:-}" ]]; then
    printf '%s' "$KIND_NODE_IMAGE"
    return 0
  fi
  kind_node_image_resolve "$K8S_VERSION"
}

readonly KOLLECT_NAMESPACE="${KOLLECT_NAMESPACE:-kollect-system}"
readonly KOLLECT_RELEASE="${KOLLECT_RELEASE:-kollect}"
readonly KOLLECT_IMAGE="${KOLLECT_IMAGE:-kollect-controller-manager:dev}"
readonly KOLLECT_HELM_CHART="${KOLLECT_HELM_CHART:-${REPO_ROOT}/charts/kollect}"

# Bounded waits for kind/Helm install (CI runners can exceed legacy 120s under load).
readonly KIND_CLUSTER_WAIT="${KIND_CLUSTER_WAIT:-300s}"
readonly KOLLECT_HELM_TIMEOUT="${KOLLECT_HELM_TIMEOUT:-300s}"
readonly KOLLECT_MANAGER_WAIT="${KOLLECT_MANAGER_WAIT:-300s}"
# Controllers-started backstop: one manager restart (crash backoff) can exceed 180s on shared runners.
readonly KOLLECT_CONTROLLERS_WAIT="${KOLLECT_CONTROLLERS_WAIT:-360s}"

# Dev ingress NodePorts (must match hack/kind/dev/cluster.yaml extraPortMappings).
readonly KIND_HOST_HTTP_PORT="${KIND_HOST_HTTP_PORT:-30080}"
readonly KIND_HOST_HTTPS_PORT="${KIND_HOST_HTTPS_PORT:-30443}"

_kind_log() { echo "[kind] $*" >&2; }

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

# Decoupling seam (LAB-DEKIND): "create/select a kind cluster" vs "assert against whatever
# cluster we are pointed at". Scenario scripts call this instead of kind_use_context so they
# can run on an existing lab cluster with KOLLECT_E2E_EXISTING_CLUSTER=1. Default (CI) path
# is unchanged: switch to kind-<cluster>. Returns 2 when the current context is off-allowlist.
kollect_e2e_select_context() {
  local cluster="$1"
  if [[ "${KOLLECT_E2E_EXISTING_CLUSTER:-0}" != "1" ]]; then
    kind_use_context "$cluster"
    return 0
  fi
  local ctx
  ctx="$(kubectl config current-context 2>/dev/null || true)"
  if ! lab_substrate_assert_context "${ctx}"; then
    return 2
  fi
  return 0
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

  local node_image
  node_image="$(_kind_effective_node_image)"
  _kind_log "Creating kind cluster ${name} (k8s ${K8S_VERSION}, image ${node_image})..."
  if ! kind create cluster \
    --name "$name" \
    --config "$config" \
    --image "$node_image" \
    --wait "$KIND_CLUSTER_WAIT"; then
    _kind_log "kind create failed; deleting orphaned cluster ${name} and retrying once..."
    kind delete cluster --name "$name" 2>/dev/null || true
    kind create cluster \
      --name "$name" \
      --config "$config" \
      --image "$node_image" \
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

kollect_manager_deployment() {
  kubectl get deployment -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true
}

kollect_diagnose_install_failure() {
  _kind_log "Install diagnostics (namespace ${KOLLECT_NAMESPACE})..."
  kubectl get pods,deployments,events -n "$KOLLECT_NAMESPACE" --sort-by=.metadata.creationTimestamp 2>/dev/null || true
  local deploy
  deploy="$(kollect_manager_deployment)"
  if [[ -n "$deploy" ]]; then
    kubectl describe deployment "$deploy" -n "$KOLLECT_NAMESPACE" 2>/dev/null || true
  fi
  kubectl describe pods -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect 2>/dev/null || true
  _kind_log "Manager logs (current container)..."
  kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=120 2>/dev/null || true
  _kind_log "Manager logs (previous container, if restarted)..."
  kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --previous --tail=120 2>/dev/null \
    || _kind_log "No previous container logs (manager did not restart)."
}

kollect_helm_install() {
  local values_file="$1"
  shift || true

  # Split repo/tag on the LAST colon only when it follows the last slash, so a registry with
  # a port (localhost:5000/kollect:v1) is not mangled. The chart renders repository:tag, so a
  # digest-pinned reference is refused here rather than silently reinterpreted.
  local image_repo image_tag
  if [[ "$KOLLECT_IMAGE" == *"@"* ]]; then
    _kind_log "KOLLECT_IMAGE '${KOLLECT_IMAGE}' is digest-pinned; the chart renders repository:tag — use a pinned v<semver> tag."
    return 1
  fi
  if [[ "${KOLLECT_IMAGE##*/}" == *:* ]]; then
    image_repo="${KOLLECT_IMAGE%:*}"
    image_tag="${KOLLECT_IMAGE##*:}"
  else
    image_repo="$KOLLECT_IMAGE"
    image_tag="latest"
  fi

  _kind_log "Installing kollect via Helm (values: ${values_file}, image ${image_repo}:${image_tag}, timeout ${KOLLECT_HELM_TIMEOUT})..."
  if ! helm upgrade --install "$KOLLECT_RELEASE" "$KOLLECT_HELM_CHART" \
    --namespace "$KOLLECT_NAMESPACE" \
    --create-namespace \
    -f "$values_file" \
    --set "image.repository=${image_repo}" \
    --set "image.tag=${image_tag}" \
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
    kollectsnapshotsinks.kollect.dev \
    kollectdatabasesinks.kollect.dev \
    kollecteventsinks.kollect.dev \
    kollectscopes.kollect.dev \
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
  local timeout="${1:-$KOLLECT_CONTROLLERS_WAIT}"
  # One shared budget for BOTH the rollout wait and the log-poll fallback, so the
  # combined worst-case wall time is bounded by ${timeout} (not 2×): capture the
  # deadline up front, then hand the log poll only whatever time the rollout left.
  local deadline=$((SECONDS + ${timeout%s}))
  local deploy
  deploy="$(kollect_manager_deployment)"
  if [[ -n "$deploy" ]]; then
    # Concrete condition first: a completed rollout absorbs pod restarts / rescheduling
    # (crash backoff after a transient webhook-cert or probe race) before we scrape logs.
    _kind_log "Waiting for deployment ${deploy} rollout (timeout ${timeout})..."
    if ! kubectl rollout status "deploy/${deploy}" -n "$KOLLECT_NAMESPACE" --timeout="$timeout"; then
      kollect_diagnose_install_failure
      return 1
    fi
  fi
  # Remaining budget for the log poll; clamp to a small floor so a rollout that
  # consumed (almost) all the budget still gets at least one scrape attempt.
  local remaining=$((deadline - SECONDS))
  (( remaining < 5 )) && remaining=5
  _kind_log "Waiting for manager controllers to start (timeout ${remaining}s)..."
  deadline=$((SECONDS + remaining))
  while (( SECONDS < deadline )); do
    if kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=400 2>/dev/null \
      | grep -Eq 'Starting Controller.*(kollecttarget|kollectinventory)'; then
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

# Resolve the substrate of the cluster the CURRENT context points at (default-deny).
# Prints the substrate kind; returns 2 when the context is not on the lab allowlist.
kollect_current_substrate() {
  local ctx
  ctx="$(kubectl config current-context 2>/dev/null || true)"
  local kind_out
  if ! kind_out="$(lab_substrate_resolve "${ctx}")"; then
    lab_substrate_err "refusing kube context '${ctx}': not on the lab substrate allowlist [$(lab_substrate_allowlist_summary)]"
    lab_substrate_err "default-deny — installs only run on a Kind cluster or an allowlisted lab cluster"
    return 2
  fi
  printf '%s' "${kind_out}"
}

# Deliver the operator image to the target cluster.
#   Kind   → build locally and side-load (`kind load docker-image`), as CI has always done.
#   other  → there is NO side-load equivalent (Talos runs containerd on bare metal, no
#            docker socket to import into), so the image MUST already exist in a registry at
#            an immutable reference. Anything else is refused loudly here rather than
#            silently running whatever the nodes cached — a stale image invalidates the run.
kollect_deliver_image() {
  local cluster="$1" substrate="$2"
  if [[ "$(lab_substrate_image_delivery "$substrate")" == "sideload" ]]; then
    kollect_build_image
    kollect_load_image "$cluster"
    return 0
  fi
  if ! lab_substrate_require_registry_image "$KOLLECT_IMAGE" "$substrate"; then
    _kind_log "Substrate '${substrate}' cannot side-load images; set KOLLECT_IMAGE to a pushed, pinned reference."
    return 1
  fi
  _kind_log "Substrate ${substrate}: using pinned registry image ${KOLLECT_IMAGE} (no side-load, no rebuild)."
  return 0
}

kollect_install_base() {
  local cluster="$1" values_file="$2"
  shift 2 || true

  local substrate
  substrate="$(kollect_current_substrate)" || return 1
  kollect_deliver_image "$cluster" "$substrate" || return 1
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
