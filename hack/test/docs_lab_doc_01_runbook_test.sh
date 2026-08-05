#!/usr/bin/env bash
# LAB-DOC-01: adaptive local-lab runbook must document real harness flags,
# isolation, schedules, tier=auto / LIMIT_REACHED, and stay wired in MkDocs.
# Offline only — no live kubectl.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PAGE="${ROOT}/docs/operator-manual/local-lab-runbook.md"
NAV="${ROOT}/mkdocs.yml"
VERIFY="${ROOT}/hack/docs/verify.sh"

fail() {
  printf 'lab-doc-01 runbook: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PAGE}" ]] || fail "${PAGE} is missing"

required_headings=(
  "## Discover an existing cluster"
  "## Isolation contract"
  "## Capacity tiers (\`tier=auto\`)"
  "## Schedules"
  "## Resume by scenario ID"
  "## Harness flags"
  "## Verdicts that are not PASS"
)

for heading in "${required_headings[@]}"; do
  grep -qF -- "${heading}" "${PAGE}" || fail "missing heading: ${heading}"
done
pass "required runbook headings present"

required_terms=(
  "kollect.dev/lab-run"
  "kollect-lab-"
  "hack/lab/run.sh"
  "--schedule"
  "--resume"
  "--tier"
  "--run-id"
  "--keep-lab"
  "--dry-run"
  "quick+sinks"
  "full-lab-day"
  "soak"
  "BLOCKED"
  "LIMIT_REACHED"
  "SKIPPED"
  "plateau"
  "preflight.sh"
  "report.sh"
  "ADR-0707"
  "lab-evidence-bundle"
  "load-test-runbook"
)

for term in "${required_terms[@]}"; do
  grep -qF -- "${term}" "${PAGE}" || fail "missing runbook term: ${term}"
done
pass "required harness/isolation/schedule terms present"

# Never recreate the cluster; discover / reuse existing kubeconfig.
grep -Eqi 'never (create|recreate|destroy).*(cluster)|discover.*(existing|cluster)|existing.*(kubeconfig|cluster)' "${PAGE}" ||
  fail "page must say the harness discovers/reuses an existing cluster and never recreates it"

# full-lab-day / soak refuse until implemented (not silent pass).
grep -Eqi 'full-lab-day.*(refuse|BLOCKED|not implemented)|soak.*(refuse|BLOCKED|not implemented)' "${PAGE}" ||
  fail "page must note full-lab-day/soak refuse/BLOCKED until implemented"

# Non-pass verdicts must never be coerced to PASS.
grep -Eqi 'never.*(coerce|count|summariz|collapse).*PASS|not.*(converted|counted).*PASS|none.*(non-pass|non pass).*PASS' "${PAGE}" ||
  fail "page must state LIMIT_REACHED/BLOCKED/SKIPPED are never coerced to PASS"

# --keep-lab / default cleanup are hints only until live scenario bodies land.
grep -Eqi 'hint|hints only|manual(ly)?|operator must (clean|delete|tear)' "${PAGE}" ||
  fail "page must document cleanup/--keep-lab as hint or manual operator duty"
if grep -Eqi 'Default cleanup \| Tear down labeled|default cleans up|Default tear-down unless' "${PAGE}"; then
  fail "page must not claim automatic teardown of lab namespaces/resources"
fi
# Capacity S/M/L guidance must not oversell automated tier gating in v1.
grep -Eqi 'documented guidance|may no-op|no-op in v1|capacity gating may no-op' "${PAGE}" ||
  fail "tier=auto section must state S/M/L guidance and that --tier may no-op in v1"

pass "cleanup hint honesty + tier no-op language present"

grep -qF -- "Local lab runbook: operator-manual/local-lab-runbook.md" "${NAV}" ||
  fail "local-lab-runbook page is missing from MkDocs navigation"

grep -qF -- "docs_lab_doc_01_runbook_test.sh" "${VERIFY}" ||
  fail "meta-test is not wired into hack/docs/verify.sh"

# Evidence bundle must no longer call harness automation "future".
EVIDENCE="${ROOT}/docs/operator-manual/lab-evidence-bundle.md"
[[ -f "${EVIDENCE}" ]] || fail "${EVIDENCE} is missing"
if grep -Eqi 'Automating that layout is future' "${EVIDENCE}"; then
  fail "lab-evidence-bundle.md still says harness automation is future"
fi
grep -qF -- "hack/lab/" "${EVIDENCE}" || fail "lab-evidence-bundle.md must point at hack/lab/"
grep -Eqi 'ADR-0707|0707-lab-harness' "${EVIDENCE}" ||
  fail "lab-evidence-bundle.md must reference ADR-0707"

pass "mkdocs nav + verify wiring + evidence-bundle currency checks passed"

echo "All LAB-DOC-01 local-lab runbook docs tests passed."
