#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Back-compat wrapper — delegates to the guided demo driver.
# Usage:
#   export GITHUB_TOKEN="$(gh auth token)"
#   bash hack/demo/kind-wide-scope/bootstrap.sh [--churn]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
args=()
for arg in "$@"; do
  args+=("$arg")
done
exec bash "${SCRIPT_DIR}/demo.sh" "${args[@]}"
