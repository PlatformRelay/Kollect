#!/usr/bin/env bash
# Sync kubebuilder CRDs into the publishable Helm chart.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${ROOT}/config/crd/bases"
DST="${ROOT}/charts/kollect/crds"

if [[ ! -d "${SRC}" ]]; then
  echo "missing ${SRC}; run: task manifests" >&2
  exit 1
fi

mkdir -p "${DST}"
cp -f "${SRC}"/*.yaml "${DST}/"
echo "synced CRDs to ${DST}"
