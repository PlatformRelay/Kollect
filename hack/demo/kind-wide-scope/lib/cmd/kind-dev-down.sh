#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${DEMO_REPO_ROOT:?DEMO_REPO_ROOT required}"
cd "${DEMO_REPO_ROOT}"
task kind-dev-down
