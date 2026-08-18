#!/usr/bin/env bash
# DIST-OH-04: the OLM bundle must digest-pin a RUNNABLE controller image.
#
# Regression lock for the v0.18.0 OperatorHub submission, which pinned
# ghcr.io/platformrelay/kollect@sha256:cc63c330... in all three CSV image fields.
# That digest is the HELM CHART published at the bare "0.18.0" OCI tag -- the same
# OCI repository holds both artifact kinds (DR-FIND-07) -- so OLM pulled a chart,
# CRI-O reported CreateContainerError "image not known", and DeployableByOLM timed
# out after 240s. Nothing in the bundle path noticed: IMAGE_DIGEST was accepted as
# an opaque string.
#
# Two guards, deliberately split by network dependency:
#   * format  (offline) -- Makefile + script: sha256:<64 lowercase hex>.
#   * runnable (network) -- operatorhub-pr.sh only, after the DRY_RUN exit, so the
#     hermetic bundle test never reaches for a registry.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

LIB="${ROOT}/hack/lib/olm-image-digest.sh"
SCRIPT="${ROOT}/hack/operatorhub-pr.sh"

fail() {
  printf 'dist olm image digest: %s\n' "$*" >&2
  exit 1
}

pass() {
  printf 'ok - %s\n' "$*"
}

[[ -f "${LIB}" ]] || fail "${LIB} is missing"
[[ -f "${SCRIPT}" ]] || fail "${SCRIPT} is missing"

# shellcheck source=/dev/null
source "${LIB}"

# ---------------------------------------------------------------- format guard

for good in \
  "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  "sha256:9f116431941d1816bfdb22f574f3394c30cf41a348082e1028b2a16b5e0ccc5c"; do
  olm_digest_format_ok "${good}" ||
    fail "olm_digest_format_ok rejected a valid digest: ${good}"
done
pass "olm_digest_format_ok accepts well-formed sha256 digests"

# Uppercase hex is rejected on purpose: registries emit lowercase, and accepting
# both would let two spellings of one digest compare unequal downstream.
while IFS= read -r bad; do
  [[ -n "${bad}" ]] || continue
  if olm_digest_format_ok "${bad}"; then
    fail "olm_digest_format_ok accepted a malformed digest: '${bad}'"
  fi
done <<'BAD'
sha256:0123456789abcdef
0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
sha256:0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef
sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg
sha512:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
v0.18.0
sha256:
BAD
pass "olm_digest_format_ok rejects malformed digests (short, unprefixed, uppercase, non-hex, tag)"

# ------------------------------------------------------- runnable/allowlist gate

# olm_manifest_is_runnable reads a manifest JSON on stdin and must ALLOWLIST the
# runnable shapes rather than denylist the Helm one -- a denylist would wave through
# every future non-runnable artifact type (SBOM, provenance, WASM, ...).
assert_runnable() {
  printf '%s' "$2" | olm_manifest_is_runnable ||
    fail "olm_manifest_is_runnable rejected a runnable manifest: $1"
  pass "runnable accepted: $1"
}

assert_not_runnable() {
  if printf '%s' "$2" | olm_manifest_is_runnable; then
    fail "olm_manifest_is_runnable ACCEPTED a non-runnable manifest: $1"
  fi
  pass "non-runnable rejected: $1"
}

assert_runnable "OCI image index" \
  '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","platform":{"architecture":"amd64","os":"linux"}}]}'

assert_runnable "docker manifest list" \
  '{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json","manifests":[{"platform":{"architecture":"amd64","os":"linux"}}]}'

assert_runnable "OCI image manifest" \
  '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json"},"layers":[]}'

assert_runnable "docker v2 image manifest" \
  '{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json"},"layers":[]}'

# The exact artifact that broke v0.18.0.
assert_not_runnable "helm chart (the v0.18.0 regression)" \
  '{"schemaVersion":2,"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json"},"layers":[{"mediaType":"application/vnd.cncf.helm.chart.content.v1.tar+gzip"}]}'

# Allowlist proof: an artifact that is neither an index nor an image config must be
# refused even though it is not a Helm chart.
assert_not_runnable "unknown OCI artifact (allowlist proof)" \
  '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.example.sbom.v1+json"},"layers":[]}'

assert_not_runnable "manifest with no config and no index mediaType" \
  '{"schemaVersion":2,"layers":[]}'

assert_not_runnable "registry error document" \
  '{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}'

