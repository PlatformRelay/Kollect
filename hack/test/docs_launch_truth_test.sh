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
  sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' CHANGELOG.md | head -1
)"

[[ "${chart_version}" == "${app_version}" ]] ||
  fail "chart version ${chart_version} does not match appVersion ${app_version}"
[[ -n "${released_version}" ]] || fail "changelog has no released version"

truth_files=(
  README.md
  docs/index.md
  docs/ROADMAP.md
  docs/roadmap/planned-features.md
  docs/RELEASE.md
  docs/_snippets/pre-beta.md
  docs/adr/README.md
  overrides/main.html
)

if grep -Eni \
  'until the first release candidate|v0\.6\.0 cut|v0\.6\.0.*next|v0\.7\.x hardening|v0\.7\.0-rc\.1 is available|frozen until v0\.7|build-order phases|validated in CI|validated in nightly load tests|publish (a|the) security architecture|after the active implementation lands|design still in flight' \
  "${truth_files[@]}"; then
  fail "obsolete release or maturity copy remains"
fi

grep -qF -- '**Pre-1.0.**' README.md ||
  fail "README does not declare the pre-1.0 compatibility model"
grep -qF -- '**Pre-1.0 API**' docs/_snippets/pre-beta.md ||
  fail "shared maturity snippet does not use the pre-1.0 model"
grep -Eq \
  "^\\*\\*Last verified:\\*\\* [0-9]{4}-[0-9]{2}-[0-9]{2} against \\*\\*v${released_version}\\*\\*\\." \
  docs/ROADMAP.md ||
  fail "roadmap Last verified line does not identify released v${released_version}"
grep -Eq \
  "^\\*\\*Last verified:\\*\\* [0-9]{4}-[0-9]{2}-[0-9]{2} against \\*\\*v${released_version}\\*\\*\\." \
  docs/roadmap/planned-features.md ||
  fail "planned-features Last verified line does not identify released v${released_version}"
if ! grep -qF -- "releases/tag/v${released_version}" overrides/main.html ||
  ! grep -qF -- "<strong>v${released_version}</strong>" overrides/main.html; then
  fail "announcement bar does not target released v${released_version}"
fi

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

printf 'docs launch truth: ok (released v%s; chart v%s)\n' \
  "${released_version}" "${chart_version}"
