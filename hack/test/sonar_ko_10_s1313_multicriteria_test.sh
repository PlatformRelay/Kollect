#!/usr/bin/env bash
# Meta test for SONAR-SEC-02 (SonarCloud go:S1313 "hardcoded IP address").
#
# The hardcoded IPs/CIDRs in the netguard SSRF dialer
# (internal/sink/netguard/**) and the endpoint deny-list
# (internal/validation/endpoint_guard.go) ARE the intended security control,
# not accidental config -- so go:S1313 there is a false positive. We suppress
# it narrowly via a scoped sonar.issue.ignore.multicriteria block in
# sonar-project.properties.
#
# This test locks in the STRUCTURE of that block: that the multicriteria master
# key lists both criterion IDs and that each carries ruleKey=go:S1313 with a
# resourceKey covering the two intended paths. It is a structural invariant
# only -- it proves the exclusion is declared as intended and does not silently
# disappear or widen; it cannot and does not assert that SonarCloud honors the
# resourceKey glob (only a live analysis can confirm the suppression fires).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROPS="${ROOT}/sonar-project.properties"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

[[ -f "${PROPS}" ]] || fail "sonar-project.properties not found at ${PROPS}"

# --- the two source paths the exclusion is meant to cover must still exist,
# so the test can't silently pass against renamed/removed code. ---
[[ -d "${ROOT}/internal/sink/netguard" ]] || fail "internal/sink/netguard does not exist; adjust the S1313 exclusion target"
[[ -f "${ROOT}/internal/validation/endpoint_guard.go" ]] || fail "internal/validation/endpoint_guard.go does not exist; adjust the S1313 exclusion target"

# --- master key must list both criterion IDs ---
master_line="$(grep -E '^sonar\.issue\.ignore\.multicriteria=' "${PROPS}" || true)"
[[ -n "${master_line}" ]] || fail "sonar.issue.ignore.multicriteria= master key not found in ${PROPS}"
master_value="${master_line#sonar.issue.ignore.multicriteria=}"
IFS=',' read -r -a IDS <<<"${master_value}"
((${#IDS[@]} >= 2)) || fail "expected at least two multicriteria IDs, got: ${master_value}"

# --- every listed ID must resolve to a go:S1313 ruleKey + a resourceKey line ---
netguard_covered=0
endpointguard_covered=0
for id in "${IDS[@]}"; do
  id="${id//[[:space:]]/}"
  rule_line="$(grep -E "^sonar\.issue\.ignore\.multicriteria\.${id}\.ruleKey=" "${PROPS}" || true)"
  res_line="$(grep -E "^sonar\.issue\.ignore\.multicriteria\.${id}\.resourceKey=" "${PROPS}" || true)"
  [[ -n "${rule_line}" ]] || fail "multicriteria id '${id}' has no ruleKey line"
  [[ -n "${res_line}" ]] || fail "multicriteria id '${id}' has no resourceKey line"
  [[ "${rule_line#*=}" == "go:S1313" ]] || fail "multicriteria id '${id}' ruleKey is not go:S1313 (got: ${rule_line#*=})"
  res_value="${res_line#*=}"
  case "${res_value}" in
    *internal/sink/netguard/*) netguard_covered=1 ;;
    *internal/validation/endpoint_guard.go) endpointguard_covered=1 ;;
  esac
done
pass "every multicriteria entry is a go:S1313 ignore with a resourceKey"

((netguard_covered == 1)) || fail "no S1313 multicriteria entry covers internal/sink/netguard/**"
((endpointguard_covered == 1)) || fail "no S1313 multicriteria entry covers internal/validation/endpoint_guard.go"
pass "S1313 ignore covers both netguard/** and endpoint_guard.go"

echo "All sonar_ko_10 S1313 multicriteria tests passed."
