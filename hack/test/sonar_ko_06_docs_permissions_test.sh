#!/usr/bin/env bash
# Unit test for SEC-04f / KO-06 + SEC-05e: docs.yaml permissions must be
# job-scoped (SonarCloud least-privilege rule) AND declare an explicit
# top-level read baseline (OpenSSF Scorecard "Token-Permissions", which flags
# a workflow with no top-level `permissions:` block).
#
# Asserts:
#   (a) the workflow declares an explicit top-level `permissions:` block set to
#       exactly `contents: read` (no longer allowed to be absent — Scorecard
#       Token-Permissions requires a read-only baseline).
#   (b) every job under `jobs:` defines its own explicit `permissions:` block,
#       so none silently inherits (and per-job overrides still win).
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

# --- (a) workflow MUST declare an explicit top-level contents: read baseline ---
TOP_PERMS="$(yq eval '.permissions' "${WORKFLOW}")"
[[ "${TOP_PERMS}" != "null" ]] || fail "workflow has no top-level permissions block (OpenSSF Scorecard Token-Permissions expects an explicit contents: read baseline)"
TOP_KEYS="$(yq eval '.permissions | keys | join(",")' "${WORKFLOW}")"
if [[ "${TOP_KEYS}" != "contents" ]]; then
  fail "workflow-top-level permissions grants more than contents: read (keys: ${TOP_KEYS})"
fi
TOP_CONTENTS="$(yq eval '.permissions.contents' "${WORKFLOW}")"
if [[ "${TOP_CONTENTS}" != "read" ]]; then
  fail "workflow-top-level permissions.contents must be 'read', got: ${TOP_CONTENTS}"
fi
pass "workflow declares an explicit top-level contents: read baseline"

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
