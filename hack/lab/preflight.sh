#!/usr/bin/env bash
# Lab harness preflight CLI (LAB-H01 / ADR-0707).
# SPDX-License-Identifier: MIT
#
# Exit codes: 0 OK · 1 usage/hard fail · 2 residue without --force · 3 STRICT missing tools
# See hack/lab/README.md and lab_preflight_usage in lib/preflight.sh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/preflight.sh
source "${SCRIPT_DIR}/lib/preflight.sh"

lab_preflight_main "$@"
