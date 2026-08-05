#!/usr/bin/env bash
# DIST-AH-01: Chart.yaml must carry Artifact Hub operator metadata for all CRDs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART="${ROOT}/charts/kollect/Chart.yaml"
CRD_DIR="${ROOT}/config/crd/bases"

fail() {
  printf 'dist artifacthub chart: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${CHART}" ]] || fail "${CHART} is missing"

for key in \
  'artifacthub.io/license: MIT' \
  "artifacthub.io/operator: 'true'" \
  'artifacthub.io/category: monitoring-logging' \
  'artifacthub.io/operatorCapabilities:' \
  'artifacthub.io/crds: |' \
  'artifacthub.io/links: |' \
  'artifacthub.io/images: |'; do
  grep -Fq "${key}" "${CHART}" || fail "Chart.yaml missing annotation ${key}"
done

pass "required Artifact Hub annotation keys present"

while IFS= read -r crd_file; do
  kind="$(grep -E '^    kind: ' "${crd_file}" | head -1 | awk '{print $2}')"
  plural="$(grep -E '^    plural: ' "${crd_file}" | head -1 | awk '{print $2}')"
  [[ -n "${kind}" && -n "${plural}" ]] || fail "could not parse kind/plural from ${crd_file}"
  grep -Fq "kind: ${kind}" "${CHART}" || fail "Chart.yaml crds missing kind ${kind}"
  grep -Fq "name: ${plural}.kollect.dev" "${CHART}" || fail "Chart.yaml crds missing ${plural}.kollect.dev"
done < <(find "${CRD_DIR}" -name 'kollect.dev_*.yaml' | sort)

pass "Chart.yaml lists all CRD kinds from config/crd/bases"

grep -Fq 'ghcr.io/platformrelay/kollect:' "${CHART}" ||
  fail "Chart.yaml images annotation must reference ghcr.io/platformrelay/kollect"

echo "All dist Artifact Hub chart tests passed."
