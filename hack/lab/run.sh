#!/usr/bin/env bash
# Lab harness resumable runner (LAB-H02 / ADR-0707).
# SPDX-License-Identifier: MIT
#
# Expands a schedule registry → ordered DR-* scenarios, supports --resume against
# artifacts/lab/<RUN_ID>/results.json, and enforces serial Wave-2b tear-down.
# Offline meta-tests use --dry-run (+ preflight fixtures). Never creates/destroys a cluster.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=lib/serial-backend.sh
source "${SCRIPT_DIR}/lib/serial-backend.sh"

lab_runner_usage() {
  cat <<'EOF'
Usage: hack/lab/run.sh [options]

Resumable schedule-driven lab runner (LAB-H02 / ADR-0707). Composes with
hack/lab/preflight.sh (called unless --skip-preflight). Never creates or
destroys a cluster — uses an existing kubeconfig.

Options:
  --schedule NAME       quick | quick+sinks | full-lab-day | soak
                        (full-lab-day / soak refuse until implemented)
  --run-id ID           Lab run id (default: lab-<utc>-<rand>)
  --resume              Skip scenarios already PASS in results.json
  --seed N              Deterministic seed (default: 0); forwarded to scenarios
  --keep-lab            Preserve lab namespaces/resources after run (default: cleanup)
  --tier auto|S|M|L     Capacity tier hint (may no-op in v1)
  --dry-run             No live cluster mutations; stubs emit machine verdicts
  --artifacts-root DIR  Root for results (default: <repo>/artifacts/lab)
  --exec-log FILE       Append executed scenario IDs (one per line; meta-tests)
  --skip-preflight      Do not call preflight.sh (advanced / nested tests)
  -h, --help            Show this help

Exit codes:
  0  OK (all runnable scenarios completed; excluded rows are SKIPPED with reasons)
  1  usage / hard failure / scenario FAIL
  2  schedule refused (BLOCKED / unimplemented)

Env:
  KOLLECT_LAB_DRY_RUN          1 when --dry-run
  KOLLECT_LAB_SEED             seed
  KOLLECT_LAB_RUN_ID           run id
  KOLLECT_LAB_PREFLIGHT_FIXTURE  forwarded to preflight offline fixtures
EOF
}

lab_runner_default_run_id() {
  printf 'lab-%s-%s\n' "$(date -u +%Y%m%dT%H%M%SZ)" "$(openssl rand -hex 3 2>/dev/null || printf '%04x' "$$")"
}

lab_runner_schedule_path() {
  local name="$1"
  local base="${SCRIPT_DIR}/schedules/${name}"
  if [[ -f "${base}.json" ]]; then
    printf '%s\n' "${base}.json"
    return 0
  fi
  if [[ -f "${base}.yaml" ]]; then
    printf '%s\n' "${base}.yaml"
    return 0
  fi
  return 1
}

lab_runner_load_schedule_json() {
  local path="$1"
  case "${path}" in
    *.json) cat "${path}" ;;
    *.yaml | *.yml)
      if command -v yq >/dev/null 2>&1; then
        yq -o=json '.' "${path}"
      else
        python3 -c 'import json,sys,yaml; json.dump(yaml.safe_load(open(sys.argv[1])), sys.stdout)' "${path}"
      fi
      ;;
    *)
      printf 'lab runner: unsupported schedule format: %s\n' "${path}" >&2
      return 1
      ;;
  esac
}

lab_runner_results_path() {
  local art_root="$1"
  local run_id="$2"
  printf '%s/%s/results.json\n' "${art_root%/}" "${run_id}"
}

lab_runner_init_results() {
  local path="$1"
  local run_id="$2"
  local schedule="$3"
  local seed="$4"
  mkdir -p "$(dirname "${path}")"
  if [[ -f "${path}" ]]; then
    return 0
  fi
  jq -nc \
    --arg run_id "${run_id}" \
    --arg schedule "${schedule}" \
    --argjson seed "${seed}" \
    '{run_id:$run_id, schedule:$schedule, seed:$seed, scenarios:[]}' >"${path}"
}

lab_runner_already_pass() {
  local path="$1"
  local sid="$2"
  [[ -f "${path}" ]] || return 1
  jq -e --arg id "${sid}" '
    [.scenarios[] | select(.id == $id and (.verdict == "PASS" or .verdict == "PASS_WITH_LIMITATION"))]
    | length > 0
  ' "${path}" >/dev/null 2>&1
}

lab_runner_upsert_result() {
  local path="$1"
  local sid="$2"
  local verdict="$3"
  local reason="$4"
  local tmp
  tmp="$(mktemp)"
  jq --arg id "${sid}" --arg verdict "${verdict}" --arg reason "${reason}" '
    .scenarios = (
      [.scenarios[] | select(.id != $id)]
      + [{id:$id, verdict:$verdict, reason:$reason}]
    )
  ' "${path}" >"${tmp}"
  mv "${tmp}" "${path}"
}

