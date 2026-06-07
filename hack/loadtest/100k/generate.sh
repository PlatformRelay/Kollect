#!/usr/bin/env bash
# Generate N namespaces × M Deployments for ~100k collected-row load tests (manual cloud gate).
set -euo pipefail

NAMESPACES="${1:-50}"
DEPLOYMENTS_PER_NS="${2:-2000}"
OUT_DIR="$(cd "$(dirname "$0")" && pwd)/manifests"

usage() {
  echo "Usage: $0 [--namespaces N] [--deployments-per-ns M]" >&2
  echo "  default: 50 namespaces × 2000 deployments" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespaces) NAMESPACES="$2"; shift 2 ;;
    --deployments-per-ns) DEPLOYMENTS_PER_NS="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) usage ;;
  esac
done

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

cat > "${OUT_DIR}/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
EOF

for ((i=1; i<=NAMESPACES; i++)); do
  ns="loadtest-$(printf '%03d' "${i}")"
  echo "  - namespace-${ns}.yaml" >> "${OUT_DIR}/kustomization.yaml"
  cat > "${OUT_DIR}/namespace-${ns}.yaml" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
  labels:
    kollect.dev/loadtest: "100k"
---
apiVersion: kollect.dev/v1alpha1
kind: KollectInventory
metadata:
  name: inventory
  namespace: ${ns}
spec:
  exportMinInterval: 30s
  databaseSinkRefs:
    - postgres-loadtest
  snapshotSinkRefs:
    - name: git-loadtest
      exportMinInterval: 1h
  suspend: false
EOF

  for ((d=1; d<=DEPLOYMENTS_PER_NS; d++)); do
    dep="dep-$(printf '%05d' "${d}")"
    cat >> "${OUT_DIR}/namespace-${ns}.yaml" <<EOF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${dep}
  namespace: ${ns}
  labels:
    app: loadtest
spec:
  replicas: 1
  selector:
    matchLabels:
      app: loadtest
      dep: ${dep}
  template:
    metadata:
      labels:
        app: loadtest
        dep: ${dep}
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.9
EOF
  done
done

echo "Wrote ${NAMESPACES} namespaces × ${DEPLOYMENTS_PER_NS} deployments → ${OUT_DIR}"
