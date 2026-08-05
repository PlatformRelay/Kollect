#!/usr/bin/env bash
# Run every lab harness offline meta-test (hack/test/lab_*_meta_test.sh).
# SPDX-License-Identifier: MIT
# Missing globs are skipped; any failing test fails the suite.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

shopt -s nullglob
tests=(hack/test/lab_*_meta_test.sh)
shopt -u nullglob

if ((${#tests[@]} == 0)); then
  printf 'lab harness meta suite: no lab_*_meta_test.sh files (ok)\n'
  exit 0
fi

failed=0
for t in "${tests[@]}"; do
  printf 'lab harness meta suite: running %s\n' "${t}"
  if ! bash "${t}"; then
    printf 'lab harness meta suite: FAIL %s\n' "${t}" >&2
    failed=1
  fi
done

if [[ "${failed}" -ne 0 ]]; then
  printf 'lab harness meta suite: one or more tests failed\n' >&2
  exit 1
fi

printf 'lab harness meta suite: all %d test(s) passed\n' "${#tests[@]}"
