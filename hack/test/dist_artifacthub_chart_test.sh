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

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to verify the artifacthub.io/images annotation"

APP_VERSION="$(yq eval '.appVersion' "${CHART}")"
[[ -n "${APP_VERSION}" && "${APP_VERSION}" != "null" ]] ||
  fail "Chart.yaml appVersion is missing"

IMAGES_BLOCK="$(yq eval '.annotations."artifacthub.io/images"' "${CHART}")"
[[ -n "${IMAGES_BLOCK}" && "${IMAGES_BLOCK}" != "null" ]] ||
  fail "Chart.yaml artifacthub.io/images annotation is empty"

# An explicit artifacthub.io/images list OVERRIDES Artifact Hub's automatic image
# extraction, so a tag that drifts from appVersion makes the hub advertise -- and
# run its security report against -- the wrong image indefinitely. Assert the tag
# equals v<appVersion> parsed from this same Chart.yaml, not merely the repo prefix.
EXPECTED_IMAGE="ghcr.io/platformrelay/kollect:v${APP_VERSION}"
found_controller_image=0
while IFS= read -r img; do
  [[ -n "${img}" && "${img}" != "null" ]] || continue
  case "${img}" in
  ghcr.io/platformrelay/kollect:* | ghcr.io/platformrelay/kollect@*)
    found_controller_image=1
    [[ "${img}" == "${EXPECTED_IMAGE}" ]] ||
      fail "artifacthub.io/images lists ${img} but Chart.yaml appVersion is ${APP_VERSION}; expected ${EXPECTED_IMAGE} (bump the annotation together with version/appVersion -- see docs/RELEASE.md step 3)"
    ;;
  *) ;;
  esac
done < <(printf '%s\n' "${IMAGES_BLOCK}" | yq eval '.[].image' -)

[[ "${found_controller_image}" -eq 1 ]] ||
  fail "artifacthub.io/images must list ${EXPECTED_IMAGE}"

pass "artifacthub.io/images tag tracks appVersion (${EXPECTED_IMAGE})"

echo "All dist Artifact Hub chart tests passed."
