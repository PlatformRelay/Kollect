#!/usr/bin/env bash
# LAB-DEKIND / U-08: webhook rejection assertions must run against an EXISTING cluster,
# not only a freshly created Kind stack — without changing the CI (Kind) path by one byte
# and without mutating the target cluster.
# Fully offline: kubectl/kind/helm are stubs; any real invocation fails the test.
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT}/hack/e2e/webhook-smoke.sh"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/e2e-webhook-existing.XXXXXX")"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'e2e webhook existing-cluster: %s\n' "$*" >&2
  exit 1
}
pass() { printf 'ok - %s\n' "$*"; }

[[ -f "${SCRIPT}" ]] || fail "missing script: ${SCRIPT}"

PROD_CTX='gke_acme-platform-4711_europe-west1_shared-cluster'
BIN="${TMP}/bin"
mkdir -p "${BIN}"

# Stub kubectl: records every call, answers from FAKE_* env.
cat >"${BIN}/kubectl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${CALLS}"
case "$*" in
  *"config current-context"*)
    printf '%s\n' "${FAKE_CTX:-}"
    ;;
  *"config view"*)
    printf '%s\n' "${FAKE_CLUSTER:-}"
    ;;
  *"config use-context"*) ;;
  *"get validatingwebhookconfiguration "*)
    [[ "${FAKE_VWC:-0}" == "1" ]] || exit 1
    printf 'validatingwebhookconfiguration.admissionregistration.k8s.io/stub\n'
    ;;
  *"get validatingwebhookconfiguration"*)
    printf 'NAME\n'
    ;;
  *"get crd certificates.cert-manager.io"*)
    [[ "${FAKE_CERT_MANAGER:-0}" == "1" ]] || exit 1
    printf 'customresourcedefinition.apiextensions.k8s.io/certificates.cert-manager.io\n'
    ;;
  *"get certificate"*)
    [[ "${FAKE_CERT_MANAGER:-0}" == "1" ]] || exit 1
    ;;
  *wait*)
    [[ "${FAKE_CERT_MANAGER:-0}" == "1" ]] || exit 1
    ;;
  *apply*)
    # The invalid sink arrives on stdin; drain it so the heredoc does not SIGPIPE.
    case "$*" in
      *" -f -"*) cat >/dev/null ;;
    esac
    if [[ "$*" == *"-f -"* ]]; then
      printf 'Error from server (Forbidden): admission webhook denied the request: spec.git is required\n' >&2
      exit 1
    fi
    printf 'kollectsnapshotsink.kollect.dev/e2e-snapshot-sink configured\n'
    ;;
  *)
    printf 'e2e webhook existing-cluster: unexpected kubectl: %s\n' "$*" >&2
    exit 97
    ;;
esac
EOF

# kind/helm must never be touched by the assertion path.
for forbidden in kind helm docker; do
  cat >"${BIN}/${forbidden}" <<EOF
