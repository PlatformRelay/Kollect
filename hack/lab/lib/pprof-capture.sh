#!/usr/bin/env bash
# LAB-H10 / PERF-LAB-01 — reusable pprof capture helpers for lab perf-kind runs.
# Source from hack/lab/perf-kind.sh — do not execute directly.
# Offline/fixture modes never call live kubectl or curl a real pprof endpoint.
# shellcheck shell=bash

: "${LAB_PPROF_LOG_PREFIX:=[lab-pprof]}"

lab_pprof_log() { printf '%s %s\n' "${LAB_PPROF_LOG_PREFIX}" "$*"; }
lab_pprof_err() { printf '%s FAIL: %s\n' "${LAB_PPROF_LOG_PREFIX}" "$*" >&2; }

# CPU profile duration bound (documented in index and capture stubs).
readonly LAB_PPROF_CPU_SECONDS="${LAB_PPROF_CPU_SECONDS:-30}"

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
  printf 'fixture'
}

lab_pprof_ensure_profiles_dir() {
  local run_dir="$1"
  mkdir -p "$(lab_pprof_profiles_dir "${run_dir}")"
}

# Write index.md header with PERF-LAB-01 columns.
lab_pprof_write_index_header() {
  local index_path="$1"
  mkdir -p "$(dirname "${index_path}")"
  cat >"${index_path}" <<'EOF'
# Profiles index (PERF-LAB-01)

| phase | profile type | duration | SHA256 | pprof version | top summary | differential | metrics window |
| --- | --- | --- | --- | --- | --- | --- | --- |
EOF
}

# Append one row to index.md (pipe-delimited fields escaped minimally).
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

# Deterministic stub profile bytes from (seed, phase, type).
lab_pprof_stub_profile_bytes() {
  local seed="$1"
  local phase="$2"
  local profile_type="$3"
  printf 'kollect-lab-pprof-fixture seed=%s phase=%s type=%s cpu_bound=%ss\n' \
    "${seed}" "${phase}" "${profile_type}" "${LAB_PPROF_CPU_SECONDS}"
}

# Capture placeholder profile + top summary for one phase/type.
# Usage: lab_pprof_capture_placeholder <run_dir> <phase> <profile_type> <seed> <fixture>
lab_pprof_capture_placeholder() {
  local run_dir="$1"
  local phase="$2"
  local profile_type="$3"
  local seed="$4"
  local fixture="${5:-1}"

  local profiles_dir phase_dir prof_file top_file duration metrics_note
  profiles_dir="$(lab_pprof_profiles_dir "${run_dir}")"
  phase_dir="${profiles_dir}/${phase}"
  mkdir -p "${phase_dir}"

  prof_file="${phase_dir}/${profile_type}.pb.gz.stub"
  top_file="${phase_dir}/${profile_type}.top.txt"

  case "${profile_type}" in
    cpu)
      duration="${LAB_PPROF_CPU_SECONDS}s"
      metrics_note="cpu window ${LAB_PPROF_CPU_SECONDS}s (go tool pprof profile?seconds=${LAB_PPROF_CPU_SECONDS})"
      ;;
    heap | allocs | goroutine)
      duration="snapshot"
      metrics_note="instantaneous ${profile_type} at phase boundary"
      ;;
    *)
      lab_pprof_err "unknown profile type: ${profile_type}"
      return 1
      ;;
  esac

  lab_pprof_stub_profile_bytes "${seed}" "${phase}" "${profile_type}" >"${prof_file}"

  {
    printf '# Top summary (fixture stub)\n'
    printf 'phase=%s type=%s seed=%s fixture=%s\n\n' "${phase}" "${profile_type}" "${seed}" "${fixture}"
    printf 'Flat%%\tFlat\tSum%%\tSum\tName\n'
    printf '0.00\t0\t100.00\t100\tmain.main\n'
    printf '0.00\t0\t50.00\t50\truntime.mallocgc\n'
  } >"${top_file}"

  local sha pprof_ver index_path rel_top
  sha="$(lab_pprof_sha256_file "${prof_file}")"
  pprof_ver="$(lab_pprof_tool_version "${fixture}")"
  index_path="$(lab_pprof_index_path "${run_dir}")"
  [[ -f "${index_path}" ]] || lab_pprof_write_index_header "${index_path}"
  rel_top="profiles/${phase}/${profile_type}.top.txt"
  lab_pprof_index_append_row "${index_path}" "${phase}" "${profile_type}" "${duration}" \
    "${sha}" "${pprof_ver}" "${rel_top}" "(none)" "${metrics_note}"

  lab_pprof_log "captured placeholder ${phase}/${profile_type} sha256=${sha}"
  return 0
}

