#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Install upstream operators that generate CRDs for the wide-scope showcase:
#   - Trivy Operator  → VulnerabilityReport
#   - cert-manager     → Certificate
#   - external-secrets → ExternalSecret
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/ui.sh
source "${SCRIPT_DIR}/lib/ui.sh"

readonly HELM="${HELM:-helm}"
readonly KUBECTL="${KUBECTL:-kubectl}"

demo_require_gum

_install_helm_repo() {
  local name="$1"
  local url="$2"
  "${HELM}" repo add "${name}" "${url}" 2>/dev/null || true
}

demo_step 1 "Platform operators (Trivy, cert-manager, external-secrets)"
demo_info "**Why:** Kollect collects *any* GVK — security reports, TLS certs, and secret sync state alongside core workloads."

_install_helm_repo jetstack https://charts.jetstack.io
_install_helm_repo aqua https://aquasecurity.github.io/helm-charts/
_install_helm_repo external-secrets https://charts.external-secrets.io
"${HELM}" repo update >/dev/null 2>&1 || "${HELM}" repo update

if ! demo_spin "Installing cert-manager (Certificate CRDs)..." \
  "${HELM}" upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace \
    --set crds.enabled=true \
    --wait --timeout 180s; then
  echo "cert-manager install failed — check helm output" >&2
  exit 1
fi

if ! demo_spin "Installing Trivy Operator (VulnerabilityReport CRDs)..." \
  "${HELM}" upgrade --install trivy-operator aqua/trivy-operator \
    --namespace trivy-system --create-namespace \
    --set trivy.ignoreUnfixed=true \
    --wait --timeout 180s; then
  echo "trivy-operator install failed — check helm output" >&2
  exit 1
fi

if ! demo_spin "Installing external-secrets (ExternalSecret CRDs)..." \
  "${HELM}" upgrade --install external-secrets external-secrets/external-secrets \
    --namespace external-secrets --create-namespace \
    --wait --timeout 180s; then
  echo "external-secrets install failed — check helm output" >&2
  exit 1
fi

demo_step 2 "Demo issuers and secret stores"
"${KUBECTL}" apply -k "${SCRIPT_DIR}/base/platform/"

demo_outcome "Platform CRDs ready — workloads will produce VulnerabilityReports; Certificates and ExternalSecrets are applied with the fleet."
