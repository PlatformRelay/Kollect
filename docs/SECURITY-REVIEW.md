# Security self-review

Structured self-review of Kollect security posture for OpenSSF Best Practices
**security_review** documentation. This is a maintainer-conducted review, not an
independent third-party audit.

## Review metadata

| Field | Value |
| --- | --- |
| **Date** | 2026-06-05 |
| **Scope** | `main` branch — operator (`cmd/`, `internal/`), Helm chart, CRD webhooks, Read API, kollect-ui SPA, CI/release pipelines |
| **Reviewers** | Konrad Heimel (maintainer, self-review) |
| **Method** | ADR/threat-model walkthrough, CI control inventory, manual code-path review of secret handling, RBAC renders, sink validators, and release workflow |
| **Related docs** | [ASSURANCE-CASE.md](ASSURANCE-CASE.md), [ADR-0104](adr/0104-security-model.md), [SECURITY.md](../SECURITY.md) |

## Scope summary

Reviewed components:

- **Collection path** — dynamic informers, CEL/JSONPath extraction, in-memory store, debounced export
- **Credential handling** — `secretRef` resolution, `BuildContext`, logging constraints
- **Tenancy** — `KollectScope`, namespace watches, SubjectAccessReview degradation
- **Sink backends** — Git, object store, Postgres, event emitters; TLS and validation
- **Admission** — validating webhooks, git sink warnings, cert serving ([ADR-0105](adr/0105-webhook-serving-cert-management.md))
- **Read API** — auth model ([ADR-0404](adr/0404-inventory-api-auth.md))
- **Supply chain** — GitHub Actions, cosign, SBOM, Trivy, gitleaks, CodeQL, govulncheck
- **UI** — static SPA; no cluster credentials in browser; API auth via deployer config

Out of scope: production cluster hardening (NetworkPolicy, mesh, sink encryption-at-rest) —
adopter responsibilities documented in [SECURITY.md](../SECURITY.md).

## Findings summary

| ID | Severity | Finding | Status |
| --- | --- | --- | --- |
| SR-01 | Info | Consolidated threat model published in ADR-0104 and ASSURANCE-CASE | **Closed** — docs landed 2026-06-05 |
| SR-02 | Info | `govulncheck`, gitleaks, CodeQL, and golangci-lint (`gosec`) run in CI | **Closed** — verified in `.github/workflows/` |
| SR-03 | Info | Release images signed (cosign) with SBOM and provenance | **Closed** — ADR-0705, release workflow |
| SR-04 | Low | No automated payload secret-leak scanner beyond profile redaction | **Accepted** — tracked as open question in ADR-0104 |
| SR-05 | Low | Solo maintainer — no second human reviewer on maintainer PRs | **Accepted** — documented in GOVERNANCE; gold blocker |
| SR-06 | Info | OpenVEX stub published; no active vulnerability suppressions | **Closed** — `security/vex.json` |

No critical or high-severity defects were identified in reviewed code paths during this
self-review. Prior fixes (git ref validation, SAR gating, redaction-at-extraction) were
verified still present.

## Residual risks

| Risk | Likelihood | Impact | Notes |
| --- | --- | --- | --- |
| Maintainer unavailability | Low | High | Succession path in [GOVERNANCE.md](../GOVERNANCE.md) |
| Misconfigured cluster RBAC grants excessive read scope | Medium | High | Document least-privilege; `task audit:rbac` for chart renders |
| Adopter enables `insecureSkipVerify` on production sinks | Medium | High | Webhook warnings; status surfaces opt-in |
| Zero-day in Go/stdlib dependency | Low | Medium | govulncheck + Dependabot; SCA SLAs |
| Compromised maintainer GitHub account | Low | Critical | 2FA on maintainer account; offline recovery backup (private) |

## Recommendations

1. **Next review** — repeat after first GA release or any security-relevant ADR merge.
2. **Coverage** — continue raising test coverage on controller and sink packages ([testing strategy](development/testing.md)).
3. **Co-maintainer** — when a trusted second maintainer joins, enable required PR reviews and update ADR-0705.
4. **RBAC CI gate** — keep `task audit:rbac` green on chart changes (ADR-0104 decision 2026-06-05).

## Sign-off

This review was conducted and documented by the project maintainer. Independent audit
may be considered before enterprise adoption or foundation incubation.

**Konrad Heimel** — 2026-06-05
