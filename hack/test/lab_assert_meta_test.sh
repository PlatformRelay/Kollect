#!/usr/bin/env bash
# LAB-H04: lab assert helpers must map DOC-02 verdicts, compare counts/checksums,
# and never coerce skip/blocked/limit/not-triggered into pass. Offline fixtures only.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ASSERT_LIB="${ROOT}/hack/lab/lib/assert.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab assert meta: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${ASSERT_LIB}" ]] || fail "missing ${ASSERT_LIB}"

# shellcheck source=../lab/lib/assert.sh
source "${ASSERT_LIB}"

# ---------------------------------------------------------------------------
# Allowed verdicts (LAB-DOC-02 + ADR-0707 BLOCKED)
# ---------------------------------------------------------------------------
required_verdicts=(
  PASS
  PASS_WITH_LIMITATION
  FAIL
  SKIPPED
  LIMIT_REACHED
  BLOCKED
  "not triggered"
)

allowed="$(lab_assert_allowed_verdicts)" || fail "lab_assert_allowed_verdicts failed"
for v in "${required_verdicts[@]}"; do
  printf '%s\n' "${allowed}" | grep -qxF -- "${v}" ||
    fail "allowed verdicts missing: ${v}"
  lab_assert_is_allowed_verdict "${v}" || fail "lab_assert_is_allowed_verdict rejected ${v}"
done
lab_assert_is_allowed_verdict "GREEN" && fail "GREEN must not be an allowed verdict"
pass "DOC-02 + BLOCKED verdicts allowed; unknown rejected"

# ---------------------------------------------------------------------------
# counts_as_pass: only PASS / PASS_WITH_LIMITATION
# ---------------------------------------------------------------------------
lab_assert_counts_as_pass PASS || fail "PASS must count as pass"
lab_assert_counts_as_pass PASS_WITH_LIMITATION || fail "PASS_WITH_LIMITATION must count as pass"
for v in FAIL SKIPPED LIMIT_REACHED BLOCKED "not triggered"; do
  if lab_assert_counts_as_pass "${v}"; then
    fail "${v} must NOT count as pass (never coerce non-pass → pass)"
  fi
done
pass "only PASS and PASS_WITH_LIMITATION count as pass"

# ---------------------------------------------------------------------------
# Non-pass (and PASS_WITH_LIMITATION) require a reason string
# ---------------------------------------------------------------------------
reason_table=(
  "PASS|"
  "PASS_WITH_LIMITATION|partial scrape vs live certs"
  "FAIL|source/store counts diverge"
  "SKIPPED|out of selected schedule"
  "LIMIT_REACHED|host RAM ceiling"
  "BLOCKED|inventory hang"
  "not triggered|Wave 4 load not run"
)

for row in "${reason_table[@]}"; do
  verdict="${row%%|*}"
  reason="${row#*|}"
  if [[ "${verdict}" == "PASS" ]]; then
    lab_assert_require_reason "${verdict}" "" || fail "PASS must allow empty reason"
    continue
  fi
  if lab_assert_require_reason "${verdict}" "" 2>/dev/null; then
    fail "${verdict} must require a non-empty reason"
  fi
  lab_assert_require_reason "${verdict}" "${reason}" ||
    fail "${verdict} rejected valid reason: ${reason}"
done
pass "reason required for every non-PASS verdict including PASS_WITH_LIMITATION"

# ---------------------------------------------------------------------------
# Emit result: structured JSON; never remaps skip/blocked/limit → pass
# ---------------------------------------------------------------------------
emit_ok="$(lab_assert_emit_result SKIPPED "out of schedule")" ||
  fail "emit SKIPPED with reason should succeed"
printf '%s' "${emit_ok}" | jq -e '
  .verdict == "SKIPPED"
  and .reason == "out of schedule"
  and (.counts_as_pass == false)
' >/dev/null || fail "emit SKIPPED JSON shape wrong: ${emit_ok}"

