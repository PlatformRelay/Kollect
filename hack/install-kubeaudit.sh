#!/usr/bin/env bash
# Install a pinned kubeaudit release with SHA256-verified tarball download.
# Checksums from https://github.com/Shopify/kubeaudit/releases/download/v${VERSION}/kubeaudit_${VERSION}_checksums.txt
# Usage: KUBEAUDIT_VERSION=0.22.2 hack/install-kubeaudit.sh [install-dir]
# Optional: KOLLECT_FORCE_SHA256=<digest> overrides the upstream expected digest (tests only).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"

VERSION="${KUBEAUDIT_VERSION:-0.22.2}"
INSTALL_DIR="${1:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  i386|i686) ARCH="386" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

TARBALL="kubeaudit_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/Shopify/kubeaudit/releases/download/v${VERSION}"
CHECKSUMS_URL="${BASE_URL}/kubeaudit_${VERSION}_checksums.txt"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

if [[ -n "${KOLLECT_FORCE_SHA256:-}" ]]; then
  EXPECTED_SHA256="${KOLLECT_FORCE_SHA256}"
else
  EXPECTED_SHA256="$(curl -fsSL "${CHECKSUMS_URL}" | awk -v file="${TARBALL}" '$2 == file {print $1}')"
fi
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to resolve checksum for ${TARBALL} from ${CHECKSUMS_URL}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}"
verify_sha256 "${TMP_DIR}/${TARBALL}" "${EXPECTED_SHA256}"

tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}" kubeaudit
install -m 0755 "${TMP_DIR}/kubeaudit" "${INSTALL_DIR}/kubeaudit"
"${INSTALL_DIR}/kubeaudit" version
