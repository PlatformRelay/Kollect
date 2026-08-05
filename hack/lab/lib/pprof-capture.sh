#!/usr/bin/env bash
# LAB-H10 / PERF-LAB-01 — reusable pprof capture helpers for lab perf-kind runs.
# Source from hack/lab/perf-kind.sh — do not execute directly.
# Offline/fixture modes never call live kubectl or curl a real pprof endpoint.
# shellcheck shell=bash

: "${LAB_PPROF_LOG_PREFIX:=[lab-pprof]}"

lab_pprof_log() { printf '%s %s\n' "${LAB_PPROF_LOG_PREFIX}" "$*"; }
lab_pprof_err() { printf '%s FAIL: %s\n' "${LAB_PPROF_LOG_PREFIX}" "$*" >&2; }

# CPU profile duration bound (documented in index; overridable via --duration up to 30s).
readonly LAB_PPROF_CPU_SECONDS_MAX=30

lab_pprof_profiles_dir() {
  local run_dir="$1"
  printf '%s/profiles\n' "${run_dir%/}"
}

lab_pprof_index_path() {
  local run_dir="$1"
  printf '%s/profiles/index.md\n' "${run_dir%/}"
}

lab_pprof_portforward_pidfile() {
  local run_dir="$1"
  printf '%s/profiles/.port-forward.pid\n' "${run_dir%/}"
}

lab_pprof_sha256_file() {
  local f="$1"
  if [[ ! -f "${f}" ]]; then
    printf 'missing'
    return 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${f}" | awk '{print $1}'
  else
    shasum -a 256 "${f}" | awk '{print $1}'
  fi
}

# Parse a simple duration like 45s / 60s → integer seconds (default 60).
lab_pprof_parse_duration_seconds() {
  local dur="${1:-60s}"
  if [[ "${dur}" =~ ^([0-9]+)s$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi
  if [[ "${dur}" =~ ^[0-9]+$ ]]; then
    printf '%s' "${dur}"
    return 0
  fi
  printf '60'
}

# CPU sample seconds: min(parsed duration, 30s cap).
lab_pprof_cpu_seconds_from_duration() {
  local dur="${1:-60s}"
  local secs
  secs="$(lab_pprof_parse_duration_seconds "${dur}")"
  if ((secs > LAB_PPROF_CPU_SECONDS_MAX)); then
    secs="${LAB_PPROF_CPU_SECONDS_MAX}"
  fi
  if ((secs < 1)); then
    secs=1
  fi
  printf '%s' "${secs}"
}

# Returns go tool pprof version string, or "fixture" when offline/fixture.
lab_pprof_tool_version() {
  local fixture="${1:-0}"
  if [[ "${fixture}" -eq 1 ]]; then
    printf 'fixture'
    return 0
  fi
  if command -v go >/dev/null 2>&1; then
    go tool pprof -help 2>&1 | head -1 | sed 's/^.*pprof //' || printf 'go-tool-pprof'
    return 0
  fi
  printf 'unknown'
}

lab_pprof_ensure_profiles_dir() {
  local run_dir="$1"
  mkdir -p "$(lab_pprof_profiles_dir "${run_dir}")"
}

lab_pprof_write_index_header() {
  local index_path="$1"
  mkdir -p "$(dirname "${index_path}")"
  cat >"${index_path}" <<'EOF'
# Profiles index (PERF-LAB-01)

| phase | profile type | duration | SHA256 | pprof version | top summary | differential | metrics window |
| --- | --- | --- | --- | --- | --- | --- | --- |
EOF
}

lab_pprof_index_append_row() {
  local index_path="$1"
  local phase="$2"
  local profile_type="$3"
  local duration="$4"
  local sha256="$5"
  local pprof_version="$6"
  local top_path="$7"
  local diff_path="$8"
  local metrics_note="$9"

  printf '| %s | %s | %s | %s | %s | %s | %s | %s |\n' \
    "${phase}" "${profile_type}" "${duration}" "${sha256}" "${pprof_version}" \
    "${top_path}" "${diff_path}" "${metrics_note}" >>"${index_path}"
}

lab_pprof_stub_profile_bytes() {
  local seed="$1"
  local phase="$2"
  local profile_type="$3"
  local cpu_seconds="$4"
  printf 'kollect-lab-pprof-fixture seed=%s phase=%s type=%s cpu_bound=%ss\n' \
    "${seed}" "${phase}" "${profile_type}" "${cpu_seconds}"
}

# Capture placeholder profile + top summary for one phase/type (--dry-run only).
lab_pprof_capture_placeholder() {
  local run_dir="$1"
  local phase="$2"
  local profile_type="$3"
  local seed="$4"
  local phase_duration="${5:-60s}"
  local objects="${6:-500}"

  local profiles_dir phase_dir prof_file top_file duration metrics_note cpu_seconds
  profiles_dir="$(lab_pprof_profiles_dir "${run_dir}")"
  phase_dir="${profiles_dir}/${phase}"
  mkdir -p "${phase_dir}"

  prof_file="${phase_dir}/${profile_type}.pb.gz.stub"
  top_file="${phase_dir}/${profile_type}.top.txt"
  cpu_seconds="$(lab_pprof_cpu_seconds_from_duration "${phase_duration}")"

  case "${profile_type}" in
    cpu)
      duration="${cpu_seconds}s"
      metrics_note="cpu window ${cpu_seconds}s (phase dwell ${phase_duration}; objects=${objects} live-only)"
      ;;
    heap | allocs | goroutine)
      duration="snapshot"
      metrics_note="instantaneous ${profile_type} at phase boundary (phase dwell ${phase_duration}; objects=${objects} live-only)"
      ;;
    *)
      lab_pprof_err "unknown profile type: ${profile_type}"
      return 1
      ;;
  esac

  lab_pprof_stub_profile_bytes "${seed}" "${phase}" "${profile_type}" "${cpu_seconds}" >"${prof_file}"

  {
    printf '# Top summary (fixture stub)\n'
    printf 'phase=%s type=%s seed=%s fixture=1 objects=%s phase_dwell=%s\n\n' \
      "${phase}" "${profile_type}" "${seed}" "${objects}" "${phase_duration}"
    printf 'Flat%%\tFlat\tSum%%\tSum\tName\n'
    printf '0.00\t0\t100.00\t100\tmain.main\n'
    printf '0.00\t0\t50.00\t50\truntime.mallocgc\n'
  } >"${top_file}"

  local sha pprof_ver index_path rel_top
  sha="$(lab_pprof_sha256_file "${prof_file}")"
  pprof_ver="$(lab_pprof_tool_version 1)"
  index_path="$(lab_pprof_index_path "${run_dir}")"
  [[ -f "${index_path}" ]] || lab_pprof_write_index_header "${index_path}"
  rel_top="profiles/${phase}/${profile_type}.top.txt"
  lab_pprof_index_append_row "${index_path}" "${phase}" "${profile_type}" "${duration}" \
    "${sha}" "${pprof_ver}" "${rel_top}" "(none)" "${metrics_note}"

  lab_pprof_log "captured placeholder ${phase}/${profile_type} sha256=${sha}"
  return 0
}

