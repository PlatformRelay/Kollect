#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Multi-tenant namespace isolation smoke (kind + running operator).
# Creates two tenant namespaces, seeds workloads, asserts per-namespace inventory rollup.

set -euo pipefail

readonly TENANT_A="kollect-tenant-a"
readonly TENANT_B="kollect-tenant-b"
readonly TIMEOUT="${WAIT_TIMEOUT:-180s}"
readonly TARGET_NAME="tenant-deployments"
# Manager log budget for failure dumps — short tails hide post-Ready collect silence.
readonly DIAG_LOG_TAIL="${DIAG_LOG_TAIL:-200}"

# Set in main once REPO_ROOT is known (CI exports REPO_ROOT; local falls back).
FIXTURES=""

log() { echo "[multitenant] $*"; }

# Enriched dump for the next red nightly: Target status/collected signal, tenant
# Deployments, filtered + unfiltered inventory HTTP, useful manager log tail.
_fail_diag() {
  local ns="${1:-}"
  local port="${2:-}"
  local reason="${3:-assert failed}"
  log "FAILURE: ${reason}"
  if [[ -n "${ns}" ]]; then
    log "--- KollectTarget/${TARGET_NAME} -n ${ns} ---"
    kubectl get "kollecttarget/${TARGET_NAME}" -n "${ns}" -o wide 2>/dev/null || true
    kubectl describe "kollecttarget/${TARGET_NAME}" -n "${ns}" 2>/dev/null || true
    log "--- KollectInventory -n ${ns} ---"
    kubectl get kollectinventory -n "${ns}" -o wide 2>/dev/null || true
    kubectl describe kollectinventory -n "${ns}" 2>/dev/null || true
    log "--- Deployments -n ${ns} ---"
    kubectl get deployments -n "${ns}" -o wide 2>/dev/null || true
    kubectl get deployment tenant-app -n "${ns}" -o yaml 2>/dev/null || true
  fi
  if [[ -n "${port}" ]]; then
    log "--- inventory HTTP ?namespace=${ns:-} ---"
    curl -sf "http://127.0.0.1:${port}/inventory?namespace=${ns}" 2>/dev/null | head -c 4000 || true
    echo >&2
    log "--- inventory HTTP unfiltered /inventory ---"
    curl -sf "http://127.0.0.1:${port}/inventory" 2>/dev/null | head -c 4000 || true
    echo >&2
  fi
  log "--- manager logs (tail=${DIAG_LOG_TAIL}) ---"
  kubectl logs -n kollect-system -l app.kubernetes.io/name=kollect --tail="${DIAG_LOG_TAIL}" 2>/dev/null || true
}

# Bounded retry for assertions racing inventory collection: attempts, delay seconds, then command.
# The final attempt runs last so its stderr explains the failure.
retry_assert() {
  local attempts="$1" delay="$2"
  shift 2
  local i
  for ((i = 1; i < attempts; i++)); do
    if "$@" 2>/dev/null; then
      return 0
    fi
    log "assertion $1 not satisfied yet (attempt ${i}/${attempts}); retrying in ${delay}s"
    sleep "${delay}"
  done
  "$@"
}

apply_tenant() {
  local ns="$1"
  local image="$2"

  log "provisioning tenant namespace ${ns}"
  kubectl create namespace "${ns}" --dry-run=client -o yaml | kubectl apply -f -
  kubectl label namespace "${ns}" pod-security.kubernetes.io/enforce=restricted --overwrite

  sed "s/\${TENANT_NS}/${ns}/g" "${FIXTURES}/tenant-profile.yaml.template" | kubectl apply -f -
  sed "s/\${TENANT_NS}/${ns}/g" "${FIXTURES}/tenant-target.yaml.template" | kubectl apply -f -

  kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tenant-app
  namespace: ${ns}
  labels:
    kollect.dev/tenant: ${ns}
spec:
  replicas: 1
  selector:
    matchLabels:
      kollect.dev/tenant: ${ns}
  template:
    metadata:
      labels:
        kollect.dev/tenant: ${ns}
    spec:
      containers:
        - name: app
          image: ${image}
EOF
}

wait_target_ready() {
  local ns="$1"
  if ! kubectl wait --for=condition=Ready "kollecttarget/${TARGET_NAME}" \
    -n "${ns}" --timeout="${TIMEOUT}"; then
    _fail_diag "${ns}" "" "KollectTarget/${TARGET_NAME} in ${ns} not Ready"
    return 1
  fi
}

# Ready alone allows "collecting 0 resource(s)". Poll the Ready message until
# collecting >= 1 so we do not race HTTP itemCount against an empty Store.
# Bounded by TIMEOUT — still fails loudly (with dumps) if collection never starts.
# Nudge reconcile each loop: Target status lags Store until the next reconcile.
target_collected_count() {
  local ns="$1"
  local msg
  msg="$(kubectl get "kollecttarget/${TARGET_NAME}" -n "${ns}" \
    -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}' 2>/dev/null || true)"
  # Message shape: profileRef "…" resolved; collecting N resource(s)
  echo "${msg}" | grep -oE 'collecting[[:space:]]+[0-9]+' | awk '{print $2}'
}

nudge_target_reconcile() {
  local ns="$1"
  kubectl annotate "kollecttarget/${TARGET_NAME}" -n "${ns}" \
    "kollect.dev/e2e-reconcile-nudge=$(date +%s%N)" --overwrite >/dev/null 2>&1 || true
}

