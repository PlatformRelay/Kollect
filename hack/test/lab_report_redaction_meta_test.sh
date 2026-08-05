#!/usr/bin/env bash
# LAB-H06: report generator + redaction gate over DOC-02 evidence trees.
# Offline only — uses hack/lab/testdata/sample-bundle (synthetic).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT="${ROOT}/hack/lab/report.sh"
REDACT_LIB="${ROOT}/hack/lab/lib/redact.sh"
SAMPLE="${ROOT}/hack/lab/testdata/sample-bundle"
GOLDEN="${ROOT}/hack/test/lab_report_schema_golden_test.sh"

fail() {
  printf 'lab report redaction: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${REPORT}" ]] || fail "missing report CLI: ${REPORT}"
[[ -x "${REPORT}" ]] || fail "report CLI must be executable: ${REPORT}"
[[ -f "${REDACT_LIB}" ]] || fail "missing redaction library: ${REDACT_LIB}"
[[ -d "${SAMPLE}" ]] || fail "missing synthetic sample bundle: ${SAMPLE}"

# shellcheck source=../lab/lib/redact.sh
source "${REDACT_LIB}"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-report-redaction.XXXXXX")"
cleanup() { rm -rf "${TMP}"; }
trap cleanup EXIT

# --- Clean synthetic sample must pass the redaction gate ---
lab_redact_scan "${SAMPLE}" \
  || fail "lab_redact_scan failed on clean synthetic sample-bundle"
pass "clean sample-bundle passes redaction scan"

# Copy sample into a writable run dir for report + negative tests
CLEAN_RUN="${TMP}/clean/dr-fixture-lab-h06"
mkdir -p "$(dirname "${CLEAN_RUN}")"
cp -R "${SAMPLE}" "${CLEAN_RUN}"

"${REPORT}" --run-dir "${CLEAN_RUN}" \
  || fail "report.sh failed on clean sample copy"

for rel in summary.md checksums.txt manifest.md scenario-matrix.md limitations.md; do
  [[ -f "${CLEAN_RUN}/${rel}" ]] || fail "report must emit/retain ${rel}"
done
pass "report emits summary.md checksums.txt and DOC-02 markdown"

# summary must be human-shaped for DOC-02 publication path
grep -Eqi 'limitation|not claimed' "${CLEAN_RUN}/summary.md" ||
  fail "summary.md must include limitations / not-claimed wording"
grep -Eqi 'scenario|matrix|verdict' "${CLEAN_RUN}/summary.md" ||
  fail "summary.md must include scenario matrix / verdict content"
grep -qF -- "RUN_ID" "${CLEAN_RUN}/summary.md" ||
  fail "summary.md must mention RUN_ID"
pass "summary.md is DOC-02 publication-shaped"

# checksums.txt: sha256 lines for retained artifacts (at least the DOC-02 files)
grep -Eq '^[a-f0-9]{64}  ' "${CLEAN_RUN}/checksums.txt" ||
  fail "checksums.txt must list sha256 digests"
for name in manifest.md scenario-matrix.md limitations.md summary.md; do
  grep -qF -- " ${name}" "${CLEAN_RUN}/checksums.txt" ||
    fail "checksums.txt missing entry for ${name}"
done
pass "checksums.txt lists DOC-02 artifact digests"

# --- Seeded fake secrets must fail the redaction gate ---
DIRTY_RUN="${TMP}/dirty/dr-fixture-lab-h06-leaky"
mkdir -p "$(dirname "${DIRTY_RUN}")"
cp -R "${SAMPLE}" "${DIRTY_RUN}"

# Seed several DOC-02 "never publish" shapes (synthetic fakes only).
cat >"${DIRTY_RUN}/timings/seeded-leaks.txt" <<'EOF'
# synthetic leak seeds for LAB-H06 redaction negative tests
clusters:
- cluster:
    server: https://example.invalid
users:
- name: fake
password=FAKESECRET_lab_h06_seed
token=ghp_FAKESECRET_lab_h06_seed_token
-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAfakekeymaterialforlabh06only
-----END RSA PRIVATE KEY-----
path=/Users/fakeuser/.kube/config
EOF

if lab_redact_scan "${DIRTY_RUN}"; then
  fail "lab_redact_scan must fail when seeded fake secrets are present"
fi
pass "seeded fake secrets fail redaction scan"

if "${REPORT}" --run-dir "${DIRTY_RUN}" 2>"${TMP}/dirty.err"; then
  fail "report.sh must refuse dirty trees (redaction gate)"
fi
grep -Eqi 'redact|forbidden|secret|publication' "${TMP}/dirty.err" ||
  fail "report failure must mention redaction/publication block: $(cat "${TMP}/dirty.err")"
pass "report.sh redaction gate blocks publication on dirty tree"

# CLI help / required flags
"${REPORT}" --help 2>&1 | grep -Eq -- '--run-dir|--run-id' ||
  fail "report --help must document --run-dir or --run-id"
if "${REPORT}" 2>/dev/null; then
  fail "report.sh must require --run-dir or --run-id"
fi
pass "report CLI documents and requires run identity"

# Thin H09 golden schema check (owned as lab_report_schema_golden_test.sh)
[[ -f "${GOLDEN}" ]] || fail "missing thin H09 golden: ${GOLDEN}"
bash "${GOLDEN}" || fail "lab_report_schema_golden_test.sh failed"
pass "thin H09 schema golden passed via redaction meta"

echo "All lab report redaction meta tests passed."
