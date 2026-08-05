#!/usr/bin/env bash
# LAB-H05: evidence collector must create a DOC-02-capable artifacts/lab/<RUN_ID>/ layout
# offline (no kubectl). Paths must be enough for H06 report + redaction to satisfy
# docs/operator-manual/lab-evidence-bundle.md (manifest, scenario matrix, limitations,
# optional pprof/timings).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COLLECT="${ROOT}/hack/lab/collect-evidence.sh"
LIB="${ROOT}/hack/lab/lib/evidence.sh"

fail() {
  printf 'lab evidence layout: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${COLLECT}" ]] || fail "missing collector CLI: ${COLLECT}"
[[ -x "${COLLECT}" ]] || fail "collector CLI must be executable: ${COLLECT}"
[[ -f "${LIB}" ]] || fail "missing evidence library: ${LIB}"

# shellcheck source=../lab/lib/evidence.sh
source "${LIB}"

# Required DOC-02-capable relative paths under a run directory (stable contract for H06).
required_files=(
  manifest.md
  scenario-matrix.md
  limitations.md
  RETENTION.notes
)
required_dirs=(
  pprof
  timings
)

TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-evidence-meta.XXXXXX")"
cleanup() { rm -rf "${TMP}"; }
trap cleanup EXIT

RUN_ID="dr-fixture-lab-h05"
OUT_ROOT="${TMP}/artifacts/lab"

# --- Collector CLI: fixture / dry-run creates layout under --out-root ---
"${COLLECT}" --run-id "${RUN_ID}" --out-root "${OUT_ROOT}" --dry-run \
  || fail "collect-evidence.sh --dry-run failed"

RUN_DIR="${OUT_ROOT}/${RUN_ID}"
[[ -d "${RUN_DIR}" ]] || fail "expected run directory ${RUN_DIR}"

for rel in "${required_files[@]}"; do
  [[ -f "${RUN_DIR}/${rel}" ]] || fail "missing required file: ${rel}"
done
pass "required DOC-02 stub files present"

for rel in "${required_dirs[@]}"; do
  [[ -d "${RUN_DIR}/${rel}" ]] || fail "missing required directory: ${rel}"
done
pass "optional pprof/timings directories present"

# Manifest stub must name RUN_ID and DOC-02 field anchors H06 can fill.
grep -qF -- "${RUN_ID}" "${RUN_DIR}/manifest.md" ||
  fail "manifest.md must record RUN_ID=${RUN_ID}"
for field in "Schedule" "Started" "Finished" "Product pin" "Cluster topology" "Lab label" "Helm release"; do
  grep -qF -- "${field}" "${RUN_DIR}/manifest.md" ||
    fail "manifest.md missing DOC-02 field anchor: ${field}"
done
pass "manifest.md has DOC-02 field anchors"

# Scenario matrix stub: ID / Verdict / Evidence columns (DOC-02 row shape).
for col in "ID" "Verdict" "Evidence"; do
  grep -qF -- "${col}" "${RUN_DIR}/scenario-matrix.md" ||
    fail "scenario-matrix.md missing column: ${col}"
done
pass "scenario-matrix.md has DOC-02 columns"

# Limitations stub must exist as an explicit section host for H06.
grep -Eqi 'limitation|not claimed' "${RUN_DIR}/limitations.md" ||
  fail "limitations.md must host a limitations / not-claimed section"
pass "limitations.md hosts limitations section"

# Library validate helper must accept the tree the collector built.
lab_evidence_validate_layout "${RUN_DIR}" \
  || fail "lab_evidence_validate_layout rejected a collector-built layout"
pass "lab_evidence_validate_layout accepts fixture tree"

# Redaction scan stub must be callable (H06 deepens); fixture with no secrets → clean.
lab_evidence_redaction_scan_stub "${RUN_DIR}" \
  || fail "lab_evidence_redaction_scan_stub failed on clean fixture"
pass "redaction scan stub callable on clean tree"

# Plant a kubeconfig-shaped fragment; stub must detect / report (simple pattern check).
printf 'clusters:\n- cluster:\n    server: https://example.invalid\n' >"${RUN_DIR}/timings/leaky.txt"
if lab_evidence_redaction_scan_stub "${RUN_DIR}"; then
  fail "redaction scan stub must flag kubeconfig-shaped clusters: blocks"
fi
pass "redaction scan stub flags clusters: pattern"

# Retention notes must mention size or retention (bounded evidence hook).
grep -Eqi 'retention|size|bound' "${RUN_DIR}/RETENTION.notes" ||
  fail "RETENTION.notes must document size/retention bounds"
pass "RETENTION.notes documents bounded retention"

# Default --out-root is artifacts/lab relative to repo when omitted (dry-run under TMP via env).
# Prefer explicit --out-root in CI; still assert CLI help / usage mentions flags.
"${COLLECT}" --help 2>&1 | grep -Eq -- '--run-id' || fail "CLI --help must mention --run-id"
"${COLLECT}" --help 2>&1 | grep -Eq -- '--out-root' || fail "CLI --help must mention --out-root"
"${COLLECT}" --help 2>&1 | grep -Eq -- '--dry-run' || fail "CLI --help must mention --dry-run"
pass "CLI documents --run-id --out-root --dry-run"

# Missing --run-id must fail (no silent invent).
if "${COLLECT}" --out-root "${OUT_ROOT}" --dry-run 2>/dev/null; then
  fail "collect-evidence.sh must require --run-id"
fi
pass "CLI requires --run-id"

echo "All lab evidence layout meta tests passed."
