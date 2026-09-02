#!/usr/bin/env bash
# Shared helpers for the hero demo harness (Forgejo in kind + golden Git sample).
set -euo pipefail

HERO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${HERO_DIR}/../../.." && pwd)"
# shellcheck source=../../kind/common.sh
source "${REPO_ROOT}/hack/kind/common.sh"

readonly HERO_CLUSTER="${HERO_CLUSTER:-kollect-hero}"
readonly HERO_CLUSTER_CONFIG="${HERO_CLUSTER_CONFIG:-${HERO_DIR}/cluster.yaml}"
readonly HERO_DEV_VALUES="${HERO_DEV_VALUES:-${REPO_ROOT}/charts/kollect/ci/hero-values.yaml}"
readonly HERO_STATE_FILE="${HERO_STATE_FILE:-/tmp/kollect-hero-state.env}"
readonly HERO_PF_PID_FILE="${HERO_PF_PID_FILE:-/tmp/kollect-hero-forgejo-pf.pid}"
readonly HERO_FORGEJO_NS="${HERO_FORGEJO_NS:-forgejo}"
readonly HERO_FORGEJO_USER="${HERO_FORGEJO_USER:-kollect}"
readonly HERO_FORGEJO_PASS="${HERO_FORGEJO_PASS:-kollect-demo}"
readonly HERO_FORGEJO_REPO="${HERO_FORGEJO_REPO:-inventory-demo}"
readonly HERO_FORGEJO_PF_PORT="${HERO_FORGEJO_PF_PORT:-13000}"
readonly HERO_INVENTORY_CLONE_DIR="${HERO_INVENTORY_CLONE_DIR:-/tmp/kollect-hero-inventory}"
readonly HERO_GIT_SECRET="${HERO_GIT_SECRET:-hero-git-credentials}"

_hero_log() { echo "[hero] $*"; }

# Thin aliases so hero scripts share kind helpers without leaking _kind_* names.
_hero_require() { _kind_require "$@"; }
_hero_detect_provider() { _kind_detect_provider "$@"; }

_hero_require_tools() {
  _kind_require_tools
  _kind_require docker "https://docs.docker.com/get-docker/"
  _kind_require git "https://git-scm.com/"
  _kind_require curl "https://curl.se/"
  _kind_require task "https://taskfile.dev/"
}

# True when kind + a container runtime are usable for a live hero smoke.
_hero_kind_available() {
  command -v kind >/dev/null 2>&1 || return 1
  command -v docker >/dev/null 2>&1 || command -v nerdctl >/dev/null 2>&1 || command -v podman >/dev/null 2>&1 || return 1
  if command -v docker >/dev/null 2>&1; then
    docker info >/dev/null 2>&1 || return 1
  fi
  return 0
}

_hero_dump_failure_diagnostics() {
  local reason="${1:-hero assert failed}"
  _hero_log "FAILURE: ${reason}"
  kubectl get kollectinventory,kollectsnapshotsink -n default -o wide 2>/dev/null || true
  kubectl describe kollectinventory/demo-inventory -n default 2>/dev/null || true
  kubectl describe kollectsnapshotsink/hero-git-sink -n default 2>/dev/null || true
  kubectl logs -n kollect-system -l app.kubernetes.io/name=kollect --tail=80 2>/dev/null || true
}

_hero_assert_inventory_ready() {
  local name="${1:-demo-inventory}"
  local timeout="${2:-30s}"
  _hero_log "Asserting KollectInventory/${name} Ready..."
  if ! kubectl wait --for=condition=Ready "kollectinventory/${name}" -n default --timeout="${timeout}"; then
    _hero_dump_failure_diagnostics "KollectInventory/${name} condition=Ready not True within ${timeout}"
    return 1
  fi
}

_hero_assert_git_connection_verified() {
  local name="${1:-hero-git-sink}"
  local timeout="${2:-30s}"
  _hero_log "Asserting KollectSnapshotSink/${name} ConnectionVerified..."
  if ! kubectl wait --for=condition=ConnectionVerified "kollectsnapshotsink/${name}" \
    -n default --timeout="${timeout}"; then
    _hero_dump_failure_diagnostics "KollectSnapshotSink/${name} condition=ConnectionVerified not True within ${timeout}"
    return 1
  fi
}

# Non-empty Git export: ≥1 committed inventory file (yaml/yml/json) outside .git.
#
# GATE-SIGPIPE-01: counts with `grep -c`, never `grep -q`. `find … | grep -q .` under
# `set -o pipefail` inverts once find has more than one pipe buffer (64 KiB on Linux) of
# paths to write: grep exits at the first match, find takes SIGPIPE and exits 141, and
# pipefail makes the pipeline report 141 — so a real, LARGE export is reported as empty
# and _hero_assert_git_export_nonempty fails the demo for a reason that does not exist.
# A small clone behaves correctly, which is why only a large fixture can catch this
# (see hack/test/hyg_sigpipe_pipefail_test.sh). `grep -c` reads to EOF, so no SIGPIPE.
_hero_export_has_inventory_files() {
  local dir="${1:-${HERO_INVENTORY_CLONE_DIR}}" count
  [[ -d "${dir}" ]] || return 1
  count="$(
    find "${dir}" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) \
      ! -path '*/.git/*' | grep -c . || true
  )"
  [[ "${count}" =~ ^[0-9]+$ ]] || return 1
  [[ "${count}" -gt 0 ]]
}

