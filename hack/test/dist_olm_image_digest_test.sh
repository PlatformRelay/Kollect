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
# generate-olm-bundle with a synthetic digest and must not need a registry.
if grep -nE '^[^#]*(curl|oras|crane|skopeo|docker (manifest|buildx imagetools))' "${ROOT}/Makefile" |
  grep -Fq 'generate-olm-bundle'; then
  fail "generate-olm-bundle must not perform a registry round-trip (keeps the bundle test hermetic)"
fi
pass "Makefile generate-olm-bundle has an offline IMAGE_DIGEST format guard"

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
