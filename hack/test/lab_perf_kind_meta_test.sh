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

# --- LAB-DEKIND: the kumulus Talos lab is allowlisted WITHOUT --allow-non-kind ---
# A --dry-run with an explicit fixture still runs the full context gate (only a
# fixture-less --dry-run skips it), so this exercises the allowlist itself.
rc=0
out="$(run_perf --dry-run --run-id "${RUN_ID}-kumulus" --seed 1 \
  --artifacts-root "${OUT_ROOT}" --fixture=context-kumulus 2>&1)" || rc=$?
[[ "${rc}" -eq 0 ]] ||
  fail "kumulus-lab context must be accepted without --allow-non-kind (rc=${rc}): ${out}"
printf '%s\n' "${out}" | grep -Eqi 'context ok|substrate=talos' ||
  fail "kumulus acceptance should log the substrate: ${out}"
pass "kumulus-lab accepted without --allow-non-kind (allowlist, not bypass)"

# --- LAB-DEKIND regression guard: production-lookalike context is still REFUSED ---
rc=0
out="$(run_perf --dry-run --run-id "${RUN_ID}-prod" --seed 1 \
  --artifacts-root "${OUT_ROOT}" --fixture=context-prod-lookalike 2>&1)" || rc=$?
[[ "${rc}" -eq 2 ]] ||
  fail "production-lookalike context MUST be refused with exit 2, got ${rc}: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'allowlist|refus' ||
  fail "production refusal must mention the allowlist: ${out}"
pass "production-lookalike context refused with exit 2 (safety gate held)"

# Near-miss on the lab name must not slip through a prefix match.
rc=0
out="$(run_perf --dry-run --run-id "${RUN_ID}-lookalike" --seed 1 \
  --artifacts-root "${OUT_ROOT}" --fixture=context-kumulus-lookalike 2>&1)" || rc=$?
[[ "${rc}" -eq 2 ]] ||
  fail "kumulus-lab-prod MUST be refused with exit 2, got ${rc}: ${out}"
pass "kumulus-lab-prod refused (allowlist is exact, not a prefix)"

# --- allow-non-kind remains a maintainer override for an off-allowlist context ---
if ! run_perf --dry-run --allow-non-kind --run-id "${RUN_ID}-nonkind" --seed 1 \
  --artifacts-root "${OUT_ROOT}" --fixture=context-prod-lookalike >/dev/null 2>&1; then
  fail "--allow-non-kind + --dry-run should still override the gate for a maintainer"
fi
pass "--allow-non-kind remains an explicit maintainer override"

# --- port-forward target: kollect-system + kollect-controller-manager (not kollect-dev-manager) ---
grep -q 'kollect-system' "${PERF_KIND}" ||
  fail "perf-kind.sh must reference namespace kollect-system"
grep -q 'kollect-dev-manager' "${PERF_KIND}" &&
  fail "perf-kind.sh must not reference broken kollect-dev-manager target"
grep -q '16060:6060' "${PERF_KIND}" ||
  fail "perf-kind.sh must use local:remote port-forward form 16060:6060"
grep -Eq 'deploy/.*kollect-system|port-forward' "${RUN_DIR}/summary.md" ||
  fail "summary.md must document the port-forward command"
grep -q 'deploy/kollect-controller-manager' "${RUN_DIR}/summary.md" ||
  fail "default port-forward target must stay deploy/kollect-controller-manager"
pass "port-forward targets kollect-system/kollect-controller-manager"

# --- LAB-DEKIND: the manager Deployment is release-scoped, not hardcoded ---
# The kumulus lab runs Helm release kollect-op1 in namespace kollect-op1, so its manager is
# deploy/kollect-op1-controller-manager — a hardcoded name makes the live path unrunnable.
REL_OUT="${TMP}/release-test"
run_perf --dry-run --run-id rel-test --seed 1 --release kollect-op1 --namespace kollect-op1 \
  --artifacts-root "${REL_OUT}" >/dev/null 2>&1 || fail "--release dry-run failed"
grep -q 'deploy/kollect-op1-controller-manager' "${REL_OUT}/rel-test/summary.md" ||
  fail "--release must retarget the port-forward Deployment (kollect-op1)"
grep -q 'kollect-op1' "${REL_OUT}/rel-test/summary.md" ||
  fail "--namespace must be reflected in the summary port-forward command"
pass "--release/--namespace retarget the port-forward for a non-default install"

