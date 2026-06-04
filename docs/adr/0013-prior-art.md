# ADR-0013: Prior art and OSS reference patterns

## Status

Accepted (living document — update as we learn)

## Context

kollect occupies a niche: **generic attribute-selection CRD + resource-selection CRD + aggregation +
multi-backend export + doc-sync**. No single OSS project combines all of these. Local shallow clones
under `references/oss/` (read-only, never shipped) inform CRD ergonomics, informers, sinks, CI, and
metrics. This ADR records what we **adopt**, **defer**, and **reject** after sampling those repos.

## OSS comparison summary

### external-secrets

| Pattern | Finding | kollect stance |
| --- | --- | --- |
| Provider plugin registry | `SecretStoreProvider` discriminated union + Go provider packages | **Adopt** — `KollectSink` `type` + internal registry/factory (ADR-0005) |
| Cluster vs namespaced stores | `ClusterSecretStore` + namespace `conditions` | **Adopt (defer Phase 3)** — inform `KollectScope` and optional sink split |
| Helm/CI | `helm-docs`, `helm-unittest`, `values.schema.json`, `helm template` → manifests | **Adopt** — Phase 0 CI parity |
| Status content | Sync metadata only, never secret bytes | **Adopt** — aligns with ADR-0006 |
| Reconciled SecretStore | ESO reconciles stores for validation/status | **Reject for Profile/Sink** — follow Flux static config (ADR-0015) |

### Flux (source-controller + notification-controller)

| Pattern | Finding | kollect stance |
| --- | --- | --- |
| Static Provider/Alert | No status subresource, no Provider reconciler | **Adopt** — `KollectProfile`, `KollectSink` static |
| `spec.suspend` | On all reconciled sources | **Adopt** — all reconciled kollect kinds |
| CEL `XValidation` | Provider-type constraints, source-controller cross-field rules | **Adopt** — CRD OpenAPI + webhooks |
| Receiver | Inbound webhook → enqueue work | **Defer** — reserve `KollectReceiver` (ADR-0022) |
| Interval reconciliation | Sources reconcile on `spec.interval` | **Reject as primary** — collection is event-driven (ADR-0014) |
| CEL in runtime | `commitStatusExpr` on Provider | **Adopt** — attribute predicates + future notification hooks |

### Argo CD

| Pattern | Finding | kollect stance |
| --- | --- | --- |
| AppProject | Allowed repos, destinations, resource GVKs, RBAC roles | **Adopt (Phase 3)** — `KollectScope` |
| ApplicationSet generators | Matrix/git/cluster generators → many Applications | **Defer** — reserve `KollectTargetSet` |
| Status conditions | `SetConditions` with evaluated types, health aggregation | **Adopt** — `Ready`/`Synced`/`Degraded` + `observedGeneration` |
| Application status size | Summaries + revision, not full manifest in status | **Adopt** — reinforces ADR-0006 |

### kube-state-metrics

| Pattern | Finding | kollect stance |
| --- | --- | --- |
| CustomResourceStateMetrics | Config-driven GVK → Prometheus from informer paths | **Adopt (Phase 4)** — metrics backend shape |
| Generic informers | Dynamic GVK registration, path-based label extraction | **Adopt** — validates `KollectTarget` engine |
| No persistence | Metrics served from cache, not etcd | **Adopt** — no inventory payload in status |
| Metrics-only scope | Observability, not stakeholder docs | **Reject as sole solution** — kollect also exports to Git/docs |

### Other cited projects (not cloned)

- **git-change-operator** — resource→Git field extraction; validate sink ergonomics when URL confirmed.
- **krateo resources-ingester** — dynamic CRD discovery + informers → DB; informs engine, not CRD shape.
- **crust-gather / influxdata/sinker** — full-state gather / JSONPath mapping; ideas for aggregation.

## Decision

Lean on OSS patterns rather than reinvent:

1. **Static config + reconciled workload split** (Flux).
2. **Backend registry** (external-secrets provider model, simplified).
3. **Tenancy boundary** (Argo AppProject → `KollectScope`).
4. **Event-driven informers** (kube-state-metrics + controller-runtime).
5. **Status as summary** (all mature refs).
6. **Helm/CI/docs toolchain** (external-secrets + Flux release hygiene).

kollect's unique value is the **combination** plus **stakeholder-facing Git/doc export** for
developer portals and audit trails.

## Consequences

### Positive

- Reduces design risk; every major choice has a production precedent.
- Clear "not invented here" boundaries speed implementation reviews.

### Negative

- Mixing patterns from GitOps (Flux/Argo), secrets (ESO), and metrics (KSM) requires careful
  documentation so users understand kollect is an **inventory/doc-sync** operator, not GitOps.

## Open questions

- **OPEN:** Phase 2 `KollectPublication` vs Phase 1 **JSON-only Git export** — is templating/Confluence
  premature before plain JSON proves the portal use case?
- **OPEN:** Multi-tenant: one cluster-scoped operator + `KollectScope`, or namespaced operator
  deployments per team (ESO `controller` field pattern)?
- **OPEN:** Confirm git-change-operator URL for deeper sink CRD comparison.
