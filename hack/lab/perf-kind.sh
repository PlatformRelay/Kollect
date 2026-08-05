#!/usr/bin/env bash
# LAB-H10 / PERF-LAB-01 — Kind-oriented quick pprof workflow entrypoint.
# SPDX-License-Identifier: MIT
#
# Phases: idle → converge(N objects) → churn → recover
# pprof is disabled in product by default; lab enables via Helm values on a **dev** release
# (for example kollect-dev). Access via localhost port-forward only — never create a public
# Service for pprof.
#
# Exit codes:
#   0  OK (dry-run fixture or live run completed)
#   1  usage / invalid args
#   2  kube context refused (non-kind / ambiguous without --allow-non-kind)
#   3  preflight / evidence / capture failure
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# shellcheck source=lib/evidence.sh
source "${SCRIPT_DIR}/lib/evidence.sh"
# shellcheck source=lib/pprof-capture.sh
source "${SCRIPT_DIR}/lib/pprof-capture.sh"

: "${LAB_PERF_KIND_LOG_PREFIX:=[perf-kind]}"

lab_perf_kind_log() { printf '%s %s\n' "${LAB_PERF_KIND_LOG_PREFIX}" "$*"; }
lab_perf_kind_err() { printf '%s FAIL: %s\n' "${LAB_PERF_KIND_LOG_PREFIX}" "$*" >&2; }

lab_perf_kind_usage() {
  cat <<'EOF'
Usage: hack/lab/perf-kind.sh --run-id <RUN_ID> [options]

Kind-oriented quick pprof workflow (LAB-H10 / PERF-LAB-01). Runs phased capture:
  idle → converge(N objects) → churn → recover

Never creates or destroys a cluster. Refuses ambiguous or non-Kind kube contexts unless
--allow-non-kind is set.

pprof is off in product Helm by default; lab runs enable it only on a **dev** release name
(for example kollect-dev) via Helm values — access via **localhost port-forward only**;
never create a public Service for pprof.

Options:
  --run-id <id>           Required. Stable lab run id → kollect.dev/lab-run=<RUN_ID>.
  --dry-run               Offline fixture: DOC-02 evidence + profiles/ index (no kubectl).
  --objects <n>           Object budget: 100 | 500 | 2000 (default: 500).
  --duration <dur>        Phase dwell hint (default: 60s); CPU profile capped at 30s.
  --seed <n>              Deterministic seed (default: 1).
  --artifacts-root <dir>  Evidence root (default: artifacts/lab under repo).
  --keep-lab              Hint: retain lab namespaces/resources (default: cleanup labeled workload).
  --allow-non-kind        Skip Kind context gate (maintainer override; still no cluster create/destroy).
  --fixture=<mode>        Offline context fixtures: context-non-kind | context-ambiguous | context-kind
  --simulate-interrupt=<phase>  Dry-run only: stop after named phase; preserve partial profiles.
  --exercise-cleanup      Dry-run only: emit cleanup log for kollect.dev/lab-run label.
  -h, --help              Show this help.

Env:
  KOLLECT_LAB_PERF_KIND_CONTEXT_FIXTURE  Same as --fixture=context-* for meta-tests.

Maintainer live Kind path is opt-in; CI verifies the offline --dry-run quick path only.
EOF
}

RUN_ID=""
DRY_RUN=0
OBJECTS=500
DURATION="60s"
SEED=1
ARTIFACTS_ROOT="${ROOT}/artifacts/lab"
KEEP_LAB=0
ALLOW_NON_KIND=0
CONTEXT_FIXTURE="${KOLLECT_LAB_PERF_KIND_CONTEXT_FIXTURE:-}"
SIMULATE_INTERRUPT=""
EXERCISE_CLEANUP=0

PERF_KIND_RUN_DIR=""
PERF_KIND_INTERRUPTED=0

lab_perf_kind_valid_run_id() {
  local id="${1:-}"
  [[ -n "${id}" ]] || return 1
  [[ "${id}" != *"/"* && "${id}" != *".."* ]] || return 1
  [[ "${id}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || return 1
  ((${#id} <= 63)) || return 1
  return 0
}

lab_perf_kind_valid_objects() {
  case "${1:-}" in
    100 | 500 | 2000) return 0 ;;
    *) return 1 ;;
  esac
}

lab_perf_kind_current_context() {
  local fixture="${1:-}"
  case "${fixture}" in
    context-kind | kind)
      printf 'kind-kollect-dev'
      return 0
      ;;
    context-non-kind | non-kind)
      printf 'gke-prod-example'
      return 0
      ;;
    context-ambiguous | ambiguous)
      printf ''
      return 0
      ;;
  esac

  if ! command -v kubectl >/dev/null 2>&1; then
    printf ''
    return 1
  fi
  kubectl config current-context 2>/dev/null || printf ''
}

