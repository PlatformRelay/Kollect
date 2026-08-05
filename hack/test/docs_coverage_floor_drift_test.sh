#!/usr/bin/env bash
# DOC-02: documented current coverage floor must match executable COVERAGE_MIN
# (Taskfile.yml + .github/workflows/ci.yaml + hack/coverage.sh fallback) and
# Codecov project target.
#
# Includes a fixture self-check: an injected mismatch must fail closed before
# the real-repo assertion runs.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'docs coverage floor drift: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

# Extract integer floors from a synthetic or real repo root.
# Prints unique values found on stdout; returns 1 on missing inputs.
collect_floors() {
  local root="$1"
  local taskfile="${root}/Taskfile.yml"
  local ci="${root}/.github/workflows/ci.yaml"
  local coverage_sh="${root}/hack/coverage.sh"
  local codecov="${root}/codecov.yml"
  local testing="${root}/docs/development/testing.md"
  local standards="${root}/docs/development/coding-standards.md"
  local adr="${root}/docs/adr/0706-testing-merge-gate-architecture.md"
  local f

  for f in "${taskfile}" "${ci}" "${coverage_sh}" "${codecov}" "${testing}" "${standards}" "${adr}"; do
    if [[ ! -f "${f}" ]]; then
      printf 'missing required source: %s\n' "${f}" >&2
      return 1
    fi
  done

  # Executable: every COVERAGE_MIN assignment in Taskfile + merge-gate ci.yaml
  # + the bare-script fallback in hack/coverage.sh.
  # (sonarcloud.yaml keeps a separate advisory floor and is intentionally excluded.)
  {
    sed -n 's/^[[:space:]]*COVERAGE_MIN:[[:space:]]*"\([0-9][0-9]*\)".*/\1/p' "${taskfile}"
    sed -n 's/^[[:space:]]*COVERAGE_MIN:[[:space:]]*"\([0-9][0-9]*\)".*/\1/p' "${ci}"
    # Bare-script fallback: MIN="${COVERAGE_MIN:-NN}"
    sed -n 's/.*\${COVERAGE_MIN:-\([0-9][0-9]*\)}.*/\1/p' "${coverage_sh}"
    # Codecov project target (enforced threshold companion to the CI floor).
    sed -n 's/^[[:space:]]*target:[[:space:]]*\([0-9][0-9]*\)%.*/\1/p' "${codecov}"
    # Living docs: current-floor claims only (not historical ADR ratchet rows).
    sed -n 's/.*\*\*Current CI floor\*\*.*\*\*\([0-9][0-9]*\)%\*\*.*/\1/p' "${testing}"
    sed -n 's/.*\*\*Coverage floor\*\*.*\*\*\([0-9][0-9]*\)%\*\*.*/\1/p' "${standards}"
    # ADR-0706 "Now (PR / main)" row is the living current claim.
    sed -n 's/.*\*\*Now (PR \/ [^)]*)\*\*.*\*\*\([0-9][0-9]*\)%\*\*.*/\1/p' "${adr}"
  } | awk 'NF' | sort -u
}

# Returns 0 and prints the shared floor when all sources agree; else 1 + stderr.
assert_floors_agree() {
  local root="$1"
  local label="$2"
  local floors
  local count

  if ! floors="$(collect_floors "${root}")"; then
    printf '%s: %s\n' "${label}" "could not collect floors" >&2
    return 1
  fi
  count="$(printf '%s\n' "${floors}" | grep -c . || true)"
  if [[ "${count}" -eq 0 ]]; then
    printf '%s: no coverage floor values found\n' "${label}" >&2
    return 1
  fi
  if [[ "${count}" -ne 1 ]]; then
    printf '%s: coverage floor drift — sources disagree: %s\n' \
      "${label}" "$(printf '%s' "${floors}" | tr '\n' ' ')" >&2
    return 1
  fi
  printf '%s\n' "${floors}"
}

