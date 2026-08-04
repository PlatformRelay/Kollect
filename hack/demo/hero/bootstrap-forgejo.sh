#!/usr/bin/env bash
# Bootstrap Forgejo: wait for headless instance, admin user, inventory repo, push token.
# Forgejo 11 removed /api/v1/install — manifests set INSTALL_LOCK + sqlite via env.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

_hero_require_tools
_hero_detect_provider
kind_use_context "$HERO_CLUSTER"

_internal_url="http://forgejo.${HERO_FORGEJO_NS}.svc.cluster.local:3000"

_wait_forgejo() {
  _hero_log "Waiting for Forgejo Deployment..."
  # With INSTALL_LOCK, /api/v1/version is available as soon as the server is up.
  kubectl wait --for=condition=Available deployment/forgejo -n "$HERO_FORGEJO_NS" --timeout=600s
  local deadline=$((SECONDS + 480))
  while (( SECONDS < deadline )); do
    if kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
      curl -fsS "${_internal_url}/api/v1/version" >/dev/null 2>&1; then
      return 0
    fi
    sleep 3
  done
  echo "Forgejo API (/api/v1/version) not ready after Available + 480s wait" >&2
  kubectl -n "$HERO_FORGEJO_NS" get pods,deploy -o wide 2>/dev/null || true
  kubectl -n "$HERO_FORGEJO_NS" logs deploy/forgejo --tail=80 2>/dev/null || true
  return 1
}

_ensure_admin() {
  if kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
    curl -fsS -u "${HERO_FORGEJO_USER}:${HERO_FORGEJO_PASS}" \
    "${_internal_url}/api/v1/user" >/dev/null 2>&1; then
    _hero_log "Admin user ${HERO_FORGEJO_USER} already exists."
    return 0
  fi

  _hero_log "Creating Forgejo admin user ${HERO_FORGEJO_USER}..."
  # forgejo admin speaks to the sqlite DB; config is generated from env by the entrypoint.
  kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
    forgejo admin user create \
      --admin \
      --username "${HERO_FORGEJO_USER}" \
      --password "${HERO_FORGEJO_PASS}" \
      --email "demo@kollect.dev" \
      --must-change-password=false
}

_create_repo() {
  if kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
    curl -fsS -u "${HERO_FORGEJO_USER}:${HERO_FORGEJO_PASS}" \
    "${_internal_url}/api/v1/repos/${HERO_FORGEJO_USER}/${HERO_FORGEJO_REPO}" >/dev/null 2>&1; then
    _hero_log "Repo ${HERO_FORGEJO_USER}/${HERO_FORGEJO_REPO} already exists."
    return 0
  fi

  _hero_log "Creating repo ${HERO_FORGEJO_USER}/${HERO_FORGEJO_REPO}..."
  kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- curl -fsS -X POST \
    -u "${HERO_FORGEJO_USER}:${HERO_FORGEJO_PASS}" \
    "${_internal_url}/api/v1/user/repos" \
    -H 'Content-Type: application/json' \
    -d "$(cat <<EOF
{
  "name": "${HERO_FORGEJO_REPO}",
  "auto_init": true,
  "default_branch": "main",
  "private": false
}
EOF
)"
}

_create_token() {
  _hero_log "Creating Forgejo API token for git push..."
  local response
  response="$(kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- curl -fsS -X POST \
    -u "${HERO_FORGEJO_USER}:${HERO_FORGEJO_PASS}" \
    "${_internal_url}/api/v1/users/${HERO_FORGEJO_USER}/tokens" \
    -H 'Content-Type: application/json' \
    -d '{"name":"kollect-hero-push","scopes":["write:repository","read:repository"]}')"

  FORGEJO_TOKEN="$(echo "$response" | sed -n 's/.*"sha1":"\([^"]*\)".*/\1/p')"
  if [[ -z "$FORGEJO_TOKEN" ]]; then
    echo "Failed to parse Forgejo token from: $response" >&2
    return 1
  fi
  export FORGEJO_TOKEN
  _hero_write_state
}

_wait_forgejo
_ensure_admin
_create_repo
_create_token

kubectl create secret generic "$HERO_GIT_SECRET" -n default \
  --from-literal=username="${HERO_FORGEJO_USER}" \
  --from-literal=token="${FORGEJO_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

_hero_log "Forgejo bootstrap complete (user=${HERO_FORGEJO_USER}, repo=${HERO_FORGEJO_REPO})."
