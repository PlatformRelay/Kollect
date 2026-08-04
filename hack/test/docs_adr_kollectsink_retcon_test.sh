#!/usr/bin/env bash
# DOC-LAUNCH-S02 / DOC-05: ADRs must not present the unified KollectSink CRD as the
# live probe/export kind (or as live CRD surface). Retired name may appear only with
# removed/family-sink framing (or as the Go-only KollectSinkSpec adapter).
# Historical rejection ADRs are out of scope.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'docs adr kollectsink retcon: %s\n' "$*" >&2
  exit 1
}

# Present-tense "Static + probe | KollectSink" table row = unified CRD as live architecture.
if grep -Eq 'Static \+ probe[[:space:]]*\|[[:space:]]*`?KollectSink`?' \
  docs/adr/0202-static-vs-reconciled.md; then
  fail "ADR-0202 still lists unified KollectSink as the live Static+probe kind"
fi

# Sequence / apply language treating KollectSink as the current probe object.
if grep -Eq 'apply KollectSink|KollectSink reconciler|ConnectionVerified` on `KollectSink' \
  docs/adr/0202-static-vs-reconciled.md; then
  fail "ADR-0202 still describes KollectSink as the live connection-probe kind"
fi

# Peers that historically spoke of KollectSink as the shipped public CRD.
peers=(
  docs/adr/0102-prior-art.md
  docs/adr/0202-static-vs-reconciled.md
  docs/adr/0204-namespaced-profiles.md
  docs/adr/0206-api-versioning-conversion.md
  docs/adr/0402-sink-backends-database-kafka.md
  docs/adr/0406-sink-registry.md
  docs/adr/0413-export-interval-scheduling.md
  docs/adr/0601-prometheus-metrics-stub.md
  docs/adr/0602-error-taxonomy.md
)

for peer in "${peers[@]}"; do
  [[ -f "${peer}" ]] || fail "missing ${peer}"

  # Any KollectSink token is a fail unless the *same hit line* carries removal /
  # family-sink / Go-adapter framing (not a soft nearby-paragraph escape).
  # Allow on-line: KollectSinkSpec, unified `KollectSink` + removed/family, ADR-0414 cites.
  set +e
  hits="$(
    grep -nE 'KollectSink' "${peer}" 2>/dev/null |
      grep -Ev 'KollectSinkSpec|unified `KollectSink`|removed|family sink|ADR-0414|not a public kind|Go-only|was removed|no longer' ||
      true
  )"
  set -e
  if [[ -n "${hits}" ]]; then
    printf '%s\n' "${hits}" >&2
    fail "${peer}: still claims unified KollectSink as live product architecture (see hits)"
  fi
done

# Positive: ADR-0202 must describe family sinks as the Static+probe category.
grep -Eq 'KollectSnapshotSink|family sink' docs/adr/0202-static-vs-reconciled.md ||
  fail "ADR-0202 does not frame family sinks as the live probe kinds"

grep -Eq 'ADR-0414|0414-sink-family' docs/adr/0202-static-vs-reconciled.md ||
  fail "ADR-0202 does not cite ADR-0414 for the family-sink replacement"

# Nav must not advertise KollectSink as an active product page.
if grep -Eqi 'KollectSink|kollectsink' mkdocs.yml; then
  fail "mkdocs.yml still navigates KollectSink as active product"
fi

printf 'docs adr kollectsink retcon: ok\n'
