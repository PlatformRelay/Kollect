#!/usr/bin/env bash
# Install a pinned Helm 3 release with SHA256-verified tarball download.
# Checksums from https://get.helm.sh/helm-${VERSION}-${OS}-${ARCH}.tar.gz.sha256
# Usage: HELM_VERSION=v3.21.4 hack/install-helm.sh [install-dir]
set -euo pipefail

VERSION="${HELM_VERSION:-v3.21.4}"
INSTALL_DIR="${1:-/usr/local/bin}"

# CI-HELMDL-01 / CI-FETCHLIB-01. Both fetches below used to be bare `curl -fsSL`. On 2026-09-01
# a single transient `curl: (7) Failed to connect to get.helm.sh` reddened `kind-smoke` -- a
# REQUIRED check -- on PR #351, a bash-only diff, and blocked the merge queue.
#
# CI-HELMDL-01 fixed it with a retry helper local to this file, and said in the same breath why:
# seven sibling hack/install-*.sh scripts shared the identical defect, a shared helper was the
# right home for the flag list, and factoring it out was a repo-wide refactor that change did not
# own. CI-FETCHLIB-01 is that refactor. The helper now lives in hack/lib/fetch.sh, every
# installer and the kind download in .github/actions/kind-e2e-setup call it, and the flag list --
# together with the reasoning for each flag, for the outer retry loop, and for what deliberately
# is NOT retried -- lives there in one place instead of in eight.
#
# WHAT STAYS HERE, because it is about helm and not about fetching: this script fetches the
# CHECKSUM FIRST and refuses to download the tarball without a digest in hand, it lands the
# checksum body in a file rather than a command substitution, and it rejects a 200 whose body is
# not a bare 64-hex digest before that body can be misreported as a "checksum mismatch".
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/fetch.sh
source "${ROOT}/hack/lib/fetch.sh"

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

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# The checksum lands in a file rather than a command substitution. Two reasons. First,
# `EXPECTED_SHA256="$(curl ...)"` propagates the failure under `set -e` only because it is a
# BARE assignment -- prefix it with local/export/readonly, as a refactor into a function
# naturally would, and the exit status becomes that of the builtin (always 0) and the guard
# below silently becomes the only thing catching a dead network. Second, fetch_to writes to a
# path so that it can delete a partial response rather than hand it on.
fetch_to "${CHECKSUM_URL}" "${TMP_DIR}/${TARBALL}.sha256" "helm ${VERSION} checksum"
EXPECTED_SHA256="$(tr -d '[:space:]' <"${TMP_DIR}/${TARBALL}.sha256")"
if [[ -z "${EXPECTED_SHA256}" ]]; then
  echo "failed to fetch checksum from ${CHECKSUM_URL}" >&2
  exit 1
fi
# get.helm.sh publishes a bare 64-character lowercase digest at this path. Anything else is a
# 200 that is not a checksum -- an interception page, a truncated body -- and must not be
# carried into the comparison, where it would be reported as a "checksum mismatch" and send the
# reader looking for a corrupt tarball that is fine.
if [[ ! "${EXPECTED_SHA256}" =~ ^[0-9a-f]{64}$ ]]; then
  echo "checksum from ${CHECKSUM_URL} is not a bare sha256 digest: ${EXPECTED_SHA256}" >&2
  exit 1
fi

fetch_to "${DOWNLOAD_URL}" "${TMP_DIR}/${TARBALL}" "helm ${VERSION} tarball"
ACTUAL_SHA256="$(sha256sum "${TMP_DIR}/${TARBALL}" | awk '{print $1}')"
if [[ "${ACTUAL_SHA256}" != "${EXPECTED_SHA256}" ]]; then
  echo "checksum mismatch for ${TARBALL}" >&2
  echo "  expected: ${EXPECTED_SHA256}" >&2
  echo "  actual:   ${ACTUAL_SHA256}" >&2
  exit 1
fi

tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
mkdir -p "${INSTALL_DIR}"
install -m 0755 "${TMP_DIR}/${OS}-${ARCH}/helm" "${INSTALL_DIR}/helm"
"${INSTALL_DIR}/helm" version --short
