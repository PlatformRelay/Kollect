#!/usr/bin/env bash
# DEMO-03: canonical Git-only hero demo must have an automated smoke path —
# assert helpers + smoke orchestrator + e2e-smoke.yaml job (not the required
# kind-smoke check). Pure contract test; does not require kind.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKFLOW="${ROOT}/.github/workflows/e2e-smoke.yaml"
ASSERT="${ROOT}/hack/demo/hero/assert.sh"
SMOKE="${ROOT}/hack/demo/hero/smoke.sh"
LIB="${ROOT}/hack/demo/hero/lib.sh"
NOOP_SETUP="${ROOT}/hack/demo/hero/ci-noop-setup.sh"

fail() {
  printf 'demo-03 hero smoke: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${WORKFLOW}" ]] || fail "missing ${WORKFLOW}"
[[ -f "${LIB}" ]] || fail "missing ${LIB}"

# --- assert.sh: real helper call sites (not comments / log strings) ---
[[ -f "${ASSERT}" ]] || fail "missing ${ASSERT} (hero assert entrypoint)"
[[ -x "${ASSERT}" ]] || fail "${ASSERT} must be executable"
# Anchor at line start so header comments / _hero_log text cannot satisfy these.
grep -Eq '^[[:space:]]*_hero_assert_inventory_ready([[:space:]]|$)' "${ASSERT}" ||
  fail "assert.sh must call _hero_assert_inventory_ready"
grep -Eq '^[[:space:]]*_hero_assert_git_connection_verified([[:space:]]|$)' "${ASSERT}" ||
  fail "assert.sh must call _hero_assert_git_connection_verified"
grep -Eq '^[[:space:]]*_hero_assert_git_export_nonempty([[:space:]]|$)' "${ASSERT}" ||
  fail "assert.sh must call _hero_assert_git_export_nonempty"
# Failure diagnostics must name the condition / surface logs (AC edge).
grep -Eq 'kubectl (logs|describe|get)' "${ASSERT}" "${LIB}" ||
  fail "assert path must surface kubectl logs/describe/get on failure"
pass "assert.sh calls Ready / ConnectionVerified / export helpers"

# --- smoke.sh: up → assert → down, skip when kind unavailable ---
[[ -f "${SMOKE}" ]] || fail "missing ${SMOKE} (hero smoke orchestrator)"
[[ -x "${SMOKE}" ]] || fail "${SMOKE} must be executable"
grep -Eq 'up\.sh|demo-hero-up' "${SMOKE}" ||
  fail "smoke.sh must invoke hero up (git-only path)"
grep -Eq 'assert\.sh' "${SMOKE}" ||
  fail "smoke.sh must invoke assert.sh"
grep -Eq 'down\.sh|demo-hero-down' "${SMOKE}" ||
  fail "smoke.sh must tear down (down.sh) so kind clusters do not leak"
# Clear skip when kind/docker unavailable (local); CI must still fail hard.
grep -Eq 'SKIP|skip' "${SMOKE}" ||
  fail "smoke.sh must print a clear SKIP when kind is unavailable"
grep -Eq '_hero_kind_available|command -v kind|kind get|kind_cluster' "${SMOKE}" ||
  fail "smoke.sh must probe kind availability before bootstrapping"
grep -Eq '^_hero_kind_available\(\)' "${LIB}" ||
  fail "lib.sh must define _hero_kind_available"
pass "smoke.sh orchestrates up → assert → down with kind skip"

# --- lib.sh must define hero aliases used by up/down/bootstrap ---
grep -Eq '^_hero_detect_provider\(\)' "${LIB}" ||
  fail "lib.sh must define _hero_detect_provider (up.sh/bootstrap call it)"
grep -Eq '^_hero_require\(\)' "${LIB}" ||
  fail "lib.sh must define _hero_require (down.sh calls it)"
pass "lib.sh defines _hero_detect_provider / _hero_require"

# --- e2e-smoke.yaml: separate non-required hero-demo-smoke job (grep; no yq) ---
JOB_ID=""
for candidate in hero-demo-smoke hero-smoke demo-hero-smoke; do
  if grep -Eq "^[[:space:]]*${candidate}:" "${WORKFLOW}"; then
    JOB_ID="${candidate}"
    break
  fi
done
[[ -n "${JOB_ID}" ]] ||
  fail "e2e-smoke.yaml missing hero-demo-smoke job (canonical demo path)"

# Must NOT rename/replace the required kind-smoke check.
grep -Eq '^[[:space:]]*kind-smoke:' "${WORKFLOW}" ||
  fail "kind-smoke job must remain (required Tier-0 check)"
pass "kind-smoke preserved; ${JOB_ID} is a separate job"

# Extract the hero job block (from its key until the next top-level job key).
JOB_BLOB="$(
  awk -v key="${JOB_ID}" '
    $0 ~ "^  " key ":" { grab=1; next }
    grab && /^  [a-zA-Z0-9_-]+:/ { exit }
    grab { print }
  ' "${WORKFLOW}"
)"
[[ -n "${JOB_BLOB}" ]] || fail "job '${JOB_ID}' body empty"