wait_target_collected() {
  local ns="$1"
  local end count
  end=$((SECONDS + ${TIMEOUT%s}))
  while (( SECONDS < end )); do
    count="$(target_collected_count "${ns}")"
    if [[ -n "${count}" && "${count}" -ge 1 ]]; then
      log "target ${ns}/${TARGET_NAME} collecting=${count}"
      return 0
    fi
    nudge_target_reconcile "${ns}"
    sleep 2
  done
  _fail_diag "${ns}" "" \
    "KollectTarget/${TARGET_NAME} in ${ns} never reported collecting >= 1 (Ready alone is insufficient)"
  return 1
}

apply_tenant_inventory() {
  local ns="$1"
  sed "s/\${TENANT_NS}/${ns}/g" "${FIXTURES}/tenant-inventory.yaml.template" | kubectl apply -f -
}

# Poll the port-forward until the inventory HTTP endpoint answers (replaces a blind sleep).
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
  local ns="$1" port="$2"
  local body
  body="$(curl -sf "http://127.0.0.1:${port}/inventory?namespace=${ns}" 2>/dev/null || true)"
  echo "$body" | grep -oE '"itemCount":[0-9]+' | head -1 | cut -d: -f2
}

wait_inventory_http_collected() {
  local ns="$1" port="$2"
  for _ in $(seq 1 30); do
    local count
    count="$(inventory_http_item_count "${ns}" "${port}")"
    if [[ -n "${count}" && "${count}" -ge 1 ]]; then
      return 0
    fi
    sleep 5
  done
  _fail_diag "${ns}" "${port}" \
    "inventory HTTP for ${ns} did not report itemCount >= 1"
  return 1
}

assert_inventory_isolated() {
  local port="$1" count_a count_b
  count_a="$(inventory_http_item_count "${TENANT_A}" "${port}")"
  count_b="$(inventory_http_item_count "${TENANT_B}" "${port}")"

  log "tenant-a itemCount=${count_a:-0} tenant-b itemCount=${count_b:-0}"
  if [[ -z "${count_a}" || "${count_a}" -lt 1 ]]; then
    _fail_diag "${TENANT_A}" "${port}" "expected tenant-a inventory to collect at least one item"
    return 1
  fi
  if [[ -z "${count_b}" || "${count_b}" -lt 1 ]]; then
    _fail_diag "${TENANT_B}" "${port}" "expected tenant-b inventory to collect at least one item"
    return 1
  fi
  if [[ "${count_a}" != "1" || "${count_b}" != "1" ]]; then
    _fail_diag "${TENANT_A}" "${port}" \
      "expected exactly one item per tenant inventory (got a=${count_a} b=${count_b})"
    return 1
  fi
}

assert_http_namespace_filter() {
  local port="$1" body_a body_b
  # Explicit returns keep this function correct when invoked in an if/retry
  # context, where `set -e` is suspended and bare failing greps would be ignored.
  body_a=$(curl -sf "http://127.0.0.1:${port}/inventory?namespace=${TENANT_A}") || return 1
  body_b=$(curl -sf "http://127.0.0.1:${port}/inventory?namespace=${TENANT_B}") || return 1

  echo "${body_a}" | grep -q '"itemCount":1' \
    || { echo "tenant-a: expected itemCount:1 in HTTP inventory body, got: ${body_a}" >&2; return 1; }
  echo "${body_b}" | grep -q '"itemCount":1' \
    || { echo "tenant-b: expected itemCount:1 in HTTP inventory body, got: ${body_b}" >&2; return 1; }
  echo "${body_a}" | grep -q "${TENANT_A}" \
    || { echo "tenant-a: expected namespace ${TENANT_A} in HTTP inventory body, got: ${body_a}" >&2; return 1; }
  echo "${body_b}" | grep -q "${TENANT_B}" \
    || { echo "tenant-b: expected namespace ${TENANT_B} in HTTP inventory body, got: ${body_b}" >&2; return 1; }

  if echo "${body_a}" | grep -q "${TENANT_B}/tenant-app"; then
    echo "tenant-a HTTP inventory leaked tenant-b workload" >&2
    return 1
  fi
  if echo "${body_b}" | grep -q "${TENANT_A}/tenant-app"; then
    echo "tenant-b HTTP inventory leaked tenant-a workload" >&2
    return 1
  fi
}

main() {
  REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
  FIXTURES="${REPO_ROOT}/test/e2e/fixtures/multitenant"
  local image="${TENANT_APP_IMAGE:-nginx:1.27-alpine}"

  apply_tenant "${TENANT_A}" "${image}"
  apply_tenant "${TENANT_B}" "${image}"

  wait_target_ready "${TENANT_A}"
  wait_target_ready "${TENANT_B}"
  # Harden against Ready-with-zero flake: ensure Store has items before HTTP asserts.
  wait_target_collected "${TENANT_A}"
  wait_target_collected "${TENANT_B}"

  apply_tenant_inventory "${TENANT_A}"
  apply_tenant_inventory "${TENANT_B}"

  pf_pid=""
  local http_port=18082
  kubectl port-forward -n kollect-system svc/kollect-controller-manager "${http_port}:8082" &
  pf_pid=$!
  trap '[[ -n "${pf_pid}" ]] && kill "${pf_pid}" 2>/dev/null || true' EXIT
  wait_port_forward_ready "${http_port}"

  wait_inventory_http_collected "${TENANT_A}" "${http_port}"
  wait_inventory_http_collected "${TENANT_B}" "${http_port}"

  # Exact-count asserts race the inventory rollup settling to exactly one item
  # per tenant; poll bounded instead of a single-shot check.
  retry_assert 6 5 assert_inventory_isolated "${http_port}"
  retry_assert 6 5 assert_http_namespace_filter "${http_port}"

  # Governance sample only; apply after collection asserts (enforcement is follow-up).
  kubectl apply -f "${FIXTURES}/tenant-scope.yaml"

  log "multi-tenant isolation OK"
}

main "$@"
