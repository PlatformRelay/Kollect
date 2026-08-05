#!/usr/bin/env bash
# LAB-H02 scenario stub for DR-2.5 (ADR-0707). Offline dry-run only until live body lands.
# SPDX-License-Identifier: MIT
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=../lib/scenario-stub.sh
source "${ROOT}/hack/lab/lib/scenario-stub.sh"
lab_scenario_stub_emit "DR-2.5"
