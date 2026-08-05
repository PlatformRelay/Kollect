# SPDX-License-Identifier: MIT
# Shared LAB-H02 scenario stub helpers. Source from scenarios/*.sh after setting SCENARIO_ID.
# shellcheck shell=bash

lab_scenario_stub_emit() {
  local scenario_id="${1:?}"
  local dry_run="${KOLLECT_LAB_DRY_RUN:-0}"
  local seed="${KOLLECT_LAB_SEED:-0}"
  local run_id="${KOLLECT_LAB_RUN_ID:-}"
  local verdict reason

  if [[ "${dry_run}" != "1" ]]; then
    verdict="BLOCKED"
    reason="stub: live scenario not implemented (pass --dry-run for offline stub PASS)"
    jq -nc \
      --arg id "${scenario_id}" \
      --arg verdict "${verdict}" \
      --arg reason "${reason}" \
      --arg dry_run "${dry_run}" \
      --arg seed "${seed}" \
      --arg run_id "${run_id}" \
      '{id:$id, verdict:$verdict, reason:$reason, dry_run:($dry_run=="1"), seed:$seed, run_id:$run_id}'
    return 0
  fi

  verdict="PASS"
  reason=""
  case "${scenario_id}" in
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
    --arg id "${scenario_id}" \
    --arg verdict "${verdict}" \
    --arg reason "${reason}" \
    --arg dry_run "${dry_run}" \
    --arg seed "${seed}" \
    --arg run_id "${run_id}" \
    '{id:$id, verdict:$verdict, reason:$reason, dry_run:($dry_run=="1"), seed:$seed, run_id:$run_id}'
}
