#!/usr/bin/env bash
# LAB-DEKIND: image delivery must follow the substrate (offline).
# `kind load docker-image` has no Talos equivalent, so on a non-Kind substrate the operator
# image MUST come from a pinned registry reference. A silent fallback to whatever the nodes
# already cached is how a stale-image run gets published as evidence — fail loud and early.
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMMON="${ROOT}/hack/kind/common.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-image-delivery.XXXXXX")"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab image delivery meta: %s\n' "$*" >&2
  exit 1
}
pass() { printf 'ok - %s\n' "$*"; }

[[ -f "${COMMON}" ]] || fail "missing ${COMMON}"

BIN="${TMP}/bin"
mkdir -p "${BIN}"

cat >"${BIN}/kubectl" <<'EOF'
#!/usr/bin/env bash
printf 'kubectl %s\n' "$*" >>"${CALLS}"
case "$*" in
  *"config current-context"*) printf '%s\n' "${FAKE_CTX:-}" ;;
  *"config view"*) printf '%s\n' "${FAKE_CLUSTER:-}" ;;
  *"get deployment"*) printf 'kollect-controller-manager\n' ;;
  *logs*) printf 'Starting Controller kollecttarget\n' ;;
  *) : ;;
esac
EOF
for tool in kind helm docker task; do
  cat >"${BIN}/${tool}" <<EOF
#!/usr/bin/env bash
printf '${tool} %s\n' "\$*" >>"\${CALLS}"
exit 0
EOF
done
chmod +x "${BIN}"/*

VALUES="${TMP}/values.yaml"
printf '{}\n' >"${VALUES}"

# Exercise kollect_install_base with the real library and stubbed tools.
install_base() {
  local calls="$1"
  shift
  : >"${calls}"
  env -u KUBECONFIG PATH="${BIN}:${PATH}" CALLS="${calls}" \
    KIND_CLUSTER_WAIT=5s KOLLECT_HELM_TIMEOUT=5s KOLLECT_MANAGER_WAIT=5s \
    KOLLECT_CONTROLLERS_WAIT=10s "$@" \
    bash -c '
      set -uo pipefail
      source "$1"
      kollect_install_base kollect-e2e "$2"
    ' _ "${COMMON}" "${VALUES}" 2>&1
}

# --- non-Kind substrate + default local image ⇒ loud, early refusal ---
CALLS_BAD="${TMP}/calls-bad"
rc=0
out="$(install_base "${CALLS_BAD}" FAKE_CTX=kumulus-lab FAKE_CLUSTER=kumulus)" || rc=$?
[[ "${rc}" -ne 0 ]] ||
  fail "non-Kind substrate with a local-only image must fail: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'registry|side-load|pin' ||
  fail "refusal must explain the registry requirement: ${out}"
grep -q '^kind load' "${CALLS_BAD}" &&
  fail "must never attempt 'kind load' on a non-Kind substrate"
grep -q '^helm' "${CALLS_BAD}" &&
  fail "must refuse BEFORE installing anything (no helm call)"
pass "non-Kind substrate refuses a local-only image before installing"

# --- non-Kind substrate + pinned registry tag ⇒ install, never side-load ---
CALLS_OK="${TMP}/calls-ok"
rc=0
out="$(install_base "${CALLS_OK}" FAKE_CTX=kumulus-lab FAKE_CLUSTER=kumulus \
  KOLLECT_IMAGE=ghcr.io/platformrelay/kollect:v0.17.0)" || rc=$?
[[ "${rc}" -eq 0 ]] || fail "pinned registry image should install on a non-Kind substrate: ${out}"
grep -q '^kind load' "${CALLS_OK}" && fail "must not side-load on a non-Kind substrate"
grep -q '^docker build' "${CALLS_OK}" && fail "must not rebuild the image for a registry install"
grep -q '^helm upgrade' "${CALLS_OK}" || fail "expected a helm install with the pinned image"
grep -q 'ghcr.io/platformrelay/kollect' "${CALLS_OK}" ||
  fail "helm install must carry the pinned registry repository"
pass "non-Kind substrate installs from the pinned registry reference only"

# --- Kind substrate keeps building + side-loading (CI path unchanged) ---
CALLS_KIND="${TMP}/calls-kind"
rc=0
out="$(install_base "${CALLS_KIND}" FAKE_CTX=kind-kollect-e2e)" || rc=$?
[[ "${rc}" -eq 0 ]] || fail "kind path must still work: ${out}"
grep -q '^kind load docker-image' "${CALLS_KIND}" ||
  fail "kind substrate must still side-load the freshly built image"
grep -q '^helm upgrade' "${CALLS_KIND}" || fail "kind substrate must still helm install"
pass "Kind substrate still builds and side-loads (CI parity)"

# --- an off-allowlist context is refused outright, before any install work ---
CALLS_PROD="${TMP}/calls-prod"
rc=0
out="$(install_base "${CALLS_PROD}" \
  FAKE_CTX='gke_acme-platform-4711_europe-west1_shared-cluster' \
  KOLLECT_IMAGE=ghcr.io/platformrelay/kollect:v0.17.0)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "off-allowlist context must not be installed onto: ${out}"
grep -q '^helm' "${CALLS_PROD}" && fail "off-allowlist context must never reach helm"
grep -q '^kind load' "${CALLS_PROD}" && fail "off-allowlist context must never reach kind load"
pass "off-allowlist context refused before any install work"

echo "All lab image delivery meta tests passed."
