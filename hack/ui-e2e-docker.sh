#!/usr/bin/env bash
# Run UI Playwright smoke tests in the official Playwright Docker image.
# Use on hosts where native browser install fails (e.g. Ubuntu 26.04 until Playwright 1.61).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UI="${ROOT}/ui"
IMAGE="mcr.microsoft.com/playwright:v1.60.0-noble"

docker run --rm \
  -v "${UI}:/work/ui" \
  -w /work/ui \
  --network host \
  -e CI=1 \
  -e VITE_MOCK_API=true \
  "${IMAGE}" \
  bash -lc 'npm ci && npx playwright test'
