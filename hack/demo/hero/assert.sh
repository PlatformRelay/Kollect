#!/usr/bin/env bash
# DEMO-03 assert: inventory Ready + Git sink ConnectionVerified + non-empty export.
# Intended for post-up checks (CI smoke, pre-recording). Names failing conditions and
# dumps kubectl describe/logs on failure — do not eyeball.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

_hero_require_tools
_hero_source_state

if ! kind_cluster_exists "$HERO_CLUSTER"; then
  echo "Cluster ${HERO_CLUSTER} not found — run: task demo-up (or bash hack/demo/hero/up.sh)" >&2
  exit 1
fi

kind_use_context "$HERO_CLUSTER"

_hero_assert_inventory_ready demo-inventory 30s
_hero_assert_git_connection_verified hero-git-sink 30s
_hero_assert_git_export_nonempty

_hero_log "Hero assert OK — Ready, ConnectionVerified, non-empty Git export."