lab_perf_kind_check_context() {
  local allow_non_kind="$1"
  local dry_run="$2"
  local fixture="${CONTEXT_FIXTURE}"

  if [[ "${dry_run}" -eq 1 && -z "${fixture}" ]]; then
    lab_perf_kind_log "dry-run: skipping live kube context check"
    return 0
  fi

  local ctx
  ctx="$(lab_perf_kind_current_context "${fixture}")"

  case "${fixture}" in
    context-ambiguous | ambiguous)
      lab_perf_kind_err "ambiguous kube context (fixture): cannot determine current context"
      return 2
      ;;
  esac

  if [[ -z "${ctx}" ]]; then
    lab_perf_kind_err "ambiguous kube context: no current context"
    return 2
  fi

  if [[ "${ctx}" == kind-* ]]; then
    lab_perf_kind_log "kube context ok: ${ctx}"
    return 0
  fi

  if [[ "${allow_non_kind}" -eq 1 ]]; then
    lab_perf_kind_log "WARN: non-Kind context ${ctx} allowed via --allow-non-kind"
    return 0
  fi

  lab_perf_kind_err "refusing non-Kind context '${ctx}' (expected kind-*); use --allow-non-kind to override"
  return 2
}

lab_perf_kind_cleanup() {
  local run_dir="${1:-}"
  local run_id="${2:-}"
  local dry_run="${3:-0}"

  lab_pprof_portforward_stop "${run_dir}" || true

  if [[ "${dry_run}" -eq 1 ]]; then
    lab_perf_kind_log "dry-run cleanup: would delete resources labeled kollect.dev/lab-run=${run_id}"
    return 0
  fi

  if command -v kubectl >/dev/null 2>&1; then
    lab_perf_kind_log "cleanup: deleting resources labeled kollect.dev/lab-run=${run_id}"
    kubectl delete all,cm,secret,sa,role,rolebinding -A \
      -l "kollect.dev/lab-run=${run_id}" \
      --ignore-not-found >/dev/null 2>&1 || true
  fi
  return 0
}

lab_perf_kind_on_interrupt() {
  PERF_KIND_INTERRUPTED=1
  lab_perf_kind_log "interrupt: tearing down port-forward; preserving partial profiles"
  if [[ -n "${PERF_KIND_RUN_DIR}" ]]; then
    lab_pprof_portforward_stop "${PERF_KIND_RUN_DIR}" || true
    if [[ -n "${RUN_ID}" && "${KEEP_LAB}" -eq 0 ]]; then
      lab_perf_kind_cleanup "${PERF_KIND_RUN_DIR}" "${RUN_ID}" "${DRY_RUN}" || true
    fi
  fi
}

lab_perf_kind_run_phase() {
  local phase="$1"
  local run_dir="$2"
  local seed="$3"
  local fixture="$4"

  lab_perf_kind_log "phase: ${phase}"
  lab_pprof_capture_phase_set "${run_dir}" "${phase}" "${seed}" "${fixture}" ||
    return 3

  if [[ -n "${SIMULATE_INTERRUPT}" && "${SIMULATE_INTERRUPT}" == "${phase}" ]]; then
    lab_perf_kind_log "simulate-interrupt after phase ${phase}"
    lab_perf_kind_on_interrupt
    return 130
  fi
  return 0
}

