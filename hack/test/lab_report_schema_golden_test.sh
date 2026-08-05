#!/usr/bin/env bash
# LAB-H09 (thin, with H06): golden schema checks for report output against the
# synthetic sample-bundle. Offline only. Invoked from lab_report_redaction_meta_test.sh
# (suite globs *_meta_test.sh only — do not rename without updating the meta caller).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT="${ROOT}/hack/lab/report.sh"
SAMPLE="${ROOT}/hack/lab/testdata/sample-bundle"

fail() {
  printf 'lab report schema golden: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -x "${REPORT}" ]] || fail "missing executable report.sh"
[[ -d "${SAMPLE}" ]] || fail "missing sample-bundle"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-report-golden.XXXXXX")"
cleanup() { rm -rf "${TMP}"; }
trap cleanup EXIT

RUN="${TMP}/dr-fixture-lab-h06"
cp -R "${SAMPLE}" "${RUN}"

"${REPORT}" --run-dir "${RUN}" || fail "report.sh failed on sample copy"

SUMMARY="${RUN}/summary.md"
[[ -f "${SUMMARY}" ]] || fail "summary.md missing after report"

# DOC-02 publication-shaped sections (align with lab-evidence-bundle.md example).
required_summary_anchors=(
  "RUN_ID"
  "Schedule"
  "Product pin"
  "Cluster topology"
  "Limitations"
  "Verdict"
)

for anchor in "${required_summary_anchors[@]}"; do
  grep -qF -- "${anchor}" "${SUMMARY}" ||
    fail "summary.md missing DOC-02 golden anchor: ${anchor}"
done
pass "summary.md has DOC-02 golden anchors"

# Scenario matrix table header shape
grep -Eq '\|[[:space:]]*ID[[:space:]]*\|[[:space:]]*Verdict[[:space:]]*\|' "${SUMMARY}" ||
  fail "summary.md must include ID | Verdict matrix table header"
pass "summary.md scenario matrix table present"

# At least one allowed verdict token appears (fixture uses PASS / SKIPPED / …)
grep -Eq 'PASS(_WITH_LIMITATION)?|SKIPPED|LIMIT_REACHED|FAIL|not triggered' "${SUMMARY}" ||
  fail "summary.md must include at least one DOC-02 verdict token"
pass "summary.md includes DOC-02 verdict tokens"

# Program-level honesty: READY WITH CONDITIONS or explicit limitations wording
grep -Eqi 'READY WITH CONDITIONS|limitations|not claimed' "${SUMMARY}" ||
  fail "summary.md must state program verdict / limitations honesty gate"
pass "summary.md honesty / limitations gate present"

# checksums.txt schema: "digest  relative-path" lines; digest is 64 hex chars
CK="${RUN}/checksums.txt"
[[ -f "${CK}" ]] || fail "checksums.txt missing"
grep -Eq '^[a-f0-9]{64}  [^[:space:]]+' "${CK}" ||
  fail "checksums.txt golden schema: expected '<sha256>  <path>' lines"
# Must not checksum itself into an unstable loop — either absent or last-written separately
if grep -qE 'checksums\.txt$' "${CK}"; then
  fail "checksums.txt must not list itself (unstable digest)"
fi
pass "checksums.txt golden schema ok"

# Manifest / matrix / limitations retained and non-stub enough for publication draft
for f in manifest.md scenario-matrix.md limitations.md; do
  [[ -s "${RUN}/${f}" ]] || fail "${f} must be non-empty after report"
done
grep -qF -- "dr-fixture-lab-h06" "${RUN}/manifest.md" ||
  fail "manifest.md must retain synthetic RUN_ID"
pass "DOC-02 markdown retained with fixture RUN_ID"

echo "All lab report schema golden tests passed."
