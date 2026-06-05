# kollect — product requirements

Binding requirements for kollect beyond engineering guidelines
([GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md)).
ADRs in [adr/README.md](adr/README.md) capture design decisions; this document captures **what we must ship** and
**priority order**.

## Early (Phase 0–1)

| Requirement | Rationale |
| --- | --- |
| **Custom / self-signed CA TLS** for Git and GitLab sinks | Platform orgs use internal CAs; human-user-0 must succeed without disabling TLS verify |
| **Validating webhooks early** | Reject invalid CEL/JSONPath and sink config at admission — no reconcile-time workarounds |
| **Helm chart day 1** | Install path matches production; CI runs `helm template`, `helm-unittest`, schema checks |
| **Prometheus metrics early** | Export latency, reconcile errors, collection counts — testable in unit/envtest and CI |
| **Connection test (first-class)** | Visible, informative, discoverable feedback when sinks/profiles are misconfigured (`kubectl`-friendly status) |
| **HTTP inventory API (core)** | Read-only access for portals and automation — not deferred to a later phase |
| **Inventory HTTP auth (K8s-native)** | TokenReview + SubjectAccessReview; `Authorization: Bearer` SA tokens; `--inventory-auth-mode=kubernetes` default — [ADR-0024](adr/0024-inventory-api-auth.md) |
| **Tested sample CRs** | Deployment, Service, Ingress, generic CRD, Helm release metadata — contract tests in CI where feasible |
| **Demo Git repo** | Public examples with SSH + token auth; GitLab-compatible endpoints |

## High priority (must not block single-cluster)

| Requirement | Rationale |
| --- | --- |
| **Multi-cluster (~60 clusters)** | Hub aggregation without forcing 60 Confluence pages or 60 Git commits per change |
| **Aggregation** | One inventory roll-up, one export commit, one doc page where possible |
| **Phase 0 one-pod-does-all** | Single deployment can collect + aggregate + export for first success path |
| **Per-cluster agents / cross-cluster collector** | Explored in [ADR-0022](adr/0022-multi-cluster-sync-rfc.md); must not block single-cluster MVP |
| **`KollectHub` CRD (hub cluster)** | Hub is declarative CRD → operator-managed Deployment → lean queue → aggregated export |
| **Lean queue transport (pluggable)** | `Transport` interface; `inprocess` → **Redis Streams** (Phase 2 spike) → NATS/Kafka via config — [ADR-0023](adr/0023-lean-queue-transport.md); backend only ships with integration/e2e proof |
| **Namespaced `KollectInventory`** | Team-owned rollup; **`KollectClusterInventory`** reserved for platform ([ADR-0004](adr/0004-crd-model.md)) |
| **Namespaced `KollectScope` (Phase 3)** | Tenancy boundary first; **`KollectClusterScope`** for platform teams as addition |

## Testing

| Requirement | Rationale |
| --- | --- |
| **Periodic end-to-end tests** | Full install → sample CRs → export/HTTP smoke; catches regressions unit tests miss |
| **Nightly + `workflow_dispatch`** | Scheduled GitHub Actions workflow; manual trigger for release candidates |

## Architecture principles

- **Schema clarity over sync location** — transformation/rendering may live at the sink (Git repo,
  portal, storage backend) or in the operator; the contract is a clear inventory schema.
- **Git sync is one option** — agent-to-agent or object-storage fan-in may be preferable; do not
  over-commit to Git-only topology.
- **Defer `KollectPublication`** until collection + aggregation + sinks are mature ([ADR-0013](adr/0013-prior-art.md)).

## Deferred

| Item | When |
| --- | --- |
| `KollectPublication` (Confluence / templated doc sync) | After collection maturity |
| JSONPath filters on sink targets | After core export path works |
| Full oauth2-proxy sidecar on HTTP API | Optional Helm sidecar (`oauth2Proxy.enabled: false`); K8s bearer auth is primary — [ADR-0024](adr/0024-inventory-api-auth.md) |

## Documentation

- Prefer **mermaid diagrams** in architecture and ADR docs.
- Public docs on **GitHub Pages** for now; custom domain reserved for later.

## See also

- [ARCHITECTURE.md](ARCHITECTURE.md) — system view and multi-cluster outlook
- [adr/0022-multi-cluster-sync-rfc.md](adr/0022-multi-cluster-sync-rfc.md) — topology options (Proposed)
- [adr/0023-lean-queue-transport.md](adr/0023-lean-queue-transport.md) — hub queue selection (Accepted)
- [adr/0024-inventory-api-auth.md](adr/0024-inventory-api-auth.md) — inventory HTTP auth (Accepted)
