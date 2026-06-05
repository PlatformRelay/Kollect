#!/usr/bin/env bash
# Run unit/envtest with an internal/ coverage profile and enforce a minimum floor.
# Excludes e2e and integration-tagged tests (default go test build tags).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

MIN="${COVERAGE_MIN:-45}"
RACE_ARGS=()
if [[ "${COVERAGE_RACE:-0}" == "1" ]]; then
  RACE_ARGS=(-race)
fi

rm -f coverage.out coverage-summary.txt

make manifests generate fmt vet
make setup-envtest >&2

export KUBEBUILDER_ASSETS="$(make -s echo-kubebuilder-assets)"
if [[ -z "${KUBEBUILDER_ASSETS}" ]]; then
  echo "failed to resolve KUBEBUILDER_ASSETS from setup-envtest" >&2
  exit 1
fi
export CGO_ENABLED="${CGO_ENABLED:-1}"

# Packages outside internal/ run without -coverprofile so they do not append to
# coverage.out; mixed multi-package cover merge can corrupt the profile.
other_pkgs="$(go list ./... | grep -v /e2e | grep -v '/internal/' | grep -v '/cmd$' || true)"
if [[ -n "${other_pkgs}" ]]; then
  # shellcheck disable=SC2086
  go test "${RACE_ARGS[@]}" -count=1 -p 1 ${other_pkgs}
fi

coverpkg="$(go list ./internal/... | paste -sd, -)"
internal_pkgs="$(go list ./internal/...)"
# shellcheck disable=SC2086
go test "${RACE_ARGS[@]}" -count=1 -p 1 -coverpkg="${coverpkg}" -coverprofile=coverage.out ${internal_pkgs}

if [[ ! -s coverage.out ]] || [[ "$(wc -l < coverage.out)" -lt 2 ]]; then
  echo "coverage.out missing or empty after go test ./internal/..." >&2
  exit 1
fi

go tool cover -func=coverage.out | tee coverage-summary.txt | tail -1
pct="$(awk '/^total:/ {gsub(/%/,"",$3); print $3}' coverage-summary.txt)"
awk -v p="${pct}" -v m="${MIN}" 'BEGIN {
  if (p+0 < m+0) {
    printf "internal/ coverage %.1f%% is below %d%% floor\n", p, m
    exit 1
  }
}'
echo "internal/ coverage ${pct}% (floor ${MIN}%)"
