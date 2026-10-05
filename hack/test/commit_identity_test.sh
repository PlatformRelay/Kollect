#!/usr/bin/env bash
# Behaviour of hack/check-commit-identity.sh on throwaway repositories.
# SPDX-License-Identifier: MIT
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
check="${repo_root}/hack/check-commit-identity.sh"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

fail() {
  printf 'commit identity test: %s\n' "$*" >&2
  exit 1
}

# commit <author name> <author email> [committer email] -- an empty commit in ${work}/repo
commit() {
  GIT_AUTHOR_NAME="$1" GIT_AUTHOR_EMAIL="$2" \
    GIT_COMMITTER_NAME="$1" GIT_COMMITTER_EMAIL="${3:-$2}" \
    git -C "${work}/repo" commit -q --allow-empty -m "c"
}

new_repo() {
  rm -rf "${work}/repo"
  git init -q "${work}/repo"
  commit "Base" "base@example.org"
  base="$(git -C "${work}/repo" rev-parse HEAD)"
}

expect_pass() {
  (cd "${work}/repo" && bash "${check}" "${base}" HEAD) >/dev/null 2>&1 ||
    fail "$1: rejected a valid range"
}

expect_fail() {
  local out
  if out="$(cd "${work}/repo" && bash "${check}" "${base}" HEAD 2>&1)"; then
    fail "$1: accepted an invalid range"
  fi
  grep -qF -- "$2" <<<"${out}" || fail "$1: output does not name ${2}: ${out}"
}

new_repo
commit "Konrad Heimel" "konrad.heimel@gmail.com"
commit "Konrad Heimel" "konrad.heimel@gmail.com" "noreply@github.com"
commit "Konrad Heimel" "6974853+konih@users.noreply.github.com"
commit "Some Contributor" "contributor@example.com"
expect_pass "allowed maintainer addresses and an external contributor"

new_repo
commit "Konrad Heimel" "konrad.heimel@work.example"
expect_fail "maintainer author on a non-allowed address" "konrad.heimel@work.example"

new_repo
commit "Konrad Heimel" "konrad.heimel@gmail.com" "konrad.heimel@work.example"
expect_fail "maintainer committer on a non-allowed address" "konrad.heimel@work.example"

# An empty range is a pass, not an error: a PR can be rebased onto its own base.
new_repo
expect_pass "empty range"

printf 'commit identity test: ok\n'
