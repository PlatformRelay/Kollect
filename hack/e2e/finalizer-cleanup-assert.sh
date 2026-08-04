#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# COV-90-S14 — Kind e2e: Target/Inventory finalizer cleanup + Degraded Warning.
#
# RED  delete Target after collection → UnregisterTarget tears down artifact,
#      finalizer removed (object gone).
# RED  delete Inventory → cleanup finalizer runs, object gone (no stuck delete).
# EDGE sink rejects teardown once (missing DB secret) → patch clears refs →
#      retry/recovery completes (finalizer removed).
# EDGE Degraded Target shows Degraded=True + Warning Event (missing sink ref).
#
# Expects kind cluster + operator from kind-e2e-setup (kollect-e2e).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-e2e}"
readonly WAIT_TIMEOUT="${WAIT_TIMEOUT:-180s}"
readonly TARGET_NAME="nginx-deployments"
readonly INVENTORY_NAME="team-inventory"
readonly RETRY_INV="finalizer-retry-inv"
readonly RETRY_SINK="finalizer-retry-pg"
readonly MISSING_SECRET="finalizer-retry-missing-dsn"
readonly DEGRADE_SINK="finalizer-degrade-missing-sink"

_log() { echo "[finalizer-cleanup] $*"; }

_fail_diag() {
  local reason="${1:-assert failed}"
  _log "FAILURE: ${reason}"
  kubectl get kollecttarget,kollectinventory,kollectsnapshotsink,kollectdatabasesink -A 2>/dev/null || true
  kubectl describe kollecttarget "${TARGET_NAME}" -n default 2>/dev/null || true
  kubectl describe kollectinventory "${INVENTORY_NAME}" -n default 2>/dev/null || true
  kubectl logs -n "${KOLLECT_NAMESPACE}" -l app.kubernetes.io/name=kollect --tail=80 2>/dev/null || true
}

# KollectTarget does not watch Inventory; bump an annotation so reconcile
# re-reads namespace sink reachability after inventory sink-ref changes.
nudge_target_reconcile() {
  local name="${1:-${TARGET_NAME}}"
  local ns="${2:-default}"
  kubectl annotate "kollecttarget/${name}" -n "${ns}" \
    "kollect.dev/e2e-reconcile-nudge=$(date +%s%N)" --overwrite
}

wait_gone() {
  local kind="$1" name="$2" ns="${3:-default}"
  local end
  end=$((SECONDS + ${WAIT_TIMEOUT%s}))
  while (( SECONDS < end )); do
    if ! kubectl get "${kind}" "${name}" -n "${ns}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  _fail_diag "${kind}/${name} still present after delete (finalizer stuck?)"
  kubectl get "${kind}" "${name}" -n "${ns}" -o yaml >&2 || true
  return 1
}

object_has_finalizer() {
  local kind="$1" name="$2" needle="$3" ns="${4:-default}"
  kubectl get "${kind}" "${name}" -n "${ns}" -o jsonpath='{.metadata.finalizers[*]}' 2>/dev/null \
    | grep -Fq "${needle}"
}

wait_finalizer_present() {
  local kind="$1" name="$2" needle="$3" ns="${4:-default}"
  local end
  end=$((SECONDS + ${WAIT_TIMEOUT%s}))
  while (( SECONDS < end )); do
    if object_has_finalizer "${kind}" "${name}" "${needle}" "${ns}"; then
      return 0
    fi
    sleep 2
  done
  _fail_diag "${kind}/${name} never gained finalizer ${needle}"
  return 1
}

wait_port_forward_ready() {
  local port="$1"
  for _ in $(seq 1 30); do
    if curl -sf "http://127.0.0.1:${port}/inventory" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "port-forward to inventory HTTP on port ${port} never became ready" >&2
  return 1
}

inventory_http_item_count() {
  local port="$1"
  local body
  body="$(curl -sf "http://127.0.0.1:${port}/inventory?namespace=default" 2>/dev/null || true)"
  echo "${body}" | grep -oE '"itemCount":[0-9]+' | head -1 | cut -d: -f2
}

wait_inventory_http_collected() {
  local port="$1"
  for _ in $(seq 1 36); do
    local count
    count="$(inventory_http_item_count "${port}")"
    if [[ -n "${count}" && "${count}" -ge 1 ]]; then
      _log "inventory HTTP itemCount=${count}"
      return 0
    fi
    sleep 5
  done
  _fail_diag "inventory HTTP did not report itemCount >= 1 (no exported/collected artifact)"
  return 1
}

