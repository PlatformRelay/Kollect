#!/usr/bin/env bash
# LAB-DEKIND: substrate allowlist + image-delivery policy meta-tests (offline).
# The allowlist is the safety gate that keeps lab tooling off production clusters.
# Every assertion here is a regression guard for that gate — default-deny must hold.
# SPDX-License-Identifier: MIT
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LIB="${ROOT}/hack/lab/lib/substrate.sh"
CONF="${ROOT}/hack/lab/substrates.conf"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/lab-substrate-meta.XXXXXX")"
trap 'rm -rf "${TMP}"' EXIT

fail() {
  printf 'lab substrate meta: %s\n' "$*" >&2
  exit 1
}
pass() { printf 'ok - %s\n' "$*"; }

[[ -f "${LIB}" ]] || fail "missing library: ${LIB}"
[[ -f "${CONF}" ]] || fail "missing checked-in allowlist: ${CONF}"

# A production context that must NEVER be admitted (shape of the operator's ambient context).
PROD_CTX='gke_acme-platform-4711_europe-west1_shared-cluster'

# Poison PATH: the allowlist must decide without talking to any cluster.
cat >"${TMP}/kubectl" <<'EOF'
#!/usr/bin/env bash
echo "lab substrate meta: unexpected live kubectl: $*" >&2
exit 99
EOF
chmod +x "${TMP}/kubectl"
export PATH="${TMP}:${PATH}"

# DECOY CWD. The allowlist parser must never let the shell expand a pattern against the
# filesystem: `for item in ${extra//,/ }` performs PATHNAME expansion, so from a directory
# holding files named after refused contexts, `KOLLECT_LAB_ALLOWED_CONTEXTS='*'` would be
# rewritten into those names before validation ever ran — the allowlist fails OPEN.
# Every `sub` call therefore runs from a directory seeded with exactly the names that must
# stay refused, so a reintroduced glob turns this suite RED instead of green. (An *empty*
# CWD would hide the bug: an unmatched `*` stays literal and is correctly rejected.)
DECOY="${TMP}/decoy"
mkdir -p "${DECOY}"
: >"${DECOY}/${PROD_CTX}"
: >"${DECOY}/kumulus-lab-prod"
: >"${DECOY}/gke-prod-example"
: >"${DECOY}/alpha"
: >"${DECOY}/bravo"
: >"${DECOY}/ninechars"
mkdir -p "${DECOY}/art"

# Run one expression against a freshly sourced library (env overrides passed as VAR=VAL).
sub() {
  local expr="$1"
  shift
  env -u KUBECONFIG "$@" bash -c '
    set -uo pipefail
    cd "$1" || exit 1
    source "$2"
    shift 2
    eval "$*"
  ' _ "${DECOY}" "${LIB}" "${expr}"
}

# --- checked-in allowlist admits the two known lab substrates ---
out="$(sub 'lab_substrate_resolve kind-kollect-e2e' 2>&1)" ||
  fail "kind-* context must be allowlisted: ${out}"
[[ "${out}" == "kind" ]] || fail "kind-* context must resolve to substrate 'kind', got: ${out}"
pass "kind-* context allowlisted as substrate kind"

out="$(sub 'lab_substrate_resolve kumulus-lab' 2>&1)" ||
  fail "kumulus-lab must be allowlisted: ${out}"
[[ "${out}" == "talos" ]] || fail "kumulus-lab must resolve to substrate 'talos', got: ${out}"
pass "kumulus-lab context allowlisted as substrate talos"

# --- default-deny: exact match, not prefix ---
rc=0
out="$(sub "lab_substrate_resolve kumulus-lab-prod" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "kumulus-lab-prod must be REFUSED (allowlist is exact, not a prefix): ${out}"
pass "kumulus-lab-prod refused (exact match, not prefix)"

rc=0
out="$(sub "lab_substrate_resolve ${PROD_CTX}" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "production context must be REFUSED: ${out}"
pass "production-lookalike context refused"

rc=0
out="$(sub 'lab_substrate_resolve ""' 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "empty context must be refused: ${out}"
pass "empty context refused"

# --- assert_context returns 2 (refusal) and names the context + default-deny ---
rc=0
out="$(sub "lab_substrate_assert_context ${PROD_CTX}" 2>&1)" || rc=$?
[[ "${rc}" -eq 2 ]] || fail "assert_context must exit 2 on a non-allowlisted context, got ${rc}: ${out}"
printf '%s\n' "${out}" | grep -Eqi 'allowlist|refus' ||
  fail "refusal must mention the allowlist: ${out}"
printf '%s\n' "${out}" | grep -Fq "${PROD_CTX}" ||
  fail "refusal must name the refused context: ${out}"
pass "assert_context refuses production context with exit 2 and a named reason"

