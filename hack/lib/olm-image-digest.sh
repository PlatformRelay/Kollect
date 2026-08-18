#!/usr/bin/env bash
# DIST-OH-04: helpers that keep the OLM bundle pinned to a RUNNABLE controller image.
#
# Context (DR-FIND-07): ONE GHCR repository, ghcr.io/platformrelay/kollect, holds two
# artifact kinds -- the Helm chart at bare semver tags (0.18.0) and the multi-arch
# controller image at v-prefixed tags (v0.18.0). A digest resolved from the wrong tag
# is still a perfectly valid digest, so every string-level check passes and the error
# only surfaces on an OpenShift cluster as CreateContainerError "image not known",
# roughly 30 minutes into the upstream hosted pipeline. These helpers move that failure
# to the moment the bundle is built or submitted.
#
# Sourced by hack/operatorhub-pr.sh and unit-tested by
# hack/test/dist_olm_image_digest_test.sh.

# olm_digest_format_ok <digest>
#
# True when <digest> is a canonical registry digest: sha256:<64 lowercase hex>.
# Uppercase hex is rejected deliberately -- registries emit lowercase, and tolerating
# both spellings would let one digest compare unequal to itself downstream.
# Pure string check, no network: safe for hermetic callers.
olm_digest_format_ok() {
  [[ "${1:-}" =~ ^sha256:[0-9a-f]{64}$ ]]
}

# olm_manifest_is_runnable  (manifest JSON on stdin)
#
# True when the manifest describes something a kubelet can actually start.
#
# This is an ALLOWLIST over the manifest's CONFIG mediaType, not a Helm denylist.
# Rejecting only application/vnd.cncf.helm.config.v1+json would wave through every
# other artifact that declares its own config type -- Helm charts today, WASM modules
# or a future packaging format tomorrow -- each failing the same way and just as late.
#
# Accepted:
#   * an image index / manifest list carrying at least one entry, or
#   * a single manifest whose CONFIG is a container image config.
#
# KNOWN LIMITATION -- this is a config-mediaType check, NOT an artifact-kind check.
# Cosign signatures and cosign-attached in-toto attestations reuse the ordinary OCI
# image config mediaType and differ only in their LAYER types, so they are accepted
# here. The release workflow pushes exactly such artifacts into this same GHCR repo.
# They are not reachable by the mistake this guard exists to catch: cosign publishes
# them under sha256-<hex> tags, never under a bare semver tag someone
# might paste. Widening the check to inspect layer types would buy nothing for that
# failure mode; the digest formats simply do not collide.
olm_manifest_is_runnable() {
  local json media_type config_type entries
  json="$(cat)"
  [[ -n "${json}" ]] || return 1

  command -v jq >/dev/null 2>&1 || {
    printf 'olm_manifest_is_runnable: jq is required to inspect the manifest\n' >&2
    return 1
  }
  printf '%s' "${json}" | jq -e . >/dev/null 2>&1 || return 1

  media_type="$(printf '%s' "${json}" | jq -r '.mediaType // ""')"
  case "${media_type}" in
  application/vnd.oci.image.index.v1+json | application/vnd.docker.distribution.manifest.list.v2+json)
    # An index with an empty manifests[] resolves to nothing at pull time.
    entries="$(printf '%s' "${json}" | jq -r '(.manifests // []) | length')"
    [[ "${entries}" -gt 0 ]] && return 0
    return 1
    ;;
  esac

  config_type="$(printf '%s' "${json}" | jq -r '.config.mediaType // ""')"
  case "${config_type}" in
  application/vnd.oci.image.config.v1+json | application/vnd.docker.container.image.v1+json)
    return 0
    ;;
  esac

  return 1
}

