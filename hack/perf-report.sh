#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Performance snapshot for local dev or CI.
# Local: agent-context/PERF-SNAPSHOT.md (gitignored maintainer path).
# CI:    artifacts/perf-snapshot.md (uploaded as a workflow artifact; never agent-context/).
# Exit 0 on success; exit 1 when unit tests fail.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BENCH_DIR="${ROOT}/artifacts/bench"
if [[ "${CI:-}" == "true" ]]; then
  SNAPSHOT="${ROOT}/artifacts/perf-snapshot.md"
  SNAPSHOT_LABEL="CI artifact"
else
  SNAPSHOT="${ROOT}/agent-context/PERF-SNAPSHOT.md"
  SNAPSHOT_LABEL="local only — do not commit"
fi

mkdir -p "$BENCH_DIR" "$(dirname "$SNAPSHOT")"

make setup-envtest >&2
KUBEBUILDER_ASSETS="$(make -s echo-kubebuilder-assets)"
export KUBEBUILDER_ASSETS
if [[ -z "${KUBEBUILDER_ASSETS}" ]]; then
  echo "failed to resolve KUBEBUILDER_ASSETS from setup-envtest" >&2
  exit 1
fi

TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"

BENCH_FILE="${BENCH_DIR}/latest.txt"
(
  go test -short -bench=. -benchmem ./internal/collect/... 2>&1 | tee "$BENCH_FILE"
) || true

UNIT_RC=0
UNIT_SUMMARY=""
UNIT_START=$(date +%s)
UNIT_TEST_ARGS=(./... -count=1 -short -timeout=12m)
if command -v jq >/dev/null 2>&1; then
  UNIT_JSON="${BENCH_DIR}/unit.json"
  UNIT_STDERR="${BENCH_DIR}/unit.stderr"
  if go test "${UNIT_TEST_ARGS[@]}" -json >"$UNIT_JSON" 2>"$UNIT_STDERR"; then
    FAIL_PKGS="$(jq -r 'select(.Action=="fail") | .Package' "$UNIT_JSON" 2>/dev/null | sort -u)"
    FAIL_COUNT="$(printf '%s\n' "$FAIL_PKGS" | sed '/^$/d' | wc -l | tr -d ' ')"
    PASS_COUNT="$(jq -r 'select(.Action=="pass" and .Test==null) | .Package' "$UNIT_JSON" 2>/dev/null | sort -u | wc -l | tr -d ' ')"
    UNIT_SUMMARY="${PASS_COUNT} packages passed"
    if [[ "${FAIL_COUNT:-0}" != "0" ]]; then
      UNIT_RC=1
      UNIT_SUMMARY="${UNIT_SUMMARY}, ${FAIL_COUNT} failed"
      echo "perf-report: failing packages:" >&2
      printf '%s\n' "$FAIL_PKGS" >&2
    fi
  else
    UNIT_RC=$?
    UNIT_SUMMARY="go test ./... failed (exit ${UNIT_RC})"
    if [[ -s "$UNIT_STDERR" ]]; then
      echo "perf-report: go test stderr (tail):" >&2
      tail -20 "$UNIT_STDERR" >&2
    fi
    if [[ -s "$UNIT_JSON" ]]; then
      FAIL_PKGS="$(jq -r 'select(.Action=="fail") | .Package' "$UNIT_JSON" 2>/dev/null | sort -u)"
      if [[ -n "$FAIL_PKGS" ]]; then
        echo "perf-report: failing packages:" >&2
        printf '%s\n' "$FAIL_PKGS" >&2
      fi
    fi
  fi
else
  if UNIT_OUT="$( { time go test ./internal/... -count=1 -short 2>&1; } 2>&1 )"; then
    UNIT_RC=0
    UNIT_SUMMARY="$(echo "$UNIT_OUT" | tail -3 | tr '\n' '; ')"
  else
    UNIT_RC=$?
    UNIT_SUMMARY="$(echo "$UNIT_OUT" | tail -5 | tr '\n' '; ')"
  fi
