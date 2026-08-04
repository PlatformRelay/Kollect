#!/usr/bin/env bash
# HYG-UI-GITIGNORE: leftover local ui/ (dist + node_modules after UI-REMOVE-01)
# must stay out of git status and must not reappear as accidental trackable noise.
# Asserts .gitignore ignores the repo-root ui/ directory.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GITIGNORE="${ROOT}/.gitignore"

fail() {
  printf 'hyg ui gitignore: %s\n' "$*" >&2
  exit 1
}

[[ -f "${GITIGNORE}" ]] || fail ".gitignore not found at ${GITIGNORE}"

# Accept rooted (/ui/) or unrooted (ui/) directory entries; optional trailing slash.
# Reject comments and negated exceptions (!ui/).
if ! grep -Eq '^[[:space:]]*/?ui/[[:space:]]*$' "${GITIGNORE}"; then
  fail ".gitignore missing /ui/ (or ui/) entry — leftover SPA must be ignored"
fi

printf 'hyg ui gitignore: ok\n'