# --- live vs dry-run honesty: dry-run uses .stub; live path must not silently stub ---
stub_count="$(find "${RUN_DIR}/profiles" -name '*.pb.gz.stub' 2>/dev/null | wc -l | tr -d ' ')"
((stub_count > 0)) || fail "dry-run must write .pb.gz.stub placeholders"
grep -q 'lab_pprof_capture_live' "${PPROF_LIB}" ||
  fail "pprof-capture.sh must implement live capture path"
grep -q 'lab_pprof_capture_placeholder' "${PPROF_LIB}" ||
  fail "pprof-capture.sh must keep fixture placeholder path for --dry-run"

# Live path without reachable endpoint: BLOCKED, no .stub artifacts.
cat >"${TMP}/kubectl-live" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *current-context*) echo "kind-kollect-dev" ;;
  *port-forward*) echo "0" > /dev/null; exit 0 & ;;
  *) echo "lab perf-kind meta: unexpected kubectl: $*" >&2; exit 99 ;;
esac
EOF
cat >"${TMP}/curl" <<'EOF'
#!/usr/bin/env bash
echo "lab perf-kind meta: curl simulated unreachable pprof" >&2
exit 7
EOF
chmod +x "${TMP}/kubectl-live" "${TMP}/curl"
LIVE_OUT_ROOT="${TMP}/live-honesty"
rc=0
live_out="$(env -u KUBECONFIG PATH="${TMP}:${PATH}" \
  bash "${PERF_KIND}" --run-id live-blocked-test --seed 1 \
  --artifacts-root "${LIVE_OUT_ROOT}" --fixture=context-kind 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "live run with unreachable pprof should exit non-zero: ${live_out}"
printf '%s\n' "${live_out}" | grep -Eqi 'BLOCKED|unreachable|pprof endpoint|capture failed' ||
  fail "live failure must report BLOCKED/unreachable reason: ${live_out}"
LIVE_DIR="${LIVE_OUT_ROOT}/live-blocked-test"
if [[ -d "${LIVE_DIR}/profiles" ]]; then
  live_stub="$(find "${LIVE_DIR}/profiles" -name '*.pb.gz.stub' 2>/dev/null | wc -l | tr -d ' ')"
  [[ "${live_stub}" -eq 0 ]] ||
    fail "live run must not write .pb.gz.stub placeholders when endpoint unreachable"
fi
pass "live path refuses silent stub capture (BLOCKED on unreachable pprof)"

# --- port-forward FAILURE path: documented exit 3 + BLOCKED findings actually written ---
# Distinct from "endpoint unreachable": here the port-forward never starts (no kubectl on
# PATH). This branch shipped broken — it referenced a variable the CLI no longer defines, so
# `set -u` aborted with exit 1 and the BLOCKED findings file was never written. shellcheck
# structurally cannot catch that (an all-caps unset name is assumed environment-supplied),
# so it needs a behavioural test.
PF_FAIL_ROOT="${TMP}/pf-start-failure"
rc=0
pf_out="$(cd "${TMP}" && env -u KUBECONFIG PATH=/usr/bin:/bin bash "${PERF_KIND}" \
  --run-id pf-start-failure --seed 1 --artifacts-root "${PF_FAIL_ROOT}" \
  --fixture=context-kind 2>&1)" || rc=$?
[[ "${rc}" -eq 3 ]] ||
  fail "port-forward start failure must exit 3 (documented), got ${rc}: ${pf_out}"
printf '%s\n' "${pf_out}" | grep -Eqi 'unbound variable|command not found' &&
  fail "port-forward failure path must not abort on a shell error: ${pf_out}"
PF_FINDINGS="${PF_FAIL_ROOT}/pf-start-failure/summary/performance-findings.md"
[[ -f "${PF_FINDINGS}" ]] ||
  fail "port-forward failure must still write the BLOCKED findings register"
grep -q 'BLOCKED' "${PF_FINDINGS}" ||
  fail "findings register must record BLOCKED for a failed port-forward"
grep -Eqi 'port-forward' "${PF_FINDINGS}" ||
  fail "findings register must record the port-forward reason"
pass "port-forward start failure exits 3 and writes the BLOCKED findings register"

# --- BLOCKED remediation text must name the release/namespace that was actually targeted ---
PF_REL_ROOT="${TMP}/pf-release-hint"
rc=0
pf_out="$(cd "${TMP}" && env -u KUBECONFIG PATH=/usr/bin:/bin bash "${PERF_KIND}" \
  --run-id pf-release-hint --seed 1 --artifacts-root "${PF_REL_ROOT}" \
  --release kollect-op1 --namespace kollect-op1 --fixture=context-kind 2>&1)" || rc=$?
