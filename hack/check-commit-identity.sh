#!/usr/bin/env bash
# Refuse maintainer commits made under an address other than the project identity.
# SPDX-License-Identifier: MIT
#
# Usage: hack/check-commit-identity.sh <base> <head>
#
# Commits authored or committed as the maintainer must use the project address or
# a GitHub noreply address; a commit made from a machine configured with another
# identity would otherwise publish that address in the public history. Other
# contributors' identities are not checked.
set -euo pipefail

[[ $# -eq 2 ]] || {
  printf 'usage: %s <base> <head>\n' "${0##*/}" >&2
  exit 2
}

maintainer="Konrad Heimel"
allowed=(
  "konrad.heimel@gmail.com"
  "6974853+konih@users.noreply.github.com"
  "noreply@github.com"
)

is_allowed() {
  local email="$1" a
  for a in "${allowed[@]}"; do
    [[ "${email}" == "${a}" ]] && return 0
  done
  return 1
}

# Fail closed: an unknown revision must not turn into an empty, passing range.
for rev in "$1" "$2"; do
  git rev-parse --verify --quiet "${rev}^{commit}" >/dev/null || {
    printf 'commit identity: unknown revision %s\n' "${rev}" >&2
    exit 2
  }
done
# Captured, not piped, so a git failure stops the script under set -e. The unit
# separator is not whitespace, so an empty field cannot shift the ones after it.
log="$(git log --format='%H%x1f%an%x1f%ae%x1f%cn%x1f%ce' "$1..$2")"

bad=()
while IFS=$'\x1f' read -r sha an ae cn ce; do
  [[ -n "${sha}" ]] || continue
  if [[ "${an}" == "${maintainer}" ]] && ! is_allowed "${ae}"; then
    bad+=("${sha:0:9} author ${ae}")
  fi
  if [[ "${cn}" == "${maintainer}" ]] && ! is_allowed "${ce}"; then
    bad+=("${sha:0:9} committer ${ce}")
  fi
done <<<"${log}"

if ((${#bad[@]} > 0)); then
  printf 'commit identity: maintainer commits use a non-project address:\n' >&2
  printf '  %s\n' "${bad[@]}" >&2
  printf 'Rewrite them, e.g. git rebase --exec with GIT_AUTHOR_EMAIL/GIT_COMMITTER_EMAIL set.\n' >&2
  exit 1
fi

printf 'commit identity: ok\n'
