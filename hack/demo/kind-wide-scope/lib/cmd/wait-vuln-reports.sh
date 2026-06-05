#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${KUBECTL:=kubectl}"
deadline=$((SECONDS + 180))
while (( SECONDS < deadline )); do
  c="$("${KUBECTL}" get vulnerabilityreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "${c}" != "0" ]]; then
    exit 0
  fi
  sleep 10
done
exit 1
