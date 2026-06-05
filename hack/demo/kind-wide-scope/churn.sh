#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Back-compat wrapper — delegates to churn/run.sh (declarative steps.yaml).
# Usage:
#   CHURN_PRESET=present bash hack/demo/kind-wide-scope/churn.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec bash "${SCRIPT_DIR}/churn/run.sh" "$@"
