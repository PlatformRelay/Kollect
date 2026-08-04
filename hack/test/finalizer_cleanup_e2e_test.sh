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

# Fail-closed (COV-90-S14 F1): empty/non-numeric itemCount is curl/PF failure —
# never treat empty as teardown success. InventorySummary always emits itemCount
# (incl. 0); success requires a non-comment compound: numeric count AND (0 or drop).
assert_artifact_teardown_fail_closed() {
  local file="$1"
  local code

  # Ban empty-as-success patterns (not only literal -z "${count}").
  if grep -Eq -- '-z[[:space:]]*"\$\{count\}"' "${file}"; then
    return 1
  fi
  if grep -Eq -- '-z[[:space:]]+count([^[:alnum:]_]|$)' "${file}"; then
    return 1
  fi
  # [[ ! "${count}" ]] / [[ ! "${count}" || … ]] empty tests.
  if grep -Eq '\[\[([^]]*![[:space:]]*"\$\{count\}"[^]]*)\]\]' "${file}"; then
    return 1
  fi
  # Empty OR'd into a success path: [[ ! "${count}" ]] || … / [[ -z … ]] || …
  if grep -Eq '(![[:space:]]*"\$\{count\}"|-z[[:space:]]*"\$\{count\}").*\]\][[:space:]]*\|\|' "${file}"; then
    return 1
  fi

  # Full-line comments must not paper-satisfy the numeric/drop compound.
  code="$(grep -Ev '^[[:space:]]*(#|$)' "${file}" || true)"

  # Require compound on a code line: numeric check AND (count == 0 || drop).
  echo "${code}" | grep -Eq \
    '\[\[[[:space:]]*"\$\{count\}"[[:space:]]*=~[[:space:]]*\^\[0-9\]\+\$[[:space:]]*\]\][[:space:]]*&&[[:space:]]*\(\(.*count[[:space:]]*==[[:space:]]*0[[:space:]]*\|\|' ||
    return 1

  return 0
}

# Paper-gate: empty-success [[ ! "${count}" ]] || … must FAIL the fail-closed check.
PAPER_FAILCLOSED="$(mktemp)"
trap 'rm -f "${PAPER_FAILCLOSED}" "${PAPER_TMP:-}"' EXIT
printf '%s\n' \
  'count="$(inventory_http_item_count "${http_port}")"' \
  'if [[ ! "${count}" ]] || (( count == 0 || count < before )); then' \
  '  return 0' \
  'fi' \
  >"${PAPER_FAILCLOSED}"
if assert_artifact_teardown_fail_closed "${PAPER_FAILCLOSED}"; then
  fail "paper-gate: empty-success [[ ! count ]] fixture must fail fail-closed check"
fi
pass "paper-gate rejects empty-success [[ ! count ]] teardown"

# Paper-gate: comment-only numeric regex must FAIL (live code could still treat empty as success).
printf '%s\n' \
  '# [[ "${count}" =~ ^[0-9]+$ ]] && (( count == 0 || count < before ))' \
  'count="$(inventory_http_item_count "${http_port}")"' \
  'if [[ -n "${count}" ]]; then return 0; fi' \
  >"${PAPER_FAILCLOSED}"
if assert_artifact_teardown_fail_closed "${PAPER_FAILCLOSED}"; then
  fail "paper-gate: comment-only numeric regex must fail fail-closed check"
fi
pass "paper-gate rejects comment-only numeric itemCount regex"

assert_artifact_teardown_fail_closed "${ASSERT}" ||
  fail "artifact teardown must require non-comment numeric itemCount AND (0 or drop); ban empty-as-success"
pass "artifact teardown fail-closed on empty/non-numeric itemCount"

# EDGE: sink rejects teardown once → retry/recovery completes.
grep -Eq 'retry|requeue|reject|missing.*(secret|sink)|databaseSink|partial' "${ASSERT}" ||
  fail "assert must cover sink-reject-once / retry-complete EDGE"

# EDGE: Degraded Target + Warning Event.
grep -Eq 'Degraded|condition=Degraded' "${ASSERT}" ||
  fail "assert must check Degraded=True on Target"
grep -Eq 'Warning|Event|events\.k8s\.io|kubectl get events' "${ASSERT}" ||
  fail "assert must check Warning Event for Degraded Target"
# KollectTarget does not watch Inventory — after sink-ref patch/restore the script
# must nudge the Target so Degraded/Ready waits observe updates.
# Anchor at real call sites (DEMO-03-FUP): comments / log strings must not satisfy.
assert_nudge_call_sites() {
  local file="$1"
  # Helper definition + annotate body.
  grep -Eq '^nudge_target_reconcile\(\)' "${file}" || return 1
  grep -Eq '^[[:space:]]*kubectl[[:space:]]+annotate[[:space:]].*kollecttarget' "${file}" || return 1
  # At least one line-start invocation (definition ends with '()', so it does not match).
  grep -Eq '^[[:space:]]*nudge_target_reconcile([[:space:]]|$)' "${file}" || return 1
  return 0
}

# Paper-gate: comment-only text that used to satisfy the loose grep must FAIL.
PAPER_TMP="$(mktemp)"
trap 'rm -f "${PAPER_FAILCLOSED}" "${PAPER_TMP}"' EXIT
printf '%s\n' \
  '# force Reconcile via annotate kollecttarget' \
  '# nudge_target_reconcile' \
  '# nudge Target after inventory sink-ref patch' \
  >"${PAPER_TMP}"
if assert_nudge_call_sites "${PAPER_TMP}"; then
  fail "paper-gate: comment-only fixture must fail nudge call-site check"
fi
pass "paper-gate rejects comment-only force-Reconcile text"

assert_nudge_call_sites "${ASSERT}" ||
  fail "assert must define/call nudge_target_reconcile (kubectl annotate kollecttarget) after inventory sink-ref patch"
# Two sink-ref mutations (degrade + restore) each need a nudge.
nudge_calls="$(grep -Ec '^[[:space:]]*nudge_target_reconcile([[:space:]]|$)' "${ASSERT}" || true)"
[[ "${nudge_calls}" -ge 2 ]] ||
  fail "assert must call nudge_target_reconcile at least twice (after sink-ref patch and restore); got ${nudge_calls}"
pass "assert script covers RED delete + EDGE retry + EDGE Degraded/Warning + Target nudge"

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
