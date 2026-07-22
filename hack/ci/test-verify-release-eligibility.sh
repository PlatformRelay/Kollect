#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUBJECT="${ROOT}/hack/ci/verify-release-eligibility.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

SHA="1111111111111111111111111111111111111111"
MAIN="2222222222222222222222222222222222222222"

cat >"${TMP}/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
args="$*"
case "${args}" in
  *"/commits/main"*) printf '%s\n' "${MOCK_MAIN}" ;;
  *"/compare/"*) printf '%s\n' "${MOCK_COMPARE}" ;;
  *"/check-runs"*) cat "${MOCK_CHECKS}" ;;
  *"/commits/"*"/pulls"*) cat "${MOCK_PULLS}" ;;
  *"/pulls/7/reviews"*) cat "${MOCK_REVIEWS}" ;;
  *) echo "unexpected gh invocation: ${args}" >&2; exit 70 ;;
esac
MOCK
chmod +x "${TMP}/gh"

write_green_checks() {
	jq -n '($names | split(" ") | to_entries | map({id: (.key + 1), name: .value, status: "completed", conclusion: "success"}))' \
		--arg names 'gitleaks verify audit-rbac vulncheck lint test build test-integration helm docker-build preflight kind-smoke pipeline-cli-smoke' >"${TMP}/checks.json"
}

run_case() {
	PATH="${TMP}:${PATH}" GITHUB_REPOSITORY=PlatformRelay/Kollect MOCK_MAIN="${MAIN}" \
		MOCK_COMPARE="${MOCK_COMPARE}" MOCK_CHECKS="${TMP}/checks.json" \
		MOCK_PULLS="${TMP}/pulls.json" MOCK_REVIEWS="${TMP}/reviews.json" \
		bash "${SUBJECT}" "${SHA}" >"${TMP}/out" 2>"${TMP}/err"
}

write_green_checks
printf '%s\n' "[{\"number\":7,\"user\":\"author\",\"merge_sha\":\"${SHA}\"}]" >"${TMP}/pulls.json"
printf '%s\n' '[{"user":"reviewer","state":"APPROVED","submitted_at":"2026-07-23T00:00:00Z"}]' >"${TMP}/reviews.json"

if PATH="${TMP}:${PATH}" GITHUB_REPOSITORY=PlatformRelay/Kollect bash "${SUBJECT}" bad-sha \
	>"${TMP}/out" 2>"${TMP}/err"; then
	echo "invalid SHA unexpectedly passed" >&2
	exit 1
fi
grep -q 'full 40-character lowercase commit SHA' "${TMP}/err"

MOCK_COMPARE=diverged
if run_case; then echo "non-main SHA unexpectedly passed" >&2; exit 1; fi
grep -q 'not reachable from protected main' "${TMP}/err"

MOCK_COMPARE=ahead
jq 'del(.[] | select(.name == "preflight"))' "${TMP}/checks.json" >"${TMP}/missing.json"
mv "${TMP}/missing.json" "${TMP}/checks.json"
if run_case; then echo "missing check unexpectedly passed" >&2; exit 1; fi
grep -q 'required exact-SHA check preflight: missing' "${TMP}/err"

write_green_checks
jq '(.[] | select(.name == "lint")).conclusion = "failure"' "${TMP}/checks.json" >"${TMP}/red.json"
mv "${TMP}/red.json" "${TMP}/checks.json"
if run_case; then echo "red check unexpectedly passed" >&2; exit 1; fi
grep -q 'required exact-SHA check lint: completed/failure' "${TMP}/err"

write_green_checks
printf '%s\n' '[{"user":"author","state":"APPROVED","submitted_at":"2026-07-23T00:00:00Z"}]' >"${TMP}/reviews.json"
if run_case; then echo "self-review unexpectedly passed" >&2; exit 1; fi
grep -q 'distinct approving reviewer' "${TMP}/err"

printf '%s\n' '[{"user":"reviewer","state":"APPROVED","submitted_at":"2026-07-23T00:00:00Z"}]' >"${TMP}/reviews.json"
run_case
grep -q 'Release eligibility passed' "${TMP}/out"

echo "verify-release-eligibility tests: ok"