rc=0
out="$(sub 'lab_substrate_assert_context kumulus-lab --offline' 2>&1)" || rc=$?
[[ "${rc}" -eq 0 ]] || fail "assert_context must admit kumulus-lab, got ${rc}: ${out}"
pass "assert_context admits kumulus-lab"

# --- env additions: valid pattern extends, unsafe pattern fails closed ---
out="$(sub 'lab_substrate_resolve talos-scratch' KOLLECT_LAB_ALLOWED_CONTEXTS='talos-scratch=talos' 2>&1)" ||
  fail "valid env addition must extend the allowlist: ${out}"
[[ "${out}" == "talos" ]] || fail "env addition must carry its substrate kind, got: ${out}"
pass "KOLLECT_LAB_ALLOWED_CONTEXTS extends the allowlist"

for bad in '*' '**' '*-prod' 'k*' 'a' 'ctx name'; do
  rc=0
  out="$(sub "lab_substrate_resolve ${PROD_CTX}" KOLLECT_LAB_ALLOWED_CONTEXTS="${bad}" 2>&1)" || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "unsafe env pattern '${bad}' must not admit ${PROD_CTX}: ${out}"
done
pass "unsafe wildcard/short env patterns never admit a production context"

rc=0
out="$(sub 'lab_substrate_load' KOLLECT_LAB_ALLOWED_CONTEXTS='*' 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "wildcard-only env pattern must fail the load closed: ${out}"
# Assert the REASON, not just the exit code: a load that failed for an unrelated cause would
# otherwise look like a working safety gate.
printf '%s\n' "${out}" | grep -Eqi "refusing unsafe context pattern '\*'" ||
  fail "load failure must name the rejected pattern '*': ${out}"
pass "wildcard-only env pattern fails the load closed, naming the pattern"

# --- the allowlist parser must not perform PATHNAME EXPANSION on its input ---
# Runs from a CWD seeded with files named after contexts that must stay refused.
for glob in '*' '?????????' '[a-z]*' '*prod*' '*-prod'; do
  rc=0
  out="$(sub 'lab_substrate_load && lab_substrate_allowlist_summary' \
    KOLLECT_LAB_ALLOWED_CONTEXTS="${glob}" 2>&1)" || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "glob '${glob}' must fail the load closed, not expand against the filesystem: ${out}"
  for decoy in "${PROD_CTX}" kumulus-lab-prod gke-prod-example alpha bravo; do
    printf '%s\n' "${out}" | grep -Fq "${decoy}(" &&
      fail "glob '${glob}' expanded to filesystem entry '${decoy}' — allowlist failed OPEN: ${out}"
  done
done
pass "env patterns are never pathname-expanded against the working directory"

# The check above passes if EITHER layer holds (the `set -f` wrapper, or quoted expansions in
# the parser). Exercise the inner parser DIRECTLY, with globbing left on, so removing one
# layer cannot silently leave the other untested.
for glob in '*' '?????????' '[a-z]*'; do
  rc=0
  out="$(sub "set +f; _lab_substrate_load_impl && lab_substrate_allowlist_summary" \
    KOLLECT_LAB_ALLOWED_CONTEXTS="${glob}" 2>&1)" || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "parser itself must not glob '${glob}' even with pathname expansion enabled: ${out}"
  for decoy in "${PROD_CTX}" kumulus-lab-prod gke-prod-example alpha bravo; do
    printf '%s\n' "${out}" | grep -Fq "${decoy}(" &&
      fail "parser expanded '${glob}' to '${decoy}' with globbing on — the quoted-expansion fix is missing: ${out}"
  done
done
pass "the parser is glob-safe on its own, not only behind the noglob wrapper"

# Structural backstop: the loop must iterate a read-split array, never a bare expansion.
grep -Eq 'for item in "\$\{items\[@\]\}"' "${LIB}" ||
  fail "the KOLLECT_LAB_ALLOWED_CONTEXTS loop must iterate a quoted array"
# Code only — the explanatory comment above the loop quotes the dangerous form on purpose.
grep -Eq '^[^#]*for [a-z]+ in \$\{extra' "${LIB}" &&
  fail "unquoted expansion of KOLLECT_LAB_ALLOWED_CONTEXTS reintroduced (pathname expansion)"
pass "no unquoted expansion of the allowlist input remains"

for glob in '*' '?????????' '[a-z]*'; do
  for decoy in "${PROD_CTX}" kumulus-lab-prod gke-prod-example; do
    rc=0
    out="$(sub "lab_substrate_resolve ${decoy}" KOLLECT_LAB_ALLOWED_CONTEXTS="${glob}" 2>&1)" || rc=$?
    [[ "${rc}" -ne 0 ]] ||
      fail "glob '${glob}' admitted '${decoy}' via pathname expansion — allowlist failed OPEN"
  done
done
pass "no filesystem-derived context is ever admitted"

