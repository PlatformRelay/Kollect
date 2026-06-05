#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Multi-tenant namespace isolation smoke (kind + running operator).
# Creates two tenant namespaces, seeds workloads, asserts per-namespace inventory rollup.

set -euo pipefail

readonly TENANT_A="kollect-tenant-a"
readonly TENANT_B="kollect-tenant-b"
readonly TIMEOUT="120s"
readonly FIXTURES="${REPO_ROOT}/test/e2e/fixtures/multitenant"

log() { echo "[multitenant] $*"; }

apply_tenant() {
  local ns="$1"
  local image="$2"

  log "provisioning tenant namespace ${ns}"
  kubectl create namespace "${ns}" --dry-run=client -o yaml | kubectl apply -f -
  kubectl label namespace "${ns}" pod-security.kubernetes.io/enforce=restricted --overwrite

  sed "s/\${TENANT_NS}/${ns}/g" "${FIXTURES}/tenant-target.yaml.template" | kubectl apply -f -
  sed "s/\${TENANT_NS}/${ns}/g" "${FIXTURES}/tenant-inventory.yaml.template" | kubectl apply -f -

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
  kubectl wait --for=condition=Ready kollecttarget/tenant-deployments \
    -n "${ns}" --timeout="${TIMEOUT}"
}

wait_inventory_reconciled() {
  local ns="$1"
  for _ in $(seq 1 24); do
    local gen obs
    gen=$(kubectl get kollectinventory tenant-inventory -n "${ns}" -o jsonpath='{.metadata.generation}')
    obs=$(kubectl get kollectinventory tenant-inventory -n "${ns}" -o jsonpath='{.status.observedGeneration}')
    if [[ -n "${obs}" && "${obs}" == "${gen}" ]]; then
      return 0
    fi
    sleep 5
  done
  kubectl describe kollectinventory tenant-inventory -n "${ns}" >&2
  return 1
}

assert_inventory_isolated() {
  local count_a count_b
  count_a=$(kubectl get kollectinventory tenant-inventory -n "${TENANT_A}" -o jsonpath='{.status.itemCount}')
  count_b=$(kubectl get kollectinventory tenant-inventory -n "${TENANT_B}" -o jsonpath='{.status.itemCount}')

  log "tenant-a itemCount=${count_a} tenant-b itemCount=${count_b}"
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
  local pf_pid
  kubectl port-forward -n kollect-system svc/kollect-controller-manager 18082:8082 &
  pf_pid=$!
  sleep 3

  local body_a body_b
  body_a=$(curl -sf "http://127.0.0.1:18082/inventory?namespace=${TENANT_A}")
  body_b=$(curl -sf "http://127.0.0.1:18082/inventory?namespace=${TENANT_B}")

  kill "${pf_pid}" 2>/dev/null || true

  echo "${body_a}" | grep -q '"itemCount":1'
  echo "${body_b}" | grep -q '"itemCount":1'
  echo "${body_a}" | grep -q "${TENANT_A}"
  echo "${body_b}" | grep -q "${TENANT_B}"

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

  # Scope only in tenant-a (governance sample; reconciler enforcement is follow-up).
  kubectl apply -f "${FIXTURES}/tenant-scope.yaml"

  wait_target_ready "${TENANT_A}"
  wait_target_ready "${TENANT_B}"
  wait_inventory_reconciled "${TENANT_A}"
  wait_inventory_reconciled "${TENANT_B}"

  assert_inventory_isolated
  assert_http_namespace_filter

  log "multi-tenant isolation OK"
}

main "$@"
