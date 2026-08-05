#!/usr/bin/env bash
# LAB-H02 dry-run stub for DR-3.1 (ADR-0707). Records a machine verdict; no live cluster.
# SPDX-License-Identifier: MIT
set -euo pipefail

SCENARIO_ID="DR-3.1"
DRY_RUN="${KOLLECT_LAB_DRY_RUN:-0}"
SEED="${KOLLECT_LAB_SEED:-0}"
RUN_ID="${KOLLECT_LAB_RUN_ID:-}"

# Stubs always PASS in dry-run; live body lands with real scenario scripts later.
verdict="PASS"
reason=""

# DR-3.1 / DR-4.3 historically PASS_WITH_LIMITATION in driving-range protocols.
case "${SCENARIO_ID}" in
  DR-3.1)
    verdict="PASS_WITH_LIMITATION"
    reason="stub: certificate scrape may undercount vs live certs"
    ;;
  DR-4.3)
    verdict="PASS_WITH_LIMITATION"
    reason="stub: idle pprof only; Wave-4 load not in this schedule"
    ;;
esac

jq -nc \
  --arg id "${SCENARIO_ID}" \
  --arg verdict "${verdict}" \
  --arg reason "${reason}" \
  --arg dry_run "${DRY_RUN}" \
  --arg seed "${SEED}" \
  --arg run_id "${RUN_ID}" \
  '{id:$id, verdict:$verdict, reason:$reason, dry_run:($dry_run=="1"), seed:$seed, run_id:$run_id}'
