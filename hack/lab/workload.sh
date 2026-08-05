#!/usr/bin/env bash
# Lab harness workload CLI (LAB-H03 / ADR-0707).
# SPDX-License-Identifier: MIT
#
# Deterministic seeded batch render + optional light churn.
# Always labels kollect.dev/lab-run=<RUN_ID>. Use --dry-run / --fixture offline.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/workload.sh
source "${SCRIPT_DIR}/lib/workload.sh"

lab_workload_main "$@"
