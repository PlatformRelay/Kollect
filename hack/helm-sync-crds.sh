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

# Mirror, don't merge. A plain `cp -f` only ever adds and overwrites, so a CRD that was renamed or
# removed upstream would linger in the chart forever -- and because the stale file is present both
# before and after the sync, the drift gate in hack/verify.sh would report no difference and ship a
# phantom CRD to users. Clearing first makes the copy a true mirror, so deletions propagate and the
# gate can see them.
rm -f "${DST}"/*.yaml
cp -f "${SRC}"/*.yaml "${DST}/"
echo "synced CRDs to ${DST}"