# olm_describe_manifest  (manifest JSON on stdin)
#
# Best-effort one-line description used in the failure message, so an operator sees
# WHAT was pinned ("a Helm chart") instead of only that the check failed.
olm_describe_manifest() {
  local json
  json="$(cat)"
  command -v jq >/dev/null 2>&1 || {
    printf 'unparseable manifest\n'
    return 0
  }
  printf '%s' "${json}" | jq -e . >/dev/null 2>&1 || {
    printf 'not valid JSON\n'
    return 0
  }
  printf '%s' "${json}" | jq -r '
    if ((.errors // null) != null) then "registry error: " + ((.errors[0].code // "unknown"))
    elif (.config.mediaType // "") == "application/vnd.cncf.helm.config.v1+json" then "a HELM CHART (bare semver tags hold the chart; the controller image lives at v<version>)"
    elif (.mediaType // "") != "" then "mediaType=" + .mediaType + (if (.config.mediaType // "") != "" then ", config=" + .config.mediaType else "" end)
    elif (.config.mediaType // "") != "" then "config=" + .config.mediaType
    else "an unrecognised OCI artifact"
    end'
}

# olm_assert_runnable_image <image-repo> <digest>
#
# Fetches <image-repo>@<digest> from the registry and hard-fails unless it is a
# runnable container image. Requires network -- callers must invoke it only on the
# real submission path, never from the hermetic bundle tests.
#
# Fails CLOSED: an unreachable registry, a missing token, or an unparseable body all
# abort the submission rather than letting an unverified digest reach a third-party repo.
olm_assert_runnable_image() {
  local image_repo="${1:?olm_assert_runnable_image: image repo required}"
  local digest="${2:?olm_assert_runnable_image: digest required}"
  local registry path token manifest description

  olm_digest_format_ok "${digest}" || {
    printf 'ERROR: IMAGE_DIGEST %s is not a canonical sha256:<64 hex> digest.\n' "${digest}" >&2
    return 1
  }

  registry="${image_repo%%/*}"
  path="${image_repo#*/}"
  if [[ "${registry}" != "ghcr.io" ]]; then
    printf 'ERROR: olm_assert_runnable_image only knows how to query ghcr.io (got %s).\n' "${registry}" >&2
    return 1
  fi

  token="$(curl -fsS "https://ghcr.io/token?scope=repository:${path}:pull&service=ghcr.io" |
    jq -r '.token // empty')" || {
    printf 'ERROR: could not obtain a GHCR pull token for %s.\n' "${image_repo}" >&2
    return 1
  }
  [[ -n "${token}" ]] || {
    printf 'ERROR: GHCR returned an empty pull token for %s.\n' "${image_repo}" >&2
    return 1
  }

  manifest="$(curl -fsS \
    -H "Authorization: Bearer ${token}" \
    -H 'Accept: application/vnd.oci.image.index.v1+json' \
    -H 'Accept: application/vnd.docker.distribution.manifest.list.v2+json' \
    -H 'Accept: application/vnd.oci.image.manifest.v1+json' \
    -H 'Accept: application/vnd.docker.distribution.manifest.v2+json' \
    "https://ghcr.io/v2/${path}/manifests/${digest}")" || {
    printf 'ERROR: %s@%s could not be fetched from GHCR.\n' "${image_repo}" "${digest}" >&2
    return 1
  }

  if printf '%s' "${manifest}" | olm_manifest_is_runnable; then
    return 0
  fi

  description="$(printf '%s' "${manifest}" | olm_describe_manifest)"
  cat >&2 <<ERR
ERROR: ${image_repo}@${digest} is not a runnable container image -- it is ${description}.

  OLM would pull this artifact and fail with CreateContainerError "image not known",
  which surfaces only as a DeployableByOLM timeout ~30 minutes into the upstream
  hosted pipeline (this is exactly what happened to the v0.18.0 submission).

  ghcr.io/platformrelay/kollect holds BOTH the Helm chart (bare tag, e.g. 0.18.0)
  and the controller image (v-prefixed tag, e.g. v0.18.0). Resolve IMAGE_DIGEST from
  the V-PREFIXED tag:

    crane digest ghcr.io/platformrelay/kollect:v<version>
    # or: docker buildx imagetools inspect ghcr.io/platformrelay/kollect:v<version> --format '{{.Manifest.Digest}}'

  or take it straight from the release workflow's build step output.
ERR
  return 1
}
