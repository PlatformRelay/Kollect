#!/usr/bin/env bash
# Install a pinned Helm 3 release (SHA256-verified via upstream get-helm-3 script).
# Usage: HELM_VERSION=v3.17.3 hack/install-helm.sh
set -euo pipefail

VERSION="${HELM_VERSION:-v3.17.3}"
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \
  | DESIRED_VERSION="${VERSION}" bash
helm version
