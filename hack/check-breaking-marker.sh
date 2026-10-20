#!/usr/bin/env bash
# Fail a pull request that declares a breaking change none of its commit titles carries.
# SPDX-License-Identifier: MIT
#
# Usage: PR_TITLE=<title> PR_BODY=<body> hack/check-breaking-marker.sh <base> <head>
#
# Pull requests land by rebase merge, so git-cliff (hack/release/cliff.toml) reads the commits and
# never the pull request: it marks a changelog entry breaking from the `!` in a commit title. A `!`
# in the PR title, or a BREAKING CHANGE footer in the PR body, is lost on merge (openspec change
# breaking-marker-check, BMC-1 to BMC-3).
#
#   - The PR title has a conventional `type!:` / `type(scope)!:` marker, or the PR body has a
#     BREAKING CHANGE (or BREAKING-CHANGE) footer line outside a fenced code block, and no commit
#     title in <base>..<head> has the marker: ::error::, exit 1.
#   - A commit title has the marker and the PR title does not: ::warning::, exit 0.
#   - Wrong arguments, an unknown revision or an unset PR_TITLE: exit 2.
#
# The PR text is data: it is only matched against regular expressions, never evaluated or echoed.
set -euo pipefail

usage() {
  printf 'usage: PR_TITLE=... PR_BODY=... %s <base> <head>\n' "${0##*/}" >&2
  exit 2
}

[[ $# -eq 2 ]] || usage
[[ -n "${PR_TITLE+set}" ]] || { printf 'breaking marker: PR_TITLE is not set\n' >&2; exit 2; }
body="${PR_BODY-}"

# The breaking marker at the start of a title, after an optional gitmoji shortcode: the same shape
# git-cliff parses once its preprocessor has stripped the shortcode (hack/release/cliff.toml).
marker_re='^(:[a-z0-9_+-]+:[[:space:]]+)?[a-z]+(\([^)]*\))?!:'
# A footer line, allowing list or quote markers and bold around the token.
footer_re='^[[:space:]]*([-*>][[:space:]]*)*(\*\*)?BREAKING[ -]CHANGE(\*\*)?:'
fence_re='^[[:space:]]*(```|~~~)'

# Fail closed: an unknown revision must not turn into an empty, passing range.
for rev in "$1" "$2"; do
  if ! git rev-parse --verify --quiet "${rev}^{commit}" >/dev/null; then
    printf 'breaking marker: unknown revision %s\n' "${rev}" >&2
    exit 2
  fi
done
# Captured, not piped, so a git failure stops the script under set -e.
subjects="$(git log --format=%s "$1..$2")"

title_marked=false
[[ "${PR_TITLE}" =~ ${marker_re} ]] && title_marked=true

body_marked=false
in_fence=false
while IFS= read -r line || [[ -n "${line}" ]]; do
  line="${line%$'\r'}"
  if [[ "${line}" =~ ${fence_re} ]]; then
    if ${in_fence}; then in_fence=false; else in_fence=true; fi
    continue
  fi
  ${in_fence} && continue
  if [[ "${line}" =~ ${footer_re} ]]; then
    body_marked=true
    break
  fi
done <<<"${body}"

marked=()
while IFS= read -r s; do
  [[ -n "${s}" ]] || continue
  [[ "${s}" =~ ${marker_re} ]] && marked+=("${s}")
done <<<"${subjects}"

if ((${#marked[@]} == 0)) && { ${title_marked} || ${body_marked}; }; then
  if ${title_marked} && ${body_marked}; then
    where="the PR title and a BREAKING CHANGE footer in the PR body"
  elif ${title_marked}; then
    where="the PR title"
  else
    where="a BREAKING CHANGE footer in the PR body"
  fi
  printf '::error title=breaking marker::A breaking change is declared in %s, but no commit title in this PR has the "!" marker. PRs are rebase-merged and git-cliff reads only commit titles, so the changelog would list it as an ordinary change. Add "!" to the title of the commit that breaks (for example feat(sink)!: ...) and put the BREAKING CHANGE footer in that commit body.\n' "${where}"
  exit 1
fi

if ((${#marked[@]} > 0)) && ! ${title_marked}; then
  for s in "${marked[@]}"; do
    # Workflow-command data encoding: a literal % is written as %25. A title is one line, so it
    # cannot start a command of its own.
    s="${s//%/%25}"
    printf '::warning title=breaking marker::commit "%s" is marked breaking, but the PR title has no "!" marker\n' "${s}"
  done
  printf 'breaking marker: ok, with %d warning(s)\n' "${#marked[@]}"
  exit 0 # warnings only
fi

printf 'breaking marker: ok (%d commit(s), %d marked breaking)\n' "$(grep -c . <<<"${subjects}" || true)" "${#marked[@]}"
