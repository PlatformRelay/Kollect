#!/usr/bin/env bash
# DOC-ADRREFS-01: every ADR-\d{4} token in the repository must resolve to a committed ADR.
#
# ADRs get renumbered (the error-taxonomy ADR moved into theme 06) and deleted (the SPA
# removal dropped the read-API ADR; the 2026 retcon dropped the hub-auth and platform-pivot
# ADRs). Code comments, CRD descriptions, chart values schema, and docs keep citing the old
# numbers for years afterwards — user-visible through `kubectl explain` CRD descriptions.
# This gate reds at the cite site instead of in a consolidated review.
#
# Scope: every tracked file. CHANGELOG.md is exempt because its entries are historical
# commit titles that must not be rewritten.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

fail() {
  printf 'docs adr refs: %s\n' "$*" >&2
  exit 1
}

[[ -d docs/adr ]] || fail "docs/adr/ missing"

hits="$(
  git ls-files -z |
    xargs -0 grep -InE 'ADR-[0-9]{4}' 2>/dev/null |
    grep -v '^CHANGELOG.md:' ||
    true
)"
if [[ -z "${hits}" ]]; then
  printf 'docs adr refs: ok (no ADR references found)\n'
  exit 0
fi

ghosts=""
while IFS= read -r line; do
  file="${line%%:*}"
  rest="${line#*:}"
  lineno="${rest%%:*}"
  [[ "${file}" == "CHANGELOG.md" ]] && continue
  # Word-boundary + trailing-digit guard via perl: ADR-12345 is not a citation
  # (extracts the exact 4-digit token only), and lowercase adr-NNNN counts too.
  for num in $(
    printf '%s' "${line}" |
      perl -ne 'while (/\b(?:adr|ADR)-[0-9]{4}(?![0-9])/g) { print lc($&), "\n" }' |
      sort -u
  ); do
    n="${num#*-}"
    [[ "${n}" =~ ^[0-9]{4}$ ]] || continue
    if ! ls "docs/adr/${n}"-*.md >/dev/null 2>&1; then
      printf 'docs adr refs: %s:%s cites %s but docs/adr/%s-*.md does not exist\n' \
        "${file}" "${lineno}" "${num}" "${n}" >&2
      ghosts=1
    fi
  done
done <<< "${hits}"

if [[ -n "${ghosts:-}" ]]; then
  fail "ghost ADR references remain (renumber or delete the citation — see hack/test/docs_adr_refs_test.sh)"
fi

printf 'docs adr refs: ok\n'
