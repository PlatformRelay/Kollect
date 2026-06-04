#!/usr/bin/env bash
# Regenerate committed artifacts and fail if anything drifts.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

copy_generated() {
  for path in config/crd/bases config/rbac/role.yaml; do
    if [[ -e "$path" ]]; then
      mkdir -p "$scratch/$(dirname "$path")"
      cp -a "$path" "$scratch/$path"
    fi
  done
  shopt -s nullglob
  for f in api/*/zz_generated.deepcopy.go; do
    mkdir -p "$scratch/$(dirname "$f")"
    cp -a "$f" "$scratch/$f"
  done
}

copy_generated

make generate manifests

echo "verify: comparing generated artifacts..."

if [[ -d "$scratch/config/crd/bases" ]] || [[ -d config/crd/bases ]]; then
  if ! diff -ru "$scratch/config/crd/bases" config/crd/bases; then
    echo "verify: drift in config/crd/bases — run 'make generate manifests'" >&2
    exit 1
  fi
fi

if [[ -f "$scratch/config/rbac/role.yaml" ]] || [[ -f config/rbac/role.yaml ]]; then
  if ! diff -u "$scratch/config/rbac/role.yaml" config/rbac/role.yaml; then
    echo "verify: drift in config/rbac/role.yaml — run 'make manifests'" >&2
    exit 1
  fi
fi

shopt -s nullglob
for f in api/*/zz_generated.deepcopy.go; do
  if ! diff -u "$scratch/$f" "$f"; then
    echo "verify: drift in $f — run 'make generate'" >&2
    exit 1
  fi
done

echo "verify: ok"
