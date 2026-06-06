# Security policy

## Supported versions

| Version | Supported |
|---------|-----------|
| `main`  | Yes       |
| Tags    | Latest release only |

## Reporting a vulnerability

**Do not** open public GitHub issues for security-sensitive reports.

Email **konrad.heimel@gmail.com** with:

- Description of the issue and impact
- Steps to reproduce (if possible)
- Affected versions or commits
- Suggested fix (optional)

You should receive an acknowledgment within a few business days. We will coordinate disclosure
and a fix release when appropriate.

## Threat model (summary)

Kollect is a cluster operator that:

- **Reads** Kubernetes resources allowed by its RBAC and SAR checks (configurable per target).
- **Writes** to external sinks and doc backends using credentials from `Secret` references only.
- **Stores** aggregated metadata in CR `status` (summaries, not full payloads — see ADRs).

Risks to consider when deploying:

- Over-broad `ClusterRole` grants increase blast radius if the manager is compromised.
- Sink endpoints must use verified TLS; credentials must not appear in CR specs or logs.
- Restrict egress with `NetworkPolicy` in production.

See [engineering guidelines](docs/development/guidelines.md) for hardening baselines (non-root runtime image, secret
handling, supply-chain checks in CI).

## Supply chain (releases)

Release builds ([`.github/workflows/release.yaml`](.github/workflows/release.yaml)) produce:

- **OCI image** — `ghcr.io/konih/kollect` and `ghcr.io/konih/kollect-ui` with SBOM and SLSA provenance attestations
- **cosign** keyless signatures (verify with release notes instructions)
- **SPDX SBOM** — `sbom.spdx.json` and `sbom-ui.spdx.json` attached to GitHub Releases
- **Checksums** — `sha256sum` manifest for install YAML and chart tarball

Prefer tagged release artifacts over `:latest` in production. Report supply-chain concerns
through the private contact above.

## Dependency and license policy (SCA)

Full thresholds and process: [SCA remediation policy](docs/security/sca-remediation-policy.md)
(satisfies [OpenSSF OSPS-VM-05.01](https://baseline.openssf.org/versions/2026-02-19.html)).

**Vulnerability SLAs:** Critical **7 days**, High **30 days**, Medium **90 days**, Low by next
minor release; **zero tolerance** for reachable CVEs (`govulncheck` must pass before merge) and for
fixable CRITICAL/HIGH in release images (Trivy).

**License classes:** Allow (MIT/Apache/BSD/…), Review (MPL/LGPL — **90 days** to confirm), Deny
(GPL/AGPL/proprietary/unknown — remove before merge or defer with a dated `security` issue).

Deferrals require a GitHub issue or ADR with expiry — see
[policy § Exceptions](docs/security/sca-remediation-policy.md#exceptions-and-deferrals).

## Static analysis and vulnerability scanning

### golangci-lint (SAST)

CI runs **golangci-lint v2** on every push and pull request (`task lint`, job **lint** in
[`.github/workflows/ci.yaml`](.github/workflows/ci.yaml)). Configuration: [`.golangci.yaml`](.golangci.yaml)
(security and correctness linters including `gosec`, `errcheck`, `govet`, `staticcheck`, `depguard`,
`gomodguard`, and `logcheck` via `hack/tooling/.custom-gcl.yml`). Pre-commit runs the same gate on changed Go files.

Run locally:

```sh
task lint
```

**CodeQL** runs on every push/PR to `main` and weekly (`.github/workflows/codeql.yaml`); results appear
under **Security → Code scanning**. See [ADR-0705](docs/adr/0705-release-supply-chain.md) for rationale.

Run locally (requires [CodeQL CLI](https://github.com/github/codeql-cli-binaries)):

```sh
# CI equivalent: init → build → analyze via Actions; local runs use the CodeQL extension or CLI.
task lint
```

### Dependabot

Repository settings (enabled 2026-06-05):

- **Dependabot alerts** — GitHub Advisory Database notifications for vulnerable dependencies
- **Dependabot security updates** — automated patch PRs for known CVEs in `go.mod` and Actions
- **Dependabot version updates** — weekly grouped PRs via [`.github/dependabot.yml`](.github/dependabot.yml)
  (`gomod` minor/patch group, `github-actions` SHA group)

Renovate is not used; Dependabot covers dependency update automation for this solo-maintainer OSS repo.

### govulncheck

CI runs [`govulncheck`](https://go.dev/security/vuln/) on every push and pull request
(`task vulncheck`, job **vulncheck** in [`.github/workflows/ci.yaml`](.github/workflows/ci.yaml)).
The scan uses the Go vulnerability database and reports issues that affect **imported packages in
this module** (including test code). The job fails when govulncheck exits non-zero.

Run locally after installing Go from `go.mod`:

```sh
task vulncheck
```

If a finding is a false positive or only affects an unused code path in a dependency, document the
exception in this file (module, advisory ID, rationale, review date) before suppressing CI — see
[SCA remediation policy § Exceptions and deferrals](docs/security/sca-remediation-policy.md#exceptions-and-deferrals).

Release images are additionally scanned with **Trivy** (CRITICAL/HIGH, fixable only) in
[`.github/workflows/release.yaml`](.github/workflows/release.yaml).

## VEX (vulnerability exceptions)

Kollect publishes an [OpenVEX](https://openvex.dev/) document at
[`docs/security/vex.json`](docs/security/vex.json). The `statements` array is **empty** today — no CVE
findings are suppressed. When a deferral or false positive requires a documented exception,
add a VEX statement and link the GitHub issue or ADR in the statement metadata.

## OpenSSF Scorecard

Supply-chain posture is tracked via the [OpenSSF Scorecard](https://securityscorecards.dev/viewer/?uri=github.com/konih/kollect)
badge in `README.md`. Implemented checks, deferred solo-maintainer items, and rationale are documented in
[ADR-0705 § OpenSSF Scorecard follow-ups](docs/adr/0705-release-supply-chain.md#openssf-scorecard-follow-ups).
