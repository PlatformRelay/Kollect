#!/usr/bin/env bash
# Lab harness preflight library (LAB-H01). Source from hack/lab/preflight.sh — do not execute.
# SPDX-License-Identifier: MIT
# shellcheck shell=bash

# Exit codes (also documented in hack/lab/README.md):
#   0  preflight OK (clean, or residue overridden with --force)
#   1  usage / invalid fixture / hard host failure
#   2  isolation residue detected without --force
#   3  required tool missing when KOLLECT_LAB_PREFLIGHT_STRICT=1

: "${LAB_PREFLIGHT_LOG_PREFIX:=[lab-preflight]}"

lab_preflight_log() { printf '%s %s\n' "${LAB_PREFLIGHT_LOG_PREFIX}" "$*"; }
lab_preflight_warn() { printf '%s WARN: %s\n' "${LAB_PREFLIGHT_LOG_PREFIX}" "$*" >&2; }
lab_preflight_err() { printf '%s FAIL: %s\n' "${LAB_PREFLIGHT_LOG_PREFIX}" "$*" >&2; }

# Resolve kubeconfig path: KUBECONFIG (first entry) or ~/.kube/config. Never creates a cluster.
lab_preflight_kubeconfig_path() {
  local kc="${KUBECONFIG:-}"
  if [[ -n "${kc}" ]]; then
    # KUBECONFIG may be a colon-separated list; use the first existing entry.
    local entry
    IFS=':' read -r -a _kc_entries <<<"${kc}"
    for entry in "${_kc_entries[@]}"; do
      [[ -n "${entry}" ]] || continue
      if [[ -f "${entry}" ]]; then
        printf '%s' "${entry}"
        return 0
      fi
    done
    # Prefer first entry even if missing (caller reports).
    printf '%s' "${_kc_entries[0]:-}"
    return 0
  fi
  printf '%s' "${HOME}/.kube/config"
}

lab_preflight_print_context() {
  local kc_path="$1"
  if ! command -v kubectl >/dev/null 2>&1; then
    lab_preflight_warn "kubectl not found; cannot print current context"
    return 0
  fi
  if [[ ! -f "${kc_path}" ]]; then
    lab_preflight_warn "kubeconfig not found at ${kc_path}; cannot print context"
    return 0
  fi
  local ctx
  if ctx="$(kubectl --kubeconfig="${kc_path}" config current-context 2>/dev/null)"; then
    lab_preflight_log "kubeconfig=${kc_path} context=${ctx}"
  else
    lab_preflight_warn "kubeconfig=${kc_path} has no current context"
  fi
}

# Offline host/tool checks. kubectl/helm/jq are optional (WARN) unless STRICT=1.
lab_preflight_check_host_tools() {
  local strict="${KOLLECT_LAB_PREFLIGHT_STRICT:-0}"
  local missing_optional=0
  local hard_fail=0

  if [[ -z "${BASH_VERSINFO:-}" ]]; then
    lab_preflight_err "bash version unknown"
    hard_fail=1
  else
    lab_preflight_log "bash ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}"
    if ((BASH_VERSINFO[0] < 4)); then
      lab_preflight_warn "bash ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]} < 4; lab scripts prefer bash 4+"
      if [[ "${strict}" == "1" ]]; then
        hard_fail=1
      fi
    fi
  fi

  local opt
  for opt in kubectl helm jq; do
    if command -v "${opt}" >/dev/null 2>&1; then
      lab_preflight_log "tool ok: ${opt}"
    else
      lab_preflight_warn "optional tool missing: ${opt}"
      missing_optional=1
    fi
  done

  if [[ "${strict}" == "1" && "${missing_optional}" -eq 1 ]]; then
    lab_preflight_err "KOLLECT_LAB_PREFLIGHT_STRICT=1 and optional tools missing"
    return 3
  fi
  if [[ "${hard_fail}" -eq 1 ]]; then
    return 1
  fi
  return 0
}

