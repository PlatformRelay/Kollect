#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Guided wide-scope Kollect showcase — problem → answer → live walkthrough → outcomes.
# Usage:
#   export GITHUB_TOKEN="$(gh auth token)"
#   bash hack/demo/kind-wide-scope/demo.sh
#   bash hack/demo/kind-wide-scope/demo.sh --churn
#   DEMO_AUTO_YES=1 bash hack/demo/kind-wide-scope/demo.sh   # non-interactive CI
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
# shellcheck source=lib/ui.sh
source "${SCRIPT_DIR}/lib/ui.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-dev}"
readonly KUBECTL="${KUBECTL:-kubectl}"
export DEMO_REPO_ROOT="${REPO_ROOT}"

RUN_CHURN=0
SKIP_PLATFORM=0

for arg in "$@"; do
  case "$arg" in
    --churn) RUN_CHURN=1 ;;
    --skip-platform) SKIP_PLATFORM=1 ;;
    -h|--help)
      cat <<EOF
Usage: $0 [--churn] [--skip-platform]

  --churn           Run churn.sh in background after bootstrap
  --skip-platform   Skip Trivy/cert-manager/external-secrets install (already present)
  DEMO_AUTO_YES=1   Accept all prompts (non-interactive)
EOF
      exit 0
      ;;
    *) echo "Unknown arg: $arg" >&2; exit 1 ;;
  esac
done

_require() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
}

demo_require_gum

demo_intro "From scattered cluster state to durable, queryable inventory"

demo_step 0 "The problem"
demo_info "Platform teams need **security posture**, **TLS expiry**, and **fleet topology** in one place — but stakeholders cannot live-list the apiserver forever."

demo_confirm "Ready to bootstrap the Kollect wide-scope demo on kind?" || exit 0

demo_step 1 "Prerequisites"
for c in kind kubectl helm task docker gh kustomize; do
  _require "$c"
done

if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)"
  export GITHUB_TOKEN
fi
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
  demo_info "!!! warning Git export needs GITHUB_TOKEN (repo scope). Continue without push credentials?"
  demo_confirm "Proceed without git-push-credentials?" || exit 1
fi

demo_link "hack/demo/kind-wide-scope/README.md" "Public walkthrough"
demo_link "hack/demo/kind-wide-scope/kustomization.yaml" "Kustomize entry"
demo_link "hack/demo/kind-wide-scope/samples/" "Annotated Kollect CR samples"

demo_step 2 "Kollect answer — operator on kind"
demo_info "Event-driven informers per GVK → debounced export → Git snapshot at konih/kollect-inventory-demo."

demo_spin "Starting kind cluster (KOLLECT_DEV_MINIMAL=1)..." \
  bash -c "cd '${REPO_ROOT}' && KOLLECT_DEV_MINIMAL=1 task kind-dev-up"

"${KUBECTL}" config use-context "kind-${CLUSTER_NAME}"

if [[ "${SKIP_PLATFORM}" -eq 0 ]]; then
  demo_step 3 "Upstream CRDs — security, TLS, secrets"
  demo_info "Headline use case: **Trivy VulnerabilityReport** CVE inventory alongside cert-manager **Certificate** and external-secrets **ExternalSecret** rows."
  bash "${SCRIPT_DIR}/install-platform.sh"
else
  demo_info "Skipping platform operator install (--skip-platform)."
fi

demo_step 4 "Git credentials"
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  demo_spin "Creating git-push-credentials..." \
    bash -c "${KUBECTL} create secret generic git-push-credentials -n default \
      --from-literal=token='${GITHUB_TOKEN}' \
      --dry-run=client -o yaml | ${KUBECTL} apply -f -"
else
  demo_info "Apply manually: hack/demo/kind-wide-scope/base/kollect/secret.example.yaml"
fi

demo_step 5 "Kollect configuration + demo fleet"
demo_info "Apply order baked into kustomization: Scope → Profiles → Targets → Sink → Inventory → workloads → platform CRs."

demo_spin "kubectl apply -k hack/demo/kind-wide-scope/..." \
  bash -c "cd '${REPO_ROOT}' && ${KUBECTL} apply -k '${SCRIPT_DIR}'"

demo_step 6 "Wait for export pipeline"
if ! demo_wait_for "Sink connection verified..." \
  "${KUBECTL}" wait --for=condition=ConnectionVerified \
    kollectsink/git-inventory-demo -n default --timeout=180s 2>/dev/null; then
  demo_info "ConnectionVerified pending — check secretRef and cluster egress."
  "${KUBECTL}" describe kollectsink git-inventory-demo -n default || true
fi

if ! demo_wait_for "Inventory Ready..." \
  "${KUBECTL}" wait --for=condition=Ready \
    kollectinventory/team-inventory -n default --timeout=240s 2>/dev/null; then
  "${KUBECTL}" describe kollectinventory team-inventory -n default || true
fi

demo_step 7 "Outcomes"
"${KUBECTL}" get kollectinventory team-inventory -n default \
  -o custom-columns=NAME:.metadata.name,ITEMS:.status.itemCount,EXPORT:.status.lastExportTime

vuln_count="$("${KUBECTL}" get vulnerabilityreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
cert_count="$("${KUBECTL}" get certificates -A -l app.kubernetes.io/part-of=demo-fleet --no-headers 2>/dev/null | wc -l | tr -d ' ')"
es_count="$("${KUBECTL}" get externalsecrets -A -l kollect.dev/inventory=enabled --no-headers 2>/dev/null | wc -l | tr -d ' ')"

demo_outcome "Live inventory includes core fleet + ${vuln_count} Trivy reports + ${cert_count} Certificates + ${es_count} ExternalSecrets (counts grow as operators reconcile)."

echo ""
demo_info "**Next steps**
- Logs:    bash hack/demo/kind-wide-scope/logs.sh
- Churn:   bash hack/demo/kind-wide-scope/churn.sh  (or re-run demo.sh --churn)
- Verify:  port-forward 8082 and curl /inventory; watch Git commits on kollect-inventory-demo"

if [[ "${RUN_CHURN}" -eq 1 ]]; then
  demo_confirm "Start 12-minute churn script in background?" && {
    nohup bash "${SCRIPT_DIR}/churn.sh" >"${SCRIPT_DIR}/churn.log" 2>&1 &
    echo $! >"${SCRIPT_DIR}/churn.pid"
    demo_outcome "churn PID=$(cat "${SCRIPT_DIR}/churn.pid") — tail -f hack/demo/kind-wide-scope/churn.log"
  }
fi

demo_outcome "Demo bootstrap complete. Open hack/demo/kind-wide-scope/README.md for the full sales-pitch walkthrough."