assert_not_runnable "empty input" ''

# --------------------------------------------- olm_assert_runnable_image (offline)

# olm_assert_runnable_image is the function that actually runs on the release path,
# so its fail-closed branches need a regression lock rather than a manual probe.
# Stub `curl` on PATH (same trick as hack/test/lab_runner_resume_meta_test.sh) and
# drive every branch offline: a registry outage must abort a submission, never wave
# an unverified digest through to a third-party repository.
STUB_DIR="$(mktemp -d)"
trap 'rm -rf "${STUB_DIR}"' EXIT
cat >"${STUB_DIR}/curl" <<'STUB'
#!/bin/sh
# Fake curl: last argument is the URL. Behaviour driven by STUB_* env vars.
#
# Every invocation is appended to $STUB_CALL_LOG so a test can assert that a
# rejection happened WITHOUT touching the network.
#
# The default bodies are written with an explicit if/else rather than
# "${VAR-{"token":"x"}}". Under dash (this is /bin/sh) parameter-expansion
# defaults terminate at the FIRST unescaped '}', so the brace-nested form
# silently emitted '{}}' for STUB_TOKEN_BODY='{}' -- invalid JSON, which sent
# the empty-token assertion down the jq-parse branch instead of the branch it
# claimed to cover.
for a in "$@"; do url="$a"; done
[ -n "${STUB_CALL_LOG:-}" ] && echo "${url}" >>"${STUB_CALL_LOG}"
case "${url}" in
*"/token?"*)
  [ "${STUB_TOKEN_FAIL:-0}" = "1" ] && exit 22
  if [ -n "${STUB_TOKEN_BODY+x}" ]; then printf '%s' "${STUB_TOKEN_BODY}"; else printf '%s' '{"token":"stub-token"}'; fi
  exit 0
  ;;
*"/manifests/"*)
  [ "${STUB_MANIFEST_FAIL:-0}" = "1" ] && exit 22
  printf '%s' "${STUB_MANIFEST_BODY-}"
  exit 0
  ;;
esac
exit 0
STUB
chmod +x "${STUB_DIR}/curl"

VALID_DIGEST="sha256:9f116431941d1816bfdb22f574f3394c30cf41a348082e1028b2a16b5e0ccc5c"
REAL_INDEX='{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","platform":{"architecture":"amd64","os":"linux"}}]}'
HELM_MANIFEST='{"schemaVersion":2,"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json"},"layers":[]}'

# Runs olm_assert_runnable_image with the stub on PATH, leaving the exit status in
# ASSERT_RC and the combined output in ASSERT_OUT. Deliberately NOT called inside a
# command substitution -- that runs the function in a subshell, where the assignment
# to ASSERT_RC is discarded and every "must be rejected" assertion reads an empty
# status and silently passes.
run_assert() {
  : >"${STUB_DIR}/calls"
  set +e
  PATH="${STUB_DIR}:${PATH}" STUB_CALL_LOG="${STUB_DIR}/calls" \
    olm_assert_runnable_image "$1" "$2" >"${STUB_DIR}/out" 2>&1
  ASSERT_RC=$?
  set -e
  ASSERT_OUT="$(cat "${STUB_DIR}/out")"
}

# Happy path: a real multi-arch index is accepted.
STUB_MANIFEST_BODY="${REAL_INDEX}" run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -eq 0 ]] ||
  fail "olm_assert_runnable_image rejected a valid image index: ${out}"
pass "olm_assert_runnable_image accepts a multi-arch image index"

# The v0.18.0 regression, offline: a Helm chart must be refused AND named.
STUB_MANIFEST_BODY="${HELM_MANIFEST}" run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] ||
  fail "olm_assert_runnable_image ACCEPTED a Helm chart manifest"
[[ "${out}" == *"HELM CHART"* ]] ||
  fail "the Helm-chart rejection must name the artifact, got: ${out}"
pass "olm_assert_runnable_image rejects a Helm chart and names it"

# Fail-closed branches. Each must abort rather than assume the digest is fine.
run_assert ghcr.io/platformrelay/kollect "latest"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a malformed digest must be rejected before any fetch"
[[ ! -s "${STUB_DIR}/calls" ]] ||
  fail "a malformed digest must be rejected BEFORE any registry call, but curl ran: $(cat "${STUB_DIR}/calls")"
pass "olm_assert_runnable_image rejects a malformed digest without touching the network"

