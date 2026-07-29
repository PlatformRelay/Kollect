#!/usr/bin/env bash
# Unit test for SEC-05e: workflow token-permission scoping (OpenSSF Scorecard
# "Token-Permissions"). changelog-sync.yaml is the only kollect workflow that
# genuinely needs `contents: write`, and only in its `sync` job (which commits
# the regenerated CHANGELOG.md back to main). The write grant must therefore be
# scoped to that single job, not granted workflow-wide.
#
# Asserts:
#   (a) top-level `permissions:` is exactly `contents: read` (read-only baseline
#       so no other job inherits write).
#   (b) the `sync` job declares its own `permissions: contents: write`, scoping
#       the write grant to the only writer.
#
# (release.yaml / renovate.yaml keep their job-level contents: write — GitHub
# release+tag creation and update-branch pushes legitimately require it; those
# are accepted residuals and intentionally out of scope here.)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/changelog-sync.yaml"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

if ! command -v yq >/dev/null 2>&1; then
  echo "yq not found; install yq (mikefarah/yq v4) to run this test" >&2
  exit 1
fi

[[ -f "${WORKFLOW}" ]] || fail "workflow file not found: ${WORKFLOW}"

# --- (a) top-level permissions must be exactly contents: read ---
TOP_PERMS="$(yq eval '.permissions' "${WORKFLOW}")"
[[ "${TOP_PERMS}" != "null" ]] || fail "changelog-sync.yaml has no top-level permissions block (expected contents: read)"
TOP_KEYS="$(yq eval '.permissions | keys | join(",")' "${WORKFLOW}")"
if [[ "${TOP_KEYS}" != "contents" ]]; then
  fail "top-level permissions must grant only contents (keys: ${TOP_KEYS})"
fi
TOP_CONTENTS="$(yq eval '.permissions.contents' "${WORKFLOW}")"
if [[ "${TOP_CONTENTS}" != "read" ]]; then
  fail "top-level permissions.contents must be 'read', got: ${TOP_CONTENTS}"
fi
pass "top-level permissions is contents: read"

# --- (b) the sync job must scope contents: write to itself ---
SYNC_WRITE="$(yq eval '.jobs.sync.permissions.contents' "${WORKFLOW}")"
if [[ "${SYNC_WRITE}" != "write" ]]; then
  fail "job 'sync' must declare permissions.contents: write (got: ${SYNC_WRITE})"
fi
pass "job 'sync' scopes contents: write to itself"

echo "All changelog-sync.yaml permissions-scoping tests passed."