lab_runner_append_excluded() {
  local path="$1"
  local schedule_json="$2"
  local id reason
  while IFS=$'\t' read -r id reason; do
    [[ -n "${id}" ]] || continue
    # Do not overwrite an existing PASS on resume.
    if lab_runner_already_pass "${path}" "${id}"; then
      continue
    fi
    lab_runner_upsert_result "${path}" "${id}" "SKIPPED" "${reason}"
  done < <(jq -r '.excluded // [] | .[] | [.id, .reason] | @tsv' <<<"${schedule_json}")
}

lab_runner_run_preflight() {
  local dry_run="$1"
  local pf_args=()
  if [[ "${dry_run}" -eq 1 ]]; then
    pf_args+=(--dry-run)
  fi
  if [[ -n "${KOLLECT_LAB_PREFLIGHT_FIXTURE:-}" ]]; then
    pf_args+=(--fixture="${KOLLECT_LAB_PREFLIGHT_FIXTURE}")
  elif [[ "${dry_run}" -eq 1 ]]; then
    # Dry-run without an explicit fixture: use clean offline fixture so meta-tests
    # and local dry-runs never touch a live cluster.
    pf_args+=(--fixture=clean)
  fi
  bash "${SCRIPT_DIR}/preflight.sh" "${pf_args[@]}"
}

