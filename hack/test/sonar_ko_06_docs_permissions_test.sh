#!/usr/bin/env bash
# Unit test for SEC-04f / KO-06: docs.yaml permissions must be job-scoped,
# not granted workflow-wide (SonarCloud least-privilege rule).
#
# Asserts:
#   (a) no workflow-top-level `permissions:` block grants anything beyond
#       `contents: read` (a block that is absent entirely also passes).
#   (b) every job under `jobs:` defines its own explicit `permissions:` block,
#       so none silently inherits a workflow-wide grant.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/docs.yaml"

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

# --- (a) workflow-top-level permissions must grant nothing beyond contents: read ---
TOP_PERMS="$(yq eval '.permissions' "${WORKFLOW}")"
if [[ "${TOP_PERMS}" != "null" ]]; then
  TOP_KEYS="$(yq eval '.permissions | keys | join(",")' "${WORKFLOW}")"
  if [[ "${TOP_KEYS}" != "contents" ]]; then
    fail "workflow-top-level permissions grants more than contents: read (keys: ${TOP_KEYS})"
  fi
  TOP_CONTENTS="$(yq eval '.permissions.contents' "${WORKFLOW}")"
  if [[ "${TOP_CONTENTS}" != "read" ]]; then
    fail "workflow-top-level permissions.contents must be 'read', got: ${TOP_CONTENTS}"
  fi
fi
pass "workflow-top-level permissions grants nothing beyond contents: read (or is absent)"

# --- (b) every job must define its own explicit permissions block ---
mapfile -t JOBS < <(yq eval '.jobs | keys | .[]' "${WORKFLOW}")
[[ ${#JOBS[@]} -gt 0 ]] || fail "no jobs found in ${WORKFLOW}"

for job in "${JOBS[@]}"; do
  JOB_PERMS="$(yq eval ".jobs.${job}.permissions" "${WORKFLOW}")"
  if [[ "${JOB_PERMS}" == "null" ]]; then
    fail "job '${job}' has no explicit permissions block (would silently inherit a workflow-wide grant)"
  fi
  pass "job '${job}' defines an explicit permissions block"
done

echo "All docs.yaml permissions-scoping tests passed."
