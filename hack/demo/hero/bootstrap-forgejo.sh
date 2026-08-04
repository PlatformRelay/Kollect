#!/usr/bin/env bash
# Bootstrap Forgejo: web install, inventory repo, API token for git push.
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
  # Available now requires /api/v1/version (see manifests/forgejo.yaml probes).
  kubectl wait --for=condition=Available deployment/forgejo -n "$HERO_FORGEJO_NS" --timeout=600s
  # Belt-and-suspenders: cold kind nodes still need headroom after Available.
  local deadline=$((SECONDS + 480))
  while (( SECONDS < deadline )); do
    if kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
      curl -fsS "${_internal_url}/api/v1/version" >/dev/null 2>&1; then
      return 0
    fi
    sleep 3
  done
  echo "Forgejo API not ready after Available + 480s API wait" >&2
  kubectl -n "$HERO_FORGEJO_NS" get pods,deploy -o wide 2>/dev/null || true
  kubectl -n "$HERO_FORGEJO_NS" logs deploy/forgejo --tail=80 2>/dev/null || true
  return 1
}

_install_forgejo() {
  if kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- \
    curl -fsS "${_internal_url}/api/v1/settings/api" >/dev/null 2>&1; then
    _hero_log "Forgejo already installed."
    return 0
  fi

  _hero_log "Running Forgejo first-time install..."
  kubectl exec -n "$HERO_FORGEJO_NS" deploy/forgejo -- curl -fsS -X POST \
    "${_internal_url}/api/v1/install" \
    -H 'Content-Type: application/json' \
    -d "$(cat <<EOF
{
  "db_type": "sqlite3",
  "db_host": "localhost:3306",
  "db_user": "",
  "db_passwd": "",
  "db_name": "gitea",
  "ssl_mode": "disable",
  "db_path": "/data/gitea.db",
  "app_name": "Forgejo",
  "repo_root_path": "/data/git/repositories",
  "lfs_root_path": "/data/git/lfs",
  "run_user": "git",
  "domain": "forgejo.forgejo.svc.cluster.local",
  "ssh_domain": "forgejo.forgejo.svc.cluster.local",
  "port": 3000,
  "http_port": 3000,
  "root_url": "${_internal_url}/",
  "log_level": "Info",
  "admin_name": "${HERO_FORGEJO_USER}",
  "admin_passwd": "${HERO_FORGEJO_PASS}",
  "admin_confirm_passwd": "${HERO_FORGEJO_PASS}",
  "admin_email": "demo@kollect.dev"
}
EOF
)"
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
_install_forgejo
_create_repo
_create_token

kubectl create secret generic "$HERO_GIT_SECRET" -n default \
  --from-literal=username="${HERO_FORGEJO_USER}" \
  --from-literal=token="${FORGEJO_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

_hero_log "Forgejo bootstrap complete (user=${HERO_FORGEJO_USER}, repo=${HERO_FORGEJO_REPO})."
