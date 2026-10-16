#!/usr/bin/env bash
# G8 probe: how BuildKit's own parser reads the three spellings. `docker buildx build --check`
# parses and lints the Dockerfile; `scratch` needs no pull. A stage named by the image line
# proves the FROM took the image after the comment/blank line.
set -uo pipefail
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP:?}"' EXIT

run() {
  local label="$1" body="$2" d="${TMP}/${3}"
  mkdir -p "${d}"
  printf '%b' "${body}" >"${d}/Dockerfile"
  echo "== ${label}"
  docker buildx build --check "${d}" 2>&1 | grep -vE '^#[0-9]+ (DONE|\[internal\]|transferring)' | tail -12
  echo "exit ${PIPESTATUS[0]}"
}

run "comment line in continuation" 'FROM \\\n# pinned below\n  scratch AS builder\nFROM scratch\nCOPY --from=builder / /\n' a
run "blank line in continuation" 'FROM \\\n\n  scratch AS builder\nFROM scratch\nCOPY --from=builder / /\n' b
run "comment ending in backslash" '# builder stage \\\nFROM scratch AS builder\nFROM scratch\nCOPY --from=builder / /\n' c
