#!/usr/bin/env bash
# Create or update PRs to OperatorHub repos for a new Kollect release.
# Submits to both k8s-operatorhub/community-operators (operatorhub.io)
# and redhat-openshift-ecosystem/community-operators-prod (OpenShift catalog).
#
# Usage: VERSION=0.17.0 IMAGE_DIGEST=sha256:... GH_TOKEN=<token> hack/operatorhub-pr.sh
#
# Required env vars:
#   VERSION       - Release version without 'v' prefix (e.g., 0.17.0)
#   IMAGE_DIGEST  - Controller image digest (e.g., sha256:abc...)
#   GH_TOKEN      - PAT with public_repo scope for fork push and upstream PRs
#
# Optional env vars:
#   FORK_OWNER      - GitHub org owning the forks (default: platformrelay)
#   DRY_RUN         - Set to 1 to validate bundle and print actions without cloning
#   GIT_USER_NAME   - Git commit author name (default: github-actions[bot])
#   GIT_USER_EMAIL  - Git commit author email (default: 41898282+github-actions[bot]@users.noreply.github.com)
#   BUNDLE_DIR      - Pre-generated bundle path (default: generates via make)

set -Eeuo pipefail

: "${VERSION:?VERSION is required (e.g., 0.17.0)}"
: "${IMAGE_DIGEST:?IMAGE_DIGEST is required (e.g., sha256:abc...)}"

FORK_OWNER="${FORK_OWNER:-platformrelay}"
GIT_USER_NAME="${GIT_USER_NAME:-github-actions[bot]}"
GIT_USER_EMAIL="${GIT_USER_EMAIL:-41898282+github-actions[bot]@users.noreply.github.com}"
BRANCH="kollect-v${VERSION}"
OPERATOR_DIR="operators/kollect"

CHECKOUT_DIR="$(pwd)"
CLEANUP_DIRS=()
cleanup() { for d in "${CLEANUP_DIRS[@]}"; do rm -rf "$d"; done; }
trap cleanup EXIT

BUNDLE_DIR="${BUNDLE_DIR:-dist/olm-bundle/${VERSION}}"
if [[ ! -d "${BUNDLE_DIR}/manifests" ]]; then
  echo "Generating OLM bundle..."
  make generate-olm-bundle VERSION="${VERSION}" IMAGE_DIGEST="${IMAGE_DIGEST}"
fi

for f in \
  "${BUNDLE_DIR}/manifests/kollect.clusterserviceversion.yaml" \
  "${BUNDLE_DIR}/metadata/annotations.yaml"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: Missing bundle file: $f" >&2
    exit 1
  fi
done

while IFS= read -r crd_file; do
  base="$(basename "${crd_file}")"
  if [[ ! -f "${BUNDLE_DIR}/manifests/${base}" ]]; then
    echo "ERROR: Missing bundle CRD: ${BUNDLE_DIR}/manifests/${base}" >&2
    exit 1
  fi
done < <(find config/crd/bases -name 'kollect.dev_*.yaml' | sort)

echo "Bundle verified: ${BUNDLE_DIR}"

if [[ "${DRY_RUN:-0}" == "1" ]]; then
  echo "DRY_RUN=1: would submit ${BUNDLE_DIR} to k8s-operatorhub/community-operators and redhat-openshift-ecosystem/community-operators-prod as ${BRANCH}"
  exit 0
fi

: "${GH_TOKEN:?GH_TOKEN is required unless DRY_RUN=1}"

submit_bundle() {
  local upstream_repo="$1"
  local openshift_versions="${2:-}"
  local repo_name="${upstream_repo##*/}"
  local fork_repo="${FORK_OWNER}/${repo_name}"

  echo ""
  echo "=== Submitting to ${upstream_repo} ==="

  local work_dir
  work_dir=$(mktemp -d)
  CLEANUP_DIRS+=("${work_dir}")

  echo "Cloning fork ${fork_repo}..."
  git clone --depth=1 \
    "https://x-access-token:${GH_TOKEN}@github.com/${fork_repo}.git" \
    "${work_dir}/${repo_name}"
  cd "${work_dir}/${repo_name}"

  git config user.name "${GIT_USER_NAME}"
  git config user.email "${GIT_USER_EMAIL}"

  git remote add upstream "https://github.com/${upstream_repo}.git"
  git fetch upstream main --depth=1
  git checkout -B "${BRANCH}" upstream/main

  mkdir -p "${OPERATOR_DIR}/${VERSION}/manifests" "${OPERATOR_DIR}/${VERSION}/metadata"
  cp "${CHECKOUT_DIR}/${BUNDLE_DIR}/manifests/"* "${OPERATOR_DIR}/${VERSION}/manifests/"
  cp "${CHECKOUT_DIR}/${BUNDLE_DIR}/metadata/"* "${OPERATOR_DIR}/${VERSION}/metadata/"

  if [[ -n "${openshift_versions}" ]]; then
    sed -i '/^annotations:/a\  com.redhat.openshift.versions: "'"${openshift_versions}"'"' \
      "${OPERATOR_DIR}/${VERSION}/metadata/annotations.yaml"
  fi

  cat > "${OPERATOR_DIR}/ci.yaml" <<'CIEOF'
updateGraph: semver-mode
reviewers:
  - platformrelay
CIEOF

  git add "${OPERATOR_DIR}/"
  git commit -s -m "operator kollect (${VERSION})"
  git push --force origin "${BRANCH}"
  echo "Pushed branch ${BRANCH} to ${fork_repo}"

  local pr_tag="[U]"
  if ! git ls-tree --name-only upstream/main -- "${OPERATOR_DIR}" | grep -q .; then
    pr_tag="[N]"
  fi

  local pr_title="operator ${pr_tag} [CI] kollect (${VERSION})"
  local pr_body
  pr_body="### New Submission

**Operator:** kollect
**Version:** ${VERSION}

Update Kollect operator to version ${VERSION}.

See [release notes](https://github.com/platformrelay/kollect/releases/tag/v${VERSION}) for changes.

---
*This PR was automatically created by the Kollect release workflow.*"

  local existing_pr
  existing_pr=$(GH_TOKEN="${GH_TOKEN}" gh pr list \
    --repo "${upstream_repo}" \
    --head "${FORK_OWNER}:${BRANCH}" \
    --state open \
    --json number \
    --jq '.[0].number // empty' 2>/dev/null || true)

  if [[ -n "${existing_pr}" ]]; then
    echo "Updating existing PR #${existing_pr}"
    GH_TOKEN="${GH_TOKEN}" gh pr edit "${existing_pr}" \
      --repo "${upstream_repo}" \
      --title "${pr_title}" \
      --body "${pr_body}"
    echo "PR updated: https://github.com/${upstream_repo}/pull/${existing_pr}"
  else
    echo "Creating new PR..."
    local pr_url
    pr_url=$(GH_TOKEN="${GH_TOKEN}" gh pr create \
      --repo "${upstream_repo}" \
      --head "${FORK_OWNER}:${BRANCH}" \
      --base main \
      --title "${pr_title}" \
      --body "${pr_body}")
    echo "PR created: ${pr_url}"
  fi

  cd "${CHECKOUT_DIR}"
}

submit_bundle "k8s-operatorhub/community-operators"
submit_bundle "redhat-openshift-ecosystem/community-operators-prod" "v4.19"
