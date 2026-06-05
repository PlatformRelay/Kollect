#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Tail operator logs relevant to collection, inventory reconcile, and git export.
# Runs kubectl logs -f in background; Ctrl+C stops all followers.
set -euo pipefail

readonly KUBECTL="${KUBECTL:-kubectl}"
readonly NS="${KOLLECT_NAMESPACE:-kollect-system}"
readonly RELEASE="${KOLLECT_RELEASE:-kollect}"

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

echo "[logs] Manager pod: ${NS}/${pod}"
echo "[logs] Press Ctrl+C to stop all followers."
echo ""

# Full manager log (primary)
"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=80 2>&1 | sed 's/^/[manager] /' &
PIDS+=($!)

# Filtered: inventory + export + git
"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=20 2>&1 \
  | grep --line-buffered -Ei 'inventory|export|git|kollectinventory|team-inventory|sink' \
  | sed 's/^/[export] /' &
PIDS+=($!)

# Filtered: target/collect engine
"${KUBECTL}" logs -n "${NS}" "${pod}" -f --tail=20 2>&1 \
  | grep --line-buffered -Ei 'target|collect|informer|profile|fleet-' \
  | sed 's/^/[collect] /' &
PIDS+=($!)

if command -v stern >/dev/null 2>&1; then
  echo "[logs] stern also available:"
  echo "  stern -n ${NS} ${RELEASE} --tail=50"
  echo "  stern -n ${NS} . --include kollectinventory --tail=30"
fi

echo ""
echo "[logs] Inventory HTTP (separate terminal):"
echo "  kubectl port-forward -n ${NS} svc/kollect-controller-manager 8082:8082"
echo "  watch -n5 'curl -sf http://127.0.0.1:8082/inventory | jq \"{itemCount, targets: [.items[].targetRef.name]}\"'"

wait
