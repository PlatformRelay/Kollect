#!/usr/bin/env bash
# Install a pinned operator-sdk release with SHA256-verified binary download.
# Checksums from https://github.com/operator-framework/operator-sdk/releases/download/${VERSION}/checksums.txt
# Usage: OPERATOR_SDK_VERSION=v1.42.3 hack/install-operator-sdk.sh [install-dir]
# Optional: KOLLECT_FORCE_SHA256=<digest> overrides the upstream expected digest (tests only).
#
# Why a release binary and not `go install`: the operator-sdk module pulls in
# github.com/proglottis/gpgme, so `go install .../cmd/operator-sdk@version` needs cgo and
# pkg-config unless it is built with CGO_ENABLED=0 -tags containers_image_openpgp. The
# published binary is checksum-verifiable and installs in seconds instead of minutes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"

VERSION="${OPERATOR_SDK_VERSION:-v1.42.3}"
INSTALL_DIR="${1:-${ROOT}/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

BINARY="operator-sdk_${OS}_${ARCH}"
BASE_URL="https://github.com/operator-framework/operator-sdk/releases/download/${VERSION}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"
DOWNLOAD_URL="${BASE_URL}/${BINARY}"

if [[ -n "${KOLLECT_FORCE_SHA256:-}" ]]; then
  EXPECTED_SHA256="${KOLLECT_FORCE_SHA256}"
else
  EXPECTED_SHA256="$(curl -fsSL "${CHECKSUMS_URL}" | awk -v file="${BINARY}" '$2 == file {print $1}')"
fi
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to resolve checksum for ${BINARY} from ${CHECKSUMS_URL}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${BINARY}"
verify_sha256 "${TMP_DIR}/${BINARY}" "${EXPECTED_SHA256}"

# The release asset is a bare binary (no tarball), so install it directly.
mkdir -p "${INSTALL_DIR}"
install -m 0755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/operator-sdk"
"${INSTALL_DIR}/operator-sdk" version
