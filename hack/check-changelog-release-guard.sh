#!/usr/bin/env bash
# Refuse to publish a regenerated CHANGELOG.md that loses a released section.
# SPDX-License-Identifier: MIT
#
# Usage: hack/check-changelog-release-guard.sh <committed.md> <regenerated.md>
#        (run from inside the git repository whose tags decide the verdict)
#
# Background -- incident 2026-08-05. The release runbook (docs/RELEASE.md) lands
# the hand-written `## [X.Y.Z]` header on main FIRST and pushes the vX.Y.Z tag
# afterwards. changelog-sync fires on that push, so git-cliff regenerates from a
# HEAD where the tag does not exist yet and renders those commits as
# `## [Unreleased]`. Commit fbb5196a3 pushed exactly that one-line demotion of
# `## [0.17.0]` back to main; docs launch-truth then resolved the released
# version as 0.16.0 and Docs CI went red on every open PR.
#
# The tag being absent is therefore NOT by itself the anomaly -- it is the
# ordinary release path, and failing on it would red every single release. Tag
# visibility is used as the discriminator instead:
#
#   exit 0  no released section lost           -> safe to commit & push
#   exit 2  the top released section was demoted to [Unreleased] and its tag is
#           not visible yet -> release in flight; refuse to push, warn, and let
#           a later push to main regenerate it once the tag exists
#   exit 1  a released section vanished that cannot be explained that way
#           (its tag IS visible, or it was not the top section, or several
#           sections went missing) -> genuine regression, fail loudly
#   exit 3  usage error
#
# Version tokens are compared as sets, never whole header lines: the header
# carries a date and a compare link that legitimately change on re-render.
set -euo pipefail

usage() {
  printf 'usage: %s <committed-changelog> <regenerated-changelog>\n' "${0##*/}" >&2
  exit 3
}

[[ $# -eq 2 ]] || usage
OLD="$1"
NEW="$2"
[[ -r "${OLD}" ]] || {
  printf 'check-changelog-release-guard: cannot read %s\n' "${OLD}" >&2
  exit 3
}
[[ -r "${NEW}" ]] || {
  printf 'check-changelog-release-guard: cannot read %s\n' "${NEW}" >&2
  exit 3
}
git rev-parse --git-dir >/dev/null 2>&1 || {
  printf 'check-changelog-release-guard: not a git repository (tag visibility is undecidable)\n' >&2
  exit 3
}

# headers FILE -- every `## [token]` in document order, Unreleased included.
headers() {
  sed -n 's/^## \[\([^]]*\)\].*/\1/p' "$1"
}

mapfile -t old_headers < <(headers "${OLD}")
mapfile -t new_headers < <(headers "${NEW}")

old_top="${old_headers[0]-}"
new_top="${new_headers[0]-}"

# Released tokens only -- anything that is not the literal [Unreleased] marker.
released_of() {
  local h
  for h in "$@"; do
    [[ "${h}" == "Unreleased" ]] && continue
    printf '%s\n' "${h}"
  done
}

mapfile -t old_released < <(released_of ${old_headers[@]+"${old_headers[@]}"})
mapfile -t new_released < <(released_of ${new_headers[@]+"${new_headers[@]}"})

contains() {
  local needle="$1" item
  shift
  for item in "$@"; do
    [[ "${item}" == "${needle}" ]] && return 0
  done
  return 1
}

missing=()
for v in ${old_released[@]+"${old_released[@]}"}; do
  contains "${v}" ${new_released[@]+"${new_released[@]}"} || missing+=("${v}")
done

if [[ ${#missing[@]} -eq 0 ]]; then
  printf 'check-changelog-release-guard: ok -- all %d released section(s) survived regeneration\n' \
    "${#old_released[@]}"
  exit 0
fi

# Release in flight: exactly the top released header was demoted to
# [Unreleased], and the tag that would have kept it released is not pushed yet.
if [[ ${#missing[@]} -eq 1 && "${missing[0]}" == "${old_top}" && "${new_top}" == "Unreleased" ]] &&
  ! git show-ref --verify --quiet "refs/tags/v${missing[0]}"; then
  printf '::warning title=changelog-sync::Release v%s appears to be in flight: CHANGELOG.md still has the hand-written "## [%s]" header but tag v%s is not pushed yet, so git-cliff re-rendered it as "## [Unreleased]". Refusing to push that demotion -- CHANGELOG.md is left as committed and the next push to main after v%s lands will regenerate it correctly.\n' \
    "${missing[0]}" "${missing[0]}" "${missing[0]}" "${missing[0]}"
  exit 2
fi

printf '::error title=changelog-sync::Regenerated CHANGELOG.md drops released section(s): %s. This is the fbb5196a3 class of regression -- refusing to push. Inspect git-cliff output and hack/release/cliff.toml before syncing again.\n' \
  "${missing[*]}"
for v in "${missing[@]}"; do
  if git show-ref --verify --quiet "refs/tags/v${v}"; then
    printf 'check-changelog-release-guard: %s is missing although tag v%s exists\n' "${v}" "${v}" >&2
  else
    printf 'check-changelog-release-guard: %s is missing and tag v%s does not exist\n' "${v}" "${v}" >&2
  fi
done
exit 1
