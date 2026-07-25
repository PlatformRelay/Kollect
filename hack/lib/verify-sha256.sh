#!/usr/bin/env bash
# Shared SHA256 verification for downloaded archives (AUD-SEC-03).
# Usage (source): verify_sha256 <file> <expected-sha256>
# Exit 0 on match; exit 1 with a clear mismatch/empty-digest error otherwise.
# shellcheck shell=bash

verify_sha256() {
  local file="${1:?file required}"
  local expected="${2:-}"
  local actual

  if [[ -z "${expected}" ]]; then
    echo "expected sha256 is empty for ${file}" >&2
    return 1
  fi
  if [[ ! -f "${file}" ]]; then
    echo "file not found for checksum verify: ${file}" >&2
    return 1
  fi

  actual="$(sha256sum "${file}" | awk '{print $1}')"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "checksum mismatch for $(basename "${file}")" >&2
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    return 1
  fi
  return 0
}
