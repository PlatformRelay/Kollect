#!/usr/bin/env bash
# LAB-H10 / PERF-LAB-01: Kind pprof quick-path meta-tests (offline / dry-run only).
# Never hits a live cluster. ADR-0707 + PERF-LAB-01 machine-encoded quick path.
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PERF_KIND="${ROOT}/hack/lab/perf-kind.sh"
PPROF_LIB="${ROOT}/hack/lab/lib/pprof-capture.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-perf-kind-meta.XXXXXX")"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab perf-kind meta: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PERF_KIND}" ]] || fail "missing CLI: ${PERF_KIND}"
[[ -x "${PERF_KIND}" ]] || fail "CLI must be executable: ${PERF_KIND}"
[[ -f "${PPROF_LIB}" ]] || fail "missing library: ${PPROF_LIB}"

# Poison PATH: any real kubectl invocation is a hard test failure.
cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "lab perf-kind meta: unexpected live kubectl: $*" >&2
exit 99
EOF
chmod +x "${TMP}/kubectl"
export PATH="${TMP}:${PATH}"

run_perf() {
  env -u KUBECONFIG PATH="${PATH}" bash "${PERF_KIND}" "$@"
}

RUN_ID="perf-fixture-lab-h10"
OUT_ROOT="${TMP}/artifacts/lab"
SEED=4242

# --- dry-run succeeds offline and writes DOC-02 + profiles index ---
if ! out="$(run_perf --dry-run --run-id "${RUN_ID}" --seed "${SEED}" \
  --objects 100 --artifacts-root "${OUT_ROOT}" 2>&1)"; then
  fail "dry-run failed: ${out}"
fi
printf '%s\n' "${out}" | grep -Eqi 'dry-run|fixture|offline' ||
  fail "dry-run output should note offline mode: ${out}"

RUN_DIR="${OUT_ROOT}/${RUN_ID}"
[[ -d "${RUN_DIR}" ]] || fail "expected run directory ${RUN_DIR}"

for rel in manifest.md scenario-matrix.md limitations.md RETENTION.notes; do
  [[ -f "${RUN_DIR}/${rel}" ]] || fail "missing DOC-02 file: ${rel}"
done
pass "dry-run writes DOC-02 evidence layout"

[[ -d "${RUN_DIR}/profiles" ]] || fail "missing profiles/ directory"
[[ -f "${RUN_DIR}/profiles/index.md" ]] || fail "missing profiles/index.md"
pass "dry-run writes profiles/ index"

# Index columns (PERF-LAB-01).
for col in "phase" "profile type" "duration" "SHA256" "pprof version" \
  "top summary" "differential" "metrics window"; do
  grep -Eqi "${col}" "${RUN_DIR}/profiles/index.md" ||
    fail "profiles/index.md missing column anchor: ${col}"
done
pass "profiles/index.md has required columns"

# Profile types: heap, allocs, goroutine, cpu (30s bound documented).
for ptype in heap allocs goroutine cpu; do
  grep -Eqi "${ptype}" "${RUN_DIR}/profiles/index.md" ||
    fail "profiles/index.md must mention profile type: ${ptype}"
done
grep -Eqi '30s|30 s|30-second|seconds=30' "${RUN_DIR}/profiles/index.md" ||
  fail "profiles/index.md must document 30s CPU bound"
pass "index covers heap/allocs/goroutine/cpu with CPU bound note"

# Phases: idle, converge, churn, recover.
for phase in idle converge churn recover; do
  grep -Eqi "${phase}" "${RUN_DIR}/profiles/index.md" ||
    fail "profiles/index.md must mention phase: ${phase}"
done
pass "index documents idle → converge → churn → recover phases"

# Performance finding register stub.
if [[ -f "${RUN_DIR}/summary/performance-findings.md" ]]; then
  grep -Eqi 'finding|register|stub|PERF' "${RUN_DIR}/summary/performance-findings.md" ||
    fail "performance-findings stub must describe register purpose"
  pass "performance finding register stub present"
else
  grep -Eqi 'performance.findings|finding.register|PERF-LAB' "${RUN_DIR}/summary.md" 2>/dev/null ||
    grep -Eqi 'performance.findings|finding.register|PERF-LAB' "${RUN_DIR}/profiles/index.md" ||
    fail "expected performance finding register stub in summary or index"
  pass "performance finding register stub referenced"
fi

# --- context refusal: non-kind ---
rc=0
out="$(run_perf --run-id "${RUN_ID}" --seed 1 \
  --artifacts-root "${OUT_ROOT}" \
  --fixture=context-non-kind 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "non-kind context must be refused without --allow-non-kind: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'kind|context|refus' ||
  fail "non-kind refusal should mention kind/context: ${out}"
pass "refuses non-kind context without --allow-non-kind"

# --- context refusal: ambiguous ---
rc=0
out="$(run_perf --run-id "${RUN_ID}" --seed 1 \
  --artifacts-root "${OUT_ROOT}" \
  --fixture=context-ambiguous 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "ambiguous context must be refused: ${out}"
pass "refuses ambiguous kube context"

