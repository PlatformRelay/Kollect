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

kollect is a cluster operator that:

- **Reads** Kubernetes resources allowed by its RBAC and SAR checks (configurable per target).
- **Writes** to external sinks and doc backends using credentials from `Secret` references only.
- **Stores** aggregated metadata in CR `status` (summaries, not full payloads — see ADRs).

Risks to consider when deploying:

- Over-broad `ClusterRole` grants increase blast radius if the manager is compromised.
- Sink endpoints must use verified TLS; credentials must not appear in CR specs or logs.
- Restrict egress with `NetworkPolicy` in production.

See [GUIDELINES.md](GUIDELINES.md) for hardening baselines (distroless image, non-root, secret
handling, supply-chain checks in CI).

## Supply chain (releases)

Release builds ([`.github/workflows/release.yaml`](.github/workflows/release.yaml)) produce:

- **OCI image** — `ghcr.io/konih/kollect` with SBOM and SLSA provenance attestations
- **cosign** keyless signatures (verify with release notes instructions)
- **SPDX SBOM** — `sbom.spdx.json` attached to GitHub Releases
- **Checksums** — `sha256sum` manifest for install YAML and chart tarball

Prefer tagged release artifacts over `:latest` in production. Report supply-chain concerns
through the private contact above.

## Dependency vulnerability scanning

CI runs [`govulncheck`](https://go.dev/security/vuln/) on every push and pull request
(`task vulncheck`, job **vulncheck** in [`.github/workflows/ci.yaml`](.github/workflows/ci.yaml)).
The scan uses the Go vulnerability database and reports issues that affect **imported packages in
this module** (including test code). The job fails when govulncheck exits non-zero.

Run locally after installing Go from `go.mod`:

```sh
task vulncheck
```

If a finding is a false positive or only affects an unused code path in a dependency, document the
exception in this file (module, advisory ID, rationale, review date) before suppressing CI.
