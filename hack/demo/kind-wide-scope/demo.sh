#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Guided wide-scope Kollect showcase — venue pitch + early-adopter lab.
# Usage:
#   bash hack/demo/kind-wide-scope/demo.sh [--check] [--churn[=fast|present|burst]] [--reveal]
#   DEMO_PERSONA=security|platform|local bash hack/demo/kind-wide-scope/demo.sh
#   DEMO_AUTO_YES=1 bash hack/demo/kind-wide-scope/demo.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
export DEMO_REPO_ROOT="${REPO_ROOT}"

# shellcheck source=lib/ui.sh
source "${SCRIPT_DIR}/lib/ui.sh"
# shellcheck source=lib/config.sh
source "${SCRIPT_DIR}/lib/config.sh"

RUN_CHURN=""
CHURN_MODE=""
RUN_REVEAL=0
REUSE_CLUSTER=0
FRESH_CLUSTER=0
SKIP_PLATFORM=0
RUN_CHECK=0

_parse_churn_arg() {
  local arg="$1"
  case "$arg" in
    --churn) CHURN_MODE="fast"; RUN_CHURN=1 ;;
    --churn=*) CHURN_MODE="${arg#--churn=}"; RUN_CHURN=1 ;;
    *) return 1 ;;
  esac
  case "${CHURN_MODE}" in
    fast|present|burst) export CHURN_PRESET="${CHURN_MODE}" ;;
    *)
      echo "unknown churn preset: ${CHURN_MODE} (use fast, present, burst)" >&2
      exit 1
      ;;
  esac
}

for arg in "$@"; do
  case "$arg" in
    --check) RUN_CHECK=1 ;;
    --reveal) RUN_REVEAL=1 ;;
    --reuse-cluster) REUSE_CLUSTER=1 ;;
    --fresh) FRESH_CLUSTER=1 ;;
    --skip-platform) SKIP_PLATFORM=1 ;;
    --churn|--churn=*) _parse_churn_arg "$arg" ;;
    -h|--help)
      cat <<EOF
Usage: $0 [options]

Venue pitch and early-adopter lab for the wide-scope kind demo.

Options:
  --check              Verify prerequisites (exit 0/1); no cluster changes
  --churn[=PRESET]     Background churn after bootstrap (default preset: fast)
                       PRESET: fast | present | burst
  --reveal             Port-forward UI + Read API and print reveal URLs
  --reuse-cluster      Skip kind bootstrap when kind-${CLUSTER_NAME} is healthy
  --fresh              Delete and recreate kind cluster before bootstrap
  --skip-platform      Skip Trivy/cert-manager/external-secrets install

Environment:
  DEMO_PERSONA         full | security | platform | local (default: full)
  DEMO_AUTO_YES=1      Non-interactive — skip Gum confirms
  DEMO_SKIP_CONTRAST=1 Skip step-0 apiserver pain contrast
  DEMO_OPEN_BROWSER=1  Open kollect-ui URL on --reveal (xdg-open)
  GITHUB_TOKEN         Git push credentials (not required for local persona)
  CHURN_PRESET         Override churn preset when using --churn

Personas:
  full       Eight GVK types + Git export (default)
  security   Lighter fleet + Trivy/certs/ESO headline
  platform   Core fleet + meta-target; skips security operators
  local      HTTP + UI only — no GitHub token

See hack/demo/kind-wide-scope/ROADMAP.md for status and troubleshooting.
EOF
      exit 0
      ;;
    *) echo "Unknown arg: $arg (try --help)" >&2; exit 1 ;;
  esac
done

if [[ "${RUN_CHECK}" -eq 1 ]]; then
  # shellcheck source=lib/check.sh
  source "${SCRIPT_DIR}/lib/check.sh"
  exit $?
fi

demo_require_gum

_choose_persona() {
  if [[ -n "${DEMO_PERSONA:-}" && "${DEMO_PERSONA}" != "full" ]]; then
    return 0
  fi
  if [[ "${DEMO_AUTO_YES:-}" == "1" ]]; then
    DEMO_PERSONA="${DEMO_PERSONA:-full}"
    export DEMO_PERSONA
    return 0
  fi
  if command -v gum >/dev/null 2>&1; then
    DEMO_PERSONA="$(gum choose --header "Demo persona" \
      "full — 8 GVK showcase + Git export" \
      "security — CVE/TLS/secrets headline, lighter fleet" \
      "platform — core fleet + meta-target, no security operators" \
      "local — UI + HTTP only, no GitHub token" \
      | awk '{print $1}')"
    export DEMO_PERSONA
  else
    DEMO_PERSONA="${DEMO_PERSONA:-full}"
    export DEMO_PERSONA
  fi
}

