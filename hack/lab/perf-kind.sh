#!/usr/bin/env bash
# LAB-H10 / PERF-LAB-01 — quick pprof workflow entrypoint (Kind and bare-metal lab substrates).
# SPDX-License-Identifier: MIT
#
# Phases: idle → converge(N objects) → churn → recover
# pprof is disabled in product by default; lab enables via Helm values (pprof.enabled: true).
# Access via localhost port-forward only — never create a public Service for pprof.
#
# Live path: kubectl -n <ns> port-forward deploy/<release>-controller-manager 16060:6060
# then curl/go tool pprof against http://127.0.0.1:16060/debug/pprof/...
#
# Substrate gate (LAB-DEKIND): the kube context must be on the lab allowlist
# (hack/lab/substrates.conf — kind-* and the kumulus Talos lab). DEFAULT-DENY: any other
# context, including an ambient production one, is refused with exit 2.
#
# Exit codes:
#   0  OK (dry-run fixture or live run completed)
#   1  usage / invalid args
#   2  kube context refused (not on the substrate allowlist / ambiguous)
#   3  preflight / evidence / capture failure (live pprof BLOCKED when unreachable)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# shellcheck source=lib/evidence.sh
source "${SCRIPT_DIR}/lib/evidence.sh"
# shellcheck source=lib/pprof-capture.sh
source "${SCRIPT_DIR}/lib/pprof-capture.sh"
# shellcheck source=lib/substrate.sh
source "${SCRIPT_DIR}/lib/substrate.sh"

: "${LAB_PERF_KIND_LOG_PREFIX:=[perf-kind]}"

# Kind/dev defaults (override with --namespace / --release).
readonly LAB_PERF_KIND_DEFAULT_NAMESPACE="${KOLLECT_NAMESPACE:-kollect-system}"
readonly LAB_PERF_KIND_DEFAULT_RELEASE="${KOLLECT_RELEASE:-kollect}"
readonly LAB_PERF_KIND_PF_LOCAL_PORT=16060
readonly LAB_PERF_KIND_PF_REMOTE_PORT=6060

lab_perf_kind_log() { printf '%s %s\n' "${LAB_PERF_KIND_LOG_PREFIX}" "$*"; }
lab_perf_kind_err() { printf '%s FAIL: %s\n' "${LAB_PERF_KIND_LOG_PREFIX}" "$*" >&2; }

