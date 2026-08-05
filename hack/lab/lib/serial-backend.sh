#!/usr/bin/env bash
# LAB-H02 — serial Wave-2b backend orchestration helpers.
# Assert tear-down of the previous backend before starting the next.
# Source this file; do not execute it directly.
# SPDX-License-Identifier: MIT
# shellcheck shell=bash

# State file format (one line):
#   CLEAR
#   UP <backend-id>
#   BEGIN <backend-id>   (reserved / about to mark up)

lab_serial_backend_reset() {
  local state_file="${1:?state file required}"
  mkdir -p "$(dirname "${state_file}")"
  printf 'CLEAR\n' >"${state_file}"
}

lab_serial_backend_assert_clear() {
  local state_file="${1:?state file required}"
  local line
  [[ -f "${state_file}" ]] || {
    printf 'lab_serial_backend: missing state file %s\n' "${state_file}" >&2
    return 1
  }
  line="$(head -n1 "${state_file}" || true)"
  if [[ "${line}" != "CLEAR" ]]; then
    printf 'lab_serial_backend: expected CLEAR, got %q\n' "${line}" >&2
    return 1
  fi
  return 0
}

# Begin a backend scenario. Fails if another backend is still UP.
lab_serial_backend_begin() {
  local state_file="${1:?state file required}"
  local backend_id="${2:?backend id required}"
  local line status current

  if [[ ! -f "${state_file}" ]]; then
    lab_serial_backend_reset "${state_file}"
  fi

  line="$(head -n1 "${state_file}" || true)"
  status="${line%% *}"
  current="${line#* }"
  if [[ "${status}" == "UP" ]]; then
    printf 'lab_serial_backend: refuse begin %s — backend %s still UP (tear down first)\n' \
      "${backend_id}" "${current}" >&2
    return 1
  fi
  if [[ "${status}" == "BEGIN" && "${current}" != "${backend_id}" ]]; then
    printf 'lab_serial_backend: refuse begin %s — begin already in progress for %s\n' \
      "${backend_id}" "${current}" >&2
    return 1
  fi
  printf 'BEGIN %s\n' "${backend_id}" >"${state_file}"
  return 0
}

lab_serial_backend_mark_up() {
  local state_file="${1:?state file required}"
  local backend_id="${2:?backend id required}"
  local line status current

  line="$(head -n1 "${state_file}" 2>/dev/null || true)"
  status="${line%% *}"
  current="${line#* }"
  if [[ "${status}" != "BEGIN" && "${status}" != "UP" ]]; then
    printf 'lab_serial_backend: mark_up %s requires BEGIN/UP, got %q\n' "${backend_id}" "${line}" >&2
    return 1
  fi
  if [[ "${current}" != "${backend_id}" ]]; then
    printf 'lab_serial_backend: mark_up id mismatch want=%s have=%s\n' "${backend_id}" "${current}" >&2
    return 1
  fi
  printf 'UP %s\n' "${backend_id}" >"${state_file}"
  return 0
}

lab_serial_backend_teardown() {
  local state_file="${1:?state file required}"
  local backend_id="${2:?backend id required}"
  local line status current

  line="$(head -n1 "${state_file}" 2>/dev/null || true)"
  status="${line%% *}"
  current="${line#* }"
  if [[ "${status}" == "CLEAR" ]]; then
    return 0
  fi
  if [[ "${current}" != "${backend_id}" ]]; then
    printf 'lab_serial_backend: teardown id mismatch want=%s have=%s\n' "${backend_id}" "${current}" >&2
    return 1
  fi
  printf 'CLEAR\n' >"${state_file}"
  return 0
}

# True if scenario id is a serial Wave-2b backend scenario.
lab_serial_backend_is_backend_scenario() {
  case "${1:-}" in
    DR-2b.*) return 0 ;;
    *) return 1 ;;
  esac
}

# Map DR-2b.* → short backend token used in state file.
lab_serial_backend_id_for_scenario() {
  case "${1:-}" in
    DR-2b.1) printf 'local-fs\n' ;;
    DR-2b.2) printf 'bare-git\n' ;;
    DR-2b.3) printf 'postgres\n' ;;
    DR-2b.4) printf 'minio\n' ;;
    DR-2b.5) printf 'nats\n' ;;
    DR-2b.6) printf 'forgejo\n' ;;
    DR-2b.7) printf 'mongodb\n' ;;
    DR-2b.8) printf 'redpanda\n' ;;
    DR-2b.9) printf 'fan-out\n' ;;
    DR-2b.10) printf 'bq-gcs\n' ;;
    DR-2b.11) printf 'github\n' ;;
    DR-2b.12) printf 'gitlab\n' ;;
    *) printf '%s\n' "${1:-unknown}" ;;
  esac
}
