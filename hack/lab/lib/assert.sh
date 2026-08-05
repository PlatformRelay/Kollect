#!/usr/bin/env bash
# LAB-H04 — assert helpers for lab scenario verdicts (LAB-DOC-02 / ADR-0707).
# Source this file; do not execute it directly.
#
# Allowed verdicts:
#   PASS, PASS_WITH_LIMITATION, FAIL, SKIPPED, LIMIT_REACHED, BLOCKED, not triggered
# Non-pass verdicts (including PASS_WITH_LIMITATION) require a reason string.
# SKIPPED / BLOCKED / LIMIT_REACHED / "not triggered" never count as pass.

# shellcheck disable=SC2034  # exported for callers that iterate the array
LAB_ASSERT_ALLOWED_VERDICTS=(
  PASS
  PASS_WITH_LIMITATION
  FAIL
  SKIPPED
  LIMIT_REACHED
  BLOCKED
  "not triggered"
)

lab_assert_allowed_verdicts() {
  local v
  for v in "${LAB_ASSERT_ALLOWED_VERDICTS[@]}"; do
    printf '%s\n' "${v}"
  done
}

lab_assert_is_allowed_verdict() {
  local want="${1:-}"
  local v
  for v in "${LAB_ASSERT_ALLOWED_VERDICTS[@]}"; do
    if [[ "${v}" == "${want}" ]]; then
      return 0
    fi
  done
  return 1
}

lab_assert_counts_as_pass() {
  case "${1:-}" in
    PASS | PASS_WITH_LIMITATION) return 0 ;;
    *) return 1 ;;
  esac
}

# PASS may have an empty reason; every other allowed verdict needs a non-empty reason.
lab_assert_require_reason() {
  local verdict="${1:-}"
  local reason="${2:-}"

  if ! lab_assert_is_allowed_verdict "${verdict}"; then
    printf 'lab_assert: unknown verdict %q\n' "${verdict}" >&2
    return 1
  fi
  if [[ "${verdict}" == "PASS" ]]; then
    return 0
  fi
  if [[ -z "${reason}" ]]; then
    printf 'lab_assert: verdict %s requires a non-empty reason\n' "${verdict}" >&2
    return 1
  fi
  return 0
}

lab_assert_emit_result() {
  local verdict="${1:-}"
  local reason="${2:-}"
  local counts_as_pass="false"

  lab_assert_require_reason "${verdict}" "${reason}" || return 1
  if lab_assert_counts_as_pass "${verdict}"; then
    counts_as_pass="true"
  fi
  jq -nc \
    --arg verdict "${verdict}" \
    --arg reason "${reason}" \
    --argjson counts_as_pass "${counts_as_pass}" \
    '{verdict: $verdict, reason: $reason, counts_as_pass: $counts_as_pass}'
}

# Compare expected vs actual count objects: {source,store,inventory,sink}.
# Always prints JSON on stdout. Exit 0 on match, 1 on mismatch / bad input.
lab_assert_compare_counts() {
  local expected="${1:-}"
  local actual="${2:-}"
  local result

  if [[ -z "${expected}" || -z "${actual}" ]]; then
    printf 'lab_assert: compare_counts requires expected and actual JSON\n' >&2
    return 1
  fi

  result="$(
    jq -nc \
      --argjson expected "${expected}" \
      --argjson actual "${actual}" '
      def keys_order: ["source", "store", "inventory", "sink"];
      def as_counts:
        if type != "object" then error("counts must be a JSON object") end
        | . as $o
        | reduce keys_order[] as $k ({}; .[$k] = ($o[$k] // null));
      ($expected | as_counts) as $e
      | ($actual | as_counts) as $a
      | [keys_order[] as $k
          | select($e[$k] != $a[$k])
          | {field: $k, expected: $e[$k], actual: $a[$k]}
        ] as $diffs
      | if ($diffs | length) == 0 then
          {outcome: "match", expected: $e, actual: $a, diffs: []}
        else
          {outcome: "mismatch", expected: $e, actual: $a, diffs: $diffs}
        end
    '
  )" || {
    printf 'lab_assert: compare_counts failed to parse JSON\n' >&2
    return 1
  }

  printf '%s\n' "${result}"
  [[ "$(jq -r '.outcome' <<<"${result}")" == "match" ]]
}