lab_perf_kind_write_summary() {
  local run_dir="$1"
  cat >"${run_dir}/summary.md" <<EOF
# perf-kind summary (stub)

Run \`${RUN_ID}\` — LAB-H10 / PERF-LAB-01 quick path.

| Field | Value |
| --- | --- |
| objects | ${OBJECTS} |
| seed | ${SEED} |
| duration hint | ${DURATION} |
| phases | idle → converge → churn → recover |
| profiles index | profiles/index.md |
| findings | summary/performance-findings.md |

pprof: enabled on dev release via Helm values only; **localhost port-forward** — no public Service.
EOF
}

lab_perf_kind_main() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --run-id)
        RUN_ID="$2"
        shift 2
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      --objects)
        OBJECTS="$2"
        shift 2
        ;;
      --duration)
        DURATION="$2"
        shift 2
        ;;
      --seed)
        SEED="$2"
        shift 2
        ;;
      --artifacts-root)
        ARTIFACTS_ROOT="$2"
        shift 2
        ;;
      --keep-lab)
        KEEP_LAB=1
        shift
        ;;
      --allow-non-kind)
        ALLOW_NON_KIND=1
        shift
        ;;
      --fixture=*)
        CONTEXT_FIXTURE="${1#--fixture=}"
        shift
        ;;
      --simulate-interrupt=*)
        SIMULATE_INTERRUPT="${1#--simulate-interrupt=}"
        DRY_RUN=1
        shift
        ;;
      --exercise-cleanup)
        EXERCISE_CLEANUP=1
        DRY_RUN=1
        shift
        ;;
      -h | --help)
        lab_perf_kind_usage
        exit 0
        ;;
      *)
        lab_perf_kind_err "unknown argument: $1"
        lab_perf_kind_usage >&2
        exit 1
        ;;
    esac
  done

  if [[ -z "${RUN_ID}" ]]; then
    lab_perf_kind_err "--run-id is required"
    lab_perf_kind_usage >&2
    exit 1
  fi
  if ! lab_perf_kind_valid_run_id "${RUN_ID}"; then
    lab_perf_kind_err "invalid --run-id (DNS1123 label): ${RUN_ID}"
    exit 1
  fi
  if ! lab_perf_kind_valid_objects "${OBJECTS}"; then
    lab_perf_kind_err "invalid --objects (100|500|2000): ${OBJECTS}"
    exit 1
  fi

  lab_perf_kind_check_context "${ALLOW_NON_KIND}" "${DRY_RUN}" || exit 2

  local run_dir fixture
  run_dir="$(lab_evidence_run_dir "${ARTIFACTS_ROOT}" "${RUN_ID}")"
  PERF_KIND_RUN_DIR="${run_dir}"
  fixture=1
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    fixture=0
  fi

  trap 'lab_perf_kind_on_interrupt' INT TERM

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    lab_perf_kind_log "dry-run / fixture mode (no kubectl mutations); writing evidence under ${run_dir}"
  fi

  lab_evidence_ensure_layout "${run_dir}" "${RUN_ID}"
  lab_pprof_ensure_profiles_dir "${run_dir}"
  lab_pprof_write_index_header "$(lab_pprof_index_path "${run_dir}")"

  # Port-forward to dev release pprof (localhost only — never public Service).
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    lab_pprof_portforward_start "${run_dir}" 16060 "svc/kollect-dev-manager:6060" --fixture
  else
    lab_pprof_portforward_start "${run_dir}" 16060 "svc/kollect-dev-manager:6060" || {
      lab_perf_kind_err "port-forward failed; is kollect-dev installed with pprof.enabled?"
      exit 3
    }
  fi

  local phase_rc=0
  for phase in idle converge churn recover; do
    lab_perf_kind_run_phase "${phase}" "${run_dir}" "${SEED}" "${fixture}" || {
      phase_rc=$?
      break
    }
  done

  # Differential summaries: base↔churn↔recovered (idle=base, recover=recovered).
  lab_pprof_write_differential "${run_dir}" idle converge heap "${SEED}" || true
  lab_pprof_write_differential "${run_dir}" converge churn cpu "${SEED}" || true
  lab_pprof_write_differential "${run_dir}" churn recover heap "${SEED}" || true
  lab_pprof_write_differential "${run_dir}" idle recover goroutine "${SEED}" || true

  lab_pprof_write_findings_stub "${run_dir}" "${RUN_ID}" "${OBJECTS}"
  lab_perf_kind_write_summary "${run_dir}"

  lab_evidence_validate_layout "${run_dir}" || exit 3
  lab_evidence_note_size "${run_dir}" || true

  if [[ "${EXERCISE_CLEANUP}" -eq 1 ]]; then
    lab_perf_kind_cleanup "${run_dir}" "${RUN_ID}" 1
  elif [[ "${KEEP_LAB}" -eq 0 && "${PERF_KIND_INTERRUPTED}" -eq 0 && "${phase_rc}" -eq 0 ]]; then
    lab_perf_kind_cleanup "${run_dir}" "${RUN_ID}" "${DRY_RUN}"
  fi

  lab_pprof_portforward_stop "${run_dir}" || true
  trap - INT TERM

  if [[ "${phase_rc}" -eq 130 ]]; then
    lab_perf_kind_log "partial run preserved under ${run_dir}/profiles/"
    exit 130
  fi
  if [[ "${phase_rc}" -ne 0 ]]; then
    exit "${phase_rc}"
  fi

  lab_perf_kind_log "perf-kind complete: ${run_dir}"
  exit 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  lab_perf_kind_main "$@"
fi
