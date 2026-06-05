#!/usr/bin/env bash
# Local e2e preflight: warn on host settings that commonly break kind kube-proxy/CoreDNS.
set -euo pipefail

_log() { echo "[e2e-preflight] $*"; }

instances="$(cat /proc/sys/fs/inotify/max_user_instances 2>/dev/null || echo 128)"
watches="$(cat /proc/sys/fs/inotify/max_user_watches 2>/dev/null || echo 0)"

if [[ "${instances}" -lt 256 ]]; then
  _log "WARN: fs.inotify.max_user_instances=${instances} (recommend >=512 for kind e2e)."
  _log "      kube-proxy may crash with 'too many open files' on busy hosts."
  _log "      Example: sudo sysctl -w fs.inotify.max_user_instances=512"
fi

if [[ "${watches}" -gt 0 && "${watches}" -lt 524288 ]]; then
  _log "WARN: fs.inotify.max_user_watches=${watches} (recommend >=524288 for kind e2e)."
fi

if [[ "${E2E_PREFLIGHT_STRICT:-}" == "1" && "${instances}" -lt 256 ]]; then
  echo "e2e preflight failed (set E2E_PREFLIGHT_STRICT=0 to override)" >&2
  exit 1
fi
