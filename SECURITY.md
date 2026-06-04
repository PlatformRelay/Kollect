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
