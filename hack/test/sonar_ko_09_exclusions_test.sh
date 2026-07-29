#!/usr/bin/env bash
# Unit test for SEC-04i / KO-09: hack/demo/** and hack/loadtest/** (demo and
# load-test scripts that curl/kubectl against a live cluster, not production
# code) must not surface SonarCloud SECURITY noise, while real production
# paths must stay analyzed.
#
# sonar.exclusions removes matched files from the analysis file set before
# any sensor runs, so it suppresses issues of every category (Bugs, Code
# Smells, Vulnerabilities, Security Hotspots) uniformly -- there is no
# separate "security exclusions" property in SonarQube/SonarCloud. This test
# parses the real sonar.exclusions value out of sonar-project.properties and
# proves, via an Ant-style path-glob matcher (the same segment semantics
# SonarCloud uses), that:
#   1. representative hack/demo/** and hack/loadtest/** files are covered by
#      at least one exclusion pattern, and
#   2. a representative production Go file is NOT covered by any pattern.
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

# --- Ant-style segment glob matcher (classic wildcard-with-backtracking
# algorithm, adapted so a whole "**" path segment plays the role of a
# single-char '*' in the original algorithm, and other segments are matched
# with bash's native fnmatch via [[ == ]] so *,? still work within a segment,
# e.g. "*_test.go" or "zz_generated.*"). ---
ant_match() {
  local pattern="$1" path="$2"
  local -a p s
  IFS='/' read -r -a p <<<"${pattern}"
  IFS='/' read -r -a s <<<"${path}"
  local m=${#p[@]} n=${#s[@]}
  local i=0 j=0 star=-1 mark=0
  while ((j < n)); do
    # shellcheck disable=SC2053 # intentional: glob-match segment against p[i]
    if ((i < m)) && [[ "${p[i]}" != "**" ]] && [[ "${s[j]}" == ${p[i]} ]]; then
      ((i++))
      ((j++))
    elif ((i < m)) && [[ "${p[i]}" == "**" ]]; then
      star=$i
      mark=$j
      ((i++))
    elif ((star != -1)); then
      i=$((star + 1))
      ((mark++))
      j=${mark}
    else
      return 1
    fi
  done
  while ((i < m)) && [[ "${p[i]}" == "**" ]]; do
    ((i++))
  done
  ((i == m))
}

# any_pattern_matches PATH PATTERN...  -- true if PATH matches any pattern.
any_pattern_matches() {
  local path="$1"
  shift
  local pat
  for pat in "$@"; do
    if ant_match "${pat}" "${path}"; then
      return 0
    fi
  done
  return 1
}

# --- Extract sonar.exclusions= value (comma-separated glob list). ---
excl_line="$(grep -E '^sonar\.exclusions=' "${PROPS}" || true)"
[[ -n "${excl_line}" ]] || fail "sonar.exclusions= not found in ${PROPS}"
excl_value="${excl_line#sonar.exclusions=}"
IFS=',' read -r -a EXCLUSIONS <<<"${excl_value}"
((${#EXCLUSIONS[@]} > 0)) || fail "sonar.exclusions= parsed to zero patterns"

# --- Confirm the demo/loadtest dirs this lane targets actually exist, so the
# test can't silently pass against paths that were renamed/removed. ---
[[ -d "${ROOT}/hack/demo" ]] || fail "hack/demo does not exist; adjust target paths"
[[ -d "${ROOT}/hack/loadtest" ]] || fail "hack/loadtest does not exist; adjust target paths"

# --- Representative demo/loadtest files: must be covered by sonar.exclusions. ---
DEMO_LOADTEST_SAMPLES=(
  "hack/demo/hero/up.sh"
  "hack/demo/hero/bootstrap-forgejo.sh"
  "hack/demo/kind-wide-scope/demo.sh"
  "hack/loadtest/100k/generate.sh"
  "hack/loadtest/gke-lab/README.md"
)
for sample in "${DEMO_LOADTEST_SAMPLES[@]}"; do
  [[ -e "${ROOT}/${sample}" ]] || fail "sample fixture ${sample} does not exist on disk; update sample list"
  if ! any_pattern_matches "${sample}" "${EXCLUSIONS[@]}"; then
    fail "expected sonar.exclusions to cover ${sample}, but no pattern matched (patterns: ${excl_value})"
  fi
done
pass "hack/demo/** and hack/loadtest/** samples are covered by sonar.exclusions"

# --- Representative production files: must NOT be excluded (no over-exclusion). ---
PRODUCTION_SAMPLES=(
  "internal/metrics/metrics.go"
  "internal/pipeline/loader.go"
)
for sample in "${PRODUCTION_SAMPLES[@]}"; do
  [[ -e "${ROOT}/${sample}" ]] || fail "sample fixture ${sample} does not exist on disk; update sample list"
  if any_pattern_matches "${sample}" "${EXCLUSIONS[@]}"; then
    fail "production path ${sample} is unexpectedly excluded by sonar.exclusions (patterns: ${excl_value})"
  fi
done
pass "production paths (internal/**) remain analyzed"

# --- Self-check the matcher against known-shape cases, so a future edit to
# ant_match can't silently degrade into a no-op that trivially "passes"
# everything (or nothing). ---
ant_match "**/hack/**" "hack/demo/hero/up.sh" || fail "matcher self-check: **/hack/** should match hack/demo/hero/up.sh"
if ant_match "**/hack/**" "internal/metrics/metrics.go"; then
  fail "matcher self-check: **/hack/** should not match internal/metrics/metrics.go"
fi
pass "ant_match self-check passed"

echo "All sonar_ko_09 exclusion tests passed."
