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
readonly FIXTURES="${REPO_ROOT}/test/e2e/fixtures/multitenant"

log() { echo "[multitenant] $*"; }

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
  if ! kubectl wait --for=condition=Ready kollecttarget/tenant-deployments \
    -n "${ns}" --timeout="${TIMEOUT}"; then
    kubectl describe kollecttarget tenant-deployments -n "${ns}" >&2 || true
    kubectl logs -n kollect-system -l app.kubernetes.io/name=kollect --tail=80 >&2 || true
    return 1
  fi
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
  echo "inventory HTTP for ${ns} did not report itemCount >= 1" >&2
  kubectl logs -n kollect-system -l app.kubernetes.io/name=kollect --tail=80 >&2 || true
  return 1
}

assert_inventory_isolated() {
  local port="$1" count_a count_b
  count_a="$(inventory_http_item_count "${TENANT_A}" "${port}")"
  count_b="$(inventory_http_item_count "${TENANT_B}" "${port}")"

  log "tenant-a itemCount=${count_a:-0} tenant-b itemCount=${count_b:-0}"
  if [[ -z "${count_a}" || "${count_a}" -lt 1 ]]; then
    echo "expected tenant-a inventory to collect at least one item" >&2
    return 1
  fi
  if [[ -z "${count_b}" || "${count_b}" -lt 1 ]]; then
    echo "expected tenant-b inventory to collect at least one item" >&2
    return 1
  fi
  if [[ "${count_a}" != "1" || "${count_b}" != "1" ]]; then
    echo "expected exactly one item per tenant inventory (got a=${count_a} b=${count_b})" >&2
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
  local image="${TENANT_APP_IMAGE:-nginx:1.27-alpine}"

  apply_tenant "${TENANT_A}" "${image}"
  apply_tenant "${TENANT_B}" "${image}"

  wait_target_ready "${TENANT_A}"
  wait_target_ready "${TENANT_B}"

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
