#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${KUBECTL:=kubectl}"
"${KUBECTL}" get deploy,svc,certificate,externalsecret -A 2>/dev/null | head -20 || true
