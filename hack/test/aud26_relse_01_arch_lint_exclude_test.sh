#!/usr/bin/env bash
# AUD26-RELSE-01: go-arch-lint must not walk agent worktree trees.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cfg="${repo_root}/.go-arch-lint.yml"
fail() { printf 'aud26-relse-01: %s\n' "$*" >&2; exit 1; }
[[ -f "$cfg" ]] || fail "missing .go-arch-lint.yml"
for path in '.claude' '.claude/worktrees' '.worktrees'; do
  grep -Eq "^[[:space:]]*-[[:space:]]*${path//./\\.}$" "$cfg" ||
    fail "exclude missing: ${path}"
done
printf 'aud26-relse-01: ok\n'
