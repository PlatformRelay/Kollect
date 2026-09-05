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
#
# GATE-SIGPIPE-01: this fixture is deliberately LARGE and the predicate deliberately COUNTS.
# The old shape was `find … | grep -q .` against a one-file fixture. Under `set -o pipefail`
# grep exits at its FIRST match and closes the pipe; if find is still writing it takes
# SIGPIPE and exits 141, and pipefail hands the caller a non-zero status for a directory
# that IS non-empty. That is a RACE, not a size threshold — it has been reproduced at
# ~21 KB of find output, a third of a pipe buffer — but small output usually wins the race,
# which is exactly why a one-file fixture passed with either implementation and so tested
# nothing at all. `wc -l` reads to EOF, so the producer never sees SIGPIPE.
#
# The same shape exists elsewhere in the tree (`kubectl logs --tail=400 | grep -Eq …` in
# hack/kind/common.sh is the sharpest). Sweeping for it repo-wide is backlog story
# GATE-SIGPIPE-02; this file only locks the hero demo's own predicate.
readonly FIXTURE_FILES=4000
# Not a safety threshold — see above. It is a floor that keeps the fixture in the range
# where the race is reliably lost, so this self-check cannot quietly stop testing anything.
readonly PIPE_BUFFER_BYTES=65536
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

