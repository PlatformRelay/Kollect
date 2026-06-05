#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Phased workload mutations (~12 min) so Git export diffs show adds/updates/deletes.
# Run after bootstrap.sh; watch logs with logs.sh in another terminal.
set -euo pipefail

readonly KUBECTL="${KUBECTL:-kubectl}"

step() {
  echo ""
  echo "=== [$(date +%H:%M:%S)] $* ==="
}

step "T+0 — baseline applied; waiting 60s for first export cycle"
sleep 60

step "T+1m — scale api-gateway 2 → 3 replicas (team-a)"
"${KUBECTL}" scale deployment/api-gateway -n team-a --replicas=3

step "T+3m — bump frontend image tag (team-b)"
sleep 120
"${KUBECTL}" set image deployment/frontend -n team-b web=nginx:1.27

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

step "T+15m — delete and recreate nginx-demo Service (default)"
sleep 120
"${KUBECTL}" delete service nginx-demo -n default --ignore-not-found
sleep 5
"${KUBECTL}" apply -f - <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: nginx-demo
  namespace: default
  labels:
    app.kubernetes.io/name: nginx
    app.kubernetes.io/part-of: demo-fleet
spec:
  selector:
    app.kubernetes.io/name: nginx
  ports:
    - port: 80
      targetPort: 80
EOF

step "Churn complete — check export commits and inventory HTTP itemCount"
