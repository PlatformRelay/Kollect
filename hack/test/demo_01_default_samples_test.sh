#!/usr/bin/env bash
# DEMO-01: default samples overlay must be credential-free, and task dev-up must
# take the hero/demo path (Forgejo) so ConnectionVerified is reachable without Secrets.
# file:// Git remotes are admission-forbidden (endpoint_guard) — not used here.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_KUSTOMIZATION="${ROOT}/config/samples/kustomization.yaml"
ADVANCED_KUSTOMIZATION="${ROOT}/config/samples/advanced/kustomization.yaml"
TASKFILE="${ROOT}/Taskfile.yml"

fail() {
  printf 'demo-01 default samples: %s\n' "$*" >&2
  exit 1
}

[[ -f "${DEFAULT_KUSTOMIZATION}" ]] || fail "missing ${DEFAULT_KUSTOMIZATION}"
[[ -f "${TASKFILE}" ]] || fail "missing Taskfile.yml"

SECRET_REQUIRING=(
  kollect_v1alpha1_kollectdatabasesink.yaml
  kollect_v1alpha1_kollectdatabasesink_bigquery.yaml
  kollect_v1alpha1_kollectdatabasesink_mongodb.yaml
  kollect_v1alpha1_kollecteventsink_kafka.yaml
  kollect_v1alpha1_kollectsnapshotsink_s3.yaml
  kollect_v1alpha1_kollectconnectiontest.yaml
)

for sample in "${SECRET_REQUIRING[@]}"; do
  if grep -Eq "^[[:space:]]*-[[:space:]]*${sample}([[:space:]]|$)" "${DEFAULT_KUSTOMIZATION}" ||
    grep -Eq "^-[[:space:]]*${sample}([[:space:]]|$)" "${DEFAULT_KUSTOMIZATION}"; then
    fail "default kustomization must not include secret-requiring sample ${sample}"
  fi
done

if grep -Eq 'kollect_v1alpha1_kollectinventory\.yaml' "${DEFAULT_KUSTOMIZATION}"; then
  fail "default kustomization must not include dual-sink kollectinventory.yaml (Postgres ref)"
fi

# Enumerate default resources with Python (portable; avoids awk character-class traps).
mapfile -t DEFAULT_RESOURCES < <(python3 - <<'PY' "${DEFAULT_KUSTOMIZATION}"
import sys
from pathlib import Path
text = Path(sys.argv[1]).read_text().splitlines()
grab = False
for line in text:
    if line.startswith("resources:"):
        grab = True
        continue
    if not grab:
        continue
    if line and not line[0].isspace() and not line.startswith("#") and not line.startswith("-"):
        break
    stripped = line.strip()
    if stripped.startswith("-"):
        print(stripped.lstrip("-").strip())
PY
)

[[ ${#DEFAULT_RESOURCES[@]} -gt 0 ]] || fail "default kustomization resources list is empty"

for resource in "${DEFAULT_RESOURCES[@]}"; do
  path="${ROOT}/config/samples/${resource}"
  [[ -f "${path}" ]] || fail "default resource missing: ${resource}"
  if grep -Eq 'endpoint:[[:space:]]*file://' "${path}"; then
    fail "default resource ${resource} uses forbidden file:// endpoint"
  fi
  if grep -Eq '^kind:[[:space:]]*Kollect(Snapshot|Database|Event)Sink' "${path}"; then
    fail "default overlay must not apply sinks (use task demo-up / advanced/); found ${resource}"
  fi
done

[[ -f "${ADVANCED_KUSTOMIZATION}" ]] || fail "missing advanced overlay"
for sample in "${SECRET_REQUIRING[@]}"; do
  grep -Eq "${sample}" "${ADVANCED_KUSTOMIZATION}" ||
    fail "advanced kustomization must list ${sample}"
done

dev_up_body="$(
  awk '
    /^  dev-up:/ { grab=1; next }
    grab && /^  [a-zA-Z0-9_-]+:/ { exit }
    grab { print }
  ' "${TASKFILE}"
)"
[[ -n "${dev_up_body}" ]] || fail "dev-up task body empty"
if ! printf '%s\n' "${dev_up_body}" | grep -Eq 'task:[[:space:]]*demo-up|task:[[:space:]]*demo-hero-up|hack/demo/hero/up\.sh'; then
  fail "dev-up must invoke demo-up / demo-hero-up (credential-free Forgejo path)"
fi
printf '%s\n' "${dev_up_body}" | grep -Eq 'config/samples/advanced' &&
  fail "dev-up must not apply config/samples/advanced by default"

printf 'demo-01 default samples: ok (dev-up → hero demo; advanced opt-in)\n'
