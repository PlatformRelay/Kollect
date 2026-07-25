#!/usr/bin/env bash
# Negative integration test: wrong checksum must not install a scanner binary.
# Downloads a real archive once, then forces a bad digest via KOLLECT_FORCE_SHA256.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

WRONG_SHA="0000000000000000000000000000000000000000000000000000000000000000"
INSTALL_DIR="${TMP}/bin"
mkdir -p "${INSTALL_DIR}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

# Pick one installer that publishes upstream checksums (fast path for CI/local).
# kubeaudit is smaller than ShellCheck and exercises OS/arch selection.
if ! KOLLECT_FORCE_SHA256="${WRONG_SHA}" \
  KUBEAUDIT_VERSION="${KUBEAUDIT_VERSION:-0.22.2}" \
  bash "${ROOT}/hack/install-kubeaudit.sh" "${INSTALL_DIR}" \
  >"${TMP}/out.txt" 2>"${TMP}/err.txt"; then
  :
else
  fail "install-kubeaudit must fail with wrong checksum"
fi

if ! grep -q 'checksum mismatch' "${TMP}/err.txt"; then
  fail "expected checksum mismatch error; got: $(cat "${TMP}/err.txt")"
fi
if [[ -x "${INSTALL_DIR}/kubeaudit" ]]; then
  fail "kubeaudit binary must not be installed after checksum failure"
fi
pass "install-kubeaudit wrong checksum fails before install"

echo "All installer negative tests passed."
