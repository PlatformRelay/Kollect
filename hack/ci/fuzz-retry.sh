#!/usr/bin/env bash
# Run one native Go fuzz target for ci.yaml's `fuzz` matrix leg (TDS-1, openspec change
# test-depth-signals).
#
#   hack/ci/fuzz-retry.sh <FuzzTarget> <pkg>
#
# Runs `go test -run='^$' -fuzz=<target> -fuzztime=30s <pkg>` and retries it at most once, and
# only when the failed attempt is exactly the fuzz engine's "context deadline exceeded" flake at
# the -fuzztime boundary (golang/go#75804) and wrote no crash corpus. Everything else fails at
# once, fail closed:
#   * a deny marker (panic:, fatal error:, Failing input written, signal: killed) is a real
#     finding or a killed process, never a flake;
#   * the flake is an exact, ordered line sequence (captured from runs 37911142180 and
#     37288739312) for the matrix target; any other text, a missing, repeated or reordered line
#     included, is `unrecognised failure text`, so a change in Go's wording shows in the log;
#   * a path under testdata/fuzz/ in git status, listing every untracked file (plain --porcelain
#     collapses a new untracked testdata/ to one directory line), or a git status that fails;
#   * an exit 0 that fuzzed nothing: no `fuzz: elapsed` line.
# The runner's own diagnostics are the `::error::fuzz-retry:` lines; tested by
# hack/test/fuzz_retry_test.sh.
set -uo pipefail

diag() {
  echo "::error::fuzz-retry: $1"
}

unrecognised() {
  diag 'unrecognised failure text'
  exit 1
}

if [[ "$#" -ne 2 || ! "${1}" =~ ^Fuzz[A-Za-z0-9_]*$ || -z "${2}" ]]; then
  diag "usage: fuzz-retry.sh <FuzzTarget> <pkg>"
  exit 2
fi
target="$1"
pkg="$2"

deny_markers=('panic:' 'fatal error:' 'Failing input written' 'signal: killed')

# The deadline flake, once `fuzz:` progress lines are removed: these lines, each exactly once and
# in this order, naming the matrix target.
is_deadline_flake() {
  local -a want=(
    "^--- FAIL: ${target} \\([0-9]+(\\.[0-9]+)?s\\)\$"
    '^    context deadline exceeded$'
    '^FAIL$'
    '^exit status 1$'
    $'^FAIL\t[^[:space:]]+\t[0-9]+(\\.[0-9]+)?s$'
  )
  local -a got=()
  local i
  mapfile -t got < <(grep -v '^fuzz: ' "$1")
  [[ "${#got[@]}" -eq "${#want[@]}" ]] || return 1
  for i in "${!got[@]}"; do
    [[ "${got[i]}" =~ ${want[i]} ]] || return 1
  done
}

out="$(mktemp)"
trap 'rm -f "${out}"' EXIT

max_attempts=2
attempt=1
while :; do
  go test -run='^$' -fuzz="${target}" -fuzztime=30s "${pkg}" 2>&1 | tee "${out}"
  rc="${PIPESTATUS[0]}"

  # Before the exit status: a deny marker fails even an attempt that exited 0.
  for marker in "${deny_markers[@]}"; do
    if grep -F -- "${marker}" "${out}" >/dev/null; then
      diag "deny marker ${marker}"
      exit 1
    fi
  done

  if [[ "${rc}" -eq 0 ]]; then
    if grep '^fuzz: elapsed' "${out}" >/dev/null; then
      exit 0
    fi
    if grep -F 'no fuzz tests to fuzz' "${out}" >/dev/null; then
      diag "no fuzz target ran"
      exit 1
    fi
    unrecognised
  fi

  is_deadline_flake "${out}" || unrecognised

  status="$(git status --porcelain --untracked-files=all)" || { diag "git status failed"; exit 1; }
  if grep -F 'testdata/fuzz/' <<<"${status}" >/dev/null; then
    diag "crash corpus under testdata/fuzz/"
    exit 1
  fi

  if [[ "${attempt}" -ge "${max_attempts}" ]]; then
    diag "deadline flake on both attempts"
    exit 1
  fi
  echo "::warning::fuzz-retry: ${target} hit the fuzz engine's \"context deadline exceeded\" flake at the -fuzztime boundary (golang/go#75804) on attempt ${attempt}; retrying once"
  attempt=$((attempt + 1))
done
