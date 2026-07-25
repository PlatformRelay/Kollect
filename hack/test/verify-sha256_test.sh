#!/usr/bin/env bash
# Unit tests for hack/lib/verify-sha256.sh (AUD-SEC-03).
# Includes a wrong-checksum negative case that must fail closed before any extract.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=../lib/verify-sha256.sh
source "${ROOT}/hack/lib/verify-sha256.sh"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

# --- positive: matching SHA256 ---
printf 'trusted-payload\n' >"${TMP}/good.bin"
GOOD_SHA="$(sha256sum "${TMP}/good.bin" | awk '{print $1}')"
if ! verify_sha256 "${TMP}/good.bin" "${GOOD_SHA}"; then
  fail "expected matching checksum to succeed"
fi
pass "matching checksum succeeds"

# --- negative: wrong checksum must fail closed ---
WRONG_SHA="0000000000000000000000000000000000000000000000000000000000000000"
if verify_sha256 "${TMP}/good.bin" "${WRONG_SHA}" 2>"${TMP}/err.txt"; then
  fail "wrong checksum must not succeed"
fi
if ! grep -q 'checksum mismatch' "${TMP}/err.txt"; then
  fail "wrong checksum must report 'checksum mismatch'; got: $(cat "${TMP}/err.txt")"
fi
pass "wrong checksum fails closed"

# --- negative: empty expected digest ---
if verify_sha256 "${TMP}/good.bin" "" 2>"${TMP}/err-empty.txt"; then
  fail "empty expected checksum must not succeed"
fi
if ! grep -q 'expected sha256 is empty' "${TMP}/err-empty.txt"; then
  fail "empty digest must report clearly; got: $(cat "${TMP}/err-empty.txt")"
fi
pass "empty expected checksum fails closed"

# --- install-script negative: OVERRIDE forces mismatch before extract ---
# Use a tiny local "archive" and point an installer verify path via the shared helper
# (install scripts call verify_sha256 before tar).
printf 'not-a-real-tarball\n' >"${TMP}/fake.tgz"
FAKE_SHA="$(sha256sum "${TMP}/fake.tgz" | awk '{print $1}')"
EXTRACT_MARKER="${TMP}/extracted"
mkdir -p "${EXTRACT_MARKER}"
# Simulate installer order: verify then extract. Wrong override must skip extract.
if verify_sha256 "${TMP}/fake.tgz" "${WRONG_SHA}" 2>/dev/null; then
  fail "override wrong checksum must fail before extract"
fi
# Prove extract was not reached by only extracting on success path.
if verify_sha256 "${TMP}/fake.tgz" "${FAKE_SHA}"; then
  # Success path would extract; marker file proves we only extract after verify.
  printf 'extracted\n' >"${EXTRACT_MARKER}/ran"
fi
[[ -f "${EXTRACT_MARKER}/ran" ]] || fail "success path should extract after verify"
# Re-run wrong checksum and ensure we do not touch a second marker.
rm -f "${EXTRACT_MARKER}/ran" "${EXTRACT_MARKER}/should-not-exist"
if verify_sha256 "${TMP}/fake.tgz" "${WRONG_SHA}" 2>/dev/null; then
  printf 'bad\n' >"${EXTRACT_MARKER}/should-not-exist"
fi
[[ ! -f "${EXTRACT_MARKER}/should-not-exist" ]] || fail "failed verify must not extract"
pass "failed verify does not extract"

echo "All verify-sha256 tests passed."