demo_intro "From scattered cluster state to durable, queryable inventory"
demo_info "$(demo_persona_label)"

if [[ "${DEMO_AUTO_YES:-}" != "1" ]]; then
  _choose_persona
else
  DEMO_PERSONA="${DEMO_PERSONA:-full}"
  export DEMO_PERSONA
fi

if demo_persona_skip_platform; then
  SKIP_PLATFORM=1
fi

if [[ "${DEMO_SKIP_CONTRAST:-}" != "1" ]]; then
  demo_step 0 "The problem"
  demo_info "Four teams, four tools, no durable history — stakeholders cannot live-list the apiserver forever."
  if demo_confirm "Show apiserver scatter contrast (kubectl get -A)?"; then
    demo_spin "Scattered cluster state (sample)..." \
      bash -c "${KUBECTL} get deploy,svc,certificate,externalsecret -A 2>/dev/null | head -20 || true"
    demo_info "Kollect answer: **one inventory**, many projections — Scope → Profile → Target → Inventory → Sink."
  fi
fi

demo_confirm "Ready to bootstrap the Kollect wide-scope demo on kind?" || exit 0

demo_step 1 "Prerequisites"
# shellcheck source=lib/check.sh
source "${SCRIPT_DIR}/lib/check.sh"

if demo_persona_git_enabled; then
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)"
    export GITHUB_TOKEN
  fi
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    demo_info "!!! warning Git export needs GITHUB_TOKEN (repo scope)."
    demo_confirm "Proceed without git-push-credentials?" || exit 1
  fi
else
  demo_info "**Local persona** — skipping Git credentials; reveal uses Read API + kollect-ui only."
fi

overlay_rel="$(demo_overlay_path)"
overlay_path="${SCRIPT_DIR}/${overlay_rel}"
demo_link "hack/demo/kind-wide-scope/README.md" "Public walkthrough"
demo_link "hack/demo/kind-wide-scope/ROADMAP.md" "Early-adopter checklist"
demo_link "hack/demo/kind-wide-scope/${overlay_rel}/" "Active kustomize overlay (${DEMO_PERSONA})"
demo_link "hack/demo/kind-wide-scope/samples/" "Annotated Kollect CR samples"

demo_step 2 "Kollect answer — operator + UI on kind"
demo_info "Event-driven informers per GVK → debounced export → Git snapshot + Read API + kollect-ui."

_kind_bootstrap() {
  if [[ "${FRESH_CLUSTER}" -eq 1 ]]; then
    demo_spin "Deleting existing cluster ${CLUSTER_NAME}..." \
      bash -c "cd '${REPO_ROOT}' && task kind-dev-down" || true
  fi
  if [[ "${REUSE_CLUSTER}" -eq 1 ]]; then
    if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}" \
      && "${KUBECTL}" config use-context "kind-${CLUSTER_NAME}" >/dev/null 2>&1 \
      && "${KUBECTL}" wait --for=condition=Ready node --all --timeout=60s >/dev/null 2>&1; then
      demo_info "Reusing healthy cluster **kind-${CLUSTER_NAME}** (--reuse-cluster)."
      return 0
    fi
    demo_info "Cluster missing or unhealthy — running full bootstrap."
  fi
  demo_spin "Starting kind cluster (KOLLECT_DEV_MINIMAL=1, demo-values UI)..." \
    bash -c "cd '${REPO_ROOT}' && DEV_VALUES='${DEMO_HELM_VALUES}' KOLLECT_DEV_MINIMAL=1 task kind-dev-up"
}

_kind_bootstrap
"${KUBECTL}" config use-context "kind-${CLUSTER_NAME}"

if [[ "${SKIP_PLATFORM}" -eq 0 ]]; then
  demo_step 3 "Upstream CRDs — security, TLS, secrets"
  demo_info "Headline: **Trivy VulnerabilityReport**, cert-manager **Certificate**, external-secrets **ExternalSecret**."
  bash "${SCRIPT_DIR}/install-platform.sh"
else
  demo_info "Skipping platform operator install (--skip-platform / platform persona)."
fi

if demo_persona_git_enabled && [[ -n "${GITHUB_TOKEN:-}" ]]; then
  demo_step 4 "Git credentials"
  demo_spin "Creating git-push-credentials..." \
    bash -c "${KUBECTL} create secret generic git-push-credentials -n default \
      --from-literal=token='${GITHUB_TOKEN}' \
      --dry-run=client -o yaml | ${KUBECTL} apply -f -"
