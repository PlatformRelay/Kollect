#!/usr/bin/env bash
# LAB-H01: lab preflight meta-tests (offline fixtures only — never hit a live cluster).
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PREFLIGHT="${ROOT}/hack/lab/preflight.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab preflight meta: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${PREFLIGHT}" ]] || fail "missing ${PREFLIGHT}"
[[ -f "${ROOT}/hack/lab/lib/preflight.sh" ]] || fail "missing hack/lab/lib/preflight.sh"
[[ -f "${ROOT}/hack/lab/README.md" ]] || fail "missing hack/lab/README.md"

# Poison PATH: any real kubectl/helm invocation is a hard test failure.
cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "lab preflight meta: unexpected live kubectl: $*" >&2
exit 99
EOF
cat >"${TMP}/helm" <<'EOF'
#!/usr/bin/env bash
echo "lab preflight meta: unexpected live helm: $*" >&2
exit 99
EOF
chmod +x "${TMP}/kubectl" "${TMP}/helm"
export PATH="${TMP}:${PATH}"

run_preflight() {
  # Drop ambient KUBECONFIG so fixtures never resolve a real cluster path by accident.
  env -u KUBECONFIG PATH="${PATH}" bash "${PREFLIGHT}" "$@"
}

# --- clean fixture: must PASS ---
if ! out="$(run_preflight --fixture=clean 2>&1)"; then
  fail "clean fixture unexpectedly failed: ${out}"
fi
printf '%s\n' "${out}" | grep -Eqi 'preflight.*(ok|pass|clean)|OK' ||
  fail "clean fixture output should report success: ${out}"
pass "clean fixture exits 0"

# Env form of clean fixture
if ! env -u KUBECONFIG PATH="${PATH}" KOLLECT_LAB_PREFLIGHT_FIXTURE=clean \
  bash "${PREFLIGHT}" >/dev/null 2>&1; then
  fail "KOLLECT_LAB_PREFLIGHT_FIXTURE=clean unexpectedly failed"
fi
pass "env clean fixture exits 0"

# --- residue without --force: must FAIL ---
rc=0
out="$(run_preflight --fixture=residue 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "residue fixture without --force unexpectedly passed: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'residue|kollect-lab|lab-run|--force' ||
  fail "residue failure should mention isolation / --force: ${out}"
pass "residue fixture without --force exits non-zero"

# --- residue with --force: must PASS ---
if ! out="$(run_preflight --fixture=residue --force 2>&1)"; then
  fail "residue fixture with --force unexpectedly failed: ${out}"
fi
printf '%s\n' "${out}" | grep -Eqi 'force|WARN|residue|ok|pass' ||
  fail "forced residue output should acknowledge override: ${out}"
pass "residue fixture with --force exits 0"

# --- dry-run + fixture still offline ---
if ! run_preflight --dry-run --fixture=clean >/dev/null 2>&1; then
  fail "--dry-run --fixture=clean unexpectedly failed"
fi
pass "--dry-run clean fixture exits 0"

# --- never create/destroy cluster language in help/README ---
help_out="$(run_preflight --help 2>&1 || true)"
printf '%s\n' "${help_out}" | grep -Eqi 'never.*(create|destroy)|existing.*(kubeconfig|cluster)' ||
  grep -Eqi 'never.*(create|destroy)|existing.*(kubeconfig|cluster)|non-Kind' \
    "${ROOT}/hack/lab/README.md" ||
  fail "README or --help must state existing kubeconfig / never create-destroy cluster"
pass "docs state cluster-agnostic / non-destructive contract"

# Poisoned kubectl/helm were never successfully invoked (exit 99 would fail cases above).
pass "no live kubectl/helm invocations"

echo "All lab preflight meta tests passed."
