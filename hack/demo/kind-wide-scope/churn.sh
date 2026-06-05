#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Phased workload mutations (~12 min) so Git export diffs show adds/updates/deletes.
# Run after demo.sh; watch logs with logs.sh in another terminal.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/ui.sh
source "${SCRIPT_DIR}/lib/ui.sh"

readonly KUBECTL="${KUBECTL:-kubectl}"

demo_require_gum
demo_intro "Churn choreography — watch inventory diffs land in Git"

step() {
  local msg="$1"
  if command -v gum >/dev/null 2>&1; then
    gum style --foreground 212 --bold "$(date +%H:%M:%S)" "${msg}"
  else
    echo ""
    echo "=== [$(date +%H:%M:%S)] ${msg} ==="
  fi
}

step "T+0 — baseline applied; waiting 60s for first export cycle"
sleep 60

step "T+1m — scale api-gateway 2 → 3 replicas (team-a)"
"${KUBECTL}" scale deployment/api-gateway -n team-a --replicas=3

step "T+3m — bump frontend image tag (team-b)"
sleep 120
"${KUBECTL}" set image deployment/frontend -n team-b web=traefik/whoami:v1.11.0

step "T+5m — patch backend labels (team-b)"
sleep 120
"${KUBECTL}" patch deployment backend -n team-b --type=merge \
  -p '{"metadata":{"labels":{"demo.kollect.dev/phase":"churn-5m"}}}'

step "T+7m — update feature-flags ConfigMap data (team-a)"
sleep 120
"${KUBECTL}" patch configmap feature-flags -n team-a --type=merge \
  -p '{"data":{"newCheckout":"true","churnMarker":"phase-7m"}}'

step "T+9m — delete catalog-sync Deployment (default)"
sleep 120
"${KUBECTL}" delete deployment catalog-sync -n default --ignore-not-found

step "T+11m — create billing-api Deployment (team-b) — add"
sleep 120
"${KUBECTL}" apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: billing-api
  namespace: team-b
  labels:
    app.kubernetes.io/name: billing-api
    app.kubernetes.io/part-of: demo-fleet
    kollect.dev/inventory: enabled
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: billing-api
  template:
    metadata:
      labels:
        app.kubernetes.io/name: billing-api
    spec:
      containers:
        - name: billing
          image: hashicorp/http-echo:1.0
          args: ["-text=billing"]
EOF

step "T+13m — suspend weekly-report CronJob (team-b)"
sleep 120
"${KUBECTL}" patch cronjob weekly-report -n team-b --type=merge -p '{"spec":{"suspend":true}}'

step "T+15m — delete and recreate storefront-demo Service (default)"
sleep 120
"${KUBECTL}" delete service storefront-demo -n default --ignore-not-found
sleep 5
"${KUBECTL}" apply -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: storefront-demo
  namespace: default
  labels:
    app.kubernetes.io/name: storefront
    app.kubernetes.io/part-of: demo-fleet
spec:
  selector:
    app.kubernetes.io/name: storefront
  ports:
    - port: 80
      targetPort: 80
EOF

demo_outcome "Churn complete — check export commits and inventory HTTP itemCount"
