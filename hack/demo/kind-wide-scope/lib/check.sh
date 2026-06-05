#!/usr/bin/env bash
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Konrad Heimel
#
# Prerequisite checker for the wide-scope demo (--check / task demo-check).
set -euo pipefail

# Private dir var — sourcing must not clobber the caller's SCRIPT_DIR.
_CHECK_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="$(cd "${_CHECK_LIB_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DEMO_DIR}/../../.." && pwd)"

readonly MIN_KIND_VERSION="0.20.0"

_version_ge() {
  local have="$1" want="$2"
  if [[ "$(printf '%s\n' "$want" "$have" | sort -V | head -1)" == "$want" ]]; then
    return 0
  fi
  return 1
}

_cmd_version() {
  local cmd="$1"
  case "$cmd" in
    kind) kind version 2>/dev/null | awk '/kind v/{print $3; exit}' | tr -d v ;;
    kubectl) kubectl version --client -o yaml 2>/dev/null | awk '/gitVersion/{print $2; exit}' | tr -d v ;;
    helm) helm version --template '{{.Version}}' 2>/dev/null | tr -d v ;;
    docker) docker version --format '{{.Client.Version}}' 2>/dev/null ;;
    gh) gh version 2>/dev/null | awk '{print $3; exit}' ;;
    go) go version 2>/dev/null | awk '{print $3}' | tr -d go ;;
    kustomize) kustomize version 2>/dev/null | head -1 | tr -d v ;;
    task) task --version 2>/dev/null | awk '{print $2}' ;;
    gum) gum --version 2>/dev/null | awk '{print $NF}' ;;
    *) echo "" ;;
  esac
}

_check_one() {
  local cmd="$1" required="$2" hint="$3"
  local min_ver="${4:-}"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    if [[ "$required" == "required" ]]; then
      printf "FAIL  %-12s missing — %s\n" "$cmd" "$hint"
      return 1
    fi
    printf "WARN  %-12s missing (optional) — %s\n" "$cmd" "$hint"
    return 0
  fi
  local ver
  ver="$(_cmd_version "$cmd")"
  if [[ -n "$min_ver" && -n "$ver" ]] && ! _version_ge "$ver" "$min_ver"; then
    printf "FAIL  %-12s v%s (need >= v%s) — %s\n" "$cmd" "$ver" "$min_ver" "$hint"
    return 1
  fi
  if [[ -n "$ver" ]]; then
    printf "OK    %-12s v%s\n" "$cmd" "$ver"
  else
    printf "OK    %-12s\n" "$cmd"
  fi
  return 0
}

_demo_check_finish() {
  local code="$1"
  if [[ "${DEMO_CHECK_STANDALONE:-0}" -eq 1 ]]; then
    exit "$code"
  fi
  return "$code"
}

fail=0
echo "Kollect wide-scope demo — prerequisite check"
echo "Repo: ${REPO_ROOT}"
echo ""

_check_one kind required "https://kind.sigs.k8s.io/" "$MIN_KIND_VERSION" || fail=1
_check_one kubectl required "https://kubernetes.io/docs/tasks/tools/" || fail=1
_check_one helm required "https://helm.sh/" || fail=1
_check_one kustomize required "https://kubectl.docs.kubernetes.io/installation/kustomize/" || fail=1
_check_one task required "task --version in repo root" || fail=1
_check_one docker required "Docker or compatible runtime for kind" || fail=1
_check_one gh optional "Git export proof — gh auth login" || true
_check_one go optional "auto-install Charm Gum via go install" || true
_check_one gum optional "guided UX — or go install github.com/charmbracelet/gum@latest" || true

echo ""
if ! kustomize build "${DEMO_DIR}" >/dev/null 2>&1; then
  echo "FAIL  kustomize build hack/demo/kind-wide-scope"
  fail=1
else
  echo "OK    kustomize build hack/demo/kind-wide-scope"
fi

echo ""
if [[ "$fail" -ne 0 ]]; then
  echo "Fix failures above, then re-run: bash hack/demo/kind-wide-scope/demo.sh --check"
  _demo_check_finish 1
fi
echo "All required checks passed."
_demo_check_finish 0
