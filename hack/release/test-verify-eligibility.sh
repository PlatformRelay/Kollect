#!/usr/bin/env bash
# Deterministic fixture tests for verify-eligibility.sh (no live GitHub API).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUBJECT="${ROOT}/hack/release/verify-eligibility.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

SHA="1111111111111111111111111111111111111111"
MAIN="2222222222222222222222222222222222222222"
TIP="3333333333333333333333333333333333333333"
OLD="4444444444444444444444444444444444444444"

REQUIRED_NAMES='gitleaks verify audit-rbac vulncheck lint test build test-integration helm docker-build preflight kind-smoke pipeline-cli-smoke'

cat >"${TMP}/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
args="$*"
case "${args}" in
  *"/commits/main"*)
    printf '%s\n' "${MOCK_MAIN}"
    ;;
  *"/compare/"*)
    printf '%s\n' "${MOCK_COMPARE}"
    ;;
  *"/check-runs"*)
    cat "${MOCK_CHECKS}"
    ;;
  *"/commits/"*"/pulls"*)
    cat "${MOCK_PULLS}"
    ;;
  *"/pulls/7/commits"*)
    # Script asks gh for .[-1].sha; return the peeled tip directly.
    jq -r '.[-1].sha' "${MOCK_PR_COMMITS}"
    ;;
  *"/pulls/7/reviews"*)
    cat "${MOCK_REVIEWS}"
    ;;
  *)
    echo "unexpected gh invocation: ${args}" >&2
    exit 70
    ;;
esac
MOCK
chmod +x "${TMP}/gh"

write_green_checks() {
	jq -n \
		--arg names "${REQUIRED_NAMES}" \
		'($names | split(" ") | to_entries | map({
			id: (.key + 1),
			name: .value,
			status: "completed",
			conclusion: "success",
			head_sha: $sha
		}))' --arg sha "${SHA}" >"${TMP}/checks.json"
}

write_pr_tip() {
	local tip="${1}"
	jq -n --arg tip "${tip}" '[{"sha": $tip}]' >"${TMP}/pr-commits.json"
}

run_case() {
	PATH="${TMP}:${PATH}" \
		GITHUB_REPOSITORY=PlatformRelay/Kollect \
		MOCK_MAIN="${MAIN}" \
		MOCK_COMPARE="${MOCK_COMPARE}" \
		MOCK_CHECKS="${TMP}/checks.json" \
		MOCK_PULLS="${TMP}/pulls.json" \
		MOCK_PR_COMMITS="${TMP}/pr-commits.json" \
		MOCK_REVIEWS="${TMP}/reviews.json" \
		bash "${SUBJECT}" "${SHA}" >"${TMP}/out" 2>"${TMP}/err"
}

fail_if_passes() {
	local label="$1"
	if run_case; then
		echo "${label} unexpectedly passed" >&2
		exit 1
	fi
}

write_green_checks
printf '%s\n' "[{\"number\":7,\"user\":\"author\",\"merge_sha\":\"${SHA}\"}]" >"${TMP}/pulls.json"
write_pr_tip "${TIP}"
printf '%s\n' "[{\"user\":\"reviewer\",\"state\":\"APPROVED\",\"submitted_at\":\"2026-07-23T00:00:00Z\",\"commit_id\":\"${TIP}\"}]" >"${TMP}/reviews.json"

if [[ ! -x "${SUBJECT}" && ! -f "${SUBJECT}" ]]; then
	echo "missing subject ${SUBJECT}" >&2
	exit 1
fi

if PATH="${TMP}:${PATH}" GITHUB_REPOSITORY=PlatformRelay/Kollect bash "${SUBJECT}" bad-sha \
	>"${TMP}/out" 2>"${TMP}/err"; then
	echo "invalid SHA unexpectedly passed" >&2
	exit 1
fi
grep -q 'full 40-character lowercase commit SHA' "${TMP}/err"

MOCK_COMPARE=diverged
fail_if_passes "non-main SHA"
grep -q 'not reachable from protected main' "${TMP}/err"

MOCK_COMPARE=ahead
jq 'del(.[] | select(.name == "preflight"))' "${TMP}/checks.json" >"${TMP}/missing.json"
mv "${TMP}/missing.json" "${TMP}/checks.json"
fail_if_passes "missing check"
grep -q 'required exact-SHA check preflight: missing' "${TMP}/err"

write_green_checks
jq '(.[] | select(.name == "lint")).conclusion = "failure"' "${TMP}/checks.json" >"${TMP}/red.json"
mv "${TMP}/red.json" "${TMP}/checks.json"
fail_if_passes "red check"
grep -q 'required exact-SHA check lint: completed/failure' "${TMP}/err"

write_green_checks
jq '(.[] | select(.name == "kind-smoke")).conclusion = "cancelled"' "${TMP}/checks.json" >"${TMP}/cancel.json"
mv "${TMP}/cancel.json" "${TMP}/checks.json"
fail_if_passes "cancelled check"
grep -q 'required exact-SHA check kind-smoke: completed/cancelled' "${TMP}/err"

write_green_checks
printf '%s\n' "[{\"user\":\"author\",\"state\":\"APPROVED\",\"submitted_at\":\"2026-07-23T00:00:00Z\",\"commit_id\":\"${TIP}\"}]" >"${TMP}/reviews.json"
fail_if_passes "self-review"
grep -q 'non-author approval of the final PR head' "${TMP}/err"

printf '%s\n' "[
  {\"user\":\"reviewer\",\"state\":\"APPROVED\",\"submitted_at\":\"2026-07-23T00:00:00Z\",\"commit_id\":\"${TIP}\"},
  {\"user\":\"reviewer\",\"state\":\"CHANGES_REQUESTED\",\"submitted_at\":\"2026-07-23T01:00:00Z\",\"commit_id\":\"${TIP}\"}
]" >"${TMP}/reviews.json"
fail_if_passes "stale superseded approval"
grep -q 'non-author approval of the final PR head' "${TMP}/err"

printf '%s\n' "[{\"user\":\"reviewer\",\"state\":\"APPROVED\",\"submitted_at\":\"2026-07-23T00:00:00Z\",\"commit_id\":\"${OLD}\"}]" >"${TMP}/reviews.json"
fail_if_passes "approval not on final PR head"
grep -q 'non-author approval of the final PR head' "${TMP}/err"

printf '%s\n' "[{\"user\":\"reviewer\",\"state\":\"APPROVED\",\"submitted_at\":\"2026-07-23T00:00:00Z\",\"commit_id\":\"${TIP}\"}]" >"${TMP}/reviews.json"
run_case
grep -q "Release eligibility passed for PlatformRelay/Kollect@${SHA}" "${TMP}/out"
grep -q "reviewed PR #7" "${TMP}/out"

echo "verify-eligibility tests: ok"