# Number of inventory files (yaml/yml/json, outside .git) under $1. Counts to EOF.
# `wc -l` rather than `grep -c .`: wc exits 0 on a zero count, so no `|| true` is needed,
# and `|| true` would turn a failed find into the string "0" — i.e. into "no files", which
# would make the empty-clone absence assertion below pass for the wrong reason. A broken
# find must be loud here, not silently agree with the assertion.
hero_inventory_count() {
  local n
  n="$(
    find "$1" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.json' \) \
      ! -path '*/.git/*' | wc -l
  )" || fail "inventory count predicate failed for '$1' (find or wc errored — NOT 'no files')"
  printf '%s\n' "${n}"
}

mkdir -p "${TMP}/empty-clone"
[[ "$(hero_inventory_count "${TMP}/empty-clone")" -eq 0 ]] ||
  fail "fixture self-check: empty clone unexpectedly had inventory files"

mkdir -p "${TMP}/full-clone"
for i in $(seq 1 "${FIXTURE_FILES}"); do
  printf 'apiVersion: v1\nkind: ConfigMap\n' >"${TMP}/full-clone/inventory${i}.yaml"
done
FIXTURE_BYTES="$(find "${TMP}/full-clone" -type f -name '*.yaml' | wc -c)"
[[ "${FIXTURE_BYTES}" -gt "${PIPE_BUFFER_BYTES}" ]] ||
  fail "fixture self-check: full clone yields only ${FIXTURE_BYTES} bytes of find output — at or below one pipe buffer, so it can no longer exercise the SIGPIPE inversion (raise FIXTURE_FILES)"
[[ "$(hero_inventory_count "${TMP}/full-clone")" -eq "${FIXTURE_FILES}" ]] ||
  fail "fixture self-check: full clone should have inventory files"

# A FAILING find must be loud, never a count. `|| fail` above is what enforces that, and
# nothing exercised it: with `|| true` in its place a directory find cannot fully read
# comes back as a plain number, which the EMPTY-CLONE assertion above would then agree
# with. Fixture: a directory find can only partly read, holding a file it CAN see.
# The fixtures above exercise hero_inventory_count, which lives in THIS file. Drive the
# real thing too: lib.sh's _hero_export_has_inventory_files is what runs against a cloned
# inventory repo, and it is the one a regression would actually break. Sourced in a
# subshell because lib.sh declares readonly globals.
lib_predicate_says() { # dir -> "present" | "absent"
  (
    set -euo pipefail
    # shellcheck source=../demo/hero/lib.sh disable=SC1091
    source "${LIB}" >/dev/null 2>&1
    if _hero_export_has_inventory_files "$1"; then printf 'present\n'; else printf 'absent\n'; fi
  )
}

BROKEN="${TMP}/broken-clone"
mkdir -p "${BROKEN}/blocked"
printf 'apiVersion: v1\n' >"${BROKEN}/visible.yaml"
chmod 000 "${BROKEN}/blocked" 2>/dev/null || true
if find "${BROKEN}" -type f >/dev/null 2>&1; then
  # Root, or a filesystem ignoring the mode: the error path is unreachable, so skip loudly
  # rather than report an assertion that did not run.
  printf '# skipped the producer-failure assertions: find can still read a 0000 directory as this user\n'
else
  broken_msg=""
  if broken_out="$(hero_inventory_count "${BROKEN}" 2>/dev/null)"; then
    broken_msg="count predicate returned '${broken_out}' for a directory find could NOT fully read — a producer error must be loud, never a count (an absence assertion would silently agree with it)"
  elif [[ "$(lib_predicate_says "${BROKEN}" 2>/dev/null)" == "present" ]]; then
    broken_msg="_hero_export_has_inventory_files reported 'present' for a directory find could NOT fully read — a failed producer must never be reported as success (lib.sh's '|| return 1' is what enforces this)"
  fi
  chmod 755 "${BROKEN}/blocked" 2>/dev/null || true
  [[ -z "${broken_msg}" ]] || fail "${broken_msg}"
fi
chmod 755 "${BROKEN}/blocked" 2>/dev/null || true

[[ "$(lib_predicate_says "${TMP}/full-clone")" == "present" ]] ||
  fail "_hero_export_has_inventory_files reported the ${FIXTURE_FILES}-file export as EMPTY — SIGPIPE inversion under pipefail"
[[ "$(lib_predicate_says "${TMP}/empty-clone")" == "absent" ]] ||
  fail "_hero_export_has_inventory_files reported an empty export as non-empty — the predicate is vacuous"

# assert.sh / lib must use the same find predicate shape.
grep -Eq "find .*\\\.yaml|find \"\\\$\{?HERO_INVENTORY|_hero_export_has_inventory_files" "${ASSERT}" "${LIB}" ||
  fail "assert/lib must use find-based non-empty export check (same as preflight)"

# ...and its consumer must READ TO EOF rather than short-circuit, for the reason above:
# this predicate runs against a real cloned inventory repo, i.e. the large-producer case.
# The lock names the PROPERTY (any EOF-reading counter), not one spelling of it — locking
# the literal `grep -c` would reject `wc -l`, which is the better form of the same fix.
EXPORT_FN="$(awk '/^_hero_export_has_inventory_files\(\)/,/^}/' "${LIB}")"
[[ -n "${EXPORT_FN}" ]] || fail "could not locate _hero_export_has_inventory_files in ${LIB}"
grep -Eq 'wc[[:space:]]+-l|grep[[:space:]]+(-[a-zA-Z-]+[[:space:]]+)*-[a-zA-Z]*c' <<<"${EXPORT_FN}" ||
  fail "_hero_export_has_inventory_files must count with a consumer that reads to EOF (wc -l, or grep -c) — a short-circuiting consumer inverts under pipefail"
if grep -Eq 'grep[[:space:]]+(-[a-zA-Z-]+[[:space:]]+)*-[a-zA-Z]*q|[[:space:]]head([[:space:]]|$)' <<<"${EXPORT_FN}"; then
  fail "_hero_export_has_inventory_files must not use a short-circuiting consumer ('grep -q' / 'head') — it closes the pipe early and inverts the result under pipefail"
fi
pass "empty-export predicate locked: counts to EOF, loud on producer failure, and lib.sh's own predicate agrees on a large fixture"

# Optional: ci-noop-setup referenced by workflow should exist when used.
if [[ -f "${NOOP_SETUP}" ]]; then
  [[ -x "${NOOP_SETUP}" ]] || fail "${NOOP_SETUP} must be executable"
  pass "ci-noop-setup.sh present"
fi

# --- DEMO-03-FUP3: headless Forgejo (INSTALL_LOCK) — /api/v1/install is gone in v11 ---
FORGEJO_MANIFEST="${ROOT}/hack/demo/hero/manifests/forgejo.yaml"
BOOTSTRAP="${ROOT}/hack/demo/hero/bootstrap-forgejo.sh"
[[ -f "${FORGEJO_MANIFEST}" ]] || fail "missing ${FORGEJO_MANIFEST}"
[[ -f "${BOOTSTRAP}" ]] || fail "missing ${BOOTSTRAP}"
grep -Eq 'FORGEJO__security__INSTALL_LOCK' "${FORGEJO_MANIFEST}" ||
  fail "forgejo manifest must set INSTALL_LOCK (skip web wizard; /api/v1/install removed)"
grep -Eq 'path:[[:space:]]*/api/v1/version' "${FORGEJO_MANIFEST}" ||
  fail "with INSTALL_LOCK, readiness/startup must probe /api/v1/version"
if grep -v '^[[:space:]]*#' "${BOOTSTRAP}" | grep -Eq 'api/v1/install'; then
  fail "bootstrap must not POST /api/v1/install (404 on Forgejo 11)"
fi
grep -Eq 'su-exec git forgejo admin user create|_ensure_admin' "${BOOTSTRAP}" ||
  fail "bootstrap must create admin via su-exec git forgejo CLI (refuses root)"
awk '/^_wait_forgejo\(\)/,/^}/' "${BOOTSTRAP}" | grep -v '^[[:space:]]*#' | grep -Eq 'api/v1/version' ||
  fail "_wait_forgejo must curl /api/v1/version once INSTALL_LOCK serves the API"
grep -Eq 'SECONDS \+ (4[8-9][0-9]|[5-9][0-9]{2}|[1-9][0-9]{3,})' "${BOOTSTRAP}" ||
  fail "bootstrap _wait_forgejo deadline must be ≥480s after Available"
pass "Forgejo headless INSTALL_LOCK + CLI admin locked"

# InventorySinkRef is object-shaped (name required) — string form fails admission.
INV_SAMPLE="${ROOT}/config/samples/demo/git-only/inventory.yaml"
[[ -f "${INV_SAMPLE}" ]] || fail "missing ${INV_SAMPLE}"
grep -Eq 'snapshotSinkRefs:[[:space:]]*$' "${INV_SAMPLE}" ||
  fail "git-only inventory must declare snapshotSinkRefs"
grep -Eq '^[[:space:]]*-[[:space:]]*name:[[:space:]]*hero-git-sink[[:space:]]*$' "${INV_SAMPLE}" ||
  fail "git-only inventory snapshotSinkRefs must use object form (- name: hero-git-sink)"
pass "demo inventory uses InventorySinkRef object form"

# Hero install must enable allowPrivateSinks for in-cluster Forgejo ClusterIP.
HERO_VALUES="${ROOT}/charts/kollect/ci/hero-values.yaml"
LIB="${ROOT}/hack/demo/hero/lib.sh"
[[ -f "${HERO_VALUES}" ]] || fail "missing ${HERO_VALUES}"
grep -Eq '^allowPrivateSinks:[[:space:]]*true[[:space:]]*$' "${HERO_VALUES}" ||
  fail "hero-values.yaml must set allowPrivateSinks: true (Forgejo ClusterIP)"
grep -Eq 'hero-values\.yaml' "${LIB}" ||
  fail "lib.sh HERO_DEV_VALUES must point at hero-values.yaml"
pass "hero values enable allowPrivateSinks"

printf 'demo-03 hero smoke: ok\n'
