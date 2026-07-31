#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

page="docs/security/security-architecture.md"

fail() {
  printf 'security architecture docs: %s\n' "$*" >&2
  exit 1
}

[[ -f "${page}" ]] || fail "${page} is missing"

required_headings=(
  "## Security model at a glance"
  "## Trust boundaries"
  "## Kubernetes authorization and tenancy"
  "## Network egress and NetGuard"
  "### Secure default"
  "### Private-sink opt-in"
  "### Always blocked"
  "### Verification"
  "### Residual risk"
  "## Credentials and sensitive data"
  "## Transport security"
  "## Runtime hardening"
  "## CI/CD and supply chain"
  "## Shared responsibility"
  "## Verification checklist"
  "## Limitations and residual risk"
)

for heading in "${required_headings[@]}"; do
  grep -qF -- "${heading}" "${page}" || fail "missing heading: ${heading}"
done

required_terms=(
  "--allow-private-sinks"
  "allowPrivateSinks"
  "SubjectAccessReview"
  "KollectScope"
  "secretRef"
  "readOnlyRootFilesystem"
  "RuntimeDefault"
  "cosign"
  "SBOM"
  "SLSA provenance"
  "Renovate"
  "OpenVEX"
  "plain HTTP"
  "NetworkPolicy"
  "HTTP_PROXY"
)

for term in "${required_terms[@]}"; do
  grep -qF -- "${term}" "${page}" || fail "missing control term: ${term}"
done

grep -qF -- "Security architecture: security/security-architecture.md" mkdocs.yml ||
  fail "security architecture page is missing from navigation"

if grep -En 'Renovate is not used|\.github/dependabot\.yml|no CVE findings are suppressed' \
  SECURITY.md docs/ASSURANCE-CASE.md docs/SECURITY-REVIEW.md docs/adr/0104-security-model.md \
  docs/adr/0705-release-supply-chain.md docs/security/sca-remediation-policy.md; then
  fail "obsolete dependency or vulnerability-exception claim remains"
fi

if grep -En 'Distroless, non-root|scans both release images|token/mTLS|hub mTLS|hub plain HTTP' \
  docs/ASSURANCE-CASE.md docs/SECURITY-REVIEW.md docs/adr/0104-security-model.md \
  docs/adr/0705-release-supply-chain.md; then
  fail "obsolete runtime, artifact, API, or hub security claim remains"
fi

review_anchor="$(
  awk -F'`' '/\\*\\*Anchor\\*\\*/ { print $2 }' docs/SECURITY-REVIEW.md
)"
[[ -n "${review_anchor}" ]] || fail "security review does not declare an exact SHA"
git cat-file -e "${review_anchor}^{commit}" ||
  fail "security review anchor ${review_anchor} is not a commit"
git merge-base --is-ancestor "${review_anchor}" HEAD ||
  fail "security review anchor ${review_anchor} is not in branch history"

jq -e '
  any(.statements[];
    .vulnerability.name == "GO-2026-5932" and
    .status == "not_affected" and
    .justification == "vulnerable_code_not_present")
' docs/security/vex.json >/dev/null ||
  fail "OpenVEX does not document the active GO-2026-5932 exception"
grep -qF -- 'next review due 2026-10-29' osv-scanner.toml ||
  fail "OSV exception does not have a next review date"

printf 'security architecture docs: ok\n'