# Canonical checksum string compare (exact after trimming surrounding whitespace).
lab_assert_compare_checksum() {
  local expected="${1:-}"
  local actual="${2:-}"
  # trim leading/trailing whitespace
  expected="${expected#"${expected%%[![:space:]]*}"}"
  expected="${expected%"${expected##*[![:space:]]}"}"
  actual="${actual#"${actual%%[![:space:]]*}"}"
  actual="${actual%"${actual##*[![:space:]]}"}"

  local outcome="mismatch"
  if [[ "${expected}" == "${actual}" ]]; then
    outcome="match"
  fi
  jq -nc \
    --arg outcome "${outcome}" \
    --arg expected "${expected}" \
    --arg actual "${actual}" \
    '{outcome: $outcome, expected: $expected, actual: $actual}'
  [[ "${outcome}" == "match" ]]
}

# Map a comparison / schedule outcome to a DOC-02 verdict JSON.
# Outcomes: match | mismatch | skipped | blocked | limit_reached | not_triggered
# Optional trailing --limitation promotes a match to PASS_WITH_LIMITATION (reason required).
lab_assert_map_outcome_to_verdict() {
  local outcome="${1:-}"
  local reason="${2:-}"
  shift 2 || true
  local limitation=0
  local arg verdict

  for arg in "$@"; do
    case "${arg}" in
      --limitation) limitation=1 ;;
      *)
        printf 'lab_assert: unknown argument %q\n' "${arg}" >&2
        return 1
        ;;
    esac
  done

  case "${outcome}" in
    match)
      if ((limitation)); then
        verdict="PASS_WITH_LIMITATION"
      else
        verdict="PASS"
      fi
      ;;
    mismatch) verdict="FAIL" ;;
    skipped) verdict="SKIPPED" ;;
    blocked) verdict="BLOCKED" ;;
    limit_reached) verdict="LIMIT_REACHED" ;;
    not_triggered) verdict="not triggered" ;;
    *)
      printf 'lab_assert: unknown outcome %q\n' "${outcome}" >&2
      return 1
      ;;
  esac

  # Never coerce non-match outcomes into a pass, even if --limitation is set.
  if [[ "${outcome}" != "match" ]] && lab_assert_counts_as_pass "${verdict}"; then
    printf 'lab_assert: internal error: non-match outcome mapped to pass\n' >&2
    return 1
  fi

  lab_assert_emit_result "${verdict}" "${reason}"
}

# Convenience: compare counts + checksums and emit a verdict.
# Usage:
#   lab_assert_verdict_from_comparison EXPECTED_COUNTS ACTUAL_COUNTS \
#       EXPECTED_CHECKSUM ACTUAL_CHECKSUM REASON [--limitation]
lab_assert_verdict_from_comparison() {
  local expected_counts="${1:-}"
  local actual_counts="${2:-}"
  local expected_checksum="${3:-}"
  local actual_checksum="${4:-}"
  local reason="${5:-}"
  shift 5 || true
  local limitation=0
  local arg
  local counts_json checksum_json
  local counts_rc=0 checksum_rc=0

  for arg in "$@"; do
    case "${arg}" in
      --limitation) limitation=1 ;;
      *)
        printf 'lab_assert: unknown argument %q\n' "${arg}" >&2
        return 1
        ;;
    esac
  done

  counts_json="$(lab_assert_compare_counts "${expected_counts}" "${actual_counts}")" || counts_rc=$?
  checksum_json="$(lab_assert_compare_checksum "${expected_checksum}" "${actual_checksum}")" || checksum_rc=$?

  if ((counts_rc != 0 || checksum_rc != 0)); then
    if [[ -z "${reason}" ]]; then
      local detail=""
      if [[ "$(jq -r '.outcome' <<<"${counts_json}" 2>/dev/null || true)" == "mismatch" ]]; then
        detail="counts mismatch"
      fi
      if [[ "$(jq -r '.outcome' <<<"${checksum_json}" 2>/dev/null || true)" == "mismatch" ]]; then
        if [[ -n "${detail}" ]]; then
          detail="${detail}; checksum mismatch"
        else
          detail="checksum mismatch"
        fi
      fi
      reason="${detail:-comparison mismatch}"
    fi
    lab_assert_map_outcome_to_verdict mismatch "${reason}"
    return 0
  fi

  if ((limitation)); then
    lab_assert_map_outcome_to_verdict match "${reason}" --limitation
  else
    lab_assert_map_outcome_to_verdict match "${reason}"
  fi
}
