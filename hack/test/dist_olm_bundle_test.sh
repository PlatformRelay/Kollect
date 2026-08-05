#!/usr/bin/env bash
# DIST-OLM-01: generate-olm-bundle must emit a registry+v1 bundle with all owned CRDs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

fail() {
  printf 'dist olm bundle: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

TEMPLATE="${ROOT}/config/olm/template/manifests/kollect.clusterserviceversion.yaml"
[[ -f "${TEMPLATE}" ]] || fail "${TEMPLATE} is missing"

grep -Fq 'make generate-olm-bundle' "${ROOT}/Makefile" ||
  fail "Makefile must define generate-olm-bundle target"

VERSION="9.9.9-test"
IMAGE_DIGEST="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
BUNDLE_DIR="dist/olm-bundle/${VERSION}"

rm -rf "${BUNDLE_DIR}"
make generate-olm-bundle VERSION="${VERSION}" IMAGE_DIGEST="${IMAGE_DIGEST}"

for path in \
  "${BUNDLE_DIR}/manifests/kollect.clusterserviceversion.yaml" \
  "${BUNDLE_DIR}/metadata/annotations.yaml"; do
  [[ -f "${path}" ]] || fail "missing ${path}"
done

while IFS= read -r crd_file; do
  base="$(basename "${crd_file}")"
  [[ -f "${BUNDLE_DIR}/manifests/${base}" ]] || fail "bundle missing CRD ${base}"
done < <(find config/crd/bases -name 'kollect.dev_*.yaml' | sort)

csv="${BUNDLE_DIR}/manifests/kollect.clusterserviceversion.yaml"
grep -Fq "kollect.v${VERSION}" "${csv}" || fail "CSV metadata.name must include version"
grep -Fq "ghcr.io/platformrelay/kollect@${IMAGE_DIGEST}" "${csv}" ||
  fail "CSV must digest-pin the controller image"
grep -Fq 'kind: KollectProfile' "${csv}" || fail "CSV alm-examples must include KollectProfile"
grep -Fq 'kind: KollectTarget' "${csv}" || fail "CSV alm-examples must include KollectTarget"

meta="${BUNDLE_DIR}/metadata/annotations.yaml"
grep -Fq 'operators.operatorframework.io.bundle.package.v1: kollect' "${meta}" ||
  fail "bundle package must be kollect"
grep -Fq 'operators.operatorframework.io.bundle.channels.v1: stable' "${meta}" ||
  fail "bundle channel must be stable"

pass "generate-olm-bundle produced complete bundle for ${VERSION}"

echo "All dist OLM bundle tests passed."
