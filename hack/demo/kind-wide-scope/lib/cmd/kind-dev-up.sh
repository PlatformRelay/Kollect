#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
set -euo pipefail

: "${DEMO_REPO_ROOT:?DEMO_REPO_ROOT required}"
: "${DEMO_HELM_VALUES:=${DEMO_REPO_ROOT}/charts/kollect/ci/demo-values.yaml}"
cd "${DEMO_REPO_ROOT}"
DEV_VALUES="${DEMO_HELM_VALUES}" KOLLECT_DEV_MINIMAL=1 task kind-dev-up