# Detect residue namespaces kollect-lab-* and/or label kollect.dev/lab-run.
# Prints residue summary to stdout; returns 0 if clean, 1 if residue found.
# When fixture is set, never calls kubectl.
lab_preflight_detect_residue() {
  local fixture="${1:-}"
  case "${fixture}" in
    clean)
      return 0
      ;;
    residue)
      printf 'kollect-lab-fixture (fixture)\n'
      printf 'labeled: kollect.dev/lab-run=fixture\n'
      return 1
      ;;
  esac

  if ! command -v kubectl >/dev/null 2>&1; then
    lab_preflight_warn "kubectl unavailable; skipping live isolation scan"
    return 0
  fi

  local kc_path
  kc_path="$(lab_preflight_kubeconfig_path)"
  local kubectl_base=(kubectl)
  if [[ -f "${kc_path}" ]]; then
    kubectl_base=(kubectl --kubeconfig="${kc_path}")
  fi

  local ns_list="" labeled=""
  ns_list="$("${kubectl_base[@]}" get ns -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"
  local residue_ns=()
  local line
  while IFS= read -r line; do
    [[ -z "${line}" ]] && continue
    if [[ "${line}" == kollect-lab-* ]]; then
      residue_ns+=("${line}")
    fi
  done <<<"${ns_list}"

  labeled="$("${kubectl_base[@]}" get ns,all,cm,secret,sa -A \
    -l 'kollect.dev/lab-run' \
    -o jsonpath='{range .items[*]}{.kind}/{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null || true)"

  local found=0
  if ((${#residue_ns[@]} > 0)); then
    found=1
    printf 'namespaces:\n'
    printf '  %s\n' "${residue_ns[@]}"
  fi
  if [[ -n "${labeled}" ]]; then
    found=1
    printf 'labeled kollect.dev/lab-run:\n'
    while IFS= read -r line; do
      [[ -z "${line}" ]] && continue
      printf '  %s\n' "${line}"
    done <<<"${labeled}"
  fi
  if [[ "${found}" -eq 1 ]]; then
    return 1
  fi
  return 0
}

lab_preflight_usage() {
  cat <<'EOF'
Usage: hack/lab/preflight.sh [options]

Cluster-agnostic lab preflight (LAB-H01 / ADR-0707). Discovers an existing
kubeconfig (KUBECONFIG or ~/.kube/config), checks host tools, and refuses to
proceed when prior lab residue is present — unless --force.

Never creates or destroys a cluster.

Options:
  --force                 Allow proceeding despite kollect-lab-* / lab-run residue
  --dry-run               Report checks only; with no fixture, skip live API scans
  --fixture=clean|residue Offline mode for meta-tests (also: KOLLECT_LAB_PREFLIGHT_FIXTURE)
  -h, --help              Show this help

Exit codes:
  0  OK
  1  usage / invalid args / hard host failure
  2  isolation residue without --force
  3  STRICT mode and optional tools missing

Env:
  KUBECONFIG                     Existing kubeconfig path (first-class; non-Kind OK)
  KOLLECT_LAB_PREFLIGHT_FIXTURE  clean|residue (offline)
  KOLLECT_LAB_PREFLIGHT_STRICT   1 = missing kubectl/helm/jq is FAIL
EOF
}

# Main entry used by the CLI. Args are the same as preflight.sh.
lab_preflight_main() {
  local force=0 dry_run=0 fixture="${KOLLECT_LAB_PREFLIGHT_FIXTURE:-}"
  local arg

  for arg in "$@"; do
    case "${arg}" in
      --force) force=1 ;;
      --dry-run) dry_run=1 ;;
      --fixture=*)
        fixture="${arg#--fixture=}"
        ;;
      -h|--help)
        lab_preflight_usage
        return 0
        ;;
      *)
        lab_preflight_err "unknown argument: ${arg}"
        lab_preflight_usage >&2
        return 1
        ;;
    esac
  done

  case "${fixture}" in
    ""|clean|residue) ;;
    *)
      lab_preflight_err "invalid fixture '${fixture}' (want clean|residue)"
      return 1
      ;;
  esac

  lab_preflight_log "never create/destroy cluster; using existing kubeconfig only"
  if [[ -n "${fixture}" ]]; then
    lab_preflight_log "fixture mode: ${fixture}"
  fi

  local tool_rc=0
  lab_preflight_check_host_tools || tool_rc=$?
  if [[ "${tool_rc}" -ne 0 ]]; then
    return "${tool_rc}"
  fi

  local kc_path
  kc_path="$(lab_preflight_kubeconfig_path)"
  lab_preflight_log "kubeconfig path: ${kc_path}"

  # Context print: fixture/dry-run skip live kubectl to keep meta-tests offline.
  if [[ -z "${fixture}" && "${dry_run}" -eq 0 ]]; then
    lab_preflight_print_context "${kc_path}"
  elif [[ -n "${fixture}" ]]; then
    lab_preflight_log "kubeconfig=${kc_path} context=(fixture; skipped live lookup)"
  else
    lab_preflight_log "dry-run: skipped live context lookup"
  fi

  local scan_fixture="${fixture}"
  if [[ -z "${scan_fixture}" && "${dry_run}" -eq 1 ]]; then
    # Dry-run without fixture: do not touch the API; treat as clean for exit status.
    lab_preflight_log "dry-run: skipped live isolation scan"
    lab_preflight_log "preflight OK (dry-run)"
    return 0
  fi

  local residue_out=""
  local residue_rc=0
  residue_out="$(lab_preflight_detect_residue "${scan_fixture}")" || residue_rc=$?

  if [[ "${residue_rc}" -ne 0 ]]; then
    lab_preflight_warn "lab residue detected:"
    printf '%s\n' "${residue_out}" >&2
    if [[ "${force}" -eq 1 ]]; then
      lab_preflight_warn "proceeding with --force despite residue"
      lab_preflight_log "preflight OK (forced)"
      return 0
    fi
    lab_preflight_err "refuse: clear kollect-lab-* namespaces / kollect.dev/lab-run resources, or pass --force"
    return 2
  fi

  lab_preflight_log "isolation clean"
  lab_preflight_log "preflight OK"
  return 0
}
