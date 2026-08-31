#!/usr/bin/env bash
# DEV-MISE-01: mise.toml must not drift from the versions CI actually installs.
#
# mise.toml exists for exactly one reason -- so a contributor's local toolchain is the one the
# merge gates run against. That premise holds only while the numbers agree. A mise.toml that
# DISAGREES with CI is worse than no mise.toml at all: it hands contributors a toolchain that
# silently differs from the gates, and nothing anywhere would say so.
#
# Go needs no value assertion. mise reads it out of go.mod via
# `idiomatic_version_file_enable_tools = ["go"]` -- the same file actions/setup-go reads through
# `go-version-file: go.mod` -- so the two structurally cannot disagree. What this gate asserts
# for Go is that structural property itself: `go` still absent from [tools], the setting still
# present, go.mod still carrying a `go` line. A well-meaning `go = "1.26.6"` added to [tools]
# would silently win over go.mod and reintroduce precisely the drift the design removed.
#
# task / node / python ARE restated literals, so they need a mechanism, and prose is not one.
# renovate.json's own comment records what happens without one: "...without this they land in
# different groups and drift apart -- which is exactly how release.yaml reached v3.21.4 while CI
# stayed on v3.17.3." The two live drift vectors here:
#   * Renovate's go-task customManager scans /^\.github/(actions|workflows)/.+\.ya?ml$/ and will
#     bump every setup-task site on the next go-task release.
#   * NO manager in this repo touches the `node-version:` / `python-version:` workflow inputs
#     (the github-actions manager handles `uses:` refs only), while Renovate's mise manager does
#     understand core node/python -- so mise.toml could run AHEAD of CI.
# renovate.json is configured to move mise.toml together with the workflows and to leave the
# mise manager off; this gate is the fail-closed backstop that does not depend on any of that
# configuration being right.
#
# Scope: every .github/workflows/*.y{,a}ml AND .github/actions/*/action.y{,a}ml -- composite
# actions included, because .github/actions/kind-e2e-setup/action.yml pins setup-task too and
# Renovate's manager covers it. Scanning both also means CI disagreeing with ITSELF reds here.
#
# Every extraction is guarded against finding nothing: an empty scan set is a hard failure, never
# a quiet pass. This repo has shipped vacuous gates before. The self-test at the bottom proves
# both directions on throwaway trees every run -- including that each vacuity guard actually
# fires -- because a gate that has never been watched failing is not a gate.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly ROOT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() { echo "ok - $*"; }

# Every YAML under .github that can carry a tool pin. A missing directory is not an error here
# (a fixture may have only one of them); an empty result is caught by the vacuity guards below.
ci_files() {
  local root="$1" dir
  for dir in "${root}/.github/workflows" "${root}/.github/actions"; do
    [[ -d "${dir}" ]] || continue
    find "${dir}" -type f \( -name '*.yml' -o -name '*.yaml' \)
  done | sort
}