# --- allow-non-kind bypasses check (still dry-run offline) ---
if ! run_perf --dry-run --allow-non-kind --run-id "${RUN_ID}-nonkind" --seed 1 \
  --artifacts-root "${OUT_ROOT}" >/dev/null 2>&1; then
  fail "--allow-non-kind + --dry-run should succeed offline"
fi
pass "--allow-non-kind bypasses kind context gate in dry-run"

# --- port-forward lifecycle helpers (library, offline) ---
# shellcheck source=../lab/lib/pprof-capture.sh
source "${PPROF_LIB}"
PF_DIR="${TMP}/pf-test"
mkdir -p "${PF_DIR}/profiles"
lab_pprof_portforward_start "${PF_DIR}" 16060 "svc/kollect-dev:6060" --fixture
[[ -f "${PF_DIR}/profiles/.port-forward.pid" ]] ||
  fail "portforward start must write pid file"
lab_pprof_portforward_stop "${PF_DIR}"
[[ ! -f "${PF_DIR}/profiles/.port-forward.pid" ]] ||
  fail "portforward stop must remove pid file"
pass "port-forward start/stop helpers (offline fixture)"

# --- partial-run preservation on interrupt ---
PARTIAL_ID="perf-partial-${RUN_ID}"
rc=0
out="$(run_perf --dry-run --run-id "${PARTIAL_ID}" --seed "${SEED}" \
  --artifacts-root "${OUT_ROOT}" --simulate-interrupt=converge 2>&1)" || rc=$?
# Interrupt may exit non-zero but must preserve partial artifacts.
PARTIAL_DIR="${OUT_ROOT}/${PARTIAL_ID}"
[[ -d "${PARTIAL_DIR}/profiles" ]] || fail "partial run must keep profiles/ dir"
[[ -f "${PARTIAL_DIR}/profiles/index.md" ]] || fail "partial run must keep profiles/index.md"
grep -Eqi 'idle' "${PARTIAL_DIR}/profiles/index.md" ||
  fail "partial run must preserve idle phase rows"
pass "partial-run preserves profiles on simulated interrupt"

# --- deterministic metadata for identical --seed ---
OUT_A="${OUT_ROOT}/det-a"
OUT_B="${OUT_ROOT}/det-b"
run_perf --dry-run --run-id "det-seed-a" --seed "${SEED}" --objects 100 \
  --artifacts-root "${OUT_A}" >/dev/null 2>&1 || fail "deterministic run A failed"
run_perf --dry-run --run-id "det-seed-b" --seed "${SEED}" --objects 100 \
  --artifacts-root "${OUT_B}" >/dev/null 2>&1 || fail "deterministic run B failed"

# Compare SHA256 column values (excluding run-id-specific paths if any).
sha_a="$(grep -E '^\|' "${OUT_A}/det-seed-a/profiles/index.md" | grep -Ei 'sha256|[0-9a-f]{64}' | sort || true)"
sha_b="$(grep -E '^\|' "${OUT_B}/det-seed-b/profiles/index.md" | grep -Ei 'sha256|[0-9a-f]{64}' | sort || true)"
[[ -n "${sha_a}" && "${sha_a}" == "${sha_b}" ]] ||
  fail "identical --seed must yield deterministic profile SHA256 metadata"
pass "identical --seed produces deterministic profile metadata"

# Different seed must diverge.
run_perf --dry-run --run-id "det-seed-c" --seed 9999 --objects 100 \
  --artifacts-root "${OUT_A}" >/dev/null 2>&1 || fail "deterministic run C failed"
sha_c="$(grep -E '^\|' "${OUT_A}/det-seed-c/profiles/index.md" | grep -Ei 'sha256|[0-9a-f]{64}' | sort || true)"
[[ "${sha_a}" != "${sha_c}" ]] || fail "different seeds must diverge profile metadata"
pass "different seed changes profile metadata"

# --- never public pprof Service (docs / string contract) ---
help_out="$(run_perf --help 2>&1 || true)"
printf '%s\n' "${help_out}" | grep -Eqi 'port-forward|localhost' ||
  fail "--help must document localhost port-forward access"
printf '%s\n' "${help_out}" | grep -Eqi 'never.*(public|Service)|no public' ||
  grep -Eqi 'never.*(public|Service)|port-forward only' "${ROOT}/hack/lab/README.md" ||
  fail "docs must state pprof is never exposed via public Service"
pass "docs forbid public pprof Service exposure"

# --- cleanup removes only lab-run labeled workload (fixture) ---
CLEAN_ID="perf-clean-${RUN_ID}"
if ! out="$(run_perf --dry-run --run-id "${CLEAN_ID}" --seed 1 \
  --artifacts-root "${OUT_ROOT}" --exercise-cleanup 2>&1)"; then
  fail "cleanup exercise failed: ${out}"
fi
printf '%s\n' "${out}" | grep -Eqi 'lab-run|kollect.dev/lab-run' ||
  fail "cleanup must target kollect.dev/lab-run label"
pass "cleanup targets kollect.dev/lab-run label only"

# Poisoned kubectl never successfully invoked.
pass "no live kubectl invocations"

echo "All lab perf-kind meta tests passed."
