# Project governance

Kollect is an open-source Kubernetes operator maintained as a personal OSS project under
[github.com/platformrelay/kollect](https://github.com/platformrelay/kollect). This document describes how
decisions are made, who is responsible for what, and how the project continues if the
maintainer is unavailable.

## Scope

This governance model applies to:

- The **kollect** operator, Helm chart, CRDs, and documentation in this repository
- The **kollect-ui** static SPA (`ui/`) and its container image
- Release artifacts published to GHCR and GitHub Releases
- Public documentation at [platformrelay.github.io/Kollect](https://platformrelay.github.io/Kollect/)

It does **not** cover downstream deployments, fork-specific policies, or private runbooks.

## Roles and responsibilities

| Role | Who | Responsibilities |
| --- | --- | --- |
| **Maintainer** | [Konrad Heimel](https://github.com/konih) | Final merge authority, releases, security response, ADR approval, CI and branch policy |
| **Contributor** | Anyone opening a PR or issue | Follow [CONTRIBUTING.md](CONTRIBUTING.md), [DCO](CONTRIBUTING.md#developer-certificate-of-origin-dco), and this Code of Conduct; propose changes via pull request |
| **Security reporter** | External researchers | Report vulnerabilities privately per [SECURITY.md](SECURITY.md) |

The maintainer is the default approver for all pull requests. There is currently **one**
maintainer (bus factor 1). Adding a co-maintainer requires an explicit update to this
document and a recorded decision in an ADR.

## Decision making

| Change type | Process |
| --- | --- |
| **Architecture** (API shape, tenancy, sinks, multi-cluster, security posture) | Write or update an [ADR](docs/adr/README.md); maintainer LGTM before merge |
| **Routine fixes and docs** | PR with green CI; maintainer review |
| **Breaking API changes** | ADR + migration notes; only after a tagged release exists ([CONTRIBUTING.md](CONTRIBUTING.md)) |
| **Release tagging** | Maintainer-only; follows [docs/RELEASE.md](docs/RELEASE.md) |
| **Security fixes** | Coordinated disclosure per [SECURITY.md](SECURITY.md); may bypass normal feature freeze |

Disputes on technical direction are resolved by the maintainer after discussion in the PR or
issue. Persistent disagreements may be documented in an ADR with accepted/rejected alternatives.

## Security contact

Report vulnerabilities **privately** to **konrad.heimel@gmail.com** — do not open public
issues for security-sensitive reports. See [SECURITY.md](SECURITY.md) for supported versions,
SLAs, and supply-chain expectations.

## Access continuity and succession

Kollect is a solo-maintainer project today. The following continuity measures are in place:

- **Source of truth** — all code, docs, and release history live in the public GitHub
  repository; tagged releases and container images are published to GHCR.
- **Recovery materials** — GitHub account recovery codes, GHCR publish credentials, and
  Sigstore/cosign release identity are stored in an **encrypted offline backup** accessible
  only to the maintainer (private runbook; not committed to this repo).
- **Succession path** — if the maintainer is permanently unavailable, project continuity
  proceeds by **transferring the repository** to a named successor or to a neutral GitHub
  organization with documented ownership. The current designated contact for succession
  planning is **konrad.heimel@gmail.com** (same as security contact).
- **Public commitment** — the maintainer will update this section when a co-maintainer or
  successor is formally appointed, or when the repository moves to an org with multiple owners.

Until a second maintainer is appointed, two-person review and bus-factor ≥ 2 remain
**documented gaps** ([ADR-0705](docs/adr/0705-release-supply-chain.md)).

## Related documents

| Document | Purpose |
| --- | --- |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community behavior standards |
| [CONTRIBUTING.md](CONTRIBUTING.md) | PR workflow, DCO, code review |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting and SCA policy |
| [docs/ASSURANCE-CASE.md](docs/ASSURANCE-CASE.md) | Security claims and countermeasures |
| [docs/adr/0104-security-model.md](docs/adr/0104-security-model.md) | Consolidated security architecture |