# Same for the file-override parser (its fields are read, never re-expanded): '[a-z]*' would
# expand to the decoy filenames if the parser globbed, and fails validation if it does not.
printf '%s\n' '[a-z]* generic' >"${DECOY}/glob.conf"
rc=0
out="$(sub "lab_substrate_resolve ${PROD_CTX}" KOLLECT_LAB_SUBSTRATES_FILE="${DECOY}/glob.conf" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "file-sourced entries must not expand into other filenames: ${out}"
pass "file-sourced patterns are not pathname-expanded either"

# --- file override is validated too (it is itself a bypass vector) ---
printf '%s\n' '* generic' >"${TMP}/wide.conf"
rc=0
out="$(sub "lab_substrate_resolve ${PROD_CTX}" KOLLECT_LAB_SUBSTRATES_FILE="${TMP}/wide.conf" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "wildcard-only file entry must not admit ${PROD_CTX}: ${out}"
pass "wildcard-only entry in an override file fails closed"

rc=0
out="$(sub "lab_substrate_resolve kind-x" KOLLECT_LAB_SUBSTRATES_FILE="${TMP}/missing.conf" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "missing allowlist file must fail closed: ${out}"
pass "missing allowlist file fails closed"

printf '# comment only\n\n' >"${TMP}/empty.conf"
rc=0
out="$(sub "lab_substrate_resolve kind-x" KOLLECT_LAB_SUBSTRATES_FILE="${TMP}/empty.conf" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "empty allowlist must admit nothing: ${out}"
pass "empty allowlist admits nothing"

# --- checked-in allowlist itself carries no over-broad entry ---
while IFS= read -r line; do
  line="${line%%#*}"
  read -r pat _rest <<<"${line}" || true
  [[ -n "${pat:-}" ]] || continue
  out="$(sub "lab_substrate_valid_pattern '${pat}' && echo VALID" 2>&1)" || true
  [[ "${out}" == "VALID" ]] ||
    fail "checked-in allowlist entry '${pat}' fails the pattern validator"
done <"${CONF}"
pass "every checked-in allowlist entry passes the pattern validator"

# --- image delivery policy: no silent stale image on a non-Kind substrate ---
out="$(sub 'lab_substrate_require_registry_image ghcr.io/platformrelay/kollect:v0.17.0 talos' 2>&1)" ||
  fail "pinned registry tag must be accepted: ${out}"
pass "pinned registry tag accepted for a non-Kind substrate"

# Digest references are refused, not recommended: the chart renders `repository:tag`
# (charts/kollect/templates/_helpers.tpl "kollect.image"), so kollect_helm_install cannot
# install one. Accepting a digest here while the install path rejects it two calls later
# would hand the operator advice that cannot work.
DIGEST='ghcr.io/platformrelay/kollect@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
rc=0
out="$(sub "lab_substrate_require_registry_image ${DIGEST} talos" 2>&1)" || rc=$?
[[ "${rc}" -ne 0 ]] || fail "digest-pinned image must be refused (the chart cannot render it): ${out}"
printf '%s\n' "${out}" | grep -Eqi 'digest' ||
  fail "digest refusal must say why: ${out}"
pass "digest-pinned image refused, matching what the chart can actually install"

# No hint text may recommend a form the install path rejects.
out="$(sub 'lab_substrate_require_registry_image kollect:dev talos' 2>&1 || true)"
printf '%s\n' "${out}" | grep -Fq '@sha256' &&
  fail "guidance must not recommend a digest the chart cannot render: ${out}"
pass "refusal guidance recommends only an installable form"

for bad_image in \
  'kollect-controller-manager:dev' \
  'kollect-controller-manager' \
  'ghcr.io/platformrelay/kollect:latest' \
  'ghcr.io/platformrelay/kollect:dev' \
  'ghcr.io/platformrelay/kollect'; do
  rc=0
  out="$(sub "lab_substrate_require_registry_image ${bad_image} talos" 2>&1)" || rc=$?
  [[ "${rc}" -ne 0 ]] ||
    fail "image '${bad_image}' must be refused on a non-Kind substrate"
  printf '%s\n' "${out}" | grep -Eqi 'registry|tag|pin' ||
    fail "refusal of '${bad_image}' must explain the registry/tag requirement: ${out}"
done
pass "unqualified / mutable-tag images refused loudly on a non-Kind substrate"

# Kind keeps side-loading: the policy must not break the CI path.
out="$(sub 'lab_substrate_image_delivery kind' 2>&1)" || fail "kind delivery lookup failed: ${out}"
[[ "${out}" == "sideload" ]] || fail "kind substrate must use sideload delivery, got: ${out}"
out="$(sub 'lab_substrate_image_delivery talos' 2>&1)" || fail "talos delivery lookup failed: ${out}"
[[ "${out}" == "registry" ]] || fail "non-kind substrate must use registry delivery, got: ${out}"
pass "image delivery mode is sideload on Kind and registry elsewhere"

echo "All lab substrate meta tests passed."