lab_perf_kind_usage() {
  cat <<EOF
Usage: hack/lab/perf-kind.sh --run-id <RUN_ID> [options]

Quick pprof workflow (LAB-H10 / PERF-LAB-01) for Kind AND bare-metal lab substrates.
Runs phased capture: idle → converge(N objects) → churn → recover

Never creates or destroys a cluster. The kube context must be on the lab substrate
allowlist (${SCRIPT_DIR}/substrates.conf: kind-* plus the kumulus Talos lab); DEFAULT-DENY
means any other context — notably an ambient production one — is refused with exit 2.

pprof is off in product Helm by default; enable \`pprof.enabled: true\` on the release.
Live capture uses localhost port-forward only (never a public Service):

  kubectl -n ${NAMESPACE} port-forward ${PF_RESOURCE} 16060:6060

Options:
  --run-id <id>           Required. Stable lab run id → kollect.dev/lab-run=<RUN_ID>.
  --dry-run               Offline fixture: DOC-02 evidence + profiles/ index (no kubectl).
  --objects <n>           Object budget for live converge/churn: 100 | 500 | 2000 (default: 500).
                          Ignored in --dry-run except as metadata in summaries.
  --duration <dur>        Phase dwell hint (default: 60s); CPU profile capped at 30s.
  --seed <n>              Deterministic seed for fixture metadata (default: 1).
  --artifacts-root <dir>  Evidence root (default: artifacts/lab under repo).
  --namespace <ns>        Manager namespace for port-forward (default: kollect-system).
  --release <name>        Helm release name; the port-forward target is
                          deploy/<release>-controller-manager (default: kollect).
                          The kumulus lab runs release kollect-op1 in namespace kollect-op1.
  --keep-lab              Hint: retain lab namespaces/resources (default: cleanup labeled workload).
  --allow-non-kind        Maintainer override for a context that is NOT on the substrate
                          allowlist. This is an escape hatch, not the way the lab runs —
                          allowlisted lab contexts need no flag. Still no cluster create/destroy.
  --fixture=<mode>        Offline context fixtures: context-kind | context-kumulus |
                          context-non-kind | context-ambiguous | context-kumulus-lookalike |
                          context-prod-lookalike
  --simulate-interrupt=<phase>  Dry-run only: stop after named phase; preserve partial profiles.
  --exercise-cleanup      Dry-run only: emit cleanup log for kollect.dev/lab-run label.
  -h, --help              Show this help.

Env:
  KOLLECT_LAB_PERF_KIND_CONTEXT_FIXTURE  Same as --fixture=context-* for meta-tests.
  KOLLECT_NAMESPACE                      Default --namespace (kollect-system).
  KOLLECT_RELEASE                        Default --release (kollect).
  KOLLECT_LAB_ALLOWED_CONTEXTS           Extra allowlist entries (validated; never a wildcard).
  KOLLECT_LAB_SUBSTRATES_FILE            Alternate allowlist file (validated the same way).

CI verifies the offline --dry-run quick path only. Live capture is maintainer opt-in and
runs on any allowlisted substrate (Kind or the kumulus Talos lab).
EOF
}

RUN_ID=""
DRY_RUN=0
OBJECTS=500
DURATION="60s"
SEED=1
ARTIFACTS_ROOT="${ROOT}/artifacts/lab"
NAMESPACE="${LAB_PERF_KIND_DEFAULT_NAMESPACE}"
RELEASE="${LAB_PERF_KIND_DEFAULT_RELEASE}"
PF_RESOURCE="deploy/${RELEASE}-controller-manager"
KEEP_LAB=0
ALLOW_NON_KIND=0
CONTEXT_FIXTURE="${KOLLECT_LAB_PERF_KIND_CONTEXT_FIXTURE:-}"
SIMULATE_INTERRUPT=""
EXERCISE_CLEANUP=0

PERF_KIND_RUN_DIR=""
PERF_KIND_INTERRUPTED=0
PERF_KIND_CAPTURE_STATUS="stub"

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
    context-kumulus | kumulus)
      printf 'kumulus-lab'
      return 0
      ;;
    context-kumulus-lookalike | kumulus-lookalike)
      # Near-miss on the lab name: must be refused (the allowlist is exact, not a prefix).
      printf 'kumulus-lab-prod'
      return 0
      ;;
    context-prod-lookalike | prod-lookalike)
      # Shape of a real ambient production context — the regression guard for the gate.
      printf 'gke_acme-platform-4711_europe-west1_shared-cluster'
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

# LAB-DEKIND: allowlist gate, not a kind-prefix test. Substrates permitted for lab work are
# enumerated in hack/lab/substrates.conf (kind-* and the kumulus Talos lab). Anything else —
# including the maintainer's ambient production context — is refused. --allow-non-kind stays
# as an explicit maintainer escape hatch; it is NOT how allowlisted lab clusters are reached.
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

  local assert_args=("${ctx}")
  # Fixtures must never reach a live cluster: skip the kubectl cluster-name confirmation.
  [[ -n "${fixture}" ]] && assert_args+=(--offline)

  if lab_substrate_assert_context "${assert_args[@]}"; then
    return 0
  fi

  if [[ "${allow_non_kind}" -eq 1 ]]; then
    lab_perf_kind_log "WARN: off-allowlist context '${ctx}' forced via --allow-non-kind (maintainer override)"
    return 0
  fi

  lab_perf_kind_err "refusing kube context '${ctx}': add it to the lab substrate allowlist, or pass --allow-non-kind to override deliberately"
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

  lab_perf_kind_log "phase: ${phase} (dwell ${DURATION}; objects=${OBJECTS} metadata)"
  lab_pprof_capture_phase_set "${run_dir}" "${phase}" "${seed}" "${fixture}" \
    "${LAB_PERF_KIND_PF_LOCAL_PORT}" "${DURATION}" "${OBJECTS}" ||
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
# perf-kind summary

Run \`${RUN_ID}\` — LAB-H10 / PERF-LAB-01 quick path (capture: ${PERF_KIND_CAPTURE_STATUS}).

| Field | Value |
| --- | --- |
| objects | ${OBJECTS} (live converge/churn; metadata-only in --dry-run) |
| seed | ${SEED} |
| duration / phase dwell | ${DURATION} |
| cpu sample cap | $(lab_pprof_cpu_seconds_from_duration "${DURATION}")s |
| phases | idle → converge → churn → recover |
| profiles index | profiles/index.md |
| findings | summary/performance-findings.md |
| helm release | ${RELEASE} (namespace ${NAMESPACE}) |
| port-forward | kubectl -n ${NAMESPACE} port-forward ${PF_RESOURCE} ${LAB_PERF_KIND_PF_LOCAL_PORT}:${LAB_PERF_KIND_PF_REMOTE_PORT} |

pprof: **localhost port-forward only** — never expose via public Service.
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
      --namespace)
        NAMESPACE="$2"
        shift 2
        ;;
      --release)
        RELEASE="$2"
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
  if ! lab_perf_kind_valid_run_id "${RELEASE}"; then
    lab_perf_kind_err "invalid --release (DNS1123 label): ${RELEASE}"
    exit 1
  fi
  PF_RESOURCE="deploy/${RELEASE}-controller-manager"

  lab_perf_kind_check_context "${ALLOW_NON_KIND}" "${DRY_RUN}" || exit 2

  local run_dir fixture
  run_dir="$(lab_evidence_run_dir "${ARTIFACTS_ROOT}" "${RUN_ID}")"
  PERF_KIND_RUN_DIR="${run_dir}"
  fixture=1
  PERF_KIND_CAPTURE_STATUS="stub"
  if [[ "${DRY_RUN}" -eq 0 ]]; then
    fixture=0
    PERF_KIND_CAPTURE_STATUS="live"
  fi

  trap 'lab_perf_kind_on_interrupt' INT TERM

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    lab_perf_kind_log "dry-run / fixture mode (no kubectl mutations); writing evidence under ${run_dir}"
  else
    lab_perf_kind_log "live capture mode: port-forward + curl/go tool pprof (objects=${OBJECTS} for converge/churn workload — workload apply follow-up)"
  fi

  lab_evidence_ensure_layout "${run_dir}" "${RUN_ID}"
  lab_pprof_ensure_profiles_dir "${run_dir}"
  lab_pprof_write_index_header "$(lab_pprof_index_path "${run_dir}")"

  if [[ "${DRY_RUN}" -eq 1 ]]; then
    lab_pprof_portforward_start "${run_dir}" "${NAMESPACE}" \
      "${LAB_PERF_KIND_PF_LOCAL_PORT}" "${LAB_PERF_KIND_PF_REMOTE_PORT}" \
      "${PF_RESOURCE}" --fixture
  else
    lab_pprof_portforward_start "${run_dir}" "${NAMESPACE}" \
      "${LAB_PERF_KIND_PF_LOCAL_PORT}" "${LAB_PERF_KIND_PF_REMOTE_PORT}" \
      "${PF_RESOURCE}" || {
      lab_perf_kind_err "port-forward failed; is kollect installed with pprof.enabled in ${NAMESPACE}?"
      lab_pprof_write_findings_blocked "${run_dir}" "${RUN_ID}" "port-forward to ${LAB_PERF_KIND_PF_RESOURCE} failed"
      exit 3
    }
    if ! lab_pprof_wait_ready "${LAB_PERF_KIND_PF_LOCAL_PORT}"; then
      lab_perf_kind_err "pprof endpoint unreachable at http://127.0.0.1:${LAB_PERF_KIND_PF_LOCAL_PORT}/debug/pprof/"
      lab_pprof_write_findings_blocked "${run_dir}" "${RUN_ID}" \
        "pprof endpoint unreachable after port-forward (enable pprof.enabled and check manager pod)"
      lab_pprof_portforward_stop "${run_dir}" || true
      exit 3
    fi
  fi

  local phase_rc=0
  for phase in idle converge churn recover; do
    lab_perf_kind_run_phase "${phase}" "${run_dir}" "${SEED}" "${fixture}" || {
      phase_rc=$?
      if [[ "${fixture}" -eq 0 && "${phase_rc}" -eq 3 ]]; then
        lab_perf_kind_err "live pprof capture failed in phase ${phase}; partial tree preserved"
        lab_pprof_write_findings_blocked "${run_dir}" "${RUN_ID}" \
          "live capture failed in phase ${phase} (curl/go tool pprof)"
        lab_pprof_portforward_stop "${run_dir}" || true
        trap - INT TERM
        exit 3
      fi
      break
    }
  done

  lab_pprof_write_differential "${run_dir}" idle converge heap "${SEED}" "${fixture}" || true
  lab_pprof_write_differential "${run_dir}" converge churn cpu "${SEED}" "${fixture}" || true
  lab_pprof_write_differential "${run_dir}" churn recover heap "${SEED}" "${fixture}" || true
  lab_pprof_write_differential "${run_dir}" idle recover goroutine "${SEED}" "${fixture}" || true

  if [[ "${PERF_KIND_CAPTURE_STATUS}" == "stub" ]]; then
    lab_pprof_write_findings_stub "${run_dir}" "${RUN_ID}" "${OBJECTS}" "${DURATION}" "stub"
  else
    lab_pprof_write_findings_stub "${run_dir}" "${RUN_ID}" "${OBJECTS}" "${DURATION}" "live"
  fi
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
