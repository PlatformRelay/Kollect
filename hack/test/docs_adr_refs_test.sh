#!/usr/bin/env bash
# DOC-ADRREFS-01: every ADR-\d{4} token in the repository must resolve to a committed ADR.
#
# ADRs get renumbered (0020-error-taxonomy -> 0602) and deleted (the SPA removal dropped
# 0408; the 2026 retcon dropped 0503/0703). Code comments, CRD descriptions, chart values
# schema, and docs keep citing the old numbers for years afterwards — user-visible through
# `kubectl explain` CRD descriptions. This gate reds at the cite site instead of in a
# consolidated review.
#
# Scope: every tracked file. CHANGELOG.md is exempt because its entries are historical
# commit titles ("Complete ADR-0020 metrics catalog") that must not be rewritten.
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
  for num in $(printf '%s' "${line}" | grep -oE 'ADR-[0-9]{4}' | sort -u); do
    n="${num#ADR-}"
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
