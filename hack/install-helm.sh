#!/usr/bin/env bash
# Install a pinned Helm 3 release with SHA256-verified tarball download.
# Checksums from https://get.helm.sh/helm-${VERSION}-${OS}-${ARCH}.tar.gz.sha256
# Usage: HELM_VERSION=v3.21.4 hack/install-helm.sh [install-dir]
set -euo pipefail

VERSION="${HELM_VERSION:-v3.21.4}"
INSTALL_DIR="${1:-/usr/local/bin}"

# CI-HELMDL-01. Both fetches below used to be bare `curl -fsSL`. On 2026-09-01 a single
# transient `curl: (7) Failed to connect to get.helm.sh` reddened `kind-smoke` -- a REQUIRED
# check -- on PR #351, a bash-only diff, and blocked the merge queue. In the same composite
# step (.github/actions/kind-e2e-setup) the `kind` download was already wrapped in a
# three-attempt loop with protocol pinning and curl-level retries. The flag set and the loop
# shape below are copied from it deliberately: the defect was an ASYMMETRY between two
# downloads in one step, so the fix is to make them the same, not to invent a third style.
#
# Two deliberate deviations from that model. (1) The kind loop falls THROUGH when all three
# attempts fail. `set -euo pipefail` still fails the job -- the next command is `chmod +x` on a
# file that was never written -- so it is not a false green, but the operator is handed a chmod
# error instead of "could not reach the origin", and the loop has already burned a pointless
# 10s sleep after its last attempt. fetch_with_retry returns non-zero naming the URL, and skips
# the sleep on the final attempt. (2) `--max-time`/`--retry-max-time` bound the TRANSFER, which
# nothing in the kind loop does: `--connect-timeout` only bounds the connect, so a body that
# stalls after the first byte hangs until the job's own timeout kills it with no diagnostic.
# 120s is a 150 KB/s floor on an 18 MB tarball -- unreachable on a working link, so it cannot
# introduce the flake class it exists to remove -- and it caps one curl invocation at ~4 min,
# hence the whole helper at ~12 min, instead of at infinity.
#
# WHY AN OUTER LOOP AT ALL, given `--retry 5` is on the command line: curl's --retry covers
# "transient" responses -- 408, 429, 5xx, timeouts -- but NOT a failed connect, unless
# --retry-connrefused/--retry-all-errors is passed. `curl: (7)` is exactly a failed connect,
# so --retry alone would not have saved PR #351. The loop covers the connect/DNS/TLS class;
# --retry covers a flaky origin that answers. Both are needed, and neither subsumes the other.
#
# WHAT THE LOOP DOES *NOT* WRAP, and why. The rule is: retry the TRANSPORT, never a decision
# about content. A retry is only defensible when the failure carries no information -- a dead
# TCP connect says nothing about get.helm.sh's contents. Once bytes have arrived in full,
# anything wrong with them (empty body, an interception page, a digest that does not match) is
# evidence, not noise: it means the mirror, the network path, or the release itself is not what
# we pinned. Retrying that would trade a hard, diagnosable failure for an intermittent one --
# and a security check that sometimes passes is strictly worse than one that always fails,
# because it gets re-run until it goes green. So verification happens exactly once, after the
# transport has succeeded, and every content failure exits non-zero immediately.
#
# Kept inline as a local function rather than factored into hack/lib/: seven other hack/install-*.sh
# scripts share this defect, and a shared fetch helper is the right home for the flag list --
# but that is a repo-wide refactor of files this change does not own. Within this script the
# function still buys the property that matters: the flags are written ONCE, so the two fetches
# cannot drift apart again the way they just did.
CURL_ATTEMPTS=3

fetch_with_retry() {
  local url="$1" dest="$2" what="$3"
  local attempt
  for ((attempt = 1; attempt <= CURL_ATTEMPTS; attempt++)); do
    if curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
      -fsSL --retry 5 --retry-delay 5 --retry-max-time 120 \
      --connect-timeout 30 --max-time 120 \
      -o "${dest}" "${url}"; then
      return 0
    fi
    rm -f "${dest}"
    if ((attempt < CURL_ATTEMPTS)); then
      echo "${what} download attempt ${attempt} failed; retrying..." >&2
      sleep 10
    fi
  done
  echo "failed to download ${what} from ${url} after ${CURL_ATTEMPTS} attempts" >&2
  return 1
}

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
# below silently becomes the only thing catching a dead network. Second, the retry loop needs
# somewhere to put a partial response so it can discard it.
fetch_with_retry "${CHECKSUM_URL}" "${TMP_DIR}/${TARBALL}.sha256" "checksum"
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

fetch_with_retry "${DOWNLOAD_URL}" "${TMP_DIR}/${TARBALL}" "helm tarball"
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