if lab_assert_emit_result BLOCKED "" 2>/dev/null; then
  fail "emit BLOCKED without reason must fail"
fi

emit_pass="$(lab_assert_emit_result PASS "")" || fail "emit PASS should succeed"
printf '%s' "${emit_pass}" | jq -e '.verdict == "PASS" and .counts_as_pass == true' >/dev/null ||
  fail "emit PASS JSON shape wrong: ${emit_pass}"

# Explicit coercion attempt: mapping helpers must keep non-pass outcomes.
for outcome in skipped blocked limit_reached not_triggered; do
  mapped="$(lab_assert_map_outcome_to_verdict "${outcome}" "fixture reason")" ||
    fail "map ${outcome} with reason should succeed"
  v="$(printf '%s' "${mapped}" | jq -r '.verdict')"
  cap="$(printf '%s' "${mapped}" | jq -r '.counts_as_pass')"
  [[ "${cap}" == "false" ]] || fail "map ${outcome} coerced to pass: ${mapped}"
  case "${outcome}" in
    skipped) [[ "${v}" == "SKIPPED" ]] || fail "expected SKIPPED got ${v}" ;;
    blocked) [[ "${v}" == "BLOCKED" ]] || fail "expected BLOCKED got ${v}" ;;
    limit_reached) [[ "${v}" == "LIMIT_REACHED" ]] || fail "expected LIMIT_REACHED got ${v}" ;;
    not_triggered) [[ "${v}" == "not triggered" ]] || fail "expected 'not triggered' got ${v}" ;;
  esac
done
pass "emit/map keep skip/blocked/limit/not-triggered as non-pass"

# ---------------------------------------------------------------------------
# Count comparison fixtures (source / store / inventory / sink)
# ---------------------------------------------------------------------------
match_expected='{"source":3,"store":3,"inventory":3,"sink":3}'
match_actual='{"source":3,"store":3,"inventory":3,"sink":3}'
mismatch_actual='{"source":3,"store":1,"inventory":3,"sink":3}'

cmp_match="$(lab_assert_compare_counts "${match_expected}" "${match_actual}")" ||
  fail "matching counts should exit 0"
printf '%s' "${cmp_match}" | jq -e '.outcome == "match"' >/dev/null ||
  fail "expected match outcome: ${cmp_match}"

if cmp_mismatch="$(lab_assert_compare_counts "${match_expected}" "${mismatch_actual}" 2>"${TMP}/cmp.err")"; then
  fail "mismatched counts must exit non-zero"
fi
# Function may print JSON on stdout even on mismatch — accept stderr or stdout.
cmp_body="${cmp_mismatch:-$(cat "${TMP}/cmp.err")}"
# Re-run capturing both streams into a file via a subshell that ignores exit.
cmp_body="$(
  set +e
  lab_assert_compare_counts "${match_expected}" "${mismatch_actual}" >"${TMP}/cmp.out" 2>"${TMP}/cmp.err2"
  cat "${TMP}/cmp.out"
)"
printf '%s' "${cmp_body}" | jq -e '.outcome == "mismatch"' >/dev/null ||
  fail "expected mismatch outcome JSON on stdout: ${cmp_body}"
printf '%s' "${cmp_body}" | jq -e '.diffs | length >= 1' >/dev/null ||
  fail "mismatch must list diffs: ${cmp_body}"
pass "count fixtures: match and mismatch"

# ---------------------------------------------------------------------------
# Canonical checksum string comparison
# ---------------------------------------------------------------------------
sum_a="sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
sum_b="sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

ck_match="$(lab_assert_compare_checksum "${sum_a}" "${sum_a}")" ||
  fail "equal checksums should exit 0"
printf '%s' "${ck_match}" | jq -e '.outcome == "match"' >/dev/null ||
  fail "checksum match JSON: ${ck_match}"