fi
UNIT_ELAPSED=$(( $(date +%s) - UNIT_START ))

BENCH_EXTRACT="$(grep -E '^BenchmarkExtract' "$BENCH_FILE" 2>/dev/null | tail -1 || true)"

BENCH_HINT="within dev tier (<500µs/op on fixture)"
if [[ -n "$BENCH_EXTRACT" ]]; then
  if echo "$BENCH_EXTRACT" | grep -qE '[0-9]+(\.[0-9]+)?ms/op'; then
    BENCH_HINT="BenchmarkExtract >500µs/op — profile extractor; check attribute count and CEL complexity."
  elif echo "$BENCH_EXTRACT" | grep -qE '([5-9][0-9]{2}|[0-9]{4,})(\.[0-9]+)?µs/op'; then
    BENCH_HINT="BenchmarkExtract >500µs/op — profile extractor; check attribute count and CEL complexity."
  fi
fi

METRICS_LIST="$(grep -E '^\s+Name:\s+"kollect_' "${ROOT}/internal/metrics/metrics.go" 2>/dev/null \
  | sed 's/.*"\(kollect_[^"]*\)".*/\1/' | sort -u | tr '\n' ', ' | sed 's/, $//')"

SCALE_TIER="dev"
if [[ "${KOLECT_LOAD_TEST:-}" == "1" ]]; then
  SCALE_TIER="load"
fi
if [[ "${CI:-}" == "true" ]]; then
  SCALE_TIER="ci"
fi

TEST_STATUS="PASS"
if [[ "$UNIT_RC" -ne 0 ]]; then
  TEST_STATUS="FAIL"
fi

cat >"$SNAPSHOT" <<EOF
# kollect performance snapshot (${SNAPSHOT_LABEL})

Generated: ${TIMESTAMP}
Git SHA: ${GIT_SHA}
Scale tier: ${SCALE_TIER}

## Unit tests

Status: **${TEST_STATUS}** (${UNIT_ELAPSED}s)
Summary: ${UNIT_SUMMARY}

## Benchmarks (internal/collect)

\`\`\`
${BENCH_EXTRACT:-<no BenchmarkExtract line — run task bench>}
\`\`\`

Bench artifact: \`artifacts/bench/latest.txt\`

## Perf-related metrics (grep internal/metrics)

${METRICS_LIST}

## Operator flags to tune

- \`--max-concurrent-reconciles-target\`
- \`--max-concurrent-reconciles-inventory\`
- \`--collect-dispatch-workers\`
- \`--collect-dispatch-queue-size\`
- \`KollectInventory.spec.exportMinInterval\` (CRD default 30s; per-inventory debounce)
- \`--reconcile-rate-limit\`
- \`--enable-pprof\`
- \`--pprof-bind-address\`

## Bottleneck hints

| Signal | Heuristic |
| --- | --- |
| BenchmarkExtract | ${BENCH_HINT} |
| kollect_workqueue_depth sustained high | Raise \`--max-concurrent-reconciles-*\` or increase \`spec.exportMinInterval\` on hot inventories |
| kollect_collect_dispatch_sync_fallback_total rising | Raise \`--collect-dispatch-workers\` or \`--collect-dispatch-queue-size\` |
| kollect_informer_objects growing | Prefer namespace-scoped targets; split profiles by GVK |
| kollect_export_bytes_total spike | Lower churn or raise debounce; verify payload hash skip |

See [docs/PERFORMANCE.md](../docs/PERFORMANCE.md) and [docs/DEVELOPMENT.md](../docs/DEVELOPMENT.md).
EOF

echo "Wrote ${SNAPSHOT}"
echo "Bench excerpt: ${BENCH_EXTRACT:-none}"
echo "Tests: ${TEST_STATUS}"

if [[ "$UNIT_RC" -ne 0 ]]; then
  exit 1
fi

exit 0