# Live capture via localhost port-forward + curl (+ go tool pprof -top when available).
lab_pprof_capture_live() {
  local run_dir="$1"
  local phase="$2"
  local profile_type="$3"
  local local_port="$4"
  local phase_duration="${5:-60s}"
  local objects="${6:-500}"

  local profiles_dir phase_dir prof_file top_file duration metrics_note cpu_seconds
  local base_url url_path curl_rc
  profiles_dir="$(lab_pprof_profiles_dir "${run_dir}")"
  phase_dir="${profiles_dir}/${phase}"
  mkdir -p "${phase_dir}"

  prof_file="${phase_dir}/${profile_type}.pb.gz"
  top_file="${phase_dir}/${profile_type}.top.txt"
  cpu_seconds="$(lab_pprof_cpu_seconds_from_duration "${phase_duration}")"
  base_url="http://127.0.0.1:${local_port}"

  case "${profile_type}" in
    heap) url_path="/debug/pprof/heap" ;;
    allocs) url_path="/debug/pprof/allocs" ;;
    goroutine) url_path="/debug/pprof/goroutine" ;;
    cpu) url_path="/debug/pprof/profile?seconds=${cpu_seconds}" ;;
    *)
      lab_pprof_err "unknown profile type: ${profile_type}"
      return 1
      ;;
  esac

  if ! command -v curl >/dev/null 2>&1; then
    lab_pprof_err "curl required for live pprof capture"
    return 1
  fi

  curl_rc=0
  curl -sf "${base_url}${url_path}" -o "${prof_file}" || curl_rc=$?
  if [[ "${curl_rc}" -ne 0 || ! -s "${prof_file}" ]]; then
    lab_pprof_err "live capture failed: ${phase}/${profile_type} from ${base_url}${url_path} (curl exit ${curl_rc})"
    rm -f "${prof_file}"
    return 1
  fi

  case "${profile_type}" in
    cpu)
      duration="${cpu_seconds}s"
      metrics_note="cpu window ${cpu_seconds}s live capture (phase dwell ${phase_duration}; objects=${objects})"
      ;;
    *)
      duration="snapshot"
      metrics_note="instantaneous ${profile_type} live capture (phase dwell ${phase_duration}; objects=${objects})"
      ;;
  esac

  if command -v go >/dev/null 2>&1; then
    go tool pprof -top "${prof_file}" >"${top_file}" 2>/dev/null || {
      printf '# Top summary unavailable (go tool pprof failed)\n' >"${top_file}"
    }
  else
    printf '# Top summary unavailable (go not installed)\n' >"${top_file}"
  fi

  local sha pprof_ver index_path rel_top
  sha="$(lab_pprof_sha256_file "${prof_file}")"
  pprof_ver="$(lab_pprof_tool_version 0)"
  index_path="$(lab_pprof_index_path "${run_dir}")"
  [[ -f "${index_path}" ]] || lab_pprof_write_index_header "${index_path}"
  rel_top="profiles/${phase}/${profile_type}.top.txt"
  lab_pprof_index_append_row "${index_path}" "${phase}" "${profile_type}" "${duration}" \
    "${sha}" "${pprof_ver}" "${rel_top}" "(none)" "${metrics_note}"

  lab_pprof_log "captured live ${phase}/${profile_type} sha256=${sha}"
  return 0
}