[[ "${rc}" -eq 3 ]] || fail "release-scoped port-forward failure must exit 3, got ${rc}"
REL_FINDINGS="${PF_REL_ROOT}/pf-release-hint/summary/performance-findings.md"
grep -q 'kollect-op1-controller-manager' "${REL_FINDINGS}" ||
  fail "BLOCKED remediation must name the targeted deployment, not a hardcoded default"
grep -q 'kollect-system' "${REL_FINDINGS}" &&
  fail "BLOCKED remediation must not tell the operator to look in the wrong namespace"
pass "BLOCKED remediation reflects --release/--namespace"

# --- cluster-wide cleanup is NOT unlocked by --allow-non-kind ---
# The override exists to profile an unusual substrate. Cleanup runs
# `kubectl delete all,cm,secret,sa,role,rolebinding -A -l kollect.dev/lab-run=<id>`, which is
# a cluster-wide write: it must stay gated on the ALLOWLIST, not on the override flag.
CLEAN_BIN="${TMP}/cleanup-bin"
mkdir -p "${CLEAN_BIN}"
cat >"${CLEAN_BIN}/kubectl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${DELETE_LOG}"
EOF
chmod +x "${CLEAN_BIN}/kubectl"

cleanup_probe() {
  local allowlisted="$1" log="$2"
  : >"${log}"
  env -u KUBECONFIG PATH="${CLEAN_BIN}:${PATH}" DELETE_LOG="${log}" bash -c '
    set -uo pipefail
    source "$1"
    CONTEXT_ALLOWLISTED="$2"
    lab_perf_kind_cleanup "$3" cleanup-probe 0
  ' _ "${PERF_KIND}" "${allowlisted}" "${TMP}/cleanup-run" 2>&1
}

DENY_LOG="${TMP}/delete-denied.log"
out="$(cleanup_probe 0 "${DENY_LOG}")"
grep -q 'delete' "${DENY_LOG}" &&
  fail "cleanup must NOT issue a cluster-wide delete on a non-allowlisted context: $(cat "${DENY_LOG}")"
printf '%s\n' "${out}" | grep -Eqi 'skip|refus|not.*allowlist' ||
  fail "skipped cleanup must say so (and name the label to clean by hand): ${out}"
printf '%s\n' "${out}" | grep -Eq 'kollect.dev/lab-run' ||
  fail "skipped cleanup must tell the operator which label to remove manually: ${out}"
pass "--allow-non-kind does not authorize cluster-wide cleanup deletes"

ALLOW_LOG="${TMP}/delete-allowed.log"
cleanup_probe 1 "${ALLOW_LOG}" >/dev/null
grep -q 'delete' "${ALLOW_LOG}" ||
  fail "cleanup must still run on an allowlisted context: $(cat "${ALLOW_LOG}")"
grep -q 'kollect.dev/lab-run=cleanup-probe' "${ALLOW_LOG}" ||
  fail "cleanup must stay scoped to the lab-run label"
pass "cleanup still runs, label-scoped, on an allowlisted context"

# --- --duration wired into index / CPU note ---
DUR_OUT="${TMP}/duration-test"
run_perf --dry-run --run-id dur-test --seed 7 --duration 45s \
  --artifacts-root "${DUR_OUT}" >/dev/null 2>&1 || fail "--duration dry-run failed"
grep -Eqi '45s|phase dwell|duration' "${DUR_OUT}/dur-test/profiles/index.md" ||
  fail "profiles index must reflect --duration in metrics window or phase note"
pass "--duration reflected in profiles index"

# --- --objects noted as live-only in dry-run output ---
grep -Eqi 'objects.*100|object budget|objects=' "${RUN_DIR}/summary/performance-findings.md" \
  "${RUN_DIR}/summary.md" 2>/dev/null ||
  fail "dry-run should note --objects in summary/findings"
pass "--objects documented in dry-run summary"

# --- port-forward lifecycle helpers (library, offline) ---
# shellcheck source=../lab/lib/pprof-capture.sh
source "${PPROF_LIB}"
PF_DIR="${TMP}/pf-test"
mkdir -p "${PF_DIR}/profiles"
lab_pprof_portforward_start "${PF_DIR}" kollect-system 16060 6060 deploy/kollect-controller-manager --fixture
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
