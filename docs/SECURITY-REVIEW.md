# Security self-review — 2026-07-31

Structured maintainer review of Kollect's application and software-supply-chain security posture.
This is not an independent penetration test, formal certification, or assurance that no
vulnerability exists.

## Review metadata

| Field | Value |
| --- | --- |
| **Date** | 2026-07-31 |
| **Anchor** | `65047b1ac2538772a6f01367e4739176c1326880` |
| **Release baseline** | v0.10.0 plus the merged NET-01 private-sink implementation |
| **Scope** | Manager, pipeline CLI, CRDs and webhooks, optional Read API, Helm and Kustomize manifests, sink transports, CI, and release publication |
| **Method** | Trust-boundary and data-flow review; authorization, credential, redaction, endpoint, TLS, runtime, CI, dependency, and release-control source trace; existing test inventory |
| **Related docs** | [Security architecture](security/security-architecture.md), [Assurance case](ASSURANCE-CASE.md), [ADR-0104](adr/0104-security-model.md), [Security policy](https://github.com/platformrelay/kollect/blob/main/SECURITY.md) |

The anchor is an exact source snapshot. Findings about a deployment still depend on the chart
values, cluster policy, granted workload RBAC, Secret access, and reachable sinks.

## Threat model reviewed

- A CR author attempts to use the manager as a confused deputy for unauthorized Kubernetes reads,
  credential use, data export, or network access.
- Selected objects or backend errors contain secrets or sensitive data that could reach sinks,
  status, Events, or logs.
- A hostile endpoint, DNS response, redirect, proxy, or weak TLS configuration redirects an export.
- A compromised manager uses its service-account permissions, referenced credentials, or network
  position.
- A dependency, CI workflow, release credential, registry artifact, or download path is tampered.

## Findings

| ID | Severity | Finding | Status / action |
| --- | --- | --- | --- |
| SR-2026-01 | P2 | The chart has no outbound sink-aware `NetworkPolicy`; the optional scaffold policy protects metrics ingress only. | **Open, adopter control** — restrict manager egress to Kubernetes API, DNS, and approved sinks. |
| SR-2026-02 | P2 | `allowPrivateSinks` intentionally lets Sink-CR authors target reachable RFC1918/ULA services. | **Accepted opt-in risk** — default off, cluster-admin install surface, always-blocked metadata/loopback/link-local ranges remain denied. |
| SR-2026-03 | P2 | Git sanitizes known credentials in errors, but generic third-party backend error strings are not centrally sanitized before status/Events. | **Open assurance gap** — avoid credentials in endpoints; add common sanitization and regression tests. |
| SR-2026-04 | P2 | The optional Read API's default mode authenticates and authorizes requests, but its built-in listener is plain HTTP and a disabled-auth development mode exists. | **Accepted while off by default** — retain Kubernetes auth and deploy behind TLS/mesh or use a local port-forward when enabled. |
| SR-2026-05 | P2 | TLS is not universally required: Kafka has no Kollect TLS surface, NATS TLS is configuration-dependent, and database URI/DSN settings control transport. | **Open hardening** — require secure backend configuration and document exceptions. |
| SR-2026-06 | P2 | `GO-2026-5932` is indefinitely ignored by OSV because the unmaintained package has no fix, though govulncheck reports no called path. | **Documented not affected** — OpenVEX + scanner rationale; re-review by 2026-10-29 or on import-graph change. |
| SR-2026-07 | P3 | Git CLI execution can inherit proxy environment variables; checked-IP pinning is not a proxy policy. | **Open hardening** — do not inject proxy variables; enforce egress policy; add a fail-closed test/fix. |
| SR-2026-08 | P2 | Validating webhooks can be disabled, removing admission-time endpoint and configuration checks; runtime Git still accepts local `file://` repositories. | **Operator-controlled risk** — keep webhooks enabled outside controlled development and protect CR write access. |
| SR-2026-09 | P3 | `KollectScope` is opt-in and multiple Scopes are resolved by name rather than rejected. | **Documented** — RBAC is the hard boundary; operate one Scope per namespace. |
| SR-2026-10 | P3 | License policy relies on import blocklists, SBOM review, and maintainer review rather than an automated license scanner. | **Accepted pre-1.0 gap** — scan SBOMs before release; automate when practical. |
| SR-2026-11 | P3 | Solo-maintainer branch policy requires PRs and CI but no second-person approval. | **Accepted governance risk** — protected release environment and eligibility gate are compensating controls; add review when a trusted maintainer joins. |

No P0 or P1 issue was identified in this source review. That conclusion is limited to the reviewed
snapshot and method; it is not a production-risk rating.

## Verified strengths

- Kubernetes RBAC bounds informer list/watch; controller SSAR gates per-object processing and fails
  closed on authorization API errors. The pipeline CLI relies directly on its kubeconfig RBAC.
- `KollectScope` gates GVK, namespace, sink, and resource-export policy when configured.
- Secret-like generic sink options are rejected; credentials use `secretRef`.
- Sensitive-key scrubbing and resource pruning run before rows enter sink export.
- Endpoint policy is repeated after DNS resolution and fails closed on mixed allowed/blocked answers.
- NET-01 is default-off, install-time, and narrower than the test-only integration bypass.
- Chart defaults are non-root, seccomp `RuntimeDefault`, read-only root, no privilege escalation,
  dropped capabilities, and bounded ephemeral storage.
- CI includes gitleaks, RBAC audit, govulncheck, lint/SAST, generated-contract checks, behavior tests,
  CodeQL, and checksum-negative tests.
- Release eligibility is evaluated against protected-main history and exact-SHA required checks.
- Operator, pipeline, and UI images are scanned, signed by digest, SBOM'd, and attested; release
  assets receive checksums, signatures, and provenance.

## Verification performed or required before publication

```sh
task verify
task lint
task test
task audit:rbac
task vulncheck
task helm-test
task scrub
bash hack/test/security_architecture_docs_test.sh
mkdocs build --strict
```

NET-01 landed through [PR #146](https://github.com/platformrelay/kollect/pull/146). Before an
announcement, verify the release candidate's resulting merged SHA and repeat the public-doc truth
and release-readiness audit.

## Review triggers

Repeat this review after changes to:

- RBAC, `KollectScope`, watch breadth, or the Read API;
- profile redaction, full-resource export, status, Events, or error propagation;
- NetGuard, proxy handling, endpoint validation, TLS, or a sink transport;
- pod security context, service-account behavior, or network policy;
- dependency exceptions, workflow permissions, release eligibility, signing, SBOM, or provenance;
- the UI/product scope or supported release artifacts.

Independent review is recommended before high-trust or regulated production adoption.