printf '%s\n' "${JOB_BLOB}" | grep -Eq 'hack/demo/hero/smoke\.sh' ||
  fail "job '${JOB_ID}' must run hack/demo/hero/smoke.sh"

# Hero owns kollect-hero — must not use the default e2e setup (kollect-e2e + helm).
if printf '%s\n' "${JOB_BLOB}" | grep -Eq 'kind-e2e-setup'; then
  SETUP="$(
    printf '%s\n' "${JOB_BLOB}" | awk '
      /setup-script:/ {
        line=$0
        sub(/^[^:]*:[[:space:]]*/, "", line)
        gsub(/[[:space:]"]/, "", line)
        print line
        exit
      }
    '
  )"
  [[ -n "${SETUP}" && "${SETUP}" != "hack/kind/e2e/setup.sh" ]] ||
    fail "job '${JOB_ID}' must override setup-script (hero creates kollect-hero; default setup would double-cluster)"
  [[ -f "${ROOT}/${SETUP}" ]] || fail "setup-script ${SETUP} missing"
  pass "kind-e2e-setup uses custom setup-script=${SETUP}"
fi

# Not a required check name — job name must not be kind-smoke.
NAME="$(
  printf '%s\n' "${JOB_BLOB}" | awk '
    /^[[:space:]]*name:/ {
      line=$0
      sub(/^[^:]*:[[:space:]]*/, "", line)
      gsub(/^["'\'']|["'\'']$/, "", line)
      print line
      exit
    }
  '
)"
[[ "${NAME}" != "kind-smoke" ]] ||
  fail "hero job must not be named kind-smoke (would become the required check)"
pass "job '${JOB_ID}' name='${NAME:-$JOB_ID}' is not the required check"

# Empty-export failure helper: assert path must fail loudly when clone has no inventory files.
# Unit-check the file-presence predicate via a temp fixture (no cluster).
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT
mkdir -p "${TMP}/empty-clone"
if find "${TMP}/empty-clone" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) \
  ! -path '*/.git/*' | grep -q .; then
  fail "fixture self-check: empty clone unexpectedly had inventory files"
fi
mkdir -p "${TMP}/full-clone"
printf 'apiVersion: v1\nkind: ConfigMap\n' >"${TMP}/full-clone/inventory.yaml"
find "${TMP}/full-clone" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) \
  ! -path '*/.git/*' | grep -q . ||
  fail "fixture self-check: full clone should have inventory files"
# assert.sh / lib must use the same find predicate shape.
grep -Eq "find .*\\\.yaml|find \"\\\$\{?HERO_INVENTORY|_hero_export_has_inventory_files" "${ASSERT}" "${LIB}" ||
  fail "assert/lib must use find-based non-empty export check (same as preflight)"
pass "empty-export predicate shape locked (seeded-empty would fail)"

# Optional: ci-noop-setup referenced by workflow should exist when used.
if [[ -f "${NOOP_SETUP}" ]]; then
  [[ -x "${NOOP_SETUP}" ]] || fail "${NOOP_SETUP} must be executable"
  pass "ci-noop-setup.sh present"
fi

# --- DEMO-03-FUP2: Forgejo Available must mean the API is ready (not just /) ---
# CI flake: Deployment Available on path:/ while /api/v1/version still 502 → bootstrap
# exits "Forgejo API not ready" exactly at the old 180s post-Available window.
FORGEJO_MANIFEST="${ROOT}/hack/demo/hero/manifests/forgejo.yaml"
BOOTSTRAP="${ROOT}/hack/demo/hero/bootstrap-forgejo.sh"
[[ -f "${FORGEJO_MANIFEST}" ]] || fail "missing ${FORGEJO_MANIFEST}"
[[ -f "${BOOTSTRAP}" ]] || fail "missing ${BOOTSTRAP}"
grep -Eq 'path:[[:space:]]*/api/v1/version' "${FORGEJO_MANIFEST}" ||
  fail "forgejo readiness/startup probe must hit /api/v1/version (not bare /)"
# Belt-and-suspenders: post-Available API wait must exceed the observed ~3min flake window.
grep -Eq 'SECONDS \+ (4[8-9][0-9]|[5-9][0-9]{2}|[1-9][0-9]{3,})' "${BOOTSTRAP}" ||
  fail "bootstrap _wait_forgejo deadline must be ≥480s after Available (was 180s flake)"
pass "Forgejo probe + bootstrap wait lock API readiness"

printf 'demo-03 hero smoke: ok\n'