else
  demo_step 4 "Git credentials"
  demo_info "Skipped — local persona or no token."
fi

demo_step 5 "Kollect configuration + demo fleet"
demo_info "Apply overlay **${DEMO_PERSONA}**: Scope → Profiles → Targets → Sink → Inventory → workloads."

demo_spin "kubectl apply -k ${overlay_rel}..." \
  bash -c "cd '${REPO_ROOT}' && ${KUBECTL} apply -k '${overlay_path}'"

demo_step 6 "Wait for export pipeline"
if demo_persona_git_enabled; then
  if ! demo_wait_for "Sink connection verified..." \
    "${KUBECTL}" wait --for=condition=ConnectionVerified \
      kollectsink/git-inventory-demo -n default --timeout=180s 2>/dev/null; then
    demo_info "ConnectionVerified pending — check secretRef and cluster egress."
    "${KUBECTL}" describe kollectsink git-inventory-demo -n default || true
  fi
else
  demo_info "Skipping ConnectionVerified — no Git sink in local persona."
fi

if ! demo_wait_for "Inventory Ready..." \
  "${KUBECTL}" wait --for=condition=Ready \
    kollectinventory/team-inventory -n default --timeout=240s 2>/dev/null; then
  "${KUBECTL}" describe kollectinventory team-inventory -n default || true
fi

demo_step 7 "Security rows (Trivy)"
vuln_count="$("${KUBECTL}" get vulnerabilityreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
  if [[ "${SKIP_PLATFORM}" -eq 0 && "${vuln_count}" == "0" ]]; then
    demo_info "Trivy **VulnerabilityReport** rows: 0 so far — security inventory typically populates in **~2–5 min** as scans complete."
    if demo_wait_for "Waiting up to 3m for first VulnerabilityReport..." \
      bash -c "deadline=\$((SECONDS+180)); while (( SECONDS < deadline )); do
      c=\$(${KUBECTL} get vulnerabilityreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')
      [[ \"\$c\" != \"0\" ]] && exit 0; sleep 10; done; exit 1" 2>/dev/null; then
    vuln_count="$("${KUBECTL}" get vulnerabilityreports -A --no-headers 2>/dev/null | wc -l | tr -d ' ')"
    demo_outcome "First VulnerabilityReports appeared (${vuln_count} rows)."
  fi
else
  demo_outcome "VulnerabilityReports: ${vuln_count} (counts grow as Trivy reconciles)."
fi

demo_step 8 "Outcomes"
"${KUBECTL}" get kollectinventory team-inventory -n default \
  -o custom-columns=NAME:.metadata.name,ITEMS:.status.itemCount,EXPORT:.status.lastExportTime

cert_count="$("${KUBECTL}" get certificates -A -l app.kubernetes.io/part-of=demo-fleet --no-headers 2>/dev/null | wc -l | tr -d ' ')"
es_count="$("${KUBECTL}" get externalsecrets -A -l kollect.dev/inventory=enabled --no-headers 2>/dev/null | wc -l | tr -d ' ')"

demo_outcome "Live inventory — fleet + ${vuln_count} Trivy + ${cert_count} Certificates + ${es_count} ExternalSecrets."

if [[ "${RUN_CHURN}" -eq 1 ]]; then
  export CHURN_PRESET="${CHURN_MODE:-${CHURN_PRESET:-fast}}"
  nohup bash "${SCRIPT_DIR}/churn/run.sh" >"${SCRIPT_DIR}/churn.log" 2>&1 &
  echo $! >"${SCRIPT_DIR}/churn.pid"
  demo_outcome "Churn started (${CHURN_PRESET}) PID=$(cat "${SCRIPT_DIR}/churn.pid") — tail -f hack/demo/kind-wide-scope/churn.log"
fi

if [[ "${RUN_REVEAL}" -eq 1 ]]; then
  # shellcheck source=lib/reveal.sh
  source "${SCRIPT_DIR}/lib/reveal.sh"
  demo_reveal_start "${SCRIPT_DIR}"
fi

if command -v gum >/dev/null 2>&1; then
  gum style --border double --border-foreground 10 --padding "1 2" \
    "Close" \
    "" \
    "samples/  — annotated Kollect CRs" \
    "docs/examples/  — Postgres, hub, events" \
    "ROADMAP.md  — early-adopter checklist"
else
  demo_info "**Close** — samples/, docs/examples/, ROADMAP.md"
fi

demo_outcome "Demo bootstrap complete ($(demo_persona_label))."
