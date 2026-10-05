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

bad=()
while IFS=$'\t' read -r sha an ae cn ce; do
  [[ -n "${sha}" ]] || continue
  if [[ "${an}" == "${maintainer}" ]] && ! is_allowed "${ae}"; then
    bad+=("${sha:0:9} author ${ae}")
  fi
  if [[ "${cn}" == "${maintainer}" ]] && ! is_allowed "${ce}"; then
    bad+=("${sha:0:9} committer ${ce}")
  fi
done < <(git log --format='%H%x09%an%x09%ae%x09%cn%x09%ce' "$1..$2")

if ((${#bad[@]} > 0)); then
  printf 'commit identity: maintainer commits use a non-project address:\n' >&2
  printf '  %s\n' "${bad[@]}" >&2
  printf 'Rewrite them, e.g. git rebase --exec with GIT_AUTHOR_EMAIL/GIT_COMMITTER_EMAIL set.\n' >&2
  exit 1
fi

printf 'commit identity: ok\n'