assert_target_ready() {
  if ! kubectl wait --for=condition=Ready "kollecttarget/${TARGET_NAME}" \
    -n default --timeout="${WAIT_TIMEOUT}"; then
    _fail_diag "KollectTarget/${TARGET_NAME} not Ready"
    return 1
  fi
}

# EDGE: Degraded Target shows Degraded=True + Warning Event.
scenario_degraded_warning() {
  _log "EDGE: Degraded Target + Warning Event"
  local prev
  prev="$(kubectl get kollectinventory "${INVENTORY_NAME}" -n default \
    -o jsonpath='{.spec.snapshotSinkRefs[0].name}' 2>/dev/null || true)"
  if [[ -z "${prev}" ]]; then
    prev="e2e-snapshot-sink"
  fi

  kubectl patch kollectinventory "${INVENTORY_NAME}" -n default --type=json \
    -p="[{\"op\":\"replace\",\"path\":\"/spec/snapshotSinkRefs\",\"value\":[{\"name\":\"${DEGRADE_SINK}\"}]}]"
  # Target does not watch Inventory — force reconcile so sink reachability updates.
  nudge_target_reconcile

  local end
  end=$((SECONDS + ${WAIT_TIMEOUT%s}))
  local degraded=0
  while (( SECONDS < end )); do
    local status
    status="$(kubectl get kollecttarget "${TARGET_NAME}" -n default \
      -o jsonpath='{range .status.conditions[?(@.type=="Degraded")]}{.status}{end}' 2>/dev/null || true)"
    if [[ "${status}" == "True" ]]; then
      degraded=1
      break
    fi
    sleep 2
  done
  if [[ "${degraded}" -ne 1 ]]; then
    _fail_diag "KollectTarget/${TARGET_NAME} never showed Degraded=True after missing sink"
    return 1
  fi
  _log "Target Degraded=True (missing sink)"

  # Warning Event from the target controller (SinkNotFound / similar).
  end=$((SECONDS + 60))
  local saw_warning=0
  while (( SECONDS < end )); do
    if kubectl get events -n default --field-selector "involvedObject.kind=KollectTarget,involvedObject.name=${TARGET_NAME}" \
      -o custom-columns=TYPE:.type,REASON:.reason --no-headers 2>/dev/null \
      | grep -Eq '^Warning'; then
      saw_warning=1
      break
    fi
    # Fallback: broader search (events.k8s.io naming varies by cluster age).
    if kubectl get events -n default -o json 2>/dev/null \
      | grep -E '"involvedObject".*"name":[[:space:]]*"'"${TARGET_NAME}"'"' -A20 \
      | grep -Eq '"type":[[:space:]]*"Warning"'; then
      saw_warning=1
      break
    fi
    sleep 2
  done
  if [[ "${saw_warning}" -ne 1 ]]; then
    _fail_diag "expected Warning Event on Degraded KollectTarget/${TARGET_NAME}"
    kubectl get events -n default --sort-by=.lastTimestamp | tail -30 >&2 || true
    return 1
  fi
  _log "Warning Event present for Degraded Target"

  # Restore sink ref so subsequent RED paths can collect again.
  kubectl patch kollectinventory "${INVENTORY_NAME}" -n default --type=json \
    -p="[{\"op\":\"replace\",\"path\":\"/spec/snapshotSinkRefs\",\"value\":[{\"name\":\"${prev}\"}]}]"
  nudge_target_reconcile
  assert_target_ready
  _log "EDGE Degraded/Warning OK; Target Ready again"
}