write_fixture_tree() {
  local dest="$1"
  local exec_min="$2"
  local doc_min="$3"
  local codecov_min="$4"
  local coverage_sh_min="${5:-$exec_min}"

  mkdir -p "${dest}/.github/workflows" "${dest}/hack" \
    "${dest}/docs/development" "${dest}/docs/adr"

  cat >"${dest}/Taskfile.yml" <<EOF
env:
  COVERAGE_MIN: "${exec_min}"
vars:
  COVERAGE_MIN: "${exec_min}"
EOF

  cat >"${dest}/.github/workflows/ci.yaml" <<EOF
jobs:
  test:
    env:
      COVERAGE_MIN: "${exec_min}"
EOF

  cat >"${dest}/hack/coverage.sh" <<EOF
#!/usr/bin/env bash
MIN="\${COVERAGE_MIN:-${coverage_sh_min}}"
EOF

  cat >"${dest}/codecov.yml" <<EOF
coverage:
  status:
    project:
      default:
        target: ${codecov_min}%
EOF

  cat >"${dest}/docs/development/testing.md" <<EOF
| **Current CI floor** | **${doc_min}%** (\`COVERAGE_MIN\` in \`Taskfile.yml\` and \`.github/workflows/ci.yaml\`) |
EOF

  cat >"${dest}/docs/development/coding-standards.md" <<EOF
| **Coverage floor** | **${doc_min}%** statement coverage on \`./internal/...\` (\`task coverage\`, \`COVERAGE_MIN\`) |
EOF

  cat >"${dest}/docs/adr/0706-testing-merge-gate-architecture.md" <<EOF
| **Now (PR / \`main\`)** | **${doc_min}%** | \`.github/workflows/ci.yaml\`, \`Taskfile.yml\` |
EOF
}

# --- RED self-check: injected doc/exec mismatch must fail closed ---
write_fixture_tree "${TMP}/mismatch" "87" "85" "87"
if assert_floors_agree "${TMP}/mismatch" "fixture-mismatch" >/dev/null 2>"${TMP}/mismatch.err"; then
  fail "injected doc/exec mismatch must fail"
fi
grep -q 'coverage floor drift' "${TMP}/mismatch.err" ||
  fail "mismatch must report drift; got: $(cat "${TMP}/mismatch.err")"
pass "injected doc/exec mismatch fails closed"

# Codecov target drift must also fail.
write_fixture_tree "${TMP}/codecov-drift" "87" "87" "90"
if assert_floors_agree "${TMP}/codecov-drift" "fixture-codecov" >/dev/null 2>"${TMP}/codecov.err"; then
  fail "injected codecov target drift must fail"
fi
grep -q 'coverage floor drift' "${TMP}/codecov.err" ||
  fail "codecov drift must report; got: $(cat "${TMP}/codecov.err")"
pass "injected codecov target drift fails closed"

# hack/coverage.sh fallback drift must also fail (DOC-02-F2).
write_fixture_tree "${TMP}/coverage-sh-drift" "87" "87" "87" "85"
if assert_floors_agree "${TMP}/coverage-sh-drift" "fixture-coverage-sh" >/dev/null 2>"${TMP}/coverage-sh.err"; then
  fail "injected coverage.sh fallback drift must fail"
fi
grep -q 'coverage floor drift' "${TMP}/coverage-sh.err" ||
  fail "coverage.sh drift must report; got: $(cat "${TMP}/coverage-sh.err")"
pass "injected coverage.sh fallback drift fails closed"

# Matching fixtures must pass (proves the checker accepts parity).
write_fixture_tree "${TMP}/match" "87" "87" "87"
match_floor="$(assert_floors_agree "${TMP}/match" "fixture-match")"
[[ "${match_floor}" == "87" ]] || fail "matching fixture expected 87, got ${match_floor}"
pass "matching fixture agrees at 87"

# --- GREEN: real repository sources ---
repo_floor="$(assert_floors_agree "${ROOT}" "repo")" ||
  fail "repo sources disagree — see stderr above"
[[ "${repo_floor}" == "90" ]] ||
  fail "repo coverage floor is ${repo_floor}, expected 90 (update docs + Taskfile + ci.yaml + codecov + coverage.sh together)"
pass "Taskfile.yml, ci.yaml, coverage.sh, codecov.yml, and current-floor docs agree at ${repo_floor}"

printf 'docs coverage floor drift: ok (floor=%s)\n' "${repo_floor}"
