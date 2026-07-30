#!/usr/bin/env bash
# Deterministic fixture tests for verify-eligibility.sh (no live GitHub API).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUBJECT="${ROOT}/hack/release/verify-eligibility.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

SHA="1111111111111111111111111111111111111111"
MAIN="2222222222222222222222222222222222222222"

REQUIRED_NAMES='gitleaks verify audit-rbac vulncheck lint test build test-integration helm docker-build preflight kind-smoke pipeline-cli-smoke'

cat >"${TMP}/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
args="$*"
# Mirror gh ≥2.x: --slurp cannot be combined with --jq/--template.
if [[ "${args}" == *--slurp* && ( "${args}" == *--jq* || "${args}" == *--template* ) ]]; then
	echo 'the `--slurp` option is not supported with `--jq` or `--template`' >&2
	exit 1
fi
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
  *)
    echo "unexpected gh invocation: ${args}" >&2
    exit 70
    ;;
esac
MOCK
chmod +x "${TMP}/gh"

write_green_checks() {
	# Shape matches `gh api --paginate --slurp` (array of pages).
	jq -n \
		--arg names "${REQUIRED_NAMES}" \
		'[{check_runs: ($names | split(" ") | to_entries | map({
			id: (.key + 1),
			name: .value,
			status: "completed",
			conclusion: "success",
			head_sha: $sha
		}))}]' --arg sha "${SHA}" >"${TMP}/checks.json"
}

run_case() {
	PATH="${TMP}:${PATH}" \
		GITHUB_REPOSITORY=PlatformRelay/Kollect \
		MOCK_MAIN="${MAIN}" \
		MOCK_COMPARE="${MOCK_COMPARE}" \
		MOCK_CHECKS="${TMP}/checks.json" \
		MOCK_PULLS="${TMP}/pulls.json" \
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
jq '.[0].check_runs |= map(select(.name != "preflight"))' "${TMP}/checks.json" >"${TMP}/missing.json"
mv "${TMP}/missing.json" "${TMP}/checks.json"
fail_if_passes "missing check"
grep -q 'required exact-SHA check preflight: missing' "${TMP}/err"

write_green_checks
jq '(.[0].check_runs[] | select(.name == "lint")).conclusion = "failure"' "${TMP}/checks.json" >"${TMP}/red.json"
mv "${TMP}/red.json" "${TMP}/checks.json"
fail_if_passes "red check"
grep -q 'required exact-SHA check lint: completed/failure' "${TMP}/err"

write_green_checks
jq '(.[0].check_runs[] | select(.name == "kind-smoke")).conclusion = "cancelled"' "${TMP}/checks.json" >"${TMP}/cancel.json"
mv "${TMP}/cancel.json" "${TMP}/checks.json"
fail_if_passes "cancelled check"
grep -q 'required exact-SHA check kind-smoke: completed/cancelled' "${TMP}/err"

write_green_checks
printf '%s\n' "[]" >"${TMP}/pulls.json"
fail_if_passes "no merged PR"
grep -q "no merged-to-main PR whose merge commit is ${SHA}" "${TMP}/err"

printf '%s\n' "[{\"number\":7,\"user\":\"author\",\"merge_sha\":\"${MAIN}\"}]" >"${TMP}/pulls.json"
fail_if_passes "merge SHA mismatch"
grep -q "no merged-to-main PR whose merge commit is ${SHA}" "${TMP}/err"

# Solo-maintainer: merged PR is enough — no non-author APPROVE required.
printf '%s\n' "[{\"number\":7,\"user\":\"author\",\"merge_sha\":\"${SHA}\"}]" >"${TMP}/pulls.json"
run_case
grep -q "Release eligibility passed for PlatformRelay/Kollect@${SHA}" "${TMP}/out"
grep -q "merged PR #7" "${TMP}/out"

echo "verify-eligibility tests: ok"
