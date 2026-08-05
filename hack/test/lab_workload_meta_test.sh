#!/usr/bin/env bash
# LAB-H03: minimal labeled workload helper meta-tests (offline / dry-run only).
# Never hits a live cluster. ADR-0707: deterministic seed, batch render, light churn.
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKLOAD="${ROOT}/hack/lab/workload.sh"
LIB="${ROOT}/hack/lab/lib/workload.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-workload-meta.XXXXXX")"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab workload meta: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${WORKLOAD}" ]] || fail "missing CLI: ${WORKLOAD}"
[[ -x "${WORKLOAD}" ]] || fail "CLI must be executable: ${WORKLOAD}"
[[ -f "${LIB}" ]] || fail "missing library: ${LIB}"

# Poison PATH: any real kubectl invocation is a hard test failure.
cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "lab workload meta: unexpected live kubectl: $*" >&2
exit 99
EOF
chmod +x "${TMP}/kubectl"
export PATH="${TMP}:${PATH}"

run_wl() {
  env -u KUBECONFIG PATH="${PATH}" bash "${WORKLOAD}" "$@"
}

RUN_ID="dr-fixture-lab-h03"
OUT_A="${TMP}/out-a"
OUT_B="${TMP}/out-b"
OUT_C="${TMP}/out-c"
OUT_CHURN="${TMP}/out-churn"

# --- require --run-id ---
rc=0
out="$(run_wl --dry-run --out-dir "${TMP}/need-run-id" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "missing --run-id must fail: ${out}"
pass "CLI requires --run-id"

# --- dry-run batch: writes manifests, no kubectl ---
mkdir -p "${OUT_A}"
if ! out="$(run_wl --run-id "${RUN_ID}" --seed 42 --mode batch --count 6 \
  --dry-run --out-dir "${OUT_A}" 2>&1)"; then
  fail "batch dry-run failed: ${out}"
fi
printf '%s\n' "${out}" | grep -Eqi 'dry-run|fixture|offline|no kubectl' ||
  fail "dry-run output should note offline/no kubectl: ${out}"

manifests=()
while IFS= read -r -d '' f; do
  manifests+=("${f}")
done < <(find "${OUT_A}" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0 | sort -z)

