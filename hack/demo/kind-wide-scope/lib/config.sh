#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Demo environment defaults (sourced by demo.sh).

: "${DEMO_PERSONA:=full}"
: "${CHURN_PRESET:=fast}"
: "${DEMO_SKIP_CONTRAST:=}"
: "${DEMO_OPEN_BROWSER:=}"
: "${DEMO_AUTO_YES:=}"
: "${CLUSTER_NAME:=kollect-dev}"
: "${KUBECTL:=kubectl}"
: "${DEMO_HELM_VALUES:=${DEMO_REPO_ROOT:-}/charts/kollect/ci/demo-values.yaml}"

demo_overlay_path() {
  local persona="${1:-${DEMO_PERSONA}}"
  case "$persona" in
    full) echo "overlays/full" ;;
    security) echo "overlays/security" ;;
    platform) echo "overlays/platform" ;;
    local) echo "overlays/local" ;;
    *)
      echo "unknown persona: ${persona}" >&2
      return 1
      ;;
  esac
}

demo_persona_skip_platform() {
  [[ "${DEMO_PERSONA}" == "platform" ]]
}

demo_persona_git_enabled() {
  [[ "${DEMO_PERSONA}" != "local" ]]
}

demo_persona_label() {
  case "${DEMO_PERSONA}" in
    full) echo "Full showcase (8 GVK types + Git export)" ;;
    security) echo "Security headline (Trivy + certs + ESO, lighter fleet)" ;;
    platform) echo "Platform fleet (core workloads + meta-target, no security operators)" ;;
    local) echo "Local-only (HTTP + UI reveal, no GitHub token)" ;;
    *) echo "${DEMO_PERSONA}" ;;
  esac
}