_hero_assert_git_export_nonempty() {
  _hero_source_state
  _hero_start_port_forward
  if [[ ! -d "${HERO_INVENTORY_CLONE_DIR}/.git" ]]; then
    _hero_clone_inventory_repo || true
  else
    git -C "${HERO_INVENTORY_CLONE_DIR}" pull -q 2>/dev/null || true
  fi
  if ! _hero_export_has_inventory_files "${HERO_INVENTORY_CLONE_DIR}"; then
    _hero_dump_failure_diagnostics "Git export empty — no inventory yaml/yml/json under ${HERO_INVENTORY_CLONE_DIR}"
    echo "No exported inventory files in ${HERO_INVENTORY_CLONE_DIR}" >&2
    return 1
  fi
  _hero_log "Git export non-empty under ${HERO_INVENTORY_CLONE_DIR}."
}

_hero_write_state() {
  cat >"$HERO_STATE_FILE" <<EOF
HERO_CLUSTER=${HERO_CLUSTER}
HERO_FORGEJO_NS=${HERO_FORGEJO_NS}
HERO_FORGEJO_USER=${HERO_FORGEJO_USER}
HERO_FORGEJO_PASS=${HERO_FORGEJO_PASS}
HERO_FORGEJO_REPO=${HERO_FORGEJO_REPO}
HERO_FORGEJO_PF_PORT=${HERO_FORGEJO_PF_PORT}
HERO_INVENTORY_CLONE_DIR=${HERO_INVENTORY_CLONE_DIR}
HERO_GIT_SECRET=${HERO_GIT_SECRET}
FORGEJO_TOKEN=${FORGEJO_TOKEN:-}
FORGEJO_INTERNAL_URL=http://forgejo.${HERO_FORGEJO_NS}.svc.cluster.local:3000
FORGEJO_HOST_URL=http://127.0.0.1:${HERO_FORGEJO_PF_PORT}
GIT_CLONE_URL=http://${HERO_FORGEJO_USER}:\${FORGEJO_TOKEN}@127.0.0.1:${HERO_FORGEJO_PF_PORT}/${HERO_FORGEJO_USER}/${HERO_FORGEJO_REPO}.git
EOF
}

_hero_source_state() {
  # State file repeats HERO_* keys that lib.sh declares readonly — only load token.
  [[ -f "$HERO_STATE_FILE" ]] || return 0
  FORGEJO_TOKEN="$(grep -E "^FORGEJO_TOKEN=" "$HERO_STATE_FILE" | head -1 | cut -d= -f2-)"
  export FORGEJO_TOKEN
}

_hero_start_port_forward() {
  if [[ -f "$HERO_PF_PID_FILE" ]] && kill -0 "$(cat "$HERO_PF_PID_FILE")" 2>/dev/null; then
    _hero_log "Forgejo port-forward already running (pid $(cat "$HERO_PF_PID_FILE"))."
    return 0
  fi

  _hero_log "Starting Forgejo port-forward localhost:${HERO_FORGEJO_PF_PORT}..."
  kubectl port-forward -n "$HERO_FORGEJO_NS" "svc/forgejo" "${HERO_FORGEJO_PF_PORT}:3000" \
    >/tmp/kollect-hero-forgejo-pf.log 2>&1 &
  echo $! >"$HERO_PF_PID_FILE"
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if curl -fsS "http://127.0.0.1:${HERO_FORGEJO_PF_PORT}/api/v1/version" >/dev/null 2>&1; then
      _hero_log "Forgejo reachable on localhost:${HERO_FORGEJO_PF_PORT}."
      return 0
    fi
    sleep 1
  done
  _hero_log "Forgejo port-forward failed — see /tmp/kollect-hero-forgejo-pf.log"
  return 1
}

_hero_stop_port_forward() {
  if [[ -f "$HERO_PF_PID_FILE" ]]; then
    kill "$(cat "$HERO_PF_PID_FILE")" 2>/dev/null || true
    rm -f "$HERO_PF_PID_FILE"
  fi
}

_hero_clone_inventory_repo() {
  _hero_source_state
  local clone_url="http://${HERO_FORGEJO_USER}:${FORGEJO_TOKEN}@127.0.0.1:${HERO_FORGEJO_PF_PORT}/${HERO_FORGEJO_USER}/${HERO_FORGEJO_REPO}.git"
  rm -rf "$HERO_INVENTORY_CLONE_DIR"
  git clone --branch main --single-branch "$clone_url" "$HERO_INVENTORY_CLONE_DIR"
}

_hero_wait_inventory_ready() {
  local name="${1:-demo-inventory}"
  _hero_log "Waiting for KollectInventory/${name} Ready..."
  kubectl wait --for=condition=Ready "kollectinventory/${name}" -n default --timeout=180s
}

_hero_wait_git_export() {
  _hero_source_state
  _hero_start_port_forward
  local deadline=$((SECONDS + 240))
  _hero_log "Waiting for first Git export in ${HERO_FORGEJO_REPO}..."
  while (( SECONDS < deadline )); do
    if git -C "$HERO_INVENTORY_CLONE_DIR" pull -q 2>/dev/null; then
      if git -C "$HERO_INVENTORY_CLONE_DIR" rev-parse HEAD >/dev/null 2>&1 \
        && _hero_export_has_inventory_files "$HERO_INVENTORY_CLONE_DIR"; then
        _hero_log "First export detected in ${HERO_INVENTORY_CLONE_DIR}."
        return 0
      fi
    else
      _hero_clone_inventory_repo 2>/dev/null || true
    fi
    sleep 5
  done
  _hero_dump_failure_diagnostics "Timed out waiting for Git export in ${HERO_FORGEJO_REPO}"
  return 1
}
