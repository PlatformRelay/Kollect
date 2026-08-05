#!/usr/bin/env bash
# LAB-H05 — evidence layout helpers for artifacts/lab/<RUN_ID>/.
# Offline/meta only: create/validate DOC-02-capable stubs; redaction scan stub for H06.
# shellcheck shell=bash

# Resolve run directory: <out_root>/<run_id>
lab_evidence_run_dir() {
  local out_root="$1"
  local run_id="$2"
  printf '%s/%s\n' "${out_root%/}" "${run_id}"
}

# Relative paths that a DOC-02-capable bundle must expose (H06 fills content).
lab_evidence_required_files() {
  printf '%s\n' manifest.md scenario-matrix.md limitations.md RETENTION.notes
}

lab_evidence_required_dirs() {
  printf '%s\n' pprof timings
}

# Write stub manifest with DOC-02 field anchors (LAB-DOC-02 manifest table).
_lab_evidence_write_manifest_stub() {
  local path="$1"
  local run_id="$2"
  cat >"${path}" <<EOF
# Lab evidence manifest (stub)

| Field | Value |
| --- | --- |
| RUN_ID | ${run_id} |
| Schedule | (fill) |
| Started (UTC) | (fill) |
| Finished (UTC) | (fill) |
| Product pin | (fill) |
| Cluster topology | (fill) |
| Lab label | kollect.dev/lab-run=${run_id} |
| Helm release / namespace | (fill) |
| Access path | (redacted — describe how, never paste kubeconfig) |
| Notable values | (fill) |

> Stub created by hack/lab evidence collector (LAB-H05). H06 fills values and redacts.
EOF
}

_lab_evidence_write_scenario_matrix_stub() {
  local path="$1"
  cat >"${path}" <<'EOF'
# Scenario matrix (stub)

| ID | Verdict | Evidence / notes |
| --- | --- | --- |
| (scenario) | (PASS\|PASS_WITH_LIMITATION\|FAIL\|SKIPPED\|LIMIT_REACHED\|not triggered) | (redacted) |

> Allowed verdicts per LAB-DOC-02. Non-pass rows must state why. H06 fills rows.
EOF
}

_lab_evidence_write_limitations_stub() {
  local path="$1"
  cat >"${path}" <<'EOF'
# Limitations / not claimed (stub)

Every publishable summary must list explicit limitations (or "not claimed") items, for example:

- Scenarios SKIPPED / LIMIT_REACHED / not triggered (with reasons)
- Wave-4 / Tier-S synthetic load not run
- kubectl top / metrics-server unavailable
- Partial scrape counts versus live cluster objects

> H06 appends the run-specific limitations list before any public wording.
EOF
}

_lab_evidence_write_retention_notes() {
  local path="$1"
  cat >"${path}" <<'EOF'
# Bounded retention / size notes

Raw evidence under artifacts/lab/<RUN_ID>/ is local-only (gitignored) and must stay bounded:

- Prefer summaries, counts, digests, and pprof tops over full payload dumps.
- Drop or rotate oversized logs; do not accumulate unbounded wave dumps across runs.
- Retention: keep the latest N named runs locally; archive offline if needed — never commit.
- Size: if a single run tree grows beyond operator comfort (for example multi-hundred MiB),
  trim wave logs and keep checksummed indexes under this layout instead.

H06 may deepen automatic size checks; this file records the policy hook for H05+.
EOF
}

# Create DOC-02-capable layout under run_dir (idempotent).
# Usage: lab_evidence_ensure_layout <run_dir> <run_id>
lab_evidence_ensure_layout() {
  local run_dir="$1"
  local run_id="$2"

  if [[ -z "${run_dir}" || -z "${run_id}" ]]; then
    printf 'lab_evidence_ensure_layout: run_dir and run_id required\n' >&2
    return 1
  fi

  mkdir -p "${run_dir}"
  local d
  while IFS= read -r d; do
    mkdir -p "${run_dir}/${d}"
  done < <(lab_evidence_required_dirs)

  # Always refresh stubs so field anchors stay aligned with DOC-02; wave dumps live elsewhere.
  _lab_evidence_write_manifest_stub "${run_dir}/manifest.md" "${run_id}"
  _lab_evidence_write_scenario_matrix_stub "${run_dir}/scenario-matrix.md"
  _lab_evidence_write_limitations_stub "${run_dir}/limitations.md"
  _lab_evidence_write_retention_notes "${run_dir}/RETENTION.notes"

  # Marker for tools that only need to know a run root was initialized.
  printf '%s\n' "${run_id}" >"${run_dir}/RUN_ID.txt"
}

# Validate required DOC-02-capable paths exist under run_dir.
lab_evidence_validate_layout() {
  local run_dir="$1"
  local rel

  if [[ -z "${run_dir}" || ! -d "${run_dir}" ]]; then
    printf 'lab_evidence_validate_layout: missing run directory: %s\n' "${run_dir:-}" >&2
    return 1
  fi

  while IFS= read -r rel; do
    if [[ ! -f "${run_dir}/${rel}" ]]; then
      printf 'lab_evidence_validate_layout: missing file: %s\n' "${rel}" >&2
      return 1
    fi
  done < <(lab_evidence_required_files)

  while IFS= read -r rel; do
    if [[ ! -d "${run_dir}/${rel}" ]]; then
      printf 'lab_evidence_validate_layout: missing directory: %s\n' "${rel}" >&2
      return 1
    fi
  done < <(lab_evidence_required_dirs)

  return 0
}

# Redaction scan stub for H06 to deepen.
# Simple pattern check: reject kubeconfig-shaped "clusters:" blocks and common secret markers.
# Returns 0 when clean, 1 when a forbidden pattern is found (prints path:match).
lab_evidence_redaction_scan_stub() {
  local run_dir="$1"
  local hit=0
  local f

  if [[ -z "${run_dir}" || ! -d "${run_dir}" ]]; then
    printf 'lab_evidence_redaction_scan_stub: missing run directory: %s\n' "${run_dir:-}" >&2
    return 1
  fi

  # Patterns aligned with LAB-DOC-02 "Never publish" (stub — not a full scrub).
  # - clusters: → kubeconfig fragments
  # - BEGIN .*PRIVATE KEY → PEM material
  # - password= / token= → connection-string / token leaks (case-insensitive via grep -E)
  while IFS= read -r -d '' f; do
    if grep -Eq '^(clusters:|users:)|BEGIN [A-Z0-9 ]*PRIVATE KEY|password=|token=' "${f}" 2>/dev/null; then
      printf 'lab_evidence_redaction_scan_stub: forbidden pattern in %s\n' "${f}" >&2
      hit=1
    fi
  done < <(find "${run_dir}" -type f -print0 2>/dev/null)

  if [[ "${hit}" -ne 0 ]]; then
    return 1
  fi
  return 0
}

# Optional: append a one-line size note (bytes) for retention hooks.
lab_evidence_note_size() {
  local run_dir="$1"
  local notes="${run_dir}/RETENTION.notes"
  local bytes

  if [[ ! -d "${run_dir}" ]]; then
    return 1
  fi
  bytes="$(du -sk "${run_dir}" 2>/dev/null | awk '{print $1 * 1024}')"
  printf '\nObserved size (approx bytes) at %s: %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" "${bytes:-unknown}" >>"${notes}"
}
