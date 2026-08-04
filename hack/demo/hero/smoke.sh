#!/usr/bin/env bash
# DEMO-03 smoke: Git-only hero path — up → assert → down.
# Skips cleanly when kind/docker are unavailable locally; fails hard under CI=true.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

VARIANT="${1:-git-only}"

if [[ "${VARIANT}" != "git-only" ]]; then
  echo "Usage: $0 [git-only]" >&2
  echo "DEMO-03 smoke covers the canonical git-only path only." >&2
  exit 2
fi

if ! _hero_kind_available; then
  msg="SKIP: kind (and a working container runtime) unavailable — cannot run hero demo smoke"
  if [[ "${CI:-}" == "true" || "${DEMO_SMOKE_REQUIRE_KIND:-}" == "1" ]]; then
    echo "${msg} (required in CI)" >&2
    exit 1
  fi
  echo "${msg}"
  exit 0
fi

_hero_require_tools
_hero_detect_provider

cleanup() {
  # Always tear down so CI/local never leak a kollect-hero cluster.
  bash "${SCRIPT_DIR}/down.sh" || true
}
trap cleanup EXIT

_hero_log "Hero smoke: bootstrapping git-only demo..."
bash "${SCRIPT_DIR}/up.sh" git-only

_hero_log "Hero smoke: asserting Ready / ConnectionVerified / non-empty export..."
bash "${SCRIPT_DIR}/assert.sh"

_hero_log "Hero smoke: OK"
# trap runs down.sh on exit
