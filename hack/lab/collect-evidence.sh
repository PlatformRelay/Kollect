#!/usr/bin/env bash
# LAB-H05 — thin CLI to create/validate a DOC-02-capable lab evidence layout.
# Offline only: no kubectl. Default out-root: artifacts/lab
#
# Usage:
#   hack/lab/collect-evidence.sh --run-id <id> [--out-root artifacts/lab] [--dry-run]
#   hack/lab/collect-evidence.sh --help
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=lib/evidence.sh
source "${ROOT}/hack/lab/lib/evidence.sh"

usage() {
  cat <<'EOF'
Usage: collect-evidence.sh --run-id <RUN_ID> [--out-root <dir>] [--dry-run]

Create (and validate) a LAB-DOC-02-capable evidence layout under
  <out-root>/<RUN_ID>/
with manifest, scenario-matrix, limitations stubs plus pprof/ and timings/ dirs.

Options:
  --run-id <id>       Required. Stable lab run id (for example dr-YYYYMMDD-<hex>).
  --out-root <dir>    Output root (default: artifacts/lab under the repo root).
  --dry-run           Fixture mode: same layout write; message notes offline/no kubectl.
  -h, --help          Show this help.

Never commits artefacts; scripts only. H06 deepens report/redaction on this layout.
EOF
}

RUN_ID=""
OUT_ROOT="${ROOT}/artifacts/lab"
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run-id)
      [[ $# -ge 2 ]] || { printf 'collect-evidence: --run-id requires a value\n' >&2; exit 2; }
      RUN_ID="$2"
      shift 2
      ;;
    --out-root)
      [[ $# -ge 2 ]] || { printf 'collect-evidence: --out-root requires a value\n' >&2; exit 2; }
      OUT_ROOT="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'collect-evidence: unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "${RUN_ID}" ]]; then
  printf 'collect-evidence: --run-id is required\n' >&2
  usage >&2
  exit 2
fi

# Reject path separators in RUN_ID so layout stays under out-root.
if [[ "${RUN_ID}" == *"/"* || "${RUN_ID}" == *".."* ]]; then
  printf 'collect-evidence: invalid --run-id (no path components): %s\n' "${RUN_ID}" >&2
  exit 2
fi

RUN_DIR="$(lab_evidence_run_dir "${OUT_ROOT}" "${RUN_ID}")"

if [[ "${DRY_RUN}" -eq 1 ]]; then
  printf 'collect-evidence: dry-run / fixture mode (no kubectl); writing layout under %s\n' "${RUN_DIR}"
fi

lab_evidence_ensure_layout "${RUN_DIR}" "${RUN_ID}"
lab_evidence_validate_layout "${RUN_DIR}"
lab_evidence_note_size "${RUN_DIR}" || true

# Stub scan on the fresh layout (should be clean); H06 deepens before publish.
if ! lab_evidence_redaction_scan_stub "${RUN_DIR}"; then
  printf 'collect-evidence: redaction scan stub reported issues under %s\n' "${RUN_DIR}" >&2
  exit 1
fi

printf 'collect-evidence: layout ready at %s\n' "${RUN_DIR}"
