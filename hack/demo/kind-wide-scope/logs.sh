#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Tail operator logs relevant to collection, inventory reconcile, and git export.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/ui.sh
source "${SCRIPT_DIR}/lib/ui.sh"

readonly KUBECTL="${KUBECTL:-kubectl}"
readonly NS="${KOLLECT_NAMESPACE:-kollect-system}"
readonly RELEASE="${KOLLECT_RELEASE:-kollect}"

demo_require_gum
demo_intro "Watch Kollect reconcile inventory and push to Git"

PIDS=()
cleanup() {
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
}
trap cleanup EXIT INT TERM

pod="$("${KUBECTL}" get pods -n "${NS}" -l app.kubernetes.io/name="${RELEASE}" \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"

if [[ -z "${pod}" ]]; then
  pod="$("${KUBECTL}" get pods -n "${NS}" -l control-plane=controller-manager \
    -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
fi

if [[ -z "${pod}" ]]; then
  echo "No manager pod in ${NS} — is the operator running?" >&2
  exit 1
fi

demo_info "Manager pod: **${NS}/${pod}** — Press Ctrl+C to stop followers."

"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=80 2>&1 | sed 's/^/[manager] /' &
PIDS+=($!)

"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=20 2>&1 \
  | grep --line-buffered -Ei 'inventory|export|git|kollectinventory|team-inventory|sink' \
  | sed 's/^/[export] /' &
PIDS+=($!)

"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=20 2>&1 \
  | grep --line-buffered -Ei 'target|collect|informer|profile|fleet-|vulnerability|certificate|externalsecret' \
  | sed 's/^/[collect] /' &
PIDS+=($!)

if command -v stern >/dev/null 2>&1; then
  demo_info "stern also available: \`stern -n ${NS} ${RELEASE} --tail=50\`"
fi

demo_info "Inventory HTTP: \`kubectl port-forward -n ${NS} svc/kollect-controller-manager 8082:8082\` then \`curl -sf http://127.0.0.1:8082/inventory | jq .itemCount\`"

wait
