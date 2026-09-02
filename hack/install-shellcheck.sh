#!/usr/bin/env bash
# Install a pinned ShellCheck release with digest-pinned tarball download.
# Upstream does not publish checksums.txt; digests below were recorded from the
# official GitHub release assets for v0.11.0 (AUD-SEC-03 digest pin).
# Usage: SHELLCHECK_VERSION=0.11.0 hack/install-shellcheck.sh [install-dir]
# Optional: KOLLECT_FORCE_SHA256=<digest> overrides the pinned digest (tests only).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"
# shellcheck source=lib/fetch.sh
source "${ROOT}/hack/lib/fetch.sh"

VERSION="${SHELLCHECK_VERSION:-0.11.0}"
INSTALL_DIR="${1:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${OS}" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac
case "${ARCH}" in
  x86_64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="aarch64" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

TARBALL="shellcheck-v${VERSION}.${OS}.${ARCH}.tar.xz"
BASE_URL="https://github.com/koalaman/shellcheck/releases/download/v${VERSION}"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

# Digest pin table for known ShellCheck release assets (no upstream checksum file).
pinned_sha256() {
  case "$1" in
    shellcheck-v0.11.0.linux.x86_64.tar.xz)
      echo "8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198"
      ;;
    shellcheck-v0.11.0.linux.aarch64.tar.xz)
      echo "12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588"
      ;;
    shellcheck-v0.11.0.darwin.aarch64.tar.xz)
      echo "56affdd8de5527894dca6dc3d7e0a99a873b0f004d7aabc30ae407d3f48b0a79"
      ;;
    shellcheck-v0.11.0.darwin.x86_64.tar.xz)
      echo "3c89db4edcab7cf1c27bff178882e0f6f27f7afdf54e859fa041fca10febe4c6"
      ;;
    *)
      return 1
      ;;
  esac
}

if [[ -n "${KOLLECT_FORCE_SHA256:-}" ]]; then
  EXPECTED_SHA256="${KOLLECT_FORCE_SHA256}"
else
  EXPECTED_SHA256="$(pinned_sha256 "${TARBALL}")" || {
    echo "no pinned sha256 for ${TARBALL}; add digest before installing" >&2
    exit 1
  }
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fetch_to "${DOWNLOAD_URL}" "${TMP_DIR}/${TARBALL}" "shellcheck v${VERSION} tarball"
verify_sha256 "${TMP_DIR}/${TARBALL}" "${EXPECTED_SHA256}"

tar -xJf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
install -m 0755 "${TMP_DIR}/shellcheck-v${VERSION}/shellcheck" "${INSTALL_DIR}/shellcheck"
"${INSTALL_DIR}/shellcheck" --version
