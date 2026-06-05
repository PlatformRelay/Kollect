#!/usr/bin/env bash
# Fail if CHANGELOG.md is stale relative to git history and hack/release/cliff.toml.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

CLIFF="${GIT_CLIFF_BIN:-${ROOT}/bin/git-cliff}"
if [[ ! -x "${CLIFF}" ]]; then
  echo "verify-changelog: installing git-cliff into bin/" >&2
  bash hack/install-git-cliff.sh v2.13.1 bin/git-cliff
  CLIFF="${ROOT}/bin/git-cliff"
fi

CLIFF_CONFIG="${CLIFF_CONFIG:-hack/release/cliff.toml}"

scratch="$(mktemp)"
trap 'rm -f "${scratch}"' EXIT

"${CLIFF}" --config "${CLIFF_CONFIG}" -o "${scratch}"

if ! diff -u CHANGELOG.md "${scratch}"; then
  echo "verify-changelog: CHANGELOG.md drift — run 'task changelog:write' and commit" >&2
  exit 1
fi

echo "verify-changelog: ok"