STUB_MANIFEST_BODY="${REAL_INDEX}" run_assert quay.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a non-ghcr.io registry must be rejected, not silently trusted"
[[ "${out}" == *"ghcr.io"* ]] || fail "the non-ghcr.io rejection should say which registry it understands"
pass "olm_assert_runnable_image rejects a registry it cannot verify"

# The two token branches are distinguished by MESSAGE. Asserting only rc != 0 made
# them interchangeable, so a stub bug quietly pointed the "empty token" case at the
# jq-parse branch and the real branch kept zero coverage.
STUB_TOKEN_FAIL=1 STUB_MANIFEST_BODY="${REAL_INDEX}" run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a failed token request must abort the submission"
[[ "${out}" == *"could not obtain a GHCR pull token"* ]] ||
  fail "a failed token request must be reported as such, got: ${out}"
pass "olm_assert_runnable_image fails closed when the token request fails"

STUB_TOKEN_BODY='{}' STUB_MANIFEST_BODY="${REAL_INDEX}" run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "an empty token must abort the submission"
[[ "${out}" == *"empty pull token"* ]] ||
  fail "an empty token must hit the empty-token branch (not jq-parse or fetch-failure), got: ${out}"
pass "olm_assert_runnable_image fails closed on an empty token"

STUB_MANIFEST_FAIL=1 run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a failed manifest fetch must abort the submission"
pass "olm_assert_runnable_image fails closed when the manifest cannot be fetched"

STUB_MANIFEST_BODY='<html>502 Bad Gateway</html>' run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a non-JSON body must abort the submission"
pass "olm_assert_runnable_image fails closed on a non-JSON response"

STUB_MANIFEST_BODY='{"errors":[{"code":"MANIFEST_UNKNOWN"}]}' run_assert ghcr.io/platformrelay/kollect "${VALID_DIGEST}"
out="${ASSERT_OUT}"
[[ "${ASSERT_RC}" -ne 0 ]] || fail "a registry error document must abort the submission"
[[ "${out}" == *"MANIFEST_UNKNOWN"* ]] || fail "a registry error should surface its code, got: ${out}"
pass "olm_assert_runnable_image fails closed on a registry error document"

# ------------------------------------------------------------ failure messages

# The rejection message must NAME the artifact. Regression lock: the first cut used
# `.errors // empty` in the jq describe expression, and because `if empty then ... end`
# emits an empty stream, every rejection printed "it is ." -- technically a failure,
# but one that told the operator nothing about what was actually pinned.
describe_of() { printf '%s' "$1" | olm_describe_manifest; }

got="$(describe_of '{"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json"}}')"
[[ "${got}" == *"HELM CHART"* ]] ||
  fail "a Helm chart must be described as such, got: '${got}'"

got="$(describe_of '{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}')"
[[ "${got}" == *"MANIFEST_UNKNOWN"* ]] ||
  fail "a registry error must surface its code, got: '${got}'"

got="$(describe_of '{"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.example.sbom.v1+json"}}')"
[[ "${got}" == *"vnd.example.sbom"* ]] ||
  fail "an unknown artifact must surface its config mediaType, got: '${got}'"

# No input shape may yield an empty description.
while IFS= read -r fixture; do
  [[ -n "${fixture}" ]] || continue
  got="$(describe_of "${fixture}")"
  [[ -n "${got}" ]] || fail "olm_describe_manifest produced an empty description for: ${fixture}"
