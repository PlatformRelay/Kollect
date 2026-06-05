#!/usr/bin/env bash
# Emit JSON or markdown performance snapshot for coordinator agents (ADR-0027).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

format="json"
output=""

for arg in "$@"; do
  case "$arg" in
    --format=*)
      format="${arg#*=}"
      ;;
    --output=*)
      output="${arg#*=}"
      ;;
    --format)
      echo "use --format=json or --format=markdown" >&2
      exit 2
      ;;
  esac
done

args=(--format="$format" --root="$ROOT")

if [[ "$format" == "markdown" || "$format" == "md" ]]; then
  if [[ -z "$output" ]]; then
    mkdir -p agent-context
    output="agent-context/PERF-SNAPSHOT.md"
  fi
fi

if [[ -n "$output" ]]; then
  args+=(--output="$output")
fi

go run ./cmd/perf-report "${args[@]}"
