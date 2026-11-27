#!/usr/bin/env bash
# Guards for the golangci-lint tooling targets.
#
# format:check: its golangci-lint step must fail on a formatting diff and on a linter error. It
# used to send stderr to /dev/null and test only for empty output, so a linter that failed to
# start (bad config, wrong binary) passed as "nothing to format".
#
# make golangci-lint: a version bump or a newer .custom-gcl.yml must rebuild. The target used to
# be the binary itself, a plain file after the custom build, so a bump left the old linter.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

pass() {
  echo "ok - $*"
}

command -v yq >/dev/null 2>&1 || fail "yq is required to run this test"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

# run_fmt_step <exit> <stdout> <stderr>: runs format:check's golangci-lint command from
# Taskfile.yml against a stub bin/golangci-lint; prints the step's exit status.
STEP="$(yq eval '.tasks["format:check"].cmds[] | select(test("golangci-lint fmt"))' "${ROOT}/Taskfile.yml")"
[[ -n "${STEP}" ]] || fail "format:check has no golangci-lint fmt step"
run_fmt_step() {
  mkdir -p "${TMP}/fmt/bin"
  printf '#!/bin/sh\nprintf "%%s" "%s"\nprintf "%%s" "%s" >&2\nexit %s\n' "$2" "$3" "$1" >"${TMP}/fmt/bin/golangci-lint"
  chmod +x "${TMP}/fmt/bin/golangci-lint"
  (cd "${TMP}/fmt" && bash -c "${STEP}" >/dev/null 2>&1) && echo 0 || echo 1
}

[[ "$(run_fmt_step 0 '' '')" == 0 ]] || fail "format:check fails on a clean tree"
pass "format:check passes a clean tree"
[[ "$(run_fmt_step 1 'diff --git a/x.go b/x.go' '')" == 1 ]] || fail "format:check passes a formatting diff"
pass "format:check fails on a formatting diff"
[[ "$(run_fmt_step 0 'diff --git a/x.go b/x.go' '')" == 1 ]] || fail "format:check passes a diff printed with exit 0"
pass "format:check fails on a diff even when the linter exits 0"
[[ "$(run_fmt_step 3 '' 'config error')" == 1 ]] || fail "format:check passes when golangci-lint itself fails"
pass "format:check fails when golangci-lint itself fails"

# plans_install <version>: does `make -n golangci-lint` against a LOCALBIN holding an installed
# v2.15.0 (binary plus stamp) plan a go install for <version>?
plans_install() {
  make -n --no-print-directory -C "${ROOT}" golangci-lint LOCALBIN="${TMP}/bin" GOLANGCI_LINT_VERSION="$1" 2>/dev/null |
    grep -q 'go install' && echo yes || echo no
}
mkdir -p "${TMP}/bin"
: >"${TMP}/bin/golangci-lint"
: >"${TMP}/bin/.golangci-lint-v2.15.0.stamp"
[[ "$(plans_install v2.15.0)" == no ]] || fail "make golangci-lint rebuilds an up-to-date pin"
pass "make golangci-lint leaves an up-to-date pin alone"
[[ "$(plans_install v2.99.0)" == yes ]] || fail "make golangci-lint keeps the old binary after a version bump"
pass "make golangci-lint rebuilds after a version bump"
if [[ -f "${ROOT}/hack/tooling/.custom-gcl.yml" ]]; then
  touch -t 200001010000 "${TMP}/bin/.golangci-lint-v2.15.0.stamp"
  [[ "$(plans_install v2.15.0)" == yes ]] || fail "make golangci-lint ignores a newer .custom-gcl.yml"
  pass "make golangci-lint rebuilds after a .custom-gcl.yml change"
fi

echo "All lint tool target tests passed."