# Write differential summary between two phases for a profile type.
# Usage: lab_pprof_write_differential <run_dir> <from_phase> <to_phase> <profile_type> <seed>
lab_pprof_write_differential() {
  local run_dir="$1"
  local from_phase="$2"
  local to_phase="$3"
  local profile_type="$4"
  local seed="$5"

  local profiles_dir diff_file rel_diff index_path
  profiles_dir="$(lab_pprof_profiles_dir "${run_dir}")"
  diff_file="${profiles_dir}/diff-${from_phase}-to-${to_phase}-${profile_type}.txt"
  rel_diff="profiles/diff-${from_phase}-to-${to_phase}-${profile_type}.txt"

  {
    printf '# Differential summary (fixture stub)\n'
    printf 'from=%s to=%s type=%s seed=%s\n\n' "${from_phase}" "${to_phase}" "${profile_type}" "${seed}"
    printf 'Showing nodes accounting for delta between %s and %s phases.\n' "${from_phase}" "${to_phase}"
    printf 'Flat%%\tFlat\tName\n'
    printf '0.00\t0\truntime.mallocgc\n'
  } >"${diff_file}"

  index_path="$(lab_pprof_index_path "${run_dir}")"
  if [[ -f "${index_path}" ]]; then
    # Append a synthetic row noting the differential artifact (phase=from→to).
    lab_pprof_index_append_row "${index_path}" "${from_phase}→${to_phase}" "${profile_type}" \
      "delta" "$(lab_pprof_sha256_file "${diff_file}")" "$(lab_pprof_tool_version 1)" \
      "(see phase rows)" "${rel_diff}" "differential ${from_phase}↔${to_phase}"
  fi

  lab_pprof_log "wrote differential ${from_phase}→${to_phase}/${profile_type}"
  return 0
}

# Capture standard profile set for a phase (heap, allocs, goroutine, cpu).
lab_pprof_capture_phase_set() {
  local run_dir="$1"
  local phase="$2"
  local seed="$3"
  local fixture="${4:-1}"
  local ptype

  for ptype in heap allocs goroutine cpu; do
    lab_pprof_capture_placeholder "${run_dir}" "${phase}" "${ptype}" "${seed}" "${fixture}" ||
      return 1
  done
  return 0
}

# Port-forward lifecycle: localhost only — never create a public Service for pprof.
# Usage: lab_pprof_portforward_start <run_dir> <local_port> <target> [--fixture]
lab_pprof_portforward_start() {
  local run_dir="$1"
  local local_port="$2"
  local target="$3"
  shift 3 || true
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
    lab_pprof_log "port-forward fixture: localhost:${local_port} → ${target} (pid $$)"
    return 0
  fi

  if ! command -v kubectl >/dev/null 2>&1; then
    lab_pprof_err "kubectl required for live port-forward"
    return 1
  fi

  kubectl port-forward "${target}" "${local_port}:6060" >/dev/null 2>&1 &
  printf '%s\n' "$!" >"${pidfile}"
  lab_pprof_log "port-forward started localhost:${local_port} → ${target} (pid $(cat "${pidfile}"))"
  return 0
}

# Stop port-forward tracked in pid file; idempotent.
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

# Write performance finding register stub under summary/.
lab_pprof_write_findings_stub() {
  local run_dir="$1"
  local run_id="$2"
  local objects="$3"

  mkdir -p "${run_dir}/summary"
  cat >"${run_dir}/summary/performance-findings.md" <<EOF
# Performance finding register (stub)

Run \`${run_id}\` — PERF-LAB-01 quick Kind pprof path (fixture/offline when \`--dry-run\`).

| Finding ID | Phase | Signal | Status | Notes |
| --- | --- | --- | --- | --- |
| PERF-001 | idle | heap/goroutine baseline | stub | objects=${objects}; see \`profiles/index.md\` |
| PERF-002 | converge | heap delta vs idle | stub | differential rows in index |
| PERF-003 | churn | cpu (${LAB_PPROF_CPU_SECONDS}s bound) | stub | live Kind maintainer opt-in |
| PERF-004 | recover | post-churn footprint | stub | compare recover↔idle in index |

> Stub created by hack/lab/perf-kind.sh. Maintainer fills findings after live Kind review.
> pprof access: **localhost port-forward only** — never expose via public Service.
EOF
}