lab_pprof_write_differential() {
  local run_dir="$1"
  local from_phase="$2"
  local to_phase="$3"
  local profile_type="$4"
  local seed="$5"
  local fixture="${6:-1}"

  local profiles_dir diff_file rel_diff index_path
  profiles_dir="$(lab_pprof_profiles_dir "${run_dir}")"
  diff_file="${profiles_dir}/diff-${from_phase}-to-${to_phase}-${profile_type}.txt"
  rel_diff="profiles/diff-${from_phase}-to-${to_phase}-${profile_type}.txt"

  local from_prof to_prof
  if [[ "${fixture}" -eq 1 ]]; then
    from_prof="${profiles_dir}/${from_phase}/${profile_type}.pb.gz.stub"
    to_prof="${profiles_dir}/${to_phase}/${profile_type}.pb.gz.stub"
  else
    from_prof="${profiles_dir}/${from_phase}/${profile_type}.pb.gz"
    to_prof="${profiles_dir}/${to_phase}/${profile_type}.pb.gz"
  fi

  if [[ "${fixture}" -eq 0 ]] && [[ -f "${from_prof}" && -f "${to_prof}" ]] && command -v go >/dev/null 2>&1; then
    go tool pprof -top -base "${from_prof}" "${to_prof}" >"${diff_file}" 2>/dev/null || {
      fixture=1
    }
  fi

  if [[ "${fixture}" -eq 1 ]]; then
    {
      printf '# Differential summary (fixture stub)\n'
      printf 'from=%s to=%s type=%s seed=%s\n\n' "${from_phase}" "${to_phase}" "${profile_type}" "${seed}"
      printf 'Showing nodes accounting for delta between %s and %s phases.\n' "${from_phase}" "${to_phase}"
      printf 'Flat%%\tFlat\tName\n'
      printf '0.00\t0\truntime.mallocgc\n'
    } >"${diff_file}"
  fi

  index_path="$(lab_pprof_index_path "${run_dir}")"
  if [[ -f "${index_path}" ]]; then
    lab_pprof_index_append_row "${index_path}" "${from_phase}→${to_phase}" "${profile_type}" \
      "delta" "$(lab_pprof_sha256_file "${diff_file}")" "$(lab_pprof_tool_version "${fixture}")" \
      "(see phase rows)" "${rel_diff}" "differential ${from_phase}↔${to_phase}"
  fi

  lab_pprof_log "wrote differential ${from_phase}→${to_phase}/${profile_type}"
  return 0
}

# Capture standard profile set for a phase.
lab_pprof_capture_phase_set() {
  local run_dir="$1"
  local phase="$2"
  local seed="$3"
  local fixture="${4:-1}"
  local local_port="${5:-16060}"
  local phase_duration="${6:-60s}"
  local objects="${7:-500}"
  local ptype fail=0

  for ptype in heap allocs goroutine cpu; do
    if [[ "${fixture}" -eq 1 ]]; then
      lab_pprof_capture_placeholder "${run_dir}" "${phase}" "${ptype}" "${seed}" \
        "${phase_duration}" "${objects}" || fail=1
    else
      lab_pprof_capture_live "${run_dir}" "${phase}" "${ptype}" "${local_port}" \
        "${phase_duration}" "${objects}" || fail=1
    fi
  done
  return "${fail}"
}

