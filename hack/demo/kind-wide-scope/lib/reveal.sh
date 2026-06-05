#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Port-forward Read API + kollect-ui and print reveal URLs for the venue finale.

DEMO_REVEAL_PF_PIDS=()

demo_reveal_cleanup() {
  local pid
  for pid in "${DEMO_REVEAL_PF_PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  DEMO_REVEAL_PF_PIDS=()
}

demo_reveal_start() {
  local script_dir="$1"
  local kubectl="${KUBECTL:-kubectl}"
  local ns="${KOLLECT_NAMESPACE:-kollect-system}"
  local read_port="${DEMO_READ_PORT:-8082}"
  local ui_port="${DEMO_UI_PORT:-8080}"
  local read_host="127.0.0.1"
  local ui_host="127.0.0.1"

  # shellcheck source=ui.sh
  source "${script_dir}/ui.sh"

  demo_reveal_cleanup
  trap demo_reveal_cleanup EXIT INT TERM

  if ! "${kubectl}" get svc kollect-controller-manager -n "${ns}" >/dev/null 2>&1; then
    demo_info "Read API service not found — is the operator installed?"
    return 1
  fi

  demo_spin "Port-forward Read API :${read_port}..." \
    "${kubectl}" port-forward -n "${ns}" "svc/kollect-controller-manager" "${read_port}:8082" &
  DEMO_REVEAL_PF_PIDS+=($!)
  sleep 2

  local ui_svc=""
  if "${kubectl}" get svc -n "${ns}" -l app.kubernetes.io/name=kollect-ui \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null | grep -q .; then
    ui_svc="$("${kubectl}" get svc -n "${ns}" -l app.kubernetes.io/name=kollect-ui \
      -o jsonpath='{.items[0].metadata.name}')"
    demo_spin "Port-forward kollect-ui :${ui_port}..." \
      "${kubectl}" port-forward -n "${ns}" "svc/${ui_svc}" "${ui_port}:8080" &
    DEMO_REVEAL_PF_PIDS+=($!)
    sleep 2
  fi

  local item_count="?"
  if command -v curl >/dev/null 2>&1; then
    item_count="$(curl -sf "http://${read_host}:${read_port}/inventory" 2>/dev/null \
      | python3 -c "import json,sys; print(json.load(sys.stdin).get('itemCount','?'))" 2>/dev/null \
      || echo "?")"
  fi

  demo_step 9 "Reveal"
  demo_outcome "Catalog itemCount: ${item_count}"

  if [[ -n "${ui_svc}" ]]; then
    demo_info "**kollect-ui:** http://${ui_host}:${ui_port}/ — filter by GVK (e.g. VulnerabilityReport) for CVE rows."
    if [[ "${DEMO_OPEN_BROWSER:-}" == "1" ]] && command -v xdg-open >/dev/null 2>&1; then
      xdg-open "http://${ui_host}:${ui_port}/" >/dev/null 2>&1 || true
    fi
  else
    demo_info "**Read API:** http://${read_host}:${read_port}/inventory — UI subchart not detected; use \`cd ui && npm run dev\` fallback."
  fi

  demo_info "**Read API curl:** \`curl -sf http://${read_host}:${read_port}/inventory | jq '{itemCount, kinds: [.items[].gvk.kind] | unique}'\`"

  if demo_persona_git_enabled 2>/dev/null && command -v gh >/dev/null 2>&1; then
    local sha msg
    sha="$(gh api repos/konih/kollect-inventory-demo/commits --jq '.[0].sha[0:7]' 2>/dev/null || true)"
    msg="$(gh api repos/konih/kollect-inventory-demo/commits --jq '.[0].commit.message' 2>/dev/null | head -1 || true)"
    if [[ -n "${sha}" ]]; then
      demo_outcome "Latest Git export: https://github.com/konih/kollect-inventory-demo/commit/${sha} — ${msg}"
    fi
  fi

  demo_info "Press Ctrl+C to stop port-forwards."
  wait
}
