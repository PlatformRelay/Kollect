#!/usr/bin/env bash
# Idempotent kollect-dev kind cluster: base operator install + optional dev addons.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../common.sh
source "${SCRIPT_DIR}/../common.sh"

readonly CLUSTER_NAME="${CLUSTER_NAME:-kollect-dev}"
readonly CLUSTER_CONFIG="${CLUSTER_CONFIG:-${SCRIPT_DIR}/cluster.yaml}"
readonly DEV_VALUES="${DEV_VALUES:-${REPO_ROOT}/charts/kollect/ci/dev-values.yaml}"
readonly CERT_DIR="${CERT_DIR:-${SCRIPT_DIR}/certs}"

_kind_require_tools
_kind_detect_provider

kind_create_cluster "$CLUSTER_NAME" "$CLUSTER_CONFIG"
kollect_install_base "$CLUSTER_NAME" "$DEV_VALUES"

if [[ "${KOLLECT_DEV_MINIMAL:-}" == "1" ]]; then
  _kind_log "KOLLECT_DEV_MINIMAL=1 — skipping ingress, TLS, Grafana, Prometheus."
  _kind_log "Dev cluster ${CLUSTER_NAME} ready (operator only)."
  exit 0
fi

_install_ingress_nginx() {
  _kind_log "Installing ingress-nginx (NodePort ${KIND_HOST_HTTP_PORT}/${KIND_HOST_HTTPS_PORT})..."
  helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx 2>/dev/null || true
  helm repo update ingress-nginx >/dev/null 2>&1 || helm repo update >/dev/null
  helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    --namespace ingress-nginx --create-namespace \
    --set controller.admissionWebhooks.enabled=false \
    --set controller.service.type=NodePort \
    --set "controller.service.nodePorts.http=${KIND_HOST_HTTP_PORT}" \
    --set "controller.service.nodePorts.https=${KIND_HOST_HTTPS_PORT}" \
    --wait --timeout 120s
}

_install_mkcert_tls() {
  if ! command -v mkcert >/dev/null 2>&1; then
    cat >&2 <<'EOF'
mkcert is not installed — skipping TLS secret for ingress.

Install mkcert for trusted *.localhost HTTPS:
  https://github.com/FiloSottile/mkcert#installation

Then re-run: task kind-dev-up
EOF
    return 0
  fi

  mkdir -p "$CERT_DIR"
  local cert_pem="${CERT_DIR}/localhost-wildcard.pem"
  local key_pem="${CERT_DIR}/localhost-wildcard-key.pem"

  if [[ ! -f "$cert_pem" || ! -f "$key_pem" ]]; then
    _kind_log "Generating mkcert TLS material in ${CERT_DIR}..."
    mkcert -install >/dev/null 2>&1 || true
    mkcert -cert-file "$cert_pem" -key-file "$key_pem" \
      localhost "*.localhost" 127.0.0.1 ::1 grafana.localhost >/dev/null
  fi

  kubectl create namespace ingress-nginx --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n ingress-nginx create secret tls kollect-local-tls \
    --cert="$cert_pem" --key="$key_pem" \
    --dry-run=client -o yaml | kubectl apply -f -

  kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n monitoring create secret tls kollect-local-tls \
    --cert="$cert_pem" --key="$key_pem" \
    --dry-run=client -o yaml | kubectl apply -f -

  helm upgrade ingress-nginx ingress-nginx/ingress-nginx \
    --namespace ingress-nginx \
    --reuse-values \
    --set controller.extraArgs.default-ssl-certificate=ingress-nginx/kollect-local-tls \
    --wait --timeout 60s
}

_install_grafana() {
  _kind_log "Installing Grafana (single replica, no persistence)..."
  helm repo add grafana https://grafana.github.io/helm-charts 2>/dev/null || true
  helm repo update grafana >/dev/null 2>&1 || helm repo update >/dev/null
  helm upgrade --install grafana grafana/grafana \
    --namespace monitoring --create-namespace \
    --set replicas=1 \
    --set persistence.enabled=false \
    --set adminPassword=admin \
    --set "ingress.enabled=true" \
    --set "ingress.ingressClassName=nginx" \
    --set "ingress.hosts[0]=grafana.localhost" \
    --set "ingress.tls[0].hosts[0]=grafana.localhost" \
    --set "ingress.tls[0].secretName=kollect-local-tls" \
    --wait --timeout 120s
}

_install_prometheus() {
  if [[ "${KOLLECT_DEV_PROMETHEUS:-}" != "1" ]]; then
    return 0
  fi
  _kind_log "Installing lightweight Prometheus (KOLLECT_DEV_PROMETHEUS=1)..."
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
  helm repo update prometheus-community >/dev/null 2>&1 || helm repo update >/dev/null
  helm upgrade --install prometheus prometheus-community/prometheus \
    --namespace monitoring \
    --set alertmanager.enabled=false \
    --set pushgateway.enabled=false \
    --set kube-state-metrics.enabled=false \
    --set prometheus-node-exporter.enabled=false \
    --set server.persistentVolume.enabled=false \
    --set server.resources.requests.memory=128Mi \
    --wait --timeout 120s
}

_install_ingress_nginx
_install_mkcert_tls
_install_grafana
_install_prometheus

_kind_log "Dev cluster ${CLUSTER_NAME} ready."
_kind_log "  kubectl context: kind-${CLUSTER_NAME}"
_kind_log "  Grafana (if TLS): https://grafana.localhost:${KIND_HOST_HTTPS_PORT}/ (admin/admin)"
_kind_log "  Skip addons next time: KOLLECT_DEV_MINIMAL=1 task kind-dev-up"