done <<'FIXTURES'
{"config":{"mediaType":"application/vnd.cncf.helm.config.v1+json"}}
{"errors":[{"code":"MANIFEST_UNKNOWN"}]}
{"schemaVersion":2,"layers":[]}
{"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}
not json at all
FIXTURES
pass "olm_describe_manifest always names the offending artifact"

# ------------------------------------------------------------- Makefile wiring

grep -Fq 'olm_digest_format_ok' "${ROOT}/Makefile" ||
  grep -Fq 'IMAGE_DIGEST must be' "${ROOT}/Makefile" ||
  fail "Makefile generate-olm-bundle must reject a malformed IMAGE_DIGEST"

# The Makefile guard must stay OFFLINE: hack/test/dist_olm_bundle_test.sh runs
# generate-olm-bundle with a synthetic digest that exists in no registry, so a
# round-trip there turns a hermetic test into a network-flaky one.
#
# Scope this to the RECIPE BODY. An earlier cut piped the whole Makefile through two
# greps and required a single line to hold both a registry tool and the target name --
# which no recipe line ever does, so the assertion was dead and an injected
# `crane digest ...` sailed through it. Extract the recipe, drop `echo` lines (the
# failure message legitimately names `docker buildx imagetools` as the fix), then look
# for a registry client.
OLM_RECIPE_RAW="$(sed -n '/^generate-olm-bundle:/,/^[^	]/p' "${ROOT}/Makefile")"
[[ -n "${OLM_RECIPE_RAW}" ]] ||
  fail "could not extract the generate-olm-bundle recipe from the Makefile"
printf '%s\n' "${OLM_RECIPE_RAW}" | grep -Fq 'IMAGE_DIGEST' ||
  fail "extracted the wrong block: the generate-olm-bundle recipe must reference IMAGE_DIGEST"

# Strip DOUBLE-QUOTED STRINGS rather than dropping whole `echo` lines. The failure
# message legitimately names `crane` inside a quoted string, which is why filtering is
# needed at all -- but dropping the line lost everything after it, and this recipe is
# one long `echo ... && \` chain, so
#   @echo "resolving digest" && curl -sfL https://ghcr.io/v2/ && \
# evaded the guard entirely. Removing only the quoted spans keeps the executable part
# of most lines under inspection.
#
# This trades one blind spot for a smaller one: a command hidden inside `sh -c "..."`
# now lives in a stripped span and is missed, where raw-text matching would have caught
# it. That is the right trade here -- an `@echo` in a recipe that is one long
# `echo ... && \` chain is a shape this Makefile actually uses, whereas `sh -c` is not --
# but it is a trade, not full coverage. The leading character class deliberately does NOT
# exclude `/` or `.`, so path-qualified calls (`./bin/crane`, `/usr/bin/curl`) still match.
OLM_RECIPE="$(printf '%s\n' "${OLM_RECIPE_RAW}" | sed 's/"[^"]*"//g')"
if printf '%s\n' "${OLM_RECIPE}" |
  grep -Eq '(^|[^[:alnum:]_-])(curl|wget|oras|crane|skopeo|regctl|podman|nerdctl|cosign)([^[:alnum:]_-]|$)|(docker|helm)[[:space:]]+(pull|push|manifest|buildx)'; then
  fail "generate-olm-bundle must not perform a registry round-trip (keeps the bundle test hermetic)"
fi
pass "Makefile generate-olm-bundle has an offline IMAGE_DIGEST format guard and no registry client"

# A malformed digest must actually fail the target.
if make generate-olm-bundle VERSION=9.9.9-badtest IMAGE_DIGEST=latest >/dev/null 2>&1; then
  rm -rf "${ROOT}/dist/olm-bundle/9.9.9-badtest"
  fail "generate-olm-bundle accepted IMAGE_DIGEST=latest"
fi
rm -rf "${ROOT}/dist/olm-bundle/9.9.9-badtest"
pass "generate-olm-bundle rejects IMAGE_DIGEST=latest"

# ------------------------------------------------------- operatorhub-pr wiring

CODE="$(grep -v '^[[:space:]]*#' "${SCRIPT}")"

printf '%s\n' "${CODE}" | grep -Fq 'olm-image-digest.sh' ||
  fail "operatorhub-pr.sh must source hack/lib/olm-image-digest.sh"
printf '%s\n' "${CODE}" | grep -Fq 'olm_assert_runnable_image' ||
  fail "operatorhub-pr.sh must call olm_assert_runnable_image before submitting"

# The registry check must run only on the real submission path. If it ran before the
# DRY_RUN exit, the offline meta-tests (which pass a synthetic digest) would start
# requiring network and fail closed on an artifact that was never meant to exist.
dry_line="$(printf '%s\n' "${CODE}" | grep -n 'DRY_RUN:-0' | head -1 | cut -d: -f1)"
runnable_line="$(printf '%s\n' "${CODE}" | grep -n 'olm_assert_runnable_image' | head -1 | cut -d: -f1)"
[[ -n "${dry_line}" && -n "${runnable_line}" ]] ||
  fail "could not locate the DRY_RUN exit and the runnable-image assertion in operatorhub-pr.sh"
[[ "${runnable_line}" -gt "${dry_line}" ]] ||
  fail "olm_assert_runnable_image (line ${runnable_line}) must run AFTER the DRY_RUN exit (line ${dry_line})"
pass "operatorhub-pr.sh gates submission on a runnable image, after the DRY_RUN exit"

echo "All dist OLM image digest tests passed."
