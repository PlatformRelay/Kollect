#!/usr/bin/env bash
# G8 probe: run the CFC-1 gate on a scratch copy whose Dockerfile builder FROM is replaced by
# one of three spellings (comment line / blank line inside a continuation, comment ending in a
# backslash). Prints the gate's exit code per case. Usage: G8-probe.sh <repo-root>
set -uo pipefail
ROOT="${1:?repo root}"
GATE="${ROOT}/hack/test/consistency_go_toolchain_test.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP:?}"' EXIT

probe() {
  local label="$1" repl="$2" d="${TMP}/${3}"
  mkdir -p "${d}/.github"
  cp "${ROOT}/go.mod" "${ROOT}/renovate.json" "${ROOT}"/Dockerfile* "${d}/"
  cp -R "${ROOT}/.devcontainer" "${d}/"
  cp -R "${ROOT}/.github/workflows" "${ROOT}/.github/actions" "${d}/.github/"
  sed -i -E "s/^FROM golang:[^ ]+ AS builder/${repl}/" "${d}/Dockerfile"
  local out rc
  out="$(bash "${GATE}" "${d}" 2>&1)" && rc=0 || rc=$?
  echo "${label}: exit ${rc}: $(grep -m1 -E 'FAIL: Dockerfile:|^ok' <<<"${out}")"
}

probe "comment line in continuation" 'FROM \\\n# pinned below\n  golang:1.99.0 AS builder' a
probe "blank line in continuation" 'FROM \\\n\n  golang:1.91.0 AS builder' b
probe "comment ending in backslash" '# builder stage \\\nFROM golang:1.90.0 AS builder' c
