#!/usr/bin/env bash
# LAB-DOC-03: public scale tiers must be evidence-addressable. While e2e-nightly
# 10k jobs stay opt-in (ubuntu-latest-8-cores unavailable), docs must not call
# Nightly 10k "Active" / "validated". Offline only — no live kubectl / CI.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNBOOK="${ROOT}/docs/operator-manual/load-test-runbook.md"
PERF="${ROOT}/docs/operator-manual/performance.md"
REQUIREMENTS="${ROOT}/docs/REQUIREMENTS.md"
ADR0603="${ROOT}/docs/adr/0603-performance-scalability.md"
LOCAL_LAB="${ROOT}/docs/operator-manual/local-lab-runbook.md"
EVIDENCE="${ROOT}/docs/operator-manual/lab-evidence-bundle.md"
WORKFLOW="${ROOT}/.github/workflows/e2e-nightly.yaml"
VERIFY="${ROOT}/hack/docs/verify.sh"

fail() {
  printf 'lab-doc-03 scale claims: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${RUNBOOK}" ]] || fail "${RUNBOOK} is missing"
[[ -f "${PERF}" ]] || fail "${PERF} is missing"
[[ -f "${REQUIREMENTS}" ]] || fail "${REQUIREMENTS} is missing"
[[ -f "${ADR0603}" ]] || fail "${ADR0603} is missing"
[[ -f "${WORKFLOW}" ]] || fail "${WORKFLOW} is missing"
[[ -f "${LOCAL_LAB}" ]] || fail "${LOCAL_LAB} is missing"
[[ -f "${EVIDENCE}" ]] || fail "${EVIDENCE} is missing"

# --- Workflow truth: 10k jobs disabled unless workflow_dispatch + run_scale_jobs ---
grep -qF -- 'load-test-10k:' "${WORKFLOW}" || fail "workflow missing load-test-10k job"
grep -qF -- 'scale-envtest-10k:' "${WORKFLOW}" || fail "workflow missing scale-envtest-10k job"
grep -qF -- 'ubuntu-latest-8-cores' "${WORKFLOW}" ||
  fail "workflow must name ubuntu-latest-8-cores runner"

ten_k_disabled=0
if grep -Eq 'if:.*workflow_dispatch.*run_scale_jobs == true' "${WORKFLOW}" &&
  grep -Eqi 'Skipped by default|currently unavailable|ubuntu-latest-8-cores runners unavailable' "${WORKFLOW}"; then
  ten_k_disabled=1
fi

if [[ "${ten_k_disabled}" -eq 1 ]]; then
  pass "workflow 10k jobs are opt-in / disabled by default"

  # --- Forbidden: Active / validated while 10k nightly remains disabled ---
  if grep -E '^\|[[:space:]]*Nightly' "${RUNBOOK}" | grep -Eqi 'Active'; then
    fail "load-test-runbook must not mark Nightly 10k as Active while workflow jobs are disabled"
  fi
  if grep -Eqi '10,?000.*(nightly|8-core).*(active|validated)|(nightly|8-core).*10,?000.*(active|validated)' "${RUNBOOK}"; then
    fail "load-test-runbook must not call 10k nightly Active/validated while jobs are disabled"
  fi
  if grep -Eqi '10k nightly on GH runners is OK' "${RUNBOOK}"; then
    fail "load-test-runbook must not claim 10k nightly on GH runners is OK while jobs are disabled"
  fi
  if grep -E '^\|[[:space:]]*Nightly' "${PERF}" | grep -Eqi 'validated|Active'; then
    fail "performance.md must not call Nightly 10k validated/Active while workflow jobs are disabled"
  fi
  if grep -E '^\|[[:space:]]*Baseline' "${PERF}" | grep -Eqi '\(validated\)|validated'; then
    fail "performance.md must not label 10k+ baseline as validated without named evidence (use planned/unverified)"
  fi
  if grep -Eqi '10,?000\+[[:space:]]*\(validated\)' "${PERF}"; then
    fail "performance.md must not use '10,000+ (validated)' while 10k CI evidence is unavailable"
  fi
  if ! grep -E '^\|[[:space:]]*Nightly' "${RUNBOOK}" | grep -Eqi 'disabled|opt-in|unverified|unavailable|planned'; then
    fail "load-test-runbook Nightly row must be labeled disabled/opt-in/unverified/planned"
  fi

  # REQUIREMENTS.md NFR-PERF-1 and ADR-0603 must not reintroduce validated/Active 10k CI claims.
  if grep -Eqi '10,?000\+[[:space:]]*validated|validated in CI tiers|10,?000.*validated.*CI|CI tiers.*validated' "${REQUIREMENTS}"; then
    fail "REQUIREMENTS.md must not claim 10,000+ validated in CI tiers while 10k nightly jobs are disabled"
  fi
  if grep -E '^\|[[:space:]]*NFR-PERF-1' "${REQUIREMENTS}" | grep -Eqi 'validated'; then
    fail "REQUIREMENTS.md NFR-PERF-1 must not use validated wording for 10k CI while jobs are opt-in"
  fi
  if ! grep -E '^\|[[:space:]]*NFR-PERF-1' "${REQUIREMENTS}" | grep -Eqi 'disabled|opt-in|unverified|unexecuted|AR-02|bounded'; then
    fail "REQUIREMENTS.md NFR-PERF-1 must state bounded CI / disabled 10k / unexecuted 100k (AR-02)"
  fi
  if grep -Eqi '10,?000\+[[:space:]]*\(validated\)|10k baseline validated|validated in CI tiers' "${ADR0603}"; then
    fail "ADR-0603 must not reintroduce 10k validated-in-CI claims while nightly 10k jobs are disabled"
  fi
  if grep -E '^\|[[:space:]]*\*\*Nightly' "${ADR0603}" | grep -Eqi 'Active|validated' | grep -Eiv 'disabled|opt-in|unverified|unexecuted'; then
    fail "ADR-0603 Nightly load row must not claim Active/validated while jobs are opt-in"
  fi
  pass "docs do not claim Active/validated for disabled 10k nightly"
else
  pass "workflow 10k jobs appear enabled — Active/validated wording gate skipped"
fi

# --- AC1: public scale tiers state shape / layer / evidence-or-planned ---
grep -Eqi 'Workload shape|Objects|Collected rows' "${RUNBOOK}" ||
  fail "runbook tier table must state workload shape (objects/rows)"
grep -Eqi 'Execution layer|Where|envtest|in-cluster|cloud' "${RUNBOOK}" ||
  fail "runbook tier table must state execution layer"
grep -Eqi 'Last evidence|Evidence|Status|unverified|disabled|planned' "${RUNBOOK}" ||
  fail "runbook tier table must state last evidence or planned/unverified/disabled"

grep -Eqi 'Workload|synthetic|Collected rows' "${PERF}" ||
  fail "performance.md scale tiers must state workload shape"
grep -Eqi 'Execution|envtest|in-cluster|How to validate|layer' "${PERF}" ||
  fail "performance.md scale tiers must state execution layer"
grep -Eqi 'Evidence|unverified|disabled|planned|opt-in' "${PERF}" ||
  fail "performance.md scale tiers must state evidence status (or planned/unverified/disabled)"
pass "scale tier tables expose shape, layer, and evidence status"

# Bounded task load-test ≠ in-cluster 10k proof.
if ! grep -Eqi 'not (equivalent|equal|a substitute)|≠|does not (prove|satisfy|equal)|synthetic extraction' "${RUNBOOK}" "${PERF}"; then
  fail "docs must distinguish bounded task load-test / synthetic extraction from in-cluster 10k proof"
fi

# 100k remains unexecuted / AR-02 gate.
grep -Eqi '100,?000|100k' "${RUNBOOK}" || fail "runbook must mention 100k design proof"
grep -Eqi 'PLANNED|unexecuted|not yet executed|AR-02' "${RUNBOOK}" ||
  fail "runbook must keep 100k explicitly planned/unexecuted"
pass "100k cloud design remains explicitly unexecuted"

# Laptop / L4.5 lab must not satisfy 100k / two-cluster gate.
for page in "${RUNBOOK}" "${PERF}" "${LOCAL_LAB}" "${EVIDENCE}"; do
  if ! grep -Eqi '100k|two-cluster|cloud (gate|claim)|does not satisfy|not (this|a substitute)|L4\.5|single-host|laptop|Talos' "${page}"; then
    fail "$(basename "${page}") must keep laptop/lab vs 100k gate distinction visible"
  fi
done
pass "laptop/lab evidence does not satisfy 100k gate (cross-pages)"

# --- Cross-links ---
grep -qF -- 'local-lab-runbook' "${RUNBOOK}" || fail "runbook must cross-link local-lab-runbook"
grep -qF -- 'lab-evidence-bundle' "${RUNBOOK}" || fail "runbook must cross-link lab-evidence-bundle"
grep -qF -- 'load-test-runbook' "${LOCAL_LAB}" || fail "local-lab-runbook must link load-test-runbook"
grep -qF -- 'load-test-runbook' "${EVIDENCE}" || fail "lab-evidence-bundle must link load-test-runbook"
grep -qF -- 'performance.md' "${RUNBOOK}" || fail "runbook must link performance.md"
pass "cross-links present"

# --- Verify wiring ---
grep -qF -- 'docs_lab_doc_03_scale_claims_test.sh' "${VERIFY}" ||
  fail "meta-test is not wired into hack/docs/verify.sh"
pass "meta-test wired into hack/docs/verify.sh"

echo "All LAB-DOC-03 scale claim docs tests passed."