((${#manifests[@]} > 0)) || fail "batch dry-run must write at least one YAML under ${OUT_A}"
pass "batch dry-run writes YAML manifests"

# Every rendered document must carry the lab-run label for this RUN_ID.
label_hits=0
for f in "${manifests[@]}"; do
  if grep -q "kollect.dev/lab-run: ${RUN_ID}" "${f}" ||
    grep -q "kollect.dev/lab-run: \"${RUN_ID}\"" "${f}" ||
    grep -q "kollect.dev/lab-run: '${RUN_ID}'" "${f}"; then
    label_hits=$((label_hits + 1))
  else
    fail "manifest missing kollect.dev/lab-run=${RUN_ID}: ${f}"
  fi
done
((label_hits == ${#manifests[@]})) || fail "not all manifests labeled"
pass "all manifests labeled kollect.dev/lab-run=${RUN_ID}"

# Namespaces follow kollect-lab-<RUN_ID>-* isolation convention when present.
if grep -R -l 'kind: Namespace' "${OUT_A}" >/dev/null 2>&1; then
  grep -R -E "name:[[:space:]]*kollect-lab-${RUN_ID}" "${OUT_A}" >/dev/null ||
    fail "Namespace names should use kollect-lab-<RUN_ID>-* prefix"
  pass "namespace isolation prefix present"
else
  pass "no Namespace objects (ok for minimal batch)"
fi

# Mixed shapes: at least two distinct kinds in batch output (minimal, not full Ubuntu mix).
mapfile -t kind_arr < <(grep -RhE '^kind:' "${OUT_A}" | awk '{print $2}' | sort -u)
kinds="${kind_arr[*]}"
((${#kind_arr[@]} >= 2)) || fail "expected mixed shapes (>=2 kinds), got: ${kinds}"
pass "batch renders mixed shapes (${kinds})"

# --- deterministic seed: same seed → identical tree ---
mkdir -p "${OUT_B}"
run_wl --run-id "${RUN_ID}" --seed 42 --mode batch --count 6 \
  --dry-run --out-dir "${OUT_B}" >/dev/null 2>&1 ||
  fail "second batch dry-run (same seed) failed"

# Compare sorted file contents (ignore directory mtimes).
checksum_tree() {
  local dir="$1"
  (
    cd "${dir}"
    find . -type f \( -name '*.yaml' -o -name '*.yml' \) -print | sort | while read -r rel; do
      # Portable checksum: sha256sum or shasum.
      if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "${rel}"
      else
        shasum -a 256 "${rel}"
      fi
    done
  )
}

sum_a="$(checksum_tree "${OUT_A}")"
sum_b="$(checksum_tree "${OUT_B}")"
[[ "${sum_a}" == "${sum_b}" ]] || fail "same seed must be deterministic:\nA:\n${sum_a}\nB:\n${sum_b}"
pass "same seed produces identical manifests"

# Different seed → different content
mkdir -p "${OUT_C}"
run_wl --run-id "${RUN_ID}" --seed 99 --mode batch --count 6 \
  --dry-run --out-dir "${OUT_C}" >/dev/null 2>&1 ||
  fail "batch dry-run (seed 99) failed"
sum_c="$(checksum_tree "${OUT_C}")"
[[ "${sum_a}" != "${sum_c}" ]] || fail "different seeds must diverge"
pass "different seed changes manifests"

# --- light churn mode: plan / manifests include create|update|delete ops ---
mkdir -p "${OUT_CHURN}"
if ! churn_out="$(run_wl --run-id "${RUN_ID}" --seed 42 --mode churn --count 6 \
  --dry-run --out-dir "${OUT_CHURN}" 2>&1)"; then
  fail "churn dry-run failed: ${churn_out}"
fi
# Churn must still label everything and emit an operation plan (file or stdout).
churn_manifests=()
while IFS= read -r -d '' f; do
  churn_manifests+=("${f}")
done < <(find "${OUT_CHURN}" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name '*.txt' \) -print0 | sort -z)
((${#churn_manifests[@]} > 0)) || fail "churn dry-run must write plan/manifests under ${OUT_CHURN}"

for f in "${churn_manifests[@]}"; do
  case "${f}" in
    *.yaml|*.yml)
      grep -q "kollect.dev/lab-run: ${RUN_ID}" "${f}" ||
        grep -q "kollect.dev/lab-run: \"${RUN_ID}\"" "${f}" ||
        fail "churn manifest missing lab-run label: ${f}"
      ;;
  esac
done

# Operation hints: create / update / delete (plan file preferred; stdout fallback).
plan_blob="$(cat "${OUT_CHURN}"/* 2>/dev/null || true)"$'\n'"${churn_out}"
printf '%s\n' "${plan_blob}" | grep -Eqi 'create' ||
  fail "churn mode should mention create ops"
printf '%s\n' "${plan_blob}" | grep -Eqi 'update' ||
  fail "churn mode should mention update ops"
printf '%s\n' "${plan_blob}" | grep -Eqi 'delete' ||
  fail "churn mode should mention delete ops"
pass "light churn mode emits create/update/delete plan"

# --- --fixture alias stays offline ---
FIX="${TMP}/fixture"
mkdir -p "${FIX}"
if ! run_wl --run-id "${RUN_ID}" --seed 7 --fixture --out-dir "${FIX}" >/dev/null 2>&1; then
  fail "--fixture mode unexpectedly failed"
fi
pass "--fixture mode exits 0 offline"

# --- help documents contract ---
help_out="$(run_wl --help 2>&1 || true)"
printf '%s\n' "${help_out}" | grep -Eq -- '--run-id' || fail "--help must mention --run-id"
printf '%s\n' "${help_out}" | grep -Eq -- '--seed' || fail "--help must mention --seed"
printf '%s\n' "${help_out}" | grep -Eq -- '--dry-run' || fail "--help must mention --dry-run"
printf '%s\n' "${help_out}" | grep -Eqi 'churn|batch' || fail "--help must mention batch/churn"
printf '%s\n' "${help_out}" | grep -Eqi 'lab-run|kollect.dev/lab-run' ||
  fail "--help must mention kollect.dev/lab-run label"
pass "CLI --help documents seed / dry-run / modes / lab-run"

# Poisoned kubectl never successfully invoked (exit 99 would fail cases above).
pass "no live kubectl invocations"

echo "All lab workload meta tests passed."
