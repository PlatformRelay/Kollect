#!/usr/bin/env bash
# UI CI gate: build, typecheck, unit tests. Visual regression runs in nightly workflow.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
UI="${ROOT}/ui"

cd "${UI}"

if [[ ! -f package-lock.json ]]; then
  echo "ui-verify: missing ui/package-lock.json — run npm install in ui/" >&2
  exit 1
fi

npm ci
npm run typecheck
npm test
npm run lint
npm run build

cd "${ROOT}"
bash hack/verify-ui-mock.sh

if [[ "${UI_VISUAL_REGRESSION:-}" == "1" ]]; then
  echo "ui-verify: visual regression stub (wire Percy/Chromatic in nightly)"
  exit 0
fi

echo "ui-verify: ok"
