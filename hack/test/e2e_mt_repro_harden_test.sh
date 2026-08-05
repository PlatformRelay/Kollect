#!/usr/bin/env bash
# E2E-MT-REPRO-HARDEN: multitenant Kind assert must diagnose empty inventory
# (Target status / Deployments / unfiltered /inventory / manager logs) and wait
# for Target collecting >= 1 before the HTTP itemCount poll — contract only, no Kind.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT}/hack/e2e/multitenant.sh"
NIGHTLY="${ROOT}/.github/workflows/e2e-nightly.yaml"
EXTENDED="${ROOT}/.github/workflows/e2e-extended.yaml"

fail() {
  printf 'e2e-mt-repro-harden: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${SCRIPT}" ]] || fail "missing ${SCRIPT}"
[[ -x "${SCRIPT}" ]] || fail "${SCRIPT} must be executable"

# Failure dump must surface Target status / collected signal (not only manager --tail=80).
grep -Eq '_fail_diag|fail_diag' "${SCRIPT}" ||
  fail "multitenant.sh must define _fail_diag (or fail_diag) for inventory/collection failures"
grep -Eq 'describe[[:space:]]+"?kollecttarget|get[[:space:]]+"?kollecttarget' "${SCRIPT}" ||
  fail "fail dump must include KollectTarget describe/get for the tenant"
grep -Eq 'apps/v1|deployments?|Deployment' "${SCRIPT}" ||
  fail "fail dump must list Deployments in the tenant namespace"
grep -Eq 'unfiltered|/inventory\"|curl.*\$\{port\}/inventory[^\?]' "${SCRIPT}" ||
  fail "fail dump must curl unfiltered /inventory (not only ?namespace=)"
grep -Eq 'DIAG_LOG_TAIL|--tail=[0-9]{3,}|tail=[0-9]{3,}' "${SCRIPT}" ||
  fail "fail dump must use a useful manager log tail (>=100 lines), not only --tail=80"
grep -Eq 'DIAG_LOG_TAIL=.*[1-9][0-9]{2,}|--tail=2[0-9]{2}' "${SCRIPT}" ||
  fail "DIAG_LOG_TAIL / --tail default must be >= 100"

# Harden: wait for Target collecting >= 1 (Ready alone allows collecting 0).
grep -Eq 'collecting|wait_target_collected|target_collected' "${SCRIPT}" ||
  fail "multitenant.sh must wait for Target collecting count >= 1 before HTTP itemCount assert"
grep -Eq 'wait_target_collected|target_collected_count' "${SCRIPT}" ||
  fail "multitenant.sh must expose wait_target_collected (or target_collected_count) helper"

# itemCount HTTP wait must call the enriched dump on timeout.
if ! grep -A20 'wait_inventory_http_collected' "${SCRIPT}" | grep -Eq '_fail_diag|fail_diag'; then
  fail "wait_inventory_http_collected must call _fail_diag on timeout"
fi

# Workflow meta job wiring (nightly + extended), same pattern as finalizer-cleanup-meta.
grep -Fq 'hack/test/e2e_mt_repro_harden_test.sh' "${NIGHTLY}" ||
  fail "e2e-nightly.yaml must run hack/test/e2e_mt_repro_harden_test.sh"
grep -Fq 'hack/test/e2e_mt_repro_harden_test.sh' "${EXTENDED}" ||
  fail "e2e-extended.yaml must run hack/test/e2e_mt_repro_harden_test.sh"

pass "multitenant fail-dump + collecting-wait + workflow wiring contract"
