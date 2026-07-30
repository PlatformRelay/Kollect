#!/usr/bin/env bash
# Fail closed unless a release commit is on protected main with exact-SHA checks
# and merge evidence (merged-to-main PR). Intended to run from a trusted checkout
# (default branch), never from a candidate tag tree.
#
# Solo-maintainer policy (2026-07-30): non-author APPROVE is not required. Residual
# risk of solo-publish is accepted; Environment `release` + tag ruleset remain.
set -euo pipefail

SHA="${1:?usage: verify-eligibility.sh <full-commit-sha>}"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}"
DEFAULT_BRANCH="${RELEASE_DEFAULT_BRANCH:-main}"

if [[ ! "${SHA}" =~ ^[0-9a-f]{40}$ ]]; then
	echo "error: release SHA must be a full 40-character lowercase commit SHA" >&2
	exit 1
fi

required_checks=(
	gitleaks verify audit-rbac vulncheck lint test build test-integration helm
	docker-build preflight kind-smoke pipeline-cli-smoke
)

main_sha="$(gh api "repos/${REPO}/commits/${DEFAULT_BRANCH}" --jq .sha)"
comparison="$(gh api "repos/${REPO}/compare/${SHA}...${main_sha}" --jq .status)"
if [[ "${comparison}" != "ahead" && "${comparison}" != "identical" ]]; then
	echo "error: release SHA ${SHA} is not reachable from protected main (${main_sha})" >&2
	exit 1
fi

# gh ≥2.x rejects combining --slurp with --jq; flatten pages in a separate jq pass.
checks_pages="$(gh api --paginate --slurp "repos/${REPO}/commits/${SHA}/check-runs?per_page=100")"
checks="$(jq '[.[].check_runs[] | select(.head_sha == "'"${SHA}"'") | {id, name, status, conclusion, head_sha}]' \
	<<<"${checks_pages}")"

failed=0
for name in "${required_checks[@]}"; do
	result="$(jq -r --arg name "${name}" '
		(map(select(.name == $name)) | sort_by(.id) | last) as $run |
		if $run == null then "missing"
		else ($run.status + "/" + ($run.conclusion // ""))
		end
	' <<<"${checks}")"
	if [[ "${result}" != "completed/success" ]]; then
		echo "error: required exact-SHA check ${name}: ${result}" >&2
		failed=1
	else
		echo "ok: ${name}"
	fi
done
if [[ "${failed}" -ne 0 ]]; then
	exit 1
fi

pulls="$(gh api "repos/${REPO}/commits/${SHA}/pulls" \
	--jq '[.[] | select(.base.ref == "'"${DEFAULT_BRANCH}"'" and .merged_at != null) | {number, user: .user.login, merge_sha: .merge_commit_sha}]')"

merged_pr=""
while IFS=$'\t' read -r number _author merge_sha; do
	[[ -n "${number}" ]] || continue
	[[ "${merge_sha}" == "${SHA}" ]] || continue
	merged_pr="${number}"
	break
done < <(jq -r '.[] | [.number, .user, .merge_sha] | @tsv' <<<"${pulls}")

if [[ -z "${merged_pr}" ]]; then
	echo "error: no merged-to-${DEFAULT_BRANCH} PR whose merge commit is ${SHA}" >&2
	exit 1
fi

echo "Release eligibility passed for ${REPO}@${SHA} (merged PR #${merged_pr}, main ${main_sha})."
