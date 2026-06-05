#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${DEMO_REPO_ROOT:?DEMO_REPO_ROOT required}"
: "${DEMO_OVERLAY_PATH:?DEMO_OVERLAY_PATH required}"
: "${KUBECTL:=kubectl}"
cd "${DEMO_REPO_ROOT}"
"${KUBECTL}" apply -k "${DEMO_OVERLAY_PATH}"