# Wait for pprof HTTP endpoint after port-forward (live only).
lab_pprof_wait_ready() {
  local local_port="$1"
  local attempts="${2:-15}"
  local i

  if ! command -v curl >/dev/null 2>&1; then
    return 1
  fi
  for ((i = 1; i <= attempts; i++)); do
    if curl -sf "http://127.0.0.1:${local_port}/debug/pprof/" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

# Port-forward lifecycle: localhost only — never create a public Service for pprof.
# Kind/dev: kubectl -n kollect-system port-forward deploy/kollect-controller-manager 16060:6060
# (pprof listens on container :6060; Service spec does not expose it — deployment forward matches load-test-runbook)
lab_pprof_portforward_start() {
  local run_dir="$1"
  local namespace="$2"
  local local_port="$3"
  local remote_port="$4"
  local resource="${5:-deploy/kollect-controller-manager}"
  shift 5 || true
  local fixture=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --fixture) fixture=1 ;;
      *)
        lab_pprof_err "portforward_start: unknown arg: $1"
        return 1
        ;;
    esac
    shift
  done

  local pidfile
  pidfile="$(lab_pprof_portforward_pidfile "${run_dir}")"
  lab_pprof_ensure_profiles_dir "${run_dir}"

  if [[ -f "${pidfile}" ]]; then
    lab_pprof_log "port-forward already tracked at ${pidfile}; stopping first"
    lab_pprof_portforward_stop "${run_dir}" || true
  fi

  if [[ "${fixture}" -eq 1 || "${KOLLECT_LAB_PPROF_FIXTURE:-0}" == "1" ]]; then
    printf '%s\n' "$$" >"${pidfile}"
    lab_pprof_log "port-forward fixture: kubectl -n ${namespace} port-forward ${resource} ${local_port}:${remote_port} (pid $$)"
    return 0
  fi

  if ! command -v kubectl >/dev/null 2>&1; then
    lab_pprof_err "kubectl required for live port-forward"
    return 1
  fi

  kubectl -n "${namespace}" port-forward "${resource}" "${local_port}:${remote_port}" >/dev/null 2>&1 &
  printf '%s\n' "$!" >"${pidfile}"
  lab_pprof_log "port-forward started: kubectl -n ${namespace} port-forward ${resource} ${local_port}:${remote_port} (pid $(cat "${pidfile}"))"
  return 0
}

lab_pprof_portforward_stop() {
  local run_dir="$1"
  local pidfile pid

  pidfile="$(lab_pprof_portforward_pidfile "${run_dir}")"
  [[ -f "${pidfile}" ]] || return 0

  pid="$(tr -d '[:space:]' <"${pidfile}")"
  if [[ -n "${pid}" && "${pid}" != "$$" ]]; then
    kill "${pid}" 2>/dev/null || true
  fi
  rm -f "${pidfile}"
  lab_pprof_log "port-forward stopped"
  return 0
}

lab_pprof_write_findings_stub() {
  local run_dir="$1"
  local run_id="$2"
  local objects="$3"
  local phase_duration="${4:-60s}"
  local capture_status="${5:-stub}"

  mkdir -p "${run_dir}/summary"
  cat >"${run_dir}/summary/performance-findings.md" <<EOF
# Performance finding register (stub)

Run \`${run_id}\` — PERF-LAB-01 quick Kind pprof path (capture status: ${capture_status}).

| Finding ID | Phase | Signal | Status | Notes |
| --- | --- | --- | --- | --- |
| PERF-001 | idle | heap/goroutine baseline | ${capture_status} | objects=${objects} (live converge); phase dwell ${phase_duration} |
| PERF-002 | converge | heap delta vs idle | ${capture_status} | differential rows in index |
| PERF-003 | churn | cpu ($(lab_pprof_cpu_seconds_from_duration "${phase_duration}")s bound) | ${capture_status} | live Kind maintainer opt-in |
| PERF-004 | recover | post-churn footprint | ${capture_status} | compare recover↔idle in index |

> pprof access: **localhost port-forward only** (\`kubectl -n kollect-system port-forward deploy/kollect-controller-manager 16060:6060\`) — never expose via public Service.
EOF
}

lab_pprof_write_findings_blocked() {
  local run_dir="$1"
  local run_id="$2"
  local reason="$3"

  mkdir -p "${run_dir}/summary"
  cat >"${run_dir}/summary/performance-findings.md" <<EOF
# Performance finding register

Run \`${run_id}\` — PERF-LAB-01 live capture **BLOCKED**.

| Finding ID | Phase | Signal | Status | Notes |
| --- | --- | --- | --- | --- |
| PERF-000 | (all) | pprof endpoint | BLOCKED | ${reason} |

Partial \`profiles/\` tree preserved when present. Enable \`pprof.enabled: true\` on the Helm release and ensure \`kubectl -n kollect-system port-forward deploy/kollect-controller-manager 16060:6060\` reaches the manager.
EOF
}
