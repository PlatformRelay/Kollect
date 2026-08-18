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
#   GH_TOKEN      - classic PAT for fork push and upstream PRs. Scopes: `public_repo`
#                   (clone/push the public forks, open + edit upstream PRs) AND
#                   `workflow` — the submission branch is cut from upstream/main, which
#                   carries .github/workflows/*, and GitHub rejects a PAT push that
#                   introduces workflow files without it once the fork drifts behind.
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
    # Portable insert-after. GNU and BSD/macOS sed disagree on BOTH `-i` (BSD reads the
    # next argument as the backup suffix) and the one-line `a\text` form, so the previous
    # `sed -i` worked on the CI runner and died locally with "invalid command code".
    # awk behaves identically on both, which matters because docs/RELEASE.md and the
    # workflow's own failure warning both tell operators to re-run this script by hand.
    local annotations_file="${OPERATOR_DIR}/${VERSION}/metadata/annotations.yaml"
    local tmp_annotations
    tmp_annotations="$(mktemp)"
    awk -v line="  com.redhat.openshift.versions: \"${openshift_versions}\"" \
      '{ print } /^annotations:/ { print line }' \
      "${annotations_file}" >"${tmp_annotations}"
    mv "${tmp_annotations}" "${annotations_file}"
  fi

  # `reviewers` takes GitHub USERNAMES, not orgs. The PR author (the OPERATORHUB_PAT
  # owner) must appear here on the UPSTREAM default branch for the pipeline to set
  # `authorized-changes` and self-merge later version bumps; an org never resolves.
  cat > "${OPERATOR_DIR}/ci.yaml" <<'CIEOF'
updateGraph: semver-mode
reviewers:
  - konih
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

  # Look the PR up by branch name and match the owner case-INSENSITIVELY. GitHub stores
  # the canonical org casing ("PlatformRelay"), so the old `--head "${FORK_OWNER}:${BRANCH}"`
  # filter silently returned nothing whenever FORK_OWNER differed in case, and the script
  # then tried to create a duplicate PR and died with "a pull request already exists".
  local fork_owner_lc
  fork_owner_lc="$(printf '%s' "${FORK_OWNER}" | tr '[:upper:]' '[:lower:]')"
  local existing_pr
  existing_pr=$(GH_TOKEN="${GH_TOKEN}" gh pr list \
    --repo "${upstream_repo}" \
    --head "${BRANCH}" \
    --state open \
    --json number,headRepositoryOwner \
    --jq "[.[] | select((.headRepositoryOwner.login // \"\" | ascii_downcase) == \"${fork_owner_lc}\")] | .[0].number // empty" 2>/dev/null || true)

  if [[ -n "${existing_pr}" ]]; then
    echo "Updating existing PR #${existing_pr}"
    # Use the REST endpoint, NOT `gh pr edit`. The latter resolves assignees/labels/
    # reviewers via GraphQL and so demands `read:org` ("The 'login' field requires one of
    # the following scopes: ['read:org']"), which would force a broader PAT than this
    # script needs. PATCH .../pulls/{n} updates title+body with `public_repo` alone.
    GH_TOKEN="${GH_TOKEN}" gh api \
      --method PATCH \
      "repos/${upstream_repo}/pulls/${existing_pr}" \
      -f title="${pr_title}" \
      -f body="${pr_body}" >/dev/null
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
