#!/usr/bin/env bash
# Install a pinned Helm 3 release with SHA256-verified tarball download.
# Checksums from https://get.helm.sh/helm-${VERSION}-${OS}-${ARCH}.tar.gz.sha256
# Usage: HELM_VERSION=v3.17.3 hack/install-helm.sh
set -euo pipefail

VERSION="${HELM_VERSION:-v3.17.3}"
VERSION_NO_V="${VERSION#v}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

TARBALL="helm-${VERSION}-${OS}-${ARCH}.tar.gz"
BASE_URL="https://get.helm.sh"
CHECKSUM_URL="${BASE_URL}/${TARBALL}.sha256"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

EXPECTED_SHA256="$(curl -fsSL "${CHECKSUM_URL}")"
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to fetch checksum from ${CHECKSUM_URL}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}"
ACTUAL_SHA256="$(sha256sum "${TMP_DIR}/${TARBALL}" | awk '{print $1}')"
if [[ "${ACTUAL_SHA256}" != "${EXPECTED_SHA256}" ]]; then
  echo "checksum mismatch for ${TARBALL}" >&2
  echo "  expected: ${EXPECTED_SHA256}" >&2
  echo "  actual:   ${ACTUAL_SHA256}" >&2
  exit 1
fi

tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
install -m 0755 "${TMP_DIR}/${OS}-${ARCH}/helm" /usr/local/bin/helm
helm version --short
