#!/usr/bin/env bash
# LAB-DOC-05: function→scenario + recovery matrix must be traceable to the
# quick+sinks registry, mark H08/injector gaps honestly, and stay wired in
# MkDocs + docs verify. Offline only — no live kubectl.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PAGE="${ROOT}/docs/operator-manual/lab-scenario-matrix.md"
NAV="${ROOT}/mkdocs.yml"
VERIFY="${ROOT}/hack/docs/verify.sh"
INDEX="${ROOT}/docs/operator-manual/index.md"
RUNBOOK="${ROOT}/docs/operator-manual/local-lab-runbook.md"
EVIDENCE="${ROOT}/docs/operator-manual/lab-evidence-bundle.md"
SCHEDULE="${ROOT}/hack/lab/schedules/quick+sinks.json"

fail() {
  printf 'lab-doc-05 scenario-matrix: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PAGE}" ]] || fail "${PAGE} is missing"
[[ -f "${SCHEDULE}" ]] || fail "${SCHEDULE} is missing"

required_headings=(
  "## Function to scenario coverage"
  "## Failure and recovery coverage"
  "## Registry ID index"
  "## Assurance gaps"
)

for heading in "${required_headings[@]}"; do
  grep -qF -- "${heading}" "${PAGE}" || fail "missing heading: ${heading}"
done
pass "required headings present"

# Every quick+sinks scenario + excluded ID must appear on the page.
mapfile -t IDS < <(python3 - <<PY
import json
from pathlib import Path
qs = json.loads(Path("${SCHEDULE}").read_text())
ids = sorted({s["id"] for s in qs.get("scenarios", [])} | {e["id"] for e in qs.get("excluded", [])})
print("\n".join(ids))
PY
)

missing=0
for id in "${IDS[@]}"; do
  if ! grep -qF -- "${id}" "${PAGE}"; then
    printf 'missing scenario id: %s\n' "${id}" >&2
    missing=1
  fi
done
[[ "${missing}" -eq 0 ]] || fail "page must mention every quick+sinks registry id"
pass "all quick+sinks registry ids present (${#IDS[@]})"

# H08 / failure injection honesty — gaps, never green cells for unsupported recovery
grep -Eqi 'H08|failure injector|LAB-H08' "${PAGE}" || fail "page must name H08 / failure injector deferral"
grep -Eqi 'assurance gap|explicit gap|not covered|gap until' "${PAGE}" ||
  fail "page must mark unsupported recovery as assurance gaps"
# Forbidden: calling failure-injection areas PASS/covered without gap language nearby is hard;
# require that sink outage / RBAC loss rows are not labeled as green PASS coverage.
if grep -E '(sink outage|RBAC loss|malformed extraction|operator restart|backpressure|oversized payload)' "${PAGE}" |
  grep -Eqi '\|[[:space:]]*PASS[[:space:]]*\|'; then
  fail "failure-mode rows must not use bare PASS cells (use gap / PASS_WITH_LIMITATION / signals)"
fi
pass "H08 and failure-mode honesty present"

# Evidence / ADR forward links
grep -Eqi 'artifacts/lab|lab-evidence-bundle|LAB-DOC-02' "${PAGE}" ||
  fail "page must link evidence artifacts / DOC-02 contract"
grep -Eqi 'ADR-0707|0707-lab-harness|REQUIREMENTS|ADR-0' "${PAGE}" ||
  fail "page must link requirements/ADRs"
pass "evidence and ADR cross-links present"

# Nav + verify
grep -qF 'operator-manual/lab-scenario-matrix.md' "${NAV}" ||
  fail "mkdocs.yml must list lab-scenario-matrix.md"
grep -qF 'docs_lab_doc_05_scenario_matrix_test.sh' "${VERIFY}" ||
  fail "hack/docs/verify.sh must invoke docs_lab_doc_05_scenario_matrix_test.sh"
pass "mkdocs nav and verify.sh wiring present"

# Cross-links from index + runbook or evidence
grep -Eqi 'lab-scenario-matrix' "${INDEX}" || fail "operator-manual/index.md must link lab-scenario-matrix"
if ! grep -Eqi 'lab-scenario-matrix' "${RUNBOOK}" && ! grep -Eqi 'lab-scenario-matrix' "${EVIDENCE}"; then
  fail "local-lab-runbook.md or lab-evidence-bundle.md must cross-link lab-scenario-matrix"
fi
grep -Eqi 'local-lab-runbook|lab-evidence-bundle|0707-lab-harness' "${PAGE}" ||
  fail "scenario matrix must cross-link runbook/evidence/ADR-0707"
pass "cross-links present"

printf 'All LAB-DOC-05 scenario matrix docs tests passed.\n'