# Every `version:` input given to go-task/setup-task, one per line.
setup_task_versions() {
  local root="$1" file
  while IFS= read -r file; do
    [[ -n "${file}" ]] || continue
    awk '
      /^[[:space:]]*#/ { next }
      /uses:[[:space:]]*go-task\/setup-task@/ { want = 1; next }
      want && /^[[:space:]]*version:[[:space:]]*/ {
        line = $0
        sub(/^[[:space:]]*version:[[:space:]]*/, "", line)
        sub(/[[:space:]]*#.*$/, "", line)
        gsub(/["\047[:space:]]/, "", line)
        if (line != "") print line
        want = 0
        next
      }
      # A new step before any version: means that use carried no pin to read.
      want && /^[[:space:]]*-[[:space:]]*(uses|name|run):/ { want = 0 }
    ' "${file}"
  done < <(ci_files "${root}")
}

# Every value of a plain workflow input key (node-version, python-version), one per line.
input_versions() {
  local root="$1" key="$2" file
  while IFS= read -r file; do
    [[ -n "${file}" ]] || continue
    awk -v key="${key}" '
      /^[[:space:]]*#/ { next }
      $0 ~ "^[[:space:]]*" key ":[[:space:]]*" {
        line = $0
        sub("^[[:space:]]*" key ":[[:space:]]*", "", line)
        sub(/[[:space:]]*#.*$/, "", line)
        gsub(/["\047[:space:]]/, "", line)
        if (line != "") print line
      }
    ' "${file}"
  done < <(ci_files "${root}")
}

# The value of one key inside mise.toml's [tools] table. Empty when absent.
mise_tool_value() {
  local file="$1" tool="$2"
  [[ -f "${file}" ]] || return 0
  awk -v tool="${tool}" '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*\[/ { section = $0; gsub(/[[:space:]]/, "", section); next }
    section == "[tools]" && $0 ~ "^[[:space:]]*" tool "[[:space:]]*=" {
      line = $0
      sub("^[[:space:]]*" tool "[[:space:]]*=[[:space:]]*", "", line)
      sub(/[[:space:]]*#.*$/, "", line)
      gsub(/["\047[:space:]]/, "", line)
      print line
    }
  ' "${file}"
}

# Compare one mise.toml literal against the CI pins it mirrors. Prints every failure and returns
# nonzero if any fired. An empty CI scan or an empty mise value is itself a failure -- that is
# the whole difference between this gate and one that passes because it looked at nothing.
assert_pin() {
  local root="$1" label="$2" mise_value="$3" ci_list="$4" origin="$5"
  local distinct count

  if [[ -z "${ci_list}" ]]; then
    echo "FAIL: no ${origin} found under ${root}/.github -- the scan set is empty, so the ${label} check would pass without comparing anything" >&2
    return 1
  fi

  distinct="$(printf '%s\n' "${ci_list}" | sort -u)"
  count="$(printf '%s\n' "${distinct}" | grep -c '')"
  if [[ "${count}" -ne 1 ]]; then
    echo "FAIL: CI disagrees with itself on ${label} -- ${origin} yields $(printf '%s' "${distinct}" | tr '\n' ' ')" >&2
    return 1
  fi

  if [[ -z "${mise_value}" ]]; then
    echo "FAIL: mise.toml has no ${label} entry under [tools], but CI pins ${label} ${distinct} -- the local toolchain is unpinned while the gates are not" >&2
    return 1
  fi

  if [[ "${mise_value}" != "${distinct}" ]]; then
    echo "FAIL: mise.toml pins ${label} = ${mise_value} but ${origin} pins ${distinct} -- contributors would run a different ${label} than the merge gates" >&2
    return 1
  fi

  return 0
}

# The whole contract for one tree. Parameterised by root so the self-test exercises exactly the
# code path the real repository takes, on throwaway fixtures.
check_mise_pins() {
  local root="$1"
  local mise="${root}/mise.toml" gomod="${root}/go.mod"
  local broken=0 go_pin

  if [[ ! -f "${mise}" ]]; then
    echo "FAIL: ${mise} does not exist -- this gate exists to keep it honest, so its absence is a failure, not a skip" >&2
    return 1
  fi

  if ! grep -Eq '^[[:space:]]*\[tools\][[:space:]]*$' "${mise}"; then
    echo "FAIL: ${mise} has no [tools] table -- every pin lookup below would return empty and pass vacuously" >&2
    return 1
  fi

  # --- Go: the structural guarantee, not a number ------------------------------------------
  go_pin="$(mise_tool_value "${mise}" go)"
  if [[ -n "${go_pin}" ]]; then
    echo "FAIL: mise.toml pins go = ${go_pin} under [tools]; that value WINS over go.mod and goes stale the next time go.mod is bumped. Remove it and let idiomatic_version_file_enable_tools read go.mod, exactly as actions/setup-go does via go-version-file" >&2
    broken=1
  fi
  if ! grep -Eq '^[[:space:]]*idiomatic_version_file_enable_tools[[:space:]]*=.*"go"' "${mise}"; then
    echo "FAIL: mise.toml no longer enables the idiomatic version file for go -- without idiomatic_version_file_enable_tools = [\"go\"] mise stops reading go.mod and Go silently becomes unmanaged" >&2
    broken=1
  fi
  if [[ ! -f "${gomod}" ]] || ! grep -Eq '^go[[:space:]]+[0-9]' "${gomod}"; then
    echo "FAIL: ${gomod} has no 'go <version>' line -- the file mise and actions/setup-go both read has nothing to read, so the Go half of this contract is vacuous" >&2
    broken=1
  fi

  # --- task / node / python: restated literals, so compare them -----------------------------
  assert_pin "${root}" task "$(mise_tool_value "${mise}" task)" \
    "$(setup_task_versions "${root}")" "the go-task/setup-task version: inputs" || broken=1
  assert_pin "${root}" node "$(mise_tool_value "${mise}" node)" \
    "$(input_versions "${root}" node-version)" "the actions/setup-node node-version: inputs" || broken=1
  assert_pin "${root}" python "$(mise_tool_value "${mise}" python)" \
    "$(input_versions "${root}" python-version)" "the actions/setup-python python-version: inputs" || broken=1

  return "${broken}"
}

# ---------------------------------------------------------------------------
# The real repository.
# ---------------------------------------------------------------------------
if check_mise_pins "${ROOT}"; then
  pass "mise.toml agrees with every task/node/python pin in .github, and Go is still read from go.mod"
else
  fail "mise.toml has drifted from CI (listed above) -- a local toolchain that differs from the gates is worse than no mise.toml at all"
fi

# A gate nobody runs protects nothing. Assert the CI step exists rather than trusting that it
# still does -- this catches the common accident (the step is dropped while the script stays).
if grep -q 'hack/test/dev_mise_pin_drift_test\.sh' "${ROOT}/.github/workflows/ci.yaml"; then
  pass "the gate is wired into .github/workflows/ci.yaml"
else
  fail "hack/test/dev_mise_pin_drift_test.sh is not invoked from .github/workflows/ci.yaml -- an unwired gate is not a gate"
fi

# ---------------------------------------------------------------------------
# Self-test. Both directions, on throwaway trees, every run.
# ---------------------------------------------------------------------------
FIXTURE="$(mktemp -d)"
trap 'rm -rf "${FIXTURE}"' EXIT

# A minimal but structurally faithful tree: one workflow AND one composite action, so the
# multi-file scan and the "CI disagrees with itself" path are both really exercised.
make_fixture() {
  local dir="$1" task_a="$2" task_b="$3" node="$4" python="$5" mise_body="$6"
  mkdir -p "${dir}/.github/workflows" "${dir}/.github/actions/go-cache"
  printf 'module x\n\ngo 1.26.6\n' >"${dir}/go.mod"
  {
    printf 'jobs:\n  lint:\n    steps:\n'
    if [[ -n "${task_a}" ]]; then
      printf '      - uses: go-task/setup-task@abc # v2.2.0\n        with:\n          version: %s\n' "${task_a}"
    fi
    if [[ -n "${node}" ]]; then
      printf '      - uses: actions/setup-node@abc # v7.0.0\n        with:\n          node-version: "%s"\n' "${node}"
    fi
    if [[ -n "${python}" ]]; then
      printf '      - uses: actions/setup-python@abc # v7.0.0\n        with:\n          python-version: "%s"\n' "${python}"
    fi
  } >"${dir}/.github/workflows/ci.yaml"
  {
    printf 'runs:\n  using: composite\n  steps:\n'
    if [[ -n "${task_b}" ]]; then
      printf '    - uses: go-task/setup-task@abc # v2.2.0\n      with:\n        version: %s\n' "${task_b}"
    fi
  } >"${dir}/.github/actions/go-cache/action.yml"
  printf '%s' "${mise_body}" >"${dir}/mise.toml"
}

HONEST_MISE='[settings]
idiomatic_version_file_enable_tools = ["go"]

[tools]
task = "3.51.1"
node = "22"
python = "3.12"
'

# Run the checker in a subshell so an internal hard `fail` is reported here rather than taking
# the whole gate down with its own message. Status and output travel together.
run_case() {
  local dir="$1"
  local status=0 output
  output="$( (check_mise_pins "${dir}") 2>&1 )" || status=$?
  printf '%s\n%s' "${status}" "${output}"
}

expect_green() {
  local dir="$1" label="$2" result status
  result="$(run_case "${dir}")"
  status="${result%%$'\n'*}"
  [[ "${status}" -eq 0 ]] ||
    fail "self-test: the gate rejected ${label}, so it would red the build on correct content; got: ${result#*$'\n'}"
  pass "self-test: ${label} passes"
}

expect_red() {
  local dir="$1" label="$2" needle="$3" result status output
  result="$(run_case "${dir}")"
  status="${result%%$'\n'*}"
  output="${result#*$'\n'}"
  [[ "${status}" -ne 0 ]] ||
    fail "self-test: the gate PASSED on ${label} -- it is vacuous"
  [[ "${output}" == *"${needle}"* ]] ||
    fail "self-test: ${label} was rejected for the wrong reason (expected it to mention '${needle}'); got: ${output}"
  pass "self-test: gate rejects ${label}"
}

# GREEN: an honest tree passes.
make_fixture "${FIXTURE}/honest" 3.51.1 3.51.1 22 3.12 "${HONEST_MISE}"
expect_green "${FIXTURE}/honest" "a tree whose mise.toml matches every CI pin"

# RED: each restated literal, one at a time. The failure must name BOTH sides, or a maintainer
# cannot act on it without re-deriving the whole comparison by hand.
make_fixture "${FIXTURE}/task-drift" 3.51.1 3.51.1 22 3.12 \
  "${HONEST_MISE//task = \"3.51.1\"/task = \"3.50.0\"}"
expect_red "${FIXTURE}/task-drift" "a mise.toml pinning task 3.50.0 while CI pins 3.51.1" \
  "mise.toml pins task = 3.50.0"

make_fixture "${FIXTURE}/node-drift" 3.51.1 3.51.1 22 3.12 \
  "${HONEST_MISE//node = \"22\"/node = \"20\"}"
expect_red "${FIXTURE}/node-drift" "a mise.toml pinning node 20 while CI pins 22" \
  "mise.toml pins node = 20"

make_fixture "${FIXTURE}/python-drift" 3.51.1 3.51.1 22 3.12 \
  "${HONEST_MISE//python = \"3.12\"/python = \"3.11\"}"
expect_red "${FIXTURE}/python-drift" "a mise.toml pinning python 3.11 while CI pins 3.12" \
  "mise.toml pins python = 3.11"

# RED: the Go structural guarantee, in both directions it can be broken.
make_fixture "${FIXTURE}/go-pinned" 3.51.1 3.51.1 22 3.12 \
  "${HONEST_MISE//\[tools\]/[tools]$'\n'go = \"1.26.6\"}"
expect_red "${FIXTURE}/go-pinned" "a go pin added under [tools], which would win over go.mod" \
  "pins go = 1.26.6"

make_fixture "${FIXTURE}/go-setting-gone" 3.51.1 3.51.1 22 3.12 \
  "${HONEST_MISE//idiomatic_version_file_enable_tools = \[\"go\"\]/}"
expect_red "${FIXTURE}/go-setting-gone" "a mise.toml that stopped reading go.mod" \
  "no longer enables the idiomatic version file"

make_fixture "${FIXTURE}/gomod-gone" 3.51.1 3.51.1 22 3.12 "${HONEST_MISE}"
printf 'module x\n' >"${FIXTURE}/gomod-gone/go.mod"
expect_red "${FIXTURE}/gomod-gone" "a go.mod with no go directive for mise to read" \
  "has no 'go <version>' line"

# RED: CI disagreeing with itself. Bumping the workflows but not the composite action is the
# shape the helm v3.21.4-vs-v3.17.3 incident in renovate.json's comment actually took.
make_fixture "${FIXTURE}/ci-self-drift" 3.51.1 3.50.0 22 3.12 "${HONEST_MISE}"
expect_red "${FIXTURE}/ci-self-drift" "a CI tree pinning two different task versions" \
  "CI disagrees with itself on task"

# VACUITY: the guards themselves. Each of these would let a broken gate report success if the
# extraction silently yielded nothing -- the exact failure mode this repo has shipped before, so
# each one is asserted rather than assumed.
make_fixture "${FIXTURE}/no-ci-pins" "" "" "" "" "${HONEST_MISE}"
expect_red "${FIXTURE}/no-ci-pins" "a CI tree with no task/node/python pins at all (empty scan set)" \
  "the scan set is empty"

make_fixture "${FIXTURE}/empty-tools" 3.51.1 3.51.1 22 3.12 \
  '[settings]
idiomatic_version_file_enable_tools = ["go"]

[tools]
'
expect_red "${FIXTURE}/empty-tools" "a mise.toml with an empty [tools] table" \
  "has no task entry under [tools]"

make_fixture "${FIXTURE}/no-tools-table" 3.51.1 3.51.1 22 3.12 \
  '[settings]
idiomatic_version_file_enable_tools = ["go"]
'
expect_red "${FIXTURE}/no-tools-table" "a mise.toml with no [tools] table" \
  "has no [tools] table"

rm -f "${FIXTURE}/honest/mise.toml"
expect_red "${FIXTURE}/honest" "a tree with no mise.toml at all" "does not exist"

echo "All mise pin drift tests passed."