# EDGE: sink rejects teardown once → recovery completes.
scenario_sink_reject_retry() {
  _log "EDGE: sink rejects teardown once → retry/recovery completes"

  kubectl apply -f - <<EOF
apiVersion: kollect.dev/v1alpha1
kind: KollectDatabaseSink
metadata:
  name: ${RETRY_SINK}
  namespace: default
spec:
  type: postgres
  connectionTest: false
  postgres:
    databaseRef:
      name: ${MISSING_SECRET}
    table: finalizer_retry_items
---
apiVersion: kollect.dev/v1alpha1
kind: KollectInventory
metadata:
  name: ${RETRY_INV}
  namespace: default
spec:
  suspend: false
  databaseSinkRefs:
    - name: ${RETRY_SINK}
EOF

  wait_finalizer_present kollectinventory "${RETRY_INV}" "kollect.dev/inventory-cleanup"

  kubectl delete kollectinventory "${RETRY_INV}" -n default --wait=false

  # Cleanup must fail while the DB secret is missing — finalizer stays, object stuck deleting.
  local stuck=0
  for _ in $(seq 1 12); do
    if kubectl get kollectinventory "${RETRY_INV}" -n default >/dev/null 2>&1 \
      && object_has_finalizer kollectinventory "${RETRY_INV}" "kollect.dev/inventory-cleanup"; then
      local ts
      ts="$(kubectl get kollectinventory "${RETRY_INV}" -n default \
        -o jsonpath='{.metadata.deletionTimestamp}' 2>/dev/null || true)"
      if [[ -n "${ts}" ]]; then
        stuck=1
        break
      fi
    fi
    sleep 2
  done
  if [[ "${stuck}" -ne 1 ]]; then
    _fail_diag "expected ${RETRY_INV} to remain with cleanup finalizer while sink rejects teardown"
    return 1
  fi
  _log "sink reject held finalizer (partial finalization)"

  # Recovery: clear sink refs so cleanupSinkExports is a no-op and the finalizer drops.
  kubectl patch kollectinventory "${RETRY_INV}" -n default --type=json \
    -p='[{"op":"replace","path":"/spec/databaseSinkRefs","value":[]}]'

  wait_gone kollectinventory "${RETRY_INV}"
  kubectl delete kollectdatabasesink "${RETRY_SINK}" -n default --ignore-not-found --wait=false
  _log "EDGE sink-reject retry/recovery OK"
}

# RED: delete Target with collected artifact → finalizer runs, artifact torn down.
scenario_target_delete() {
  _log "RED: delete Target with collected artifact"

  assert_target_ready
  wait_finalizer_present kollecttarget "${TARGET_NAME}" "kollect.dev/target-cleanup"

  local http_port=18084
  local pf_pid=""
  kubectl port-forward -n "${KOLLECT_NAMESPACE}" svc/kollect-controller-manager "${http_port}:8082" &
  pf_pid=$!
  # shellcheck disable=SC2064
  trap "kill '${pf_pid}' 2>/dev/null || true" RETURN
  wait_port_forward_ready "${http_port}"
  wait_inventory_http_collected "${http_port}"

  local before
  before="$(inventory_http_item_count "${http_port}")"
  _log "pre-delete itemCount=${before}"

  kubectl delete kollecttarget "${TARGET_NAME}" -n default --wait=false
  wait_gone kollecttarget "${TARGET_NAME}"
  _log "Target deleted (finalizer removed)"

  # UnregisterTarget tears down the in-memory collection artifact.
  local end count
  end=$((SECONDS + ${WAIT_TIMEOUT%s}))
  while (( SECONDS < end )); do
    count="$(inventory_http_item_count "${http_port}")"
    if [[ -z "${count}" || "${count}" -lt "${before}" || "${count}" -eq 0 ]]; then
      _log "artifact torn down (itemCount ${before} → ${count:-0})"
      kill "${pf_pid}" 2>/dev/null || true
      pf_pid=""
      trap - RETURN
      return 0
    fi
    sleep 3
  done
  _fail_diag "inventory HTTP itemCount did not drop after Target delete (artifact not torn down)"
  kill "${pf_pid}" 2>/dev/null || true
  return 1
}

# RED: delete Inventory → cleanup finalizer completes (no stuck object).
scenario_inventory_delete() {
  _log "RED: delete Inventory (cleanup finalizer)"

  if ! kubectl get kollectinventory "${INVENTORY_NAME}" -n default >/dev/null 2>&1; then
    _log "re-applying team-inventory for cleanup assert"
    kubectl apply -f "${REPO_ROOT}/config/samples/e2e/team-inventory.yaml"
  fi
  wait_finalizer_present kollectinventory "${INVENTORY_NAME}" "kollect.dev/inventory-cleanup"

  kubectl delete kollectinventory "${INVENTORY_NAME}" -n default --wait=false
  wait_gone kollectinventory "${INVENTORY_NAME}"
  _log "Inventory deleted (cleanup finalizer removed)"
}

main() {
  REPO_ROOT="${REPO_ROOT:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
  _kind_require kubectl
  kind_use_context "${CLUSTER_NAME}"

  # Matrix-isolated jobs run setup only; bootstrap samples when missing.
  if ! kubectl get kollecttarget "${TARGET_NAME}" -n default >/dev/null 2>&1; then
    _log "Bootstrapping e2e sample CRs for finalizer-cleanup job..."
    bash "${REPO_ROOT}/hack/kind/e2e/bootstrap-samples.sh"
  fi

  scenario_degraded_warning
  scenario_sink_reject_retry
  scenario_target_delete
  scenario_inventory_delete

  _log "finalizer-cleanup asserts OK"
}

main "$@"
