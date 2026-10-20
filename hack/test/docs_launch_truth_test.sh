#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'docs launch truth: %s\n' "$*" >&2
  exit 1
}

chart_version="$(sed -n 's/^version: //p' charts/kollect/Chart.yaml | head -1)"
app_version="$(sed -n 's/^appVersion: //p' charts/kollect/Chart.yaml | tr -d '"' | head -1)"
released_version="$(
  sed -n 's/^## \[\([0-9][^]-]*\)\].*/\1/p' CHANGELOG.md | head -1
)"

[[ "${chart_version}" == "${app_version}" ]] ||
  fail "chart version ${chart_version} does not match appVersion ${app_version}"
[[ -n "${released_version}" ]] || fail "changelog has no released version"

# Docs claims track the chart version, not the newest changelog heading. Release prep bumps
# Chart.yaml in a PR; CHANGELOG.md gains the new heading only after the tag, from the post-merge
# sync bot under [skip ci]. Keyed off the changelog, this check verified the prep PR against the
# previous release and the stale claims surfaced later, in an unrelated docs PR (v0.16, v0.18,
# v0.21). Keyed off the chart, the prep PR is where they fail.
#
# The public site claims GA releases only. released_version above skips pre-release headings, and
# while the chart carries one (an RC soak) the docs stay on the last GA. Ordering compares GA cores
# because sort -V ranks 0.22.0-rc.1 above 0.22.0: a GA chart may equal the last release, an RC
# chart must be for a strictly newer one.
chart_core="${chart_version%%-*}"
newest="$(printf '%s\n%s\n' "${released_version}" "${chart_core}" | sort -V | tail -1)"
if [[ "${chart_version}" == *-* ]]; then
  [[ "${newest}" == "${chart_core}" && "${chart_core}" != "${released_version}" ]] ||
    fail "chart version ${chart_version} is older than released v${released_version}"
  target_version="${released_version}"
else
  [[ "${newest}" == "${chart_core}" ]] ||
    fail "chart version ${chart_version} is older than released v${released_version}"
  target_version="${chart_version}"
fi

truth_files=(
  README.md
  docs/index.md
  docs/ROADMAP.md
  docs/roadmap/planned-features.md
  docs/RELEASE.md
  docs/_snippets/pre-beta.md
  docs/adr/README.md
  overrides/main.html
  docs/operator-manual/index.md
  docs/operator-manual/upgrading.md
  docs/operator-manual/production-checklist.md
  docs/operator-manual/troubleshooting.md
  docs/PLATFORM-DECISIONS.md
  docs/REQUIREMENTS.md
  docs/operator-manual/stability.md
  docs/getting-started/install.md
  SECURITY.md
)

if grep -Eni \
  'until the first release candidate|v0\.6\.0 cut|v0\.6\.0.*next|v0\.7\.x hardening|v0\.7\.0-rc\.1 is available|frozen until v0\.7|build-order phases|validated in CI|validated in nightly load tests|publish (a|the) security architecture|after the active implementation lands|design still in flight|may change without notice|no external adopters' \
  "${truth_files[@]}"; then
  fail "obsolete release or maturity copy remains"
fi

# The public-alpha stability policy has one canonical page. Every front-door page points at it:
# README and SECURITY.md by a repository link, the docs pages through the shared snippet, so a
# hand-written notice that drifts from the policy cannot replace it unnoticed.
stability_page=docs/operator-manual/stability.md
[[ -f "${stability_page}" ]] || fail "stability and support page ${stability_page} is missing"
for f in README.md SECURITY.md; do
  grep -qF -- "](${stability_page})" "${f}" || fail "${f} does not link ${stability_page}"
done
grep -qF -- '](/operator-manual/stability.md)' docs/_snippets/pre-beta.md ||
  fail "shared maturity snippet does not link the stability page"
for f in docs/index.md docs/ROADMAP.md docs/operator-manual/upgrading.md; do
  grep -qxF -- '--8<-- "pre-beta.md"' "${f}" || fail "${f} does not include the shared maturity snippet"
done

grep -qF -- '**Pre-1.0.**' README.md ||
  fail "README does not declare the pre-1.0 compatibility model"
grep -qF -- '**Pre-1.0 API**' docs/_snippets/pre-beta.md ||
  fail "shared maturity snippet does not use the pre-1.0 model"
grep -Eq \
  "^\\*\\*Last verified:\\*\\* [0-9]{4}-[0-9]{2}-[0-9]{2} against \\*\\*v${target_version}\\*\\*\\." \
  docs/ROADMAP.md ||
  fail "roadmap Last verified line does not identify v${target_version}"
grep -Eq \
  "^\\*\\*Last verified:\\*\\* [0-9]{4}-[0-9]{2}-[0-9]{2} against \\*\\*v${target_version}\\*\\*\\." \
  docs/roadmap/planned-features.md ||
  fail "planned-features Last verified line does not identify v${target_version}"

# The `Last verified` line is not the page's only version claim. A stale
# `## Shipped in vX.Y.Z` heading contradicts it on the same page and used to pass
# green, so a release bump could fix one claim and leave the other rotting. Both
# now derive from the same ${target_version}.
#
# Matched at any heading depth and without assuming a `v` prefix, so demoting the
# heading to `###` or dropping the `v` cannot smuggle a stale version past it.
# Every match must be the exact released h2; checking all of them, not the first,
# is the point -- a correct heading above a stale one must not shield it.
stale_shipped="$(
  grep -E '^#{2,} Shipped in ' docs/ROADMAP.md |
    grep -vxF "## Shipped in v${target_version}" || true
)"
if [[ -n "${stale_shipped}" ]]; then
  printf 'docs launch truth: offending roadmap heading(s):\n%s\n' "${stale_shipped}" >&2
  fail "roadmap 'Shipped in' heading does not read '## Shipped in v${target_version}'"
fi
# The other direction: no heading at all is the vacuous green, where deleting the
# claim rather than updating it would otherwise silence the check.
grep -qxF "## Shipped in v${target_version}" docs/ROADMAP.md ||
  fail "roadmap has no '## Shipped in v${target_version}' heading"
if ! grep -qF -- "releases/tag/v${target_version}" overrides/main.html ||
  ! grep -qF -- "<strong>v${target_version}</strong>" overrides/main.html; then
  fail "announcement bar does not target v${target_version}"
fi
# The install guide pins the chart it tells users to install; the pin moves with the release.
grep -qF -- "charts/kollect --version ${target_version} " docs/getting-started/install.md ||
  fail "install guide does not pin the chart to --version ${target_version}"

if grep -En \
  '^### (BigQuery sink|NATS event sink)|KollectClusterSink|Hub federated mTLS|Git sink.*Confluence' \
  docs/roadmap/planned-features.md; then
  fail "shipped or rejected architecture remains in the forward-looking backlog"
fi

grep -qF -- '| Container image (pipeline CLI) |' docs/RELEASE.md ||
  fail "release outputs omit the pipeline CLI image"
grep -qF -- "kollect-pipeline@\${PIPELINE_DIGEST}" docs/RELEASE.md ||
  fail "release verification omits the pipeline CLI image"

pipeline_status="$(
  awk -F'|' 'index($0, "[0801]"){gsub(/^[[:space:]]+|[[:space:]]+$/, "", $4); print $4}' \
    docs/adr/README.md
)"
[[ "${pipeline_status}" == "Accepted" ]] ||
  fail "ADR-0801 index status is ${pipeline_status:-missing}, expected Accepted"

printf 'docs launch truth: ok (docs target v%s; last released v%s)\n' \
  "${target_version}" "${released_version}"
