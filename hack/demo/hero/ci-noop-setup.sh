#!/usr/bin/env bash
# No-op setup for kind-e2e-setup when the scenario owns its own kind cluster.
# Hero smoke creates/tears down kollect-hero via hack/demo/hero/{up,down}.sh;
# the default hack/kind/e2e/setup.sh would create a second kollect-e2e cluster.
set -euo pipefail

echo "[hero-ci] noop setup — hero smoke owns the kind cluster (kollect-hero)"
command -v kind >/dev/null 2>&1 || {
  echo "kind is required but not on PATH (kind-e2e-setup should have installed it)" >&2
  exit 1
}
kind version