lab_runner_main() {
  local schedule=""
  local run_id=""
  local resume=0
  local seed=0
  local keep_lab=0
  local tier="auto"
  local dry_run=0
  local artifacts_root="${ROOT}/artifacts/lab"
  local exec_log=""
  local skip_preflight=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --schedule)
        schedule="${2:-}"
        shift 2
        ;;
      --schedule=*)
        schedule="${1#*=}"
        shift
        ;;
      --run-id)
        run_id="${2:-}"
        shift 2
        ;;
      --run-id=*)
        run_id="${1#*=}"
        shift
        ;;
      --resume)
        resume=1
        shift
        ;;
      --seed)
        seed="${2:-0}"
        shift 2
        ;;
      --seed=*)
        seed="${1#*=}"
        shift
        ;;
      --keep-lab)
        keep_lab=1
        shift
        ;;
      --tier)
        tier="${2:-auto}"
        shift 2
        ;;
      --tier=*)
        tier="${1#*=}"
        shift
        ;;
      --dry-run)
        dry_run=1
        shift
        ;;
      --artifacts-root)
        artifacts_root="${2:-}"
        shift 2
        ;;
      --artifacts-root=*)
        artifacts_root="${1#*=}"
        shift
        ;;
      --exec-log)
        exec_log="${2:-}"
        shift 2
        ;;
      --exec-log=*)
        exec_log="${1#*=}"
        shift
        ;;
      --skip-preflight)
        skip_preflight=1
        shift
        ;;
      -h | --help)
        lab_runner_usage
        return 0
        ;;
      *)
        printf 'lab runner: unknown argument: %s\n' "$1" >&2
        lab_runner_usage >&2
        return 1
        ;;
    esac
  done

  if [[ -z "${schedule}" ]]; then
    printf 'lab runner: --schedule is required\n' >&2
    lab_runner_usage >&2
    return 1
  fi
  case "${tier}" in
    auto | S | M | L) ;;
    *)
      printf 'lab runner: invalid --tier %q (want auto|S|M|L)\n' "${tier}" >&2
      return 1
      ;;
  esac

  local sched_path
  if ! sched_path="$(lab_runner_schedule_path "${schedule}")"; then
    printf 'lab runner: unknown schedule %q (no registry file)\n' "${schedule}" >&2
    return 1
  fi

  local schedule_json
  schedule_json="$(lab_runner_load_schedule_json "${sched_path}")"

  local implemented
  implemented="$(jq -r '.implemented // false' <<<"${schedule_json}")"
  if [[ "${implemented}" != "true" ]]; then
    local refuse
    refuse="$(jq -r '.refuse_reason // "BLOCKED: schedule not implemented"' <<<"${schedule_json}")"
    printf '%s\n' "${refuse}" >&2
    return 2
  fi

  if [[ -z "${run_id}" ]]; then
    run_id="$(lab_runner_default_run_id)"
  fi

  if [[ "${skip_preflight}" -eq 0 ]]; then
    lab_runner_run_preflight "${dry_run}" || {
      printf 'lab runner: preflight failed\n' >&2
      return 1
    }
  fi

  local results
  results="$(lab_runner_results_path "${artifacts_root}" "${run_id}")"
  lab_runner_init_results "${results}" "${run_id}" "${schedule}" "${seed}"

  # Ensure schedule/seed fields match this invocation.
  local tmp_hdr
  tmp_hdr="$(mktemp)"
  jq --arg schedule "${schedule}" --argjson seed "${seed}" \
    '.schedule=$schedule | .seed=$seed' "${results}" >"${tmp_hdr}"
  mv "${tmp_hdr}" "${results}"

  local serial_state
  serial_state="$(dirname "${results}")/serial-backend.state"
  lab_serial_backend_reset "${serial_state}"

  export KOLLECT_LAB_DRY_RUN="${dry_run}"
  export KOLLECT_LAB_SEED="${seed}"
  export KOLLECT_LAB_RUN_ID="${run_id}"
  export KOLLECT_LAB_TIER="${tier}"
  export KOLLECT_LAB_KEEP_LAB="${keep_lab}"

  if [[ -n "${exec_log}" ]]; then
    : >"${exec_log}"
  fi

  local sid backend backend_id scenario_script out verdict reason rc fail_count=0
  while IFS= read -r sid; do
    [[ -n "${sid}" ]] || continue

    if [[ "${resume}" -eq 1 ]] && lab_runner_already_pass "${results}" "${sid}"; then
      printf 'lab runner: resume skip %s (already PASS)\n' "${sid}"
      continue
    fi

    backend="$(jq -r --arg id "${sid}" '
      (.scenarios // [])[] | select(.id == $id) | .backend // empty
    ' <<<"${schedule_json}")"
    if [[ -z "${backend}" ]] && lab_serial_backend_is_backend_scenario "${sid}"; then
      backend="$(lab_serial_backend_id_for_scenario "${sid}")"
    fi

    if [[ -n "${backend}" ]]; then
      backend_id="${backend}"
      lab_serial_backend_begin "${serial_state}" "${backend_id}" || return 1
      lab_serial_backend_mark_up "${serial_state}" "${backend_id}" || return 1
    fi

    scenario_script="${SCRIPT_DIR}/scenarios/${sid}.sh"
    if [[ ! -x "${scenario_script}" && ! -f "${scenario_script}" ]]; then
      verdict="BLOCKED"
      reason="scenario script missing: scenarios/${sid}.sh"
      lab_runner_upsert_result "${results}" "${sid}" "${verdict}" "${reason}"
      printf 'lab runner: %s → %s (%s)\n' "${sid}" "${verdict}" "${reason}"
      fail_count=$((fail_count + 1))
      if [[ -n "${backend}" ]]; then
        lab_serial_backend_teardown "${serial_state}" "${backend_id}" || true
      fi
      continue
    fi

    if [[ -n "${exec_log}" ]]; then
      printf '%s\n' "${sid}" >>"${exec_log}"
    fi

    rc=0
    out="$(bash "${scenario_script}" 2>&1)" || rc=$?
    if [[ "${rc}" -ne 0 ]]; then
      verdict="FAIL"
      reason="scenario exit ${rc}: ${out}"
      # Truncate overly long reasons for JSON hygiene.
      reason="${reason:0:500}"
      lab_runner_upsert_result "${results}" "${sid}" "${verdict}" "${reason}"
      fail_count=$((fail_count + 1))
    else
      verdict="$(jq -r '.verdict // empty' <<<"${out}" 2>/dev/null || true)"
      reason="$(jq -r '.reason // empty' <<<"${out}" 2>/dev/null || true)"
      if [[ -z "${verdict}" ]]; then
        verdict="FAIL"
        reason="scenario produced non-JSON or missing verdict: ${out:0:200}"
        fail_count=$((fail_count + 1))
      fi
      # Non-PASS must never have an empty reason (never empty green cells).
      if [[ "${verdict}" != "PASS" && -z "${reason}" ]]; then
        reason="machine: missing skip/limit/block reason from scenario"
      fi
      lab_runner_upsert_result "${results}" "${sid}" "${verdict}" "${reason}"
      if [[ "${verdict}" == "FAIL" ]]; then
        fail_count=$((fail_count + 1))
      fi
    fi

    printf 'lab runner: %s → %s\n' "${sid}" "${verdict}"

    if [[ -n "${backend}" ]]; then
      # Dry-run stubs always tear down; live scripts must call teardown or we force it.
      lab_serial_backend_teardown "${serial_state}" "${backend_id}" || return 1
    fi
  done < <(jq -r '.scenarios[].id' <<<"${schedule_json}")

  lab_runner_append_excluded "${results}" "${schedule_json}"

  if [[ "${keep_lab}" -eq 1 ]]; then
    printf 'lab runner: --keep-lab set; skipping cleanup hint\n'
  else
    printf 'lab runner: cleanup lab resources (default; --keep-lab to retain)\n'
  fi

  # Tier is accepted for forward-compat; v1 may no-op.
  if [[ "${tier}" != "auto" ]]; then
    printf 'lab runner: tier=%s recorded (no-op capacity gate in v1)\n' "${tier}"
  fi

  printf 'lab runner: wrote %s\n' "${results}"

  if [[ "${fail_count}" -gt 0 ]]; then
    return 1
  fi
  return 0
}

lab_runner_main "$@"
