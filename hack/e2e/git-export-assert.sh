#!/usr/bin/env bash
# Bounded git export SHA assert for nightly e2e.
# Without GITHUB_TOKEN: verifies inventory HTTP payload hash only (in-cluster export may degrade on public git sink).
# With GITHUB_TOKEN + GIT_EXPORT_TEST_REPO: clones remote and compares exported file SHA256 to inventory HTTP body.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../kind/common.sh
source "${SCRIPT_DIR}/../kind/common.sh"

_kind_require kubectl
kind_use_context "${CLUSTER_NAME:-kollect-e2e}"

_log() { echo "[git-export] $*"; }

# Matrix-isolated nightly jobs run setup only; bootstrap samples before export assert.
if ! kubectl get kollectinventory team-inventory -n default >/dev/null 2>&1; then
  _log "Bootstrapping e2e sample CRs for git-export job..."
  bash "${REPO_ROOT}/hack/kind/e2e/bootstrap-samples.sh"
fi

_log "Waiting for inventory collection via HTTP..."
kubectl port-forward -n "$KOLLECT_NAMESPACE" svc/kollect-controller-manager 18083:8082 &
PF_PID=$!
trap 'kill "$PF_PID" 2>/dev/null || true' EXIT
sleep 3

INVENTORY_JSON=""
for i in $(seq 1 60); do
  INVENTORY_JSON="$(curl -sf http://127.0.0.1:18083/inventory 2>/dev/null || true)"
  if echo "$INVENTORY_JSON" | grep -qE '"itemCount":[1-9][0-9]*'; then
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    echo "inventory HTTP did not report collected items within timeout" >&2
    echo "$INVENTORY_JSON" | head -c 500 >&2 || true
    kubectl logs -n "$KOLLECT_NAMESPACE" -l app.kubernetes.io/name=kollect --tail=80 || true
    exit 1
  fi
  sleep 5
done

EXPECTED_SHA="$(printf '%s' "$INVENTORY_JSON" | sha256sum | awk '{print $1}')"
_log "inventory payload sha256=${EXPECTED_SHA}"

if [[ -z "${GITHUB_TOKEN:-}" || -z "${GIT_EXPORT_TEST_REPO:-}" ]]; then
  _log "GIT_EXPORT_TEST_REPO unset; skipping remote git clone SHA assert (inventory HTTP hash captured)."
  exit 0
fi

REPO_URL="${GIT_EXPORT_TEST_REPO}"
OBJECT_PATH="inventory/default/team-inventory.json"
CLONE_DIR="$(mktemp -d)"
trap 'kill "$PF_PID" 2>/dev/null || true; rm -rf "$CLONE_DIR"' EXIT

_log "Cloning ${REPO_URL} to verify export SHA..."
git -c http.extraHeader="Authorization: Bearer ${GITHUB_TOKEN}" \
  clone --depth 1 "$REPO_URL" "$CLONE_DIR"

REMOTE_SHA="$(sha256sum "${CLONE_DIR}/${OBJECT_PATH}" | awk '{print $1}')"
if [[ "$REMOTE_SHA" != "$EXPECTED_SHA" ]]; then
  echo "export SHA mismatch: remote=${REMOTE_SHA} inventory_http=${EXPECTED_SHA}" >&2
  exit 1
fi

_log "Remote git export SHA matches inventory HTTP payload."
