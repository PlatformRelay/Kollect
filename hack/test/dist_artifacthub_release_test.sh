#!/usr/bin/env bash
# DIST-AH-02: release workflow pushes Artifact Hub metadata via oras and keeps DR-FIND-07 guard.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/release.yaml"
REPO_YML="${ROOT}/artifacthub-repo.yml"

fail() {
  printf 'dist artifacthub release: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${WORKFLOW}" ]] || fail "${WORKFLOW} is missing"
[[ -f "${REPO_YML}" ]] || fail "${REPO_YML} is missing"

grep -Fq '00000000-0000-4000-8000-000000000000' "${REPO_YML}" ||
  fail "artifacthub-repo.yml must use placeholder repositoryID"

grep -Fq 'setup-oras' "${WORKFLOW}" ||
  fail "release workflow must set up oras"

grep -Fq 'oras push' "${WORKFLOW}" ||
  fail "release workflow must oras push Artifact Hub metadata"

grep -Fq ':artifacthub.io' "${WORKFLOW}" ||
  fail "release workflow must push the artifacthub.io metadata tag on the chart repo"

grep -Fq 'artifacthub-repo.yml' "${WORKFLOW}" ||
  fail "release workflow must reference artifacthub-repo.yml"

grep -Fq 'Guard against image/chart GHCR tag collision (DR-FIND-07)' "${WORKFLOW}" ||
  fail "release workflow must retain DR-FIND-07 guard"

# oras push must come after the DR-FIND-07 guard step.
oras_line="$(grep -n 'oras push' "${WORKFLOW}" | head -1 | cut -d: -f1)"
guard_line="$(grep -n 'Guard against image/chart GHCR tag collision (DR-FIND-07)' "${WORKFLOW}" | head -1 | cut -d: -f1)"
[[ -n "${oras_line}" && -n "${guard_line}" ]] || fail "could not locate oras push or DR-FIND-07 guard"
(( oras_line > guard_line )) || fail "oras push must run after DR-FIND-07 guard"

pass "release workflow wires oras push after DR-FIND-07 guard"

echo "All dist Artifact Hub release tests passed."
