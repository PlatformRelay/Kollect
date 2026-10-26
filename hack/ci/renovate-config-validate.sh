#!/usr/bin/env bash
# renovate-config-validate.sh [root] -- validate renovate.json (repo config) and
# .github/renovate-config.json (global config) with renovate-config-validator from the
# digest-pinned renovate-image of .github/workflows/renovate.yaml (openspec change
# dependency-update-automation, DUA-5). Runs in the required lint job of ci.yaml; guarded by
# hack/test/renovate_policy_test.sh (T8 wiring, T16 behaviour).
#
# - The image is the renovate-image input of the renovatebot/github-action step, so the
#   validator is the Renovate that runs. An image without a version and a digest is refused
#   before docker is called: a floating tag would validate against a Renovate no one reviewed.
# - Each file is validated in its own mode: renovate.json with --strict --no-global (repo
#   config; a file named on the command line is global config unless --no-global is given),
#   .github/renovate-config.json with --strict (global config). --strict also fails a config
#   that needs migration.
# - The container gets no network (the image pull is the daemon's, before it starts), a
#   read-only mount of the root at /repo, and runs as the caller's uid:gid, so the files are
#   readable whatever the checkout's mode (the image's own user, uid 12021, cannot read a
#   mode-600 file and would fail closed).
# - Any non-zero exit of either call fails the script (1 for findings, 99 for a CLI error).
#
# Needs bash, yq (mikefarah v4) and docker.
set -euo pipefail

root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
workflow="${root}/.github/workflows/renovate.yaml"

if [[ ! -f "${workflow}" ]]; then
  echo "renovate-config-validate: ${workflow} not found" >&2
  exit 1
fi

image="$(yq eval '.jobs[].steps[] | select((.uses // "") | test("^renovatebot/github-action@")) | .with["renovate-image"] // ""' "${workflow}")"
if [[ ! "${image}" =~ ^ghcr\.io/renovatebot/renovate:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}$ ]]; then
  echo "renovate-config-validate: the renovate-image of ${workflow} is '${image}', expected one ghcr.io/renovatebot/renovate:<X.Y.Z>@sha256:<64 hex> -- refusing to validate with an image that is not pinned by digest" >&2
  exit 1
fi

validate() {
  echo "renovate-config-validate: renovate-config-validator $* (${image})"
  docker run --rm --network none --user "$(id -u):$(id -g)" -v "${root}:/repo:ro" -w /repo \
    --entrypoint renovate-config-validator "${image}" "$@"
}

rc=0
validate --strict --no-global renovate.json || rc=1
validate --strict .github/renovate-config.json || rc=1
if [[ "${rc}" != 0 ]]; then
  echo "renovate-config-validate: FAILED -- fix the findings above (a migration the validator asks for included)" >&2
fi
exit "${rc}"