ck_mis="$(
  set +e
  lab_assert_compare_checksum "${sum_a}" "${sum_b}" >"${TMP}/ck.out" 2>/dev/null
  cat "${TMP}/ck.out"
)"
printf '%s' "${ck_mis}" | jq -e '.outcome == "mismatch"' >/dev/null ||
  fail "checksum mismatch JSON: ${ck_mis}"
# confirm non-zero exit
if lab_assert_compare_checksum "${sum_a}" "${sum_b}" >/dev/null 2>&1; then
  fail "unequal checksums must exit non-zero"
fi
pass "canonical checksum compare"

# ---------------------------------------------------------------------------
# Map comparison outcomes → verdicts (table)
# ---------------------------------------------------------------------------
# match + no limitation → PASS
v_pass="$(lab_assert_map_outcome_to_verdict match "")" || fail "map match → PASS"
printf '%s' "${v_pass}" | jq -e '.verdict == "PASS" and .counts_as_pass == true' >/dev/null ||
  fail "match map: ${v_pass}"

# match + limitation reason → PASS_WITH_LIMITATION
v_lim="$(lab_assert_map_outcome_to_verdict match "cert undercount vs live" --limitation)" ||
  fail "map match --limitation"
printf '%s' "${v_lim}" | jq -e '
  .verdict == "PASS_WITH_LIMITATION"
  and .counts_as_pass == true
  and (.reason | length > 0)
' >/dev/null || fail "limitation map: ${v_lim}"

if lab_assert_map_outcome_to_verdict match "" --limitation 2>/dev/null; then
  fail "PASS_WITH_LIMITATION without reason must fail"
fi

# mismatch → FAIL (reason required)
if lab_assert_map_outcome_to_verdict mismatch "" 2>/dev/null; then
  fail "mismatch without reason must fail"
fi
v_fail="$(lab_assert_map_outcome_to_verdict mismatch "store count 1 != expected 3")" ||
  fail "map mismatch with reason"
printf '%s' "${v_fail}" | jq -e '.verdict == "FAIL" and .counts_as_pass == false' >/dev/null ||
  fail "fail map: ${v_fail}"

# Combined helper: counts + checksum → verdict
combined="$(
  lab_assert_verdict_from_comparison \
    "${match_expected}" "${match_actual}" \
    "${sum_a}" "${sum_a}" \
    ""
)" || fail "combined match should PASS"
printf '%s' "${combined}" | jq -e '.verdict == "PASS"' >/dev/null ||
  fail "combined match: ${combined}"

combined_fail="$(
  set +e
  lab_assert_verdict_from_comparison \
    "${match_expected}" "${mismatch_actual}" \
    "${sum_a}" "${sum_a}" \
    "store undercount" >"${TMP}/comb.out" 2>/dev/null
  cat "${TMP}/comb.out"
)"
printf '%s' "${combined_fail}" | jq -e '.verdict == "FAIL"' >/dev/null ||
  fail "combined mismatch should FAIL: ${combined_fail}"

combined_ck="$(
  set +e
  lab_assert_verdict_from_comparison \
    "${match_expected}" "${match_actual}" \
    "${sum_a}" "${sum_b}" \
    "export checksum diverged" >"${TMP}/comb2.out" 2>/dev/null
  cat "${TMP}/comb2.out"
)"
printf '%s' "${combined_ck}" | jq -e '.verdict == "FAIL"' >/dev/null ||
  fail "checksum diverge should FAIL: ${combined_ck}"

# Combined with --limitation on full match → PASS_WITH_LIMITATION
combined_lim="$(
  lab_assert_verdict_from_comparison \
    "${match_expected}" "${match_actual}" \
    "${sum_a}" "${sum_a}" \
    "emulator fidelity caveat" \
    --limitation
)" || fail "combined limitation"
printf '%s' "${combined_lim}" | jq -e '.verdict == "PASS_WITH_LIMITATION"' >/dev/null ||
  fail "combined limitation: ${combined_lim}"

pass "outcome→verdict mapping and combined comparison"

echo "All lab assert meta tests passed."
