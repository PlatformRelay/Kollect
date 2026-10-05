#!/usr/bin/env bash
# Install a pinned zizmor release with SHA256-verified tarball download (CWS-1).
#
# zizmor publishes no checksums file in its releases, so unlike install-gitleaks.sh the expected
# digests are pinned here, one per asset, taken from the release's asset digests. The install
# fails closed: a version bump that does not update the digest stops at `checksum mismatch`
# rather than running an unverified binary.
# Usage: ZIZMOR_VERSION=1.30.1 hack/install-zizmor.sh [install-dir]
# Optional: KOLLECT_FORCE_SHA256=<digest> overrides the pinned digest (tests only).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"
# shellcheck source=lib/fetch.sh
source "${ROOT}/hack/lib/fetch.sh"

VERSION="${ZIZMOR_VERSION:-1.30.1}"
INSTALL_DIR="${1:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Asset name and pinned SHA256 per platform. The digests are the `sha256:` values the GitHub
# release API reports for zizmor v1.30.1.
case "${OS}/${ARCH}" in
  linux/x86_64)
    ASSET="zizmor-x86_64-unknown-linux-gnu.tar.gz"
    PINNED_SHA256="e65324f4430c2717591937edcec90ccbefaf14c174f8ec9415e03ca875b46e1a"
    ;;
  linux/aarch64 | linux/arm64)
    ASSET="zizmor-aarch64-unknown-linux-gnu.tar.gz"
    PINNED_SHA256="7ff1dce33bdd18fd2a4affe63bdd47efcccca97b2cec1c1863ec26e9e2647540"
    ;;
  darwin/x86_64)
    ASSET="zizmor-x86_64-apple-darwin.tar.gz"
    PINNED_SHA256="10e6b18b11ea07e515a16f0f0518c7b07527bc9977c1fd5698181ce7f3554202"
    ;;
  darwin/arm64)
    ASSET="zizmor-aarch64-apple-darwin.tar.gz"
    PINNED_SHA256="e28d22b087f9ebb8d99da6e740d348c930f559961c7c3f12badda54f882195a2"
    ;;
  *)
    echo "unsupported platform for zizmor: ${OS}/${ARCH}" >&2
    exit 1
    ;;
esac

DOWNLOAD_URL="https://github.com/zizmorcore/zizmor/releases/download/v${VERSION}/${ASSET}"
EXPECTED_SHA256="${KOLLECT_FORCE_SHA256:-${PINNED_SHA256}}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

fetch_to "${DOWNLOAD_URL}" "${TMP_DIR}/${ASSET}" "zizmor ${VERSION} tarball"
verify_sha256 "${TMP_DIR}/${ASSET}" "${EXPECTED_SHA256}"

tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}" zizmor
install -m 0755 "${TMP_DIR}/zizmor" "${INSTALL_DIR}/zizmor"
"${INSTALL_DIR}/zizmor" --version
