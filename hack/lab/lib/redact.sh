#!/usr/bin/env bash
# LAB-H06 — redaction gate for lab evidence trees (LAB-DOC-02 / ADR-0707).
# Offline/meta: scan artifacts/lab/<RUN_ID>/ (or any run dir) before publication.
# Source this file; do not execute it directly.
# shellcheck shell=bash

# Patterns aligned with docs/operator-manual/lab-evidence-bundle.md "Never publish".
# Returns 0 when clean, 1 when a forbidden pattern is found (prints path on stderr).
lab_redact_forbidden_regex() {
  # Line-anchored kubeconfig fragments, PEM material, credential assignments,
  # and home-directory fingerprints. Intentionally narrow to avoid flagging
  # DOC-02 prose that says "never paste kubeconfig".
  printf '%s' \
    '^(clusters:|users:)|BEGIN [A-Z0-9 ]*PRIVATE KEY|(^|[^[:alnum:]_])(password|token)=' \
    '|/Users/|/home/|client-key-data:|client-certificate-data:'
}

# Scan every regular file under run_dir. Prints hits to stderr.
# Usage: lab_redact_scan <run_dir>
lab_redact_scan() {
  local run_dir="$1"
  local hit=0
  local f
  local regex

  if [[ -z "${run_dir}" || ! -d "${run_dir}" ]]; then
    printf 'lab_redact_scan: missing run directory: %s\n' "${run_dir:-}" >&2
    return 1
  fi

  regex="$(lab_redact_forbidden_regex)"

  while IFS= read -r -d '' f; do
    # Skip empty files; still scan placeholders that may hold leaks.
    if grep -Eq -- "${regex}" "${f}" 2>/dev/null; then
      printf 'lab_redact_scan: forbidden pattern in %s\n' "${f}" >&2
      hit=1
    fi
  done < <(find "${run_dir}" -type f -print0 2>/dev/null)

  if [[ "${hit}" -ne 0 ]]; then
    return 1
  fi
  return 0
}

# Publication gate: scan then emit a clear block message for report.sh callers.
# Usage: lab_redact_gate <run_dir>
# Exit 0 clean; 1 blocked / missing dir.
lab_redact_gate() {
  local run_dir="$1"

  if ! lab_redact_scan "${run_dir}"; then
    printf 'lab_redact_gate: publication blocked — redaction scan found forbidden secrets/patterns under %s\n' \
      "${run_dir}" >&2
    return 1
  fi
  return 0
}
