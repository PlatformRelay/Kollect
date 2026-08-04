#!/usr/bin/env bash
# Meta test for SONAR-QUAL-01 (docker:S8431 "Dockerfile should have a HEALTHCHECK").
#
# Operator/runtime images are health-checked by Kubernetes probes (liveness/
# readiness), not Docker HEALTHCHECK. Adding HEALTHCHECK to bookworm-slim /
# pipeline images would duplicate that contract and often needs a shell/curl
# that we deliberately avoid on the runtime USER. Suppress docker:S8431 narrowly
# for the two product Dockerfiles via sonar.issue.ignore.multicriteria.
#
# Structural invariant only — proves the exclusion is declared as intended and
# does not silently disappear or widen; live SonarCloud analysis confirms the
# suppression fires.
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
[[ -f "${ROOT}/Dockerfile" ]] || fail "Dockerfile missing; adjust docker:S8431 exclusion"
[[ -f "${ROOT}/Dockerfile.pipeline" ]] || fail "Dockerfile.pipeline missing; adjust docker:S8431 exclusion"

master_line="$(grep -E '^sonar\.issue\.ignore\.multicriteria=' "${PROPS}" || true)"
[[ -n "${master_line}" ]] || fail "sonar.issue.ignore.multicriteria= master key not found in ${PROPS}"
master_value="${master_line#sonar.issue.ignore.multicriteria=}"
IFS=',' read -r -a IDS <<<"${master_value}"

dockerfile_covered=0
pipeline_covered=0
s8431_count=0
for id in "${IDS[@]}"; do
  id="${id//[[:space:]]/}"
  rule_line="$(grep -E "^sonar\.issue\.ignore\.multicriteria\.${id}\.ruleKey=" "${PROPS}" || true)"
  res_line="$(grep -E "^sonar\.issue\.ignore\.multicriteria\.${id}\.resourceKey=" "${PROPS}" || true)"
  [[ -n "${rule_line}" ]] || fail "multicriteria id '${id}' has no ruleKey line"
  [[ -n "${res_line}" ]] || fail "multicriteria id '${id}' has no resourceKey line"
  rule_value="${rule_line#*=}"
  [[ "${rule_value}" == "docker:S8431" ]] || continue
  s8431_count=$((s8431_count + 1))
  res_value="${res_line#*=}"
  case "${res_value}" in
    *Dockerfile.pipeline*) pipeline_covered=1 ;;
    *Dockerfile*) dockerfile_covered=1 ;;
  esac
done

((s8431_count >= 2)) || fail "expected at least two docker:S8431 multicriteria entries, found ${s8431_count}"
pass "docker:S8431 multicriteria entries present (${s8431_count})"

((dockerfile_covered == 1)) || fail "no docker:S8431 multicriteria entry covers Dockerfile"
((pipeline_covered == 1)) || fail "no docker:S8431 multicriteria entry covers Dockerfile.pipeline"
pass "docker:S8431 ignore covers Dockerfile and Dockerfile.pipeline"

echo "All sonar_ko_11 docker:S8431 multicriteria tests passed."
