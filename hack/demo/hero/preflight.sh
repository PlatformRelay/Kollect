#!/usr/bin/env bash
# Pre-recording checks: inventory Ready, Forgejo reachable, first export landed.
# Delegates Ready / ConnectionVerified / export asserts to assert.sh (DEMO-03).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

_hero_require_tools
_hero_source_state

if ! kind_cluster_exists "$HERO_CLUSTER"; then
  echo "Cluster ${HERO_CLUSTER} not found — run: task demo-hero-up" >&2
  exit 1
fi

kind_use_context "$HERO_CLUSTER"

bash "${SCRIPT_DIR}/assert.sh"

_hero_log "Checking Forgejo API..."
_hero_start_port_forward
curl -fsS "http://127.0.0.1:${HERO_FORGEJO_PF_PORT}/api/v1/version" >/dev/null

_hero_log "Pre-flight OK — safe to record."
