#!/usr/bin/env bash
# Install a pinned Polaris release with SHA256-verified tarball download.
# Checksums from https://github.com/FairwindsOps/polaris/releases/download/${VERSION}/checksums.txt
# Usage: POLARIS_VERSION=9.6.0 hack/install-polaris.sh [install-dir]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/fetch.sh
source "${ROOT}/hack/lib/fetch.sh"

VERSION="${POLARIS_VERSION:-9.6.0}"
INSTALL_DIR="${1:-/usr/local/bin}"

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

TARBALL="polaris_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/FairwindsOps/polaris/releases/download/${VERSION}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# The checksum body lands in a FILE rather than a command substitution, and the tarball is not
# fetched until a digest is in hand. `EXPECTED_SHA256="$(curl ... | awk ...)"` propagated a dead
# network under `set -e` only because it was a BARE assignment: prefix it with local/export, as a
# refactor into a function naturally would, and the exit status becomes the builtin's (always 0),
# leaving the `[[ -z ]]` guard below as the only thing standing between a failed fetch and an
# empty expected digest.
fetch_to "${CHECKSUMS_URL}" "${TMP_DIR}/checksums.txt" "polaris ${VERSION} checksums"
EXPECTED_SHA256="$(awk -v file="${TARBALL}" '$2 == file {print $1}' "${TMP_DIR}/checksums.txt")"
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to resolve checksum for ${TARBALL} from ${CHECKSUMS_URL}" >&2
  exit 1
fi

fetch_to "${DOWNLOAD_URL}" "${TMP_DIR}/${TARBALL}" "polaris ${VERSION} tarball"
ACTUAL_SHA256="$(sha256sum "${TMP_DIR}/${TARBALL}" | awk '{print $1}')"
if [[ "${ACTUAL_SHA256}" != "${EXPECTED_SHA256}" ]]; then
  echo "checksum mismatch for ${TARBALL}" >&2
  echo "  expected: ${EXPECTED_SHA256}" >&2
  echo "  actual:   ${ACTUAL_SHA256}" >&2
  exit 1
fi

tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
install -m 0755 "${TMP_DIR}/polaris" "${INSTALL_DIR}/polaris"
"${INSTALL_DIR}/polaris" version
