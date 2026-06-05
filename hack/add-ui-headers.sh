#!/usr/bin/env bash
# One-time helper: prepend SPDX + copyright headers to ui/src files that lack them.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UI_SRC="${ROOT}/ui/src"

SPDX="// SPDX-License-Identifier: MIT"
COPYRIGHT="// Copyright (c) 2026 Konrad Heimel"
SPDX_CSS="/* SPDX-License-Identifier: MIT */"
COPYRIGHT_CSS="/* Copyright (c) 2026 Konrad Heimel */"

add_ts_header() {
  local f="$1"
  if head -n 1 "$f" | grep -qF "SPDX-License-Identifier"; then
    return 0
  fi
  local tmp
  tmp="$(mktemp)"
  {
    echo "$SPDX"
    echo "$COPYRIGHT"
    echo
    cat "$f"
  } >"$tmp"
  mv "$tmp" "$f"
}

add_css_header() {
  local f="$1"
  if head -n 1 "$f" | grep -qF "SPDX-License-Identifier"; then
    return 0
  fi
  local tmp
  tmp="$(mktemp)"
  {
    echo "$SPDX_CSS"
    echo "$COPYRIGHT_CSS"
    echo
    cat "$f"
  } >"$tmp"
  mv "$tmp" "$f"
}

while IFS= read -r -d '' f; do
  case "$f" in
    *.ts|*.tsx) add_ts_header "$f" ;;
    *.css) add_css_header "$f" ;;
  esac
done < <(find "$UI_SRC" -type f \( -name '*.ts' -o -name '*.tsx' -o -name '*.css' \) -print0)

echo "add-ui-headers: done"
