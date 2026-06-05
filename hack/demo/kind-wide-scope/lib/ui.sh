#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Charm Gum helpers for guided, presentable demo scripts (bubble-style shell UX).
set -euo pipefail

DEMO_TITLE="${DEMO_TITLE:-Kollect wide-scope demo}"
DEMO_REPO_ROOT="${DEMO_REPO_ROOT:-}"

_gum_install() {
  if command -v gum >/dev/null 2>&1; then
    return 0
  fi
  echo "[demo] gum not found — installing Charm Gum for guided output..." >&2
  if command -v go >/dev/null 2>&1; then
    GOBIN="${GOBIN:-$(go env GOPATH 2>/dev/null)/bin}"
    export GOBIN
    go install github.com/charmbracelet/gum@latest
    export PATH="${GOBIN}:${PATH}"
  fi
  if ! command -v gum >/dev/null 2>&1; then
    echo "[demo] Install gum manually: https://github.com/charmbracelet/gum#installation" >&2
    return 1
  fi
}

demo_require_gum() {
  _gum_install || true
}

demo_intro() {
  local subtitle="${1:-Turn live cluster state into durable inventory}"
  if command -v gum >/dev/null 2>&1; then
    gum style \
      --border double \
      --border-foreground 212 \
      --padding "1 2" \
      --bold \
      "${DEMO_TITLE}" \
      "" \
      "${subtitle}"
    echo ""
  else
    echo "=== ${DEMO_TITLE} ==="
    echo "${subtitle}"
    echo ""
  fi
}

demo_step() {
  local n="$1"
  local title="$2"
  if command -v gum >/dev/null 2>&1; then
    gum style --foreground 212 --bold "Step ${n}" "${title}"
  else
    echo ""
    echo "[Step ${n}] ${title}"
  fi
}

demo_info() {
  if command -v gum >/dev/null 2>&1; then
    gum format <<<"$*"
  else
    echo "$*"
  fi
}

demo_confirm() {
  local prompt="${1:-Continue?}"
  if [[ "${DEMO_AUTO_YES:-}" == "1" ]]; then
    return 0
  fi
  if command -v gum >/dev/null 2>&1 && [[ -t 0 && -t 1 ]]; then
    gum confirm "${prompt}" --default=true
  else
    read -r -p "${prompt} [Y/n] " ans
    [[ -z "${ans}" || "${ans}" =~ ^[Yy] ]]
  fi
}

demo_spin() {
  local title="$1"
  shift
  if command -v gum >/dev/null 2>&1; then
    gum spin --spinner dot --title "${title}" -- "$@"
  else
    echo "[demo] ${title}"
    "$@"
  fi
}

demo_wait_for() {
  local title="$1"
  shift
  if command -v gum >/dev/null 2>&1; then
    gum spin --spinner line --title "${title}" -- "$@" || return 1
  else
    echo "[demo] ${title}"
    "$@" || return 1
  fi
}

demo_link() {
  local path="$1"
  local label="${2:-$path}"
  if [[ -n "${DEMO_REPO_ROOT}" ]]; then
    echo "  → ${label} (${DEMO_REPO_ROOT}/${path})"
  else
    echo "  → ${label} (${path})"
  fi
}

demo_outcome() {
  if command -v gum >/dev/null 2>&1; then
    gum style --border rounded --border-foreground 10 --padding "0 1" "$*"
  else
    echo "[outcome] $*"
  fi
}

demo_fail() {
  demo_outcome "$*"
  exit 1
}
