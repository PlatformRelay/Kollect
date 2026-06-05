#!/usr/bin/env bash
# Fail if ui/src source files lack SPDX + copyright headers.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UI_SRC="${ROOT}/ui/src"

SPDX="// SPDX-License-Identifier: MIT"
COPYRIGHT="// Copyright (c) 2026 Konrad Heimel"
SPDX_CSS="/* SPDX-License-Identifier: MIT */"
COPYRIGHT_CSS="/* Copyright (c) 2026 Konrad Heimel */"

fail=0

check_ts() {
  local f="$1"
  if ! head -n 5 "$f" | grep -qF "$SPDX"; then
    echo "verify-ui-headers: missing SPDX in $f" >&2
    fail=1
  fi
  if ! head -n 5 "$f" | grep -qF "$COPYRIGHT"; then
    echo "verify-ui-headers: missing copyright in $f" >&2
    fail=1
  fi
}

check_css() {
  local f="$1"
  if ! head -n 5 "$f" | grep -qF "$SPDX_CSS"; then
    echo "verify-ui-headers: missing SPDX in $f" >&2
    fail=1
  fi
  if ! head -n 5 "$f" | grep -qF "$COPYRIGHT_CSS"; then
    echo "verify-ui-headers: missing copyright in $f" >&2
    fail=1
  fi
}

while IFS= read -r -d '' f; do
  case "$f" in
    *.ts|*.tsx) check_ts "$f" ;;
    *.css) check_css "$f" ;;
  esac
done < <(find "$UI_SRC" -type f \( -name '*.ts' -o -name '*.tsx' -o -name '*.css' \) -print0)

if [[ "$fail" -ne 0 ]]; then
  echo "verify-ui-headers: add headers with: bash hack/add-ui-headers.sh" >&2
  exit 1
fi

echo "verify-ui-headers: ok"
