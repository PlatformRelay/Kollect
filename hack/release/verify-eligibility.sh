#!/usr/bin/env bash
# Fail closed unless a release commit is on protected main with exact-SHA checks
# and non-stale review evidence. Intended to run from a trusted checkout (default
# branch), never from a candidate tag tree.
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

checks="$(gh api --paginate --slurp "repos/${REPO}/commits/${SHA}/check-runs?per_page=100" \
	--jq '[.[].check_runs[] | select(.head_sha == "'"${SHA}"'") | {id, name, status, conclusion, head_sha}]')"

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

approved_pr=""
while IFS=$'\t' read -r number author merge_sha; do
	[[ -n "${number}" ]] || continue
	[[ "${merge_sha}" == "${SHA}" ]] || continue

	tip="$(gh api --paginate "repos/${REPO}/pulls/${number}/commits?per_page=100" \
		--jq '.[-1].sha')"
	if [[ ! "${tip}" =~ ^[0-9a-f]{40}$ ]]; then
		echo "error: could not resolve final head commit for PR #${number}" >&2
		continue
	fi

	reviews="$(gh api --paginate --slurp "repos/${REPO}/pulls/${number}/reviews?per_page=100" \
		--jq '[.[][] | {user: .user.login, state, submitted_at, commit_id}]')"

	if jq -e --arg author "${author}" --arg tip "${tip}" '
		# Latest review per reviewer (submitted_at ascending, last wins).
		group_by(.user) | map(sort_by(.submitted_at) | last) |
		map(select(
			.state == "APPROVED"
			and .user != $author
			and .commit_id == $tip
		)) | length > 0
	' <<<"${reviews}" >/dev/null; then
		approved_pr="${number}"
		break
	fi
done < <(jq -r '.[] | [.number, .user, .merge_sha] | @tsv' <<<"${pulls}")

if [[ -z "${approved_pr}" ]]; then
	echo "error: no merged-to-${DEFAULT_BRANCH} PR for ${SHA} has a non-author approval of the final PR head" >&2
	exit 1
fi

echo "Release eligibility passed for ${REPO}@${SHA} (reviewed PR #${approved_pr}, main ${main_sha})."
