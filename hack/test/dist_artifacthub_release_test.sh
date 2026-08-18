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

# repositoryID must be a well-formed UUID. Both the pre-registration placeholder
# (00000000-0000-4000-8000-000000000000) and the real ID pasted from the Artifact
# Hub control panel after registration are valid — this gate must never block its
# own remediation. The "registration still pending" fact is documented, with a
# date, in artifacthub-repo.yml itself and in ADR-0708.
REPOSITORY_ID="$(grep -E '^repositoryID:[[:space:]]*' "${REPO_YML}" | head -1 |
  sed -E 's/^repositoryID:[[:space:]]*//; s/[[:space:]]*$//; s/^["'"'"']//; s/["'"'"']$//')"
[[ -n "${REPOSITORY_ID}" ]] || fail "artifacthub-repo.yml has no repositoryID"
[[ "${REPOSITORY_ID}" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] ||
  fail "artifacthub-repo.yml repositoryID '${REPOSITORY_ID}' is not a well-formed UUID"

pass "artifacthub-repo.yml repositoryID is a well-formed UUID"

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
REPORT_STEP="Report Artifact Hub metadata push outcome"
PUBLISH_STEP="Publish GitHub Release"

oras_idx="$(step_index "${ORAS_STEP}")"
push_idx="$(step_index "${PUSH_STEP}")"
report_idx="$(step_index "${REPORT_STEP}")"
publish_idx="$(step_index "${PUBLISH_STEP}")"

for pair in "${ORAS_STEP}:${oras_idx}" "${PUSH_STEP}:${push_idx}" "${REPORT_STEP}:${report_idx}" "${PUBLISH_STEP}:${publish_idx}"; do
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
[[ "${publish_idx}" -lt "${report_idx}" ]] ||
  fail "'${REPORT_STEP}' (index ${report_idx}) must run AFTER '${PUBLISH_STEP}' (index ${publish_idx})"

# Every step placed after the signed publish must be soft-fail, the reporting
# step included -- a hard-failing reporter would reintroduce exactly the failure
# mode this ordering exists to prevent.
for name in "${ORAS_STEP}" "${PUSH_STEP}" "${REPORT_STEP}"; do
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

# DIST-AH-02: the `ignore` regex is the only thing standing between Artifact Hub and a
# recurring "image not found" scan-failure mail. Assert it BEHAVIOURALLY -- match the
# pattern against real tags -- because a grep for the literal string would keep passing
# after someone widened it to '^.*$' and silently delisted the whole chart.
IGNORE_RE="$(yq eval '.ignore[] | select(.name == "kollect") | .version' "${REPO_YML}")"
[[ -n "${IGNORE_RE}" && "${IGNORE_RE}" != "null" ]] ||
  fail "artifacthub-repo.yml must carry an ignore entry for the kollect package"

matches_ignore() { printf '%s\n' "$1" | grep -Eq "${IGNORE_RE}"; }

# Must be ignored. v-prefixed tags are the controller IMAGE, not charts. Bare 0.9.0-0.13.0
# are charts that hardcode `image.tag: latest`, a tag that was never published, so Artifact
# Hub's scanner fails on every one of them (0.12.0 additionally IS an image, not a chart).
for ignored in \
  v0.9.0 v0.13.0 v0.18.0 v1.0.0 \
  0.9.0 0.10.0 0.11.0 0.12.0 0.13.0; do
  matches_ignore "${ignored}" ||
    fail "ignore regex '${IGNORE_RE}' must cover ${ignored}"
done
pass "ignore regex covers the v-prefixed image tags and the broken 0.9.0-0.13.0 charts"

# Must NOT be ignored: every chart from 0.14.0 on defaults image.tag to v<appVersion>
# and is genuinely installable. Delisting these would hide the live releases.
for listed in \
  0.14.0 0.15.0 0.16.0 0.17.0 0.18.0 0.19.0 1.0.0 0.1.0 0.130.0; do
  if matches_ignore "${listed}"; then
    fail "ignore regex '${IGNORE_RE}' must NOT cover ${listed} -- that chart is installable and belongs on Artifact Hub"
  fi
done
pass "ignore regex leaves 0.14.0+ charts listed"

echo "All dist Artifact Hub release tests passed."
