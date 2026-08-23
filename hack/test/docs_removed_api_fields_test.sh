#!/usr/bin/env bash
# DOC-SAMPLES-01: docs prose must not name API fields the CRDs do not have.
#
# The YAML schema gate (test/docs/docs_yaml_schema_test.go) validates every YAML
# example under docs/, but a removed field also lives in prose: table rows,
# mermaid edge labels, shell comments, glossary entries. A YAML gate is blind to
# all of those, and a reader copies from a table as readily as from a fence.
#
# `sinkRefs` is the security-adjacent case that motivated this contract. ADR-0414
# split the field per sink family (snapshotSinkRefs / databaseSinkRefs /
# eventSinkRefs); KollectScope and KollectClusterScope never carried a combined
# `sinkRefs`. Structural CRD schemas PRUNE unknown fields instead of rejecting
# them, so a scope written from the old documentation applies cleanly, loses its
# allowlist silently, and internal/scope/scope.go then treats the empty allowlist
# as no restriction at all -- a wider scope than the page depicted, with no error
# anywhere.
#
# Scope of this check: docs/ only. Generated CRD manifests under charts/ and
# config/crd/ carry the token inside kubebuilder-generated descriptions and are
# owned by api/v1alpha1, not by the docs.
#
# Escape hatch: docs/adr/ and docs/rfc/ record decisions as they were taken, so a
# hit there is allowed when the SAME line carries superseding framing. A nearby
# paragraph does not count -- same-line only, matching the convention in
# hack/test/docs_adr_kollectsink_retcon_test.sh.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'docs removed api fields: %s\n' "$*" >&2
  exit 1
}

# Field names the kollect CRDs do not have. Case-sensitive on purpose: the
# leading [^A-Za-z] guard already excludes snapshotSinkRefs / databaseSinkRefs /
# eventSinkRefs, which spell the token with a capital S.
removed_fields=(
  'sinkRefs'
)

# Same-line framing that marks a hit as a deliberate historical record.
framing='ADR-0414|superseded|kollect-doc: superseded|kollect-doc: proposed|no longer|was removed|split per family|per-family'

[[ -d docs ]] || fail "docs/ is missing"

# Build artefacts live under docs/ but are not docs. hack/docs/verify.sh runs
# `task lint:markdown` (which does `npm ci --prefix docs`) BEFORE this gate, so
# docs/node_modules is present by the time this runs: 167 vendored YAML/Markdown
# files that would both satisfy the non-vacuity floor on their own and red the
# build if a dependency happened to vendor one of the tokens. Prune them from the
# corpus, exactly as test/docs/docs_yaml_schema_test.go does.
prune_args=(-name node_modules -prune -o -name 'site' -prune -o -name '.*' -prune -o)

# Non-vacuity: the corpus must actually be searched. docs/ holds ~130 real pages.
page_count="$(
  find docs "${prune_args[@]}" -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' \) -print |
    wc -l | tr -d ' '
)"
if ((page_count < 100)); then
  fail "only ${page_count} authored doc files found under docs/ (build artefacts excluded) -- the search collapsed, so a pass proves nothing"
fi

for token in "${removed_fields[@]}"; do
  set +e
  hits="$(
    find docs "${prune_args[@]}" -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' \) -print0 |
      xargs -0 grep -nE "(^|[^A-Za-z])${token}" /dev/null 2>/dev/null
  )"
  set -e

  [[ -n "${hits}" ]] || continue

  # Live product pages: zero tolerance. There is no framing that makes a removed
  # field correct on a reference page.
  live_hits="$(printf '%s\n' "${hits}" | grep -Ev '^docs/(adr|rfc)/' || true)"
  if [[ -n "${live_hits}" ]]; then
    printf '%s\n' "${live_hits}" >&2
    fail "docs name the removed field '${token}'; use the per-family refs (snapshotSinkRefs / databaseSinkRefs / eventSinkRefs, ADR-0414)"
  fi

  # Decision records: allowed only with superseding framing on the hit line.
  record_hits="$(printf '%s\n' "${hits}" | grep -E '^docs/(adr|rfc)/' | grep -Ev "${framing}" || true)"
  if [[ -n "${record_hits}" ]]; then
    printf '%s\n' "${record_hits}" >&2
    fail "decision records mention '${token}' without same-line superseding framing (cite ADR-0414 on the line)"
  fi
done

# Positive: the replacement surface must actually be documented, so this contract
# cannot be satisfied by deleting the sink-allowlist docs outright.
for page in docs/crds/kollectscope.md docs/crds/kollectclusterscope.md; do
  [[ -f "${page}" ]] || fail "missing ${page}"

  for family in snapshotSinkRefs databaseSinkRefs eventSinkRefs; do
    grep -qF -- "${family}" "${page}" ||
      fail "${page} does not document ${family} -- the per-family allowlist replaced sinkRefs, it did not disappear"
  done
done

printf 'docs removed api fields: ok\n'
