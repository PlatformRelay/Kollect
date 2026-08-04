#!/usr/bin/env bash
# COV-90-S14: finalizer-cleanup Kind assert must exist and be wired into
# e2e-nightly + e2e-extended (not e2e-smoke — DEMO-03 owns the hero job).
# Pure contract test; does not require kind.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ASSERT="${ROOT}/hack/e2e/finalizer-cleanup-assert.sh"
NIGHTLY="${ROOT}/.github/workflows/e2e-nightly.yaml"
EXTENDED="${ROOT}/.github/workflows/e2e-extended.yaml"
SMOKE="${ROOT}/.github/workflows/e2e-smoke.yaml"

fail() {
  printf 'finalizer-cleanup e2e: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

# --- assert script exists, executable, covers AC scenarios ---
[[ -f "${ASSERT}" ]] || fail "missing ${ASSERT}"
[[ -x "${ASSERT}" ]] || fail "${ASSERT} must be executable"

# RED: Target delete → finalizer clears (no stuck object).
grep -Eq 'kollecttarget|KollectTarget' "${ASSERT}" ||
  fail "assert must exercise KollectTarget delete/finalizer cleanup"
grep -Eq 'finalizer|DeletionTimestamp|delete' "${ASSERT}" ||
  fail "assert must delete a Target/Inventory and wait for finalizer removal"

# Artifact teardown signal (UnregisterTarget / inventory HTTP / empty export path).
grep -Eq 'inventory|Unregister|itemCount|artifact|cleanup' "${ASSERT}" ||
  fail "assert must verify collected/exported artifact teardown after delete"

# EDGE: sink rejects teardown once → retry/recovery completes.
grep -Eq 'retry|requeue|reject|missing.*(secret|sink)|databaseSink|partial' "${ASSERT}" ||
  fail "assert must cover sink-reject-once / retry-complete EDGE"

# EDGE: Degraded Target + Warning Event.
grep -Eq 'Degraded|condition=Degraded' "${ASSERT}" ||
  fail "assert must check Degraded=True on Target"
grep -Eq 'Warning|Event|events\.k8s\.io|kubectl get events' "${ASSERT}" ||
  fail "assert must check Warning Event for Degraded Target"
# KollectTarget does not watch Inventory — after sink-ref patch/restore the script
# must nudge the Target (annotate/patch) so Degraded/Ready waits observe updates.
grep -Eq 'annotate[[:space:]].*kollecttarget|nudge.*[Tt]arget|force.*[Rr]econcile' "${ASSERT}" ||
  fail "assert must annotate/nudge KollectTarget after inventory sink-ref patch (no Inventory watch)"
pass "assert script covers RED delete + EDGE retry + EDGE Degraded/Warning"

# --- e2e-nightly.yaml matrix includes finalizer-cleanup scenario ---
[[ -f "${NIGHTLY}" ]] || fail "missing ${NIGHTLY}"
grep -Eq 'finalizer-cleanup-assert\.sh' "${NIGHTLY}" ||
  fail "e2e-nightly.yaml must invoke hack/e2e/finalizer-cleanup-assert.sh"
grep -Eq 'scenario:[[:space:]]*finalizer-cleanup' "${NIGHTLY}" ||
  fail "e2e-nightly.yaml matrix must include scenario: finalizer-cleanup"
pass "e2e-nightly.yaml wires finalizer-cleanup scenario"

# --- e2e-extended.yaml matrix includes finalizer-cleanup scenario ---
[[ -f "${EXTENDED}" ]] || fail "missing ${EXTENDED}"
grep -Eq 'finalizer-cleanup-assert\.sh' "${EXTENDED}" ||
  fail "e2e-extended.yaml must invoke hack/e2e/finalizer-cleanup-assert.sh"
grep -Eq 'scenario:[[:space:]]*finalizer-cleanup' "${EXTENDED}" ||
  fail "e2e-extended.yaml matrix must include scenario: finalizer-cleanup"
pass "e2e-extended.yaml wires finalizer-cleanup scenario"

# --- must NOT steal DEMO-03's e2e-smoke hero lane ---
if [[ -f "${SMOKE}" ]] && grep -Eq 'finalizer-cleanup-assert\.sh' "${SMOKE}"; then
  fail "e2e-smoke.yaml must not invoke finalizer-cleanup-assert.sh (DEMO-03 owns hero job)"
fi
pass "e2e-smoke.yaml left alone (DEMO-03)"

printf 'finalizer-cleanup e2e: ok\n'
