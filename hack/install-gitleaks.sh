#!/usr/bin/env bash
# Install a pinned gitleaks release with SHA256-verified tarball download.
# Checksums from https://github.com/gitleaks/gitleaks/releases/download/v${VERSION}/gitleaks_${VERSION}_checksums.txt
# Usage: GITLEAKS_VERSION=8.30.1 hack/install-gitleaks.sh [install-dir]
# Optional: KOLLECT_FORCE_SHA256=<digest> overrides the upstream expected digest (tests only).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"
# shellcheck source=lib/fetch.sh
source "${ROOT}/hack/lib/fetch.sh"

VERSION="${GITLEAKS_VERSION:-8.30.1}"
INSTALL_DIR="${1:-/usr/local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ARCH="x64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

TARBALL="gitleaks_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/gitleaks/gitleaks/releases/download/v${VERSION}"
CHECKSUMS_URL="${BASE_URL}/gitleaks_${VERSION}_checksums.txt"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# The checksum body lands in a FILE rather than a command substitution, and the tarball is not
# fetched until a digest is in hand. `EXPECTED_SHA256="$(curl ... | awk ...)"` propagated a dead
# network under `set -e` only because it was a BARE assignment: prefix it with local/export, as a
# refactor into a function naturally would, and the exit status becomes the builtin's (always 0),
# leaving the `[[ -z ]]` guard below as the only thing standing between a failed fetch and an
# empty expected digest.
if [[ -n "${KOLLECT_FORCE_SHA256:-}" ]]; then
  EXPECTED_SHA256="${KOLLECT_FORCE_SHA256}"
else
  fetch_to "${CHECKSUMS_URL}" "${TMP_DIR}/checksums.txt" "gitleaks ${VERSION} checksums"
  EXPECTED_SHA256="$(awk -v file="${TARBALL}" '$2 == file {print $1}' "${TMP_DIR}/checksums.txt")"
fi
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to resolve checksum for ${TARBALL} from ${CHECKSUMS_URL}" >&2
  exit 1
fi

fetch_to "${DOWNLOAD_URL}" "${TMP_DIR}/${TARBALL}" "gitleaks ${VERSION} tarball"
verify_sha256 "${TMP_DIR}/${TARBALL}" "${EXPECTED_SHA256}"

tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}" gitleaks
install -m 0755 "${TMP_DIR}/gitleaks" "${INSTALL_DIR}/gitleaks"
"${INSTALL_DIR}/gitleaks" version