#!/usr/bin/env bash
printf 'e2e webhook existing-cluster: forbidden ${forbidden} invocation: %s\n' "\$*" >>"\${CALLS}"
exit 96
EOF
done
chmod +x "${BIN}"/*

run_smoke() {
  local calls="$1"
  shift
  : >"${calls}"
  env -u KUBECONFIG PATH="${BIN}:${PATH}" CALLS="${calls}" "$@" \
    bash "${SCRIPT}" 2>&1
}

# --- 1. CI path unchanged: no new env ⇒ kind context switch + mutating apply ---
CALLS_CI="${TMP}/calls-ci"
rc=0
out="$(run_smoke "${CALLS_CI}" FAKE_CTX=kind-kollect-e2e FAKE_VWC=1 FAKE_CERT_MANAGER=1)" || rc=$?
[[ "${rc}" -eq 0 ]] || fail "default (Kind/CI) path must still pass, rc=${rc}: ${out}"
grep -q 'config use-context kind-kollect-e2e' "${CALLS_CI}" ||
  fail "default path must still switch to the kind-kollect-e2e context"
grep -Eq '^apply --dry-run=none -f .*snapshot-sink.yaml$' "${CALLS_CI}" ||
  fail "default path must still apply the valid sample for real (CI parity)"
grep -Eq '^apply .*--dry-run=server -f .*snapshot-sink.yaml' "${CALLS_CI}" &&
  fail "default path must not downgrade the sample apply to a server dry run"
grep -q 'forbidden' "${CALLS_CI}" && fail "assertion path must not shell out to kind/helm/docker"
pass "default path unchanged: kind context switch + real apply"

# --- 2. existing-cluster mode refuses a non-allowlisted (production) context ---
CALLS_PROD="${TMP}/calls-prod"
rc=0
out="$(run_smoke "${CALLS_PROD}" KOLLECT_E2E_EXISTING_CLUSTER=1 FAKE_CTX="${PROD_CTX}" FAKE_VWC=1)" || rc=$?
[[ "${rc}" -eq 2 ]] ||
  fail "existing-cluster mode MUST refuse a production context with exit 2, got ${rc}: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'allowlist|refus' ||
  fail "refusal must mention the allowlist: ${out}"
grep -q 'use-context' "${CALLS_PROD}" &&
  fail "refused run must never switch kube context"
grep -q 'apply' "${CALLS_PROD}" &&
  fail "refused run must never apply anything (not even a server dry-run)"
pass "existing-cluster mode refuses a production context before touching it"

# --- 3. existing-cluster mode fails loudly and early when webhooks are not installed ---
CALLS_NOWH="${TMP}/calls-nowh"
rc=0
out="$(run_smoke "${CALLS_NOWH}" KOLLECT_E2E_EXISTING_CLUSTER=1 FAKE_CTX=kumulus-lab \
  FAKE_CLUSTER=kumulus FAKE_VWC=0 FAKE_CERT_MANAGER=0)" || rc=$?
[[ "${rc}" -eq 4 ]] ||
  fail "missing webhook stack must exit 4 (precondition), got ${rc}: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'webhook.*(not installed|not present|disabled)' ||
  fail "missing webhook stack must say so explicitly: ${out}"
grep -q 'apply' "${CALLS_NOWH}" &&
  fail "must not attempt assertions when the webhook stack is absent"
pass "existing-cluster mode reports a missing webhook stack as a precondition (exit 4)"

# --- 4. existing-cluster mode asserts read-only against a live release ---
CALLS_OK="${TMP}/calls-ok"
rc=0
out="$(run_smoke "${CALLS_OK}" KOLLECT_E2E_EXISTING_CLUSTER=1 FAKE_CTX=kumulus-lab \
  FAKE_CLUSTER=kumulus FAKE_VWC=1 FAKE_CERT_MANAGER=0 \
  KOLLECT_RELEASE=kollect-op1 KOLLECT_NAMESPACE=kollect-op1)" || rc=$?
[[ "${rc}" -eq 0 ]] || fail "existing-cluster assertions must pass on an allowlisted lab, rc=${rc}: ${out}"
grep -q 'use-context' "${CALLS_OK}" && fail "existing-cluster mode must not switch kube context"
while IFS= read -r call; do
  [[ "${call}" == apply* ]] || continue
  [[ "${call}" == *"--dry-run=server"* ]] ||
    fail "existing-cluster mode must only server-dry-run; mutating call: ${call}"
done <"${CALLS_OK}"
grep -q 'apply --dry-run=server' "${CALLS_OK}" ||
  fail "existing-cluster mode must still exercise the reject/accept assertions"
grep -q 'kollect-op1' "${CALLS_OK}" ||
  fail "existing-cluster mode must honour KOLLECT_RELEASE/KOLLECT_NAMESPACE"
grep -q 'forbidden' "${CALLS_OK}" && fail "existing-cluster mode must not shell out to kind/helm/docker"
pass "existing-cluster mode is read-only (server dry-run) and release-parameterised"

# --- 5. the runbook documents the existing-cluster invocation ---
grep -rq 'KOLLECT_E2E_EXISTING_CLUSTER' "${ROOT}/hack/kind/README.md" "${ROOT}/hack/lab/README.md" ||
  fail "existing-cluster mode must be documented in hack/kind/README.md or hack/lab/README.md"
pass "existing-cluster mode documented"

echo "All e2e webhook existing-cluster tests passed."
