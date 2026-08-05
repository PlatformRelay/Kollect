#!/usr/bin/env bash
# LAB-H06 — report + redaction over a DOC-02 evidence tree (artifacts/lab/<RUN_ID>/).
# Offline: no kubectl. Emits summary.md + checksums.txt; gates on lib/redact.sh.
#
# Usage:
#   hack/lab/report.sh --run-dir <path>
#   hack/lab/report.sh --run-id <id> [--out-root artifacts/lab]
#   hack/lab/report.sh --help
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/evidence.sh
source "${ROOT}/hack/lab/lib/evidence.sh"
# shellcheck source=lib/redact.sh
source "${ROOT}/hack/lab/lib/redact.sh"

usage() {
  cat <<'EOF'
Usage: report.sh --run-dir <dir>
       report.sh --run-id <RUN_ID> [--out-root <dir>]

From an existing LAB-H05 evidence tree, emit a DOC-02 publication draft:
  summary.md          human summary (manifest + matrix + limitations)
  checksums.txt       SHA-256 digests of retained artifacts (excludes itself)

Then run the redaction gate (lib/redact.sh). Dirty trees fail non-zero —
seeded fake secrets must not pass.

Options:
  --run-dir <dir>     Evidence run directory (artifacts/lab/<RUN_ID>/).
  --run-id <id>       Resolve via --out-root (default: <repo>/artifacts/lab).
  --out-root <dir>    With --run-id only (default: artifacts/lab under repo).
  -h, --help          Show this help.

Never commits artefacts; scripts only. Aligns with
docs/operator-manual/lab-evidence-bundle.md.
EOF
}

RUN_DIR=""
RUN_ID=""
OUT_ROOT="${ROOT}/artifacts/lab"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run-dir)
      [[ $# -ge 2 ]] || { printf 'report: --run-dir requires a value\n' >&2; exit 2; }
      RUN_DIR="$2"
      shift 2
      ;;
    --run-id)
      [[ $# -ge 2 ]] || { printf 'report: --run-id requires a value\n' >&2; exit 2; }
      RUN_ID="$2"
      shift 2
      ;;
    --out-root)
      [[ $# -ge 2 ]] || { printf 'report: --out-root requires a value\n' >&2; exit 2; }
      OUT_ROOT="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'report: unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -n "${RUN_DIR}" && -n "${RUN_ID}" ]]; then
  printf 'report: use either --run-dir or --run-id, not both\n' >&2
  exit 2
fi

if [[ -z "${RUN_DIR}" && -z "${RUN_ID}" ]]; then
  printf 'report: --run-dir or --run-id is required\n' >&2
  usage >&2
  exit 2
fi

if [[ -n "${RUN_ID}" ]]; then
  if [[ "${RUN_ID}" == *"/"* || "${RUN_ID}" == *".."* ]]; then
    printf 'report: invalid --run-id (no path components): %s\n' "${RUN_ID}" >&2
    exit 2
  fi
  RUN_DIR="$(lab_evidence_run_dir "${OUT_ROOT}" "${RUN_ID}")"
fi

if [[ ! -d "${RUN_DIR}" ]]; then
  printf 'report: run directory does not exist: %s\n' "${RUN_DIR}" >&2
  exit 1
fi

lab_evidence_validate_layout "${RUN_DIR}" || {
  printf 'report: evidence layout invalid under %s (need H05 stubs)\n' "${RUN_DIR}" >&2
  exit 1
}

# --- Emit human summary suitable for DOC-02 publication path ---
_lab_report_write_summary() {
  local run_dir="$1"
  local run_id
  local summary="${run_dir}/summary.md"

  if [[ -f "${run_dir}/RUN_ID.txt" ]]; then
    run_id="$(tr -d '[:space:]' <"${run_dir}/RUN_ID.txt")"
  else
    run_id="$(basename "${run_dir}")"
  fi

  {
    cat <<EOF
# Lab evidence summary — \`${run_id}\`

Publication draft for [lab evidence bundle](../../docs/operator-manual/lab-evidence-bundle.md).
Raw tree stays local-only; this summary is the human DOC-02 path after redaction.

**Program verdict:** READY WITH CONDITIONS (see Limitations below).

## Manifest

EOF
    # Drop the H1 from manifest.md to avoid double titles; keep the table/body.
    if [[ -f "${run_dir}/manifest.md" ]]; then
      sed '1{/^# /d;}' "${run_dir}/manifest.md"
    fi

    cat <<'EOF'

## Scenario matrix

EOF
    if [[ -f "${run_dir}/scenario-matrix.md" ]]; then
      sed '1{/^# /d;}' "${run_dir}/scenario-matrix.md"
    fi

    cat <<'EOF'

## Limitations / not claimed

EOF
    if [[ -f "${run_dir}/limitations.md" ]]; then
      sed '1{/^# /d;}' "${run_dir}/limitations.md"
    fi

    cat <<EOF

## Integrity

Bundle checksums: \`checksums.txt\` (SHA-256 of retained artifacts under this run directory).
EOF
  } >"${summary}"
}

# --- SHA-256 checksums for retained artifacts (exclude checksums.txt itself) ---
_lab_report_write_checksums() {
  local run_dir="$1"
  local out="${run_dir}/checksums.txt"
  local tmp
  local f
  local rel
  local digest

  tmp="$(mktemp "${TMPDIR:-/tmp}/lab-checksums.XXXXXX")"

  (
    cd "${run_dir}"
    # Stable order; skip the checksums file we are writing and any prior copy.
    find . -type f ! -name 'checksums.txt' -print | LC_ALL=C sort | while IFS= read -r f; do
      rel="${f#./}"
      if command -v sha256sum >/dev/null 2>&1; then
        # sha256sum prints "digest  path" — rewrite to relative path only.
        digest="$(sha256sum -- "${rel}" | awk '{print $1}')"
      else
        digest="$(shasum -a 256 -- "${rel}" | awk '{print $1}')"
      fi
      printf '%s  %s\n' "${digest}" "${rel}"
    done
  ) >"${tmp}"

  mv "${tmp}" "${out}"
}

_lab_report_write_summary "${RUN_DIR}"
_lab_report_write_checksums "${RUN_DIR}"

# Redaction gate last: seeded secrets must fail; clean synthetic sample passes.
if ! lab_redact_gate "${RUN_DIR}"; then
  printf 'report: redaction gate failed — refuse publication for %s\n' "${RUN_DIR}" >&2
  exit 1
fi

printf 'report: publication draft ready at %s (summary.md + checksums.txt)\n' "${RUN_DIR}"
