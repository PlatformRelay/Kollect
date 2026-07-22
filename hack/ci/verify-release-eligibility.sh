#!/usr/bin/env bash
# Fail closed unless a release commit is reviewed on main with every required check green.
set -euo pipefail

SHA="${1:?usage: verify-release-eligibility.sh <full-commit-sha>}"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY required}"

if [[ ! "${SHA}" =~ ^[0-9a-f]{40}$ ]]; then
	echo "error: release SHA must be a full 40-character lowercase commit SHA" >&2
	exit 1
fi

required_checks=(
	gitleaks verify audit-rbac vulncheck lint test build test-integration helm
	docker-build preflight kind-smoke pipeline-cli-smoke
)

main_sha="$(gh api "repos/${REPO}/commits/main" --jq .sha)"
comparison="$(gh api "repos/${REPO}/compare/${SHA}...${main_sha}" --jq .status)"
if [[ "${comparison}" != "ahead" && "${comparison}" != "identical" ]]; then
	echo "error: release SHA ${SHA} is not reachable from protected main (${main_sha})" >&2
	exit 1
fi

checks="$(gh api --paginate --slurp "repos/${REPO}/commits/${SHA}/check-runs?per_page=100" \
	--jq '[.[].check_runs[] | {id, name, status, conclusion}]')"

failed=0
for name in "${required_checks[@]}"; do
	result="$(jq -r --arg name "${name}" '
		(map(select(.name == $name)) | sort_by(.id) | last) as $run |
		if $run == null then "missing" else ($run.status + "/" + ($run.conclusion // "")) end
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
	--jq '[.[] | select(.base.ref == "main" and .merged_at != null) | {number, user: .user.login, merge_sha: .merge_commit_sha}]')"
approved_pr=""
while IFS=$'\t' read -r number author merge_sha; do
	[[ -n "${number}" ]] || continue
	[[ "${merge_sha}" == "${SHA}" ]] || continue
	reviews="$(gh api --paginate --slurp "repos/${REPO}/pulls/${number}/reviews?per_page=100" \
		--jq '[.[][] | {user: .user.login, state, submitted_at}]')"
	if jq -e --arg author "${author}" '
		map(select(.state == "APPROVED" and .user != $author)) | length > 0
	' <<<"${reviews}" >/dev/null; then
		approved_pr="${number}"
		break
	fi
done < <(jq -r '.[] | [.number, .user, .merge_sha] | @tsv' <<<"${pulls}")

if [[ -z "${approved_pr}" ]]; then
	echo "error: no merged-to-main PR with a distinct approving reviewer is associated with ${SHA}" >&2
	exit 1
fi

echo "Release eligibility passed for ${REPO}@${SHA} (reviewed PR #${approved_pr}, main ${main_sha})."
