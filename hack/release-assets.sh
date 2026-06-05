#!/usr/bin/env bash
# Build release artifacts under dist/ for a tagged version.
# Usage: hack/release-assets.sh <version> <image-repo>
# Example: hack/release-assets.sh 0.1.0 ghcr.io/konih/kollect
set -euo pipefail

VERSION="${1:?version required (e.g. 0.1.0)}"
IMAGE="${2:?image repository required (e.g. ghcr.io/konih/kollect)}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="${ROOT}/dist"
WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

mkdir -p "${DIST}"
cp -a "${ROOT}/config" "${WORK}/config"

KUSTOMIZE="${ROOT}/bin/kustomize"
if [[ ! -x "${KUSTOMIZE}" ]]; then
  make -C "${ROOT}" kustomize >/dev/null
fi

(
  cd "${WORK}/config/manager"
  "${KUSTOMIZE}" edit set image "controller=${IMAGE}:${VERSION}"
)

"${KUSTOMIZE}" build "${WORK}/config/default" >"${DIST}/install.yaml"

awk 'FNR==1 && NR>1 {print "---"} {print}' "${ROOT}"/config/crd/bases/*.yaml \
  >"${DIST}/install-crds.yaml"

bash "${ROOT}/hack/helm-sync-crds.sh"
helm package "${ROOT}/charts/kollect" \
  --destination "${DIST}" \
  --version "${VERSION}" \
  --app-version "${VERSION}"

(
  cd "${DIST}"
  files=(install-crds.yaml install.yaml "kollect-${VERSION}.tgz")
  if [[ -f "sbom.spdx.json" ]]; then
    files+=("sbom.spdx.json")
  fi
  if [[ -f "sbom-ui.spdx.json" ]]; then
    files+=("sbom-ui.spdx.json")
  fi
  sha256sum "${files[@]}" >checksums.txt
)

echo "release assets written to ${DIST}/"
