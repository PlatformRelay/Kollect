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

command -v yq >/dev/null 2>&1 ||
  fail "yq (mikefarah/yq v4) is required to inspect the release job step graph"

step_index() {
  yq eval ".jobs.release.steps | to_entries | map(select(.value.name == \"$1\")) | .[0].key" "${WORKFLOW}"
}

step_soft_fail() {
  yq eval ".jobs.release.steps[] | select(.name == \"$1\") | .[\"continue-on-error\"]" "${WORKFLOW}"
}

ORAS_STEP="Set up oras"
PUSH_STEP="Push Artifact Hub metadata"
PUBLISH_STEP="Publish GitHub Release"

oras_idx="$(step_index "${ORAS_STEP}")"
push_idx="$(step_index "${PUSH_STEP}")"
publish_idx="$(step_index "${PUBLISH_STEP}")"

for pair in "${ORAS_STEP}:${oras_idx}" "${PUSH_STEP}:${push_idx}" "${PUBLISH_STEP}:${publish_idx}"; do
  name="${pair%:*}"
  idx="${pair##*:}"
  [[ -n "${idx}" && "${idx}" != "null" ]] ||
    fail "release job has no step named '${name}' (step-graph assertions below would pass vacuously)"
done

# ADR-0708: "soft-fail hub jobs preserve tag-release success". Artifact Hub
# repository metadata is discoverability only, so it must never be able to abort
# the release job after the images and chart are already pushed and cosign-signed
# (which would leave a tag with signed GHCR artifacts but no GitHub Release).
[[ "${publish_idx}" -lt "${oras_idx}" ]] ||
  fail "'${ORAS_STEP}' (index ${oras_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"
[[ "${publish_idx}" -lt "${push_idx}" ]] ||
  fail "'${PUSH_STEP}' (index ${push_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"

for name in "${ORAS_STEP}" "${PUSH_STEP}"; do
  [[ "$(step_soft_fail "${name}")" == "true" ]] ||
    fail "'${name}' must declare continue-on-error: true so an Artifact Hub failure cannot fail the release job"
done

pass "Artifact Hub steps run after the GitHub Release publish and are soft-fail"

# Soft-fail must not be silent. `continue-on-error` pins steps.<id>.conclusion to
# "success", so the reporting step has to read steps.<id>.outcome.
grep -Fq 'steps.push-artifacthub-metadata.outcome' "${WORKFLOW}" ||
  fail "release workflow must read steps.push-artifacthub-metadata.outcome (conclusion is always 'success' under continue-on-error)"
grep -Fq '::warning title=Artifact Hub metadata not published' "${WORKFLOW}" ||
  fail "a failed Artifact Hub push must emit a ::warning:: annotation"
grep -Fq 'Artifact Hub repository metadata was NOT published' "${WORKFLOW}" ||
  fail "a failed Artifact Hub push must write a GITHUB_STEP_SUMMARY line"

pass "failed Artifact Hub push is reported via ::warning:: and job summary"

echo "All dist Artifact Hub release tests passed."
