#!/usr/bin/env bash
# DOC-04: every tracked Kustomization under config/samples/demo must render
# under default load restrictions (no sibling file escapes, no --load-restrictor
# LoadRestrictionsNone). Pure render contract — no cluster, no credentials.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEMO_ROOT="${ROOT}/config/samples/demo"

fail() {
  printf 'demo-04 samples kustomize: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -d "${DEMO_ROOT}" ]] || fail "missing ${DEMO_ROOT}"

if command -v kubectl >/dev/null 2>&1; then
  build_one() { kubectl kustomize "$1"; }
  BUILDER="kubectl kustomize"
elif command -v kustomize >/dev/null 2>&1; then
  build_one() { kustomize build "$1"; }
  BUILDER="kustomize build"
else
  fail "need kubectl or kustomize on PATH to render demo samples"
fi

mapfile -t KUST_FILES < <(
  find "${DEMO_ROOT}" \( \
    -name 'kustomization.yaml' -o \
    -name 'kustomization.yml' -o \
    -name 'Kustomization' \
  \) -type f | LC_ALL=C sort
)

[[ ${#KUST_FILES[@]} -gt 0 ]] || fail "no kustomization.y*ml under ${DEMO_ROOT}"

# Hero bootstrap advertises both variants — refuse a tree that drops either.
found_git_only=0
found_git_postgres=0
for kust in "${KUST_FILES[@]}"; do
  dir="$(dirname "${kust}")"
  rel="${dir#"${ROOT}"/}"
  case "${rel}" in
    */git-only) found_git_only=1 ;;
    */git-postgres) found_git_postgres=1 ;;
  esac
  err_file="$(mktemp)"
  if ! build_one "${dir}" >/dev/null 2>"${err_file}"; then
    msg="$(tr '\n' ' ' <"${err_file}" | sed 's/[[:space:]]\+/ /g')"
    rm -f "${err_file}"
    fail "${BUILDER} ${rel} failed: ${msg}"
  fi
  rm -f "${err_file}"
  pass "${BUILDER} ${rel}"
done

[[ "${found_git_only}" -eq 1 ]] || fail "expected config/samples/demo/git-only Kustomization"
[[ "${found_git_postgres}" -eq 1 ]] || fail "expected config/samples/demo/git-postgres Kustomization"

printf 'demo-04 samples kustomize: ok (%d tree(s) via %s)\n' "${#KUST_FILES[@]}" "${BUILDER}"
