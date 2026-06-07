# ADR-0102: Prior art and OSS reference patterns

> What we adopt, defer, and reject from Flux, external-secrets, kube-state-metrics, and Argo CD.

**Theme:** 01 · Foundations · **Status:** Current (living — update as we learn)

## Context

Kollect occupies a niche: **generic attribute-selection CRD + resource-selection CRD + aggregation +
multi-backend export** (Git, object storage, Postgres, Kafka). No single OSS project combines all
of these. **Doc-sync / Confluence publication is rejected** ([ADR-0702](0702-doc-sync-templating.md)). Local shallow clones
under `references/oss/` (read-only, never shipped) inform CRD ergonomics, informers, sinks, CI, and
metrics. This ADR records what we **adopt**, **defer**, and **reject** after sampling those repos.

Primary OSS references we **actually use** for design and CI patterns:

- **Flux** (source-controller, notification-controller) — static Provider, suspend, CEL validations
- **external-secrets (ESO)** — provider registry, Helm chart hygiene, status without secret bytes
- **kube-state-metrics (KSM)** — config-driven GVK → Prometheus; generic informers
- **Argo CD** — AppProject tenancy, Application status conditions

## OSS comparison summary

### external-secrets

| Pattern | Finding | Kollect stance |
| --- | --- | --- |
| Provider plugin registry | `SecretStoreProvider` discriminated union + Go provider packages | **Adopt** — `KollectSink` `type` + internal registry/factory ([ADR-0402](0402-sink-backends-database-kafka.md)) |
| Cluster vs namespaced stores | `ClusterSecretStore` + namespace `conditions` | **Adopt (Phase 1)** — `KollectScope` + optional `watchNamespaces` / `tenantMode` ([ADR-0203](0203-namespaced-multi-tenancy.md)) |
| Helm/CI | `helm-docs`, `helm-unittest`, `values.schema.json`, `helm template` → manifests | **Adopt** — Helm chart **day 1** ([REQUIREMENTS.md](../REQUIREMENTS.md)) |
| Status content | Sync metadata only, never secret bytes | **Adopt** — aligns with ADR-0103 |
| Reconciled SecretStore | ESO reconciles stores for validation/status | **Reject for Profile/Sink** — follow Flux static config (ADR-0202) |

### Flux (source-controller + notification-controller)

| Pattern | Finding | Kollect stance |
| --- | --- | --- |
| Static Provider/Alert | No status subresource, no Provider reconciler | **Adopt** — `KollectProfile`, `KollectSink` static |
| `spec.suspend` | On all reconciled sources | **Adopt** — all reconciled Kollect kinds |
| CEL `XValidation` | Provider-type constraints, source-controller cross-field rules | **Adopt** — CRD OpenAPI + **validating webhooks early** |
| Receiver | Inbound webhook → enqueue work | **Defer** — reserve `KollectReceiver` |
| Interval reconciliation | Sources reconcile on `spec.interval` | **Reject as primary** — collection is event-driven (ADR-0301) |
| CEL in runtime | `commitStatusExpr` on Provider | **Adopt** — attribute predicates + future notification hooks |

### Argo CD

| Pattern | Finding | Kollect stance |
| --- | --- | --- |
| AppProject | Allowed repos, destinations, resource GVKs, RBAC roles | **Adopt (Phase 1)** — namespaced `KollectScope` ([ADR-0203](0203-namespaced-multi-tenancy.md)) |
| ApplicationSet generators | Matrix/git/cluster generators → many Applications | **Defer** — reserve `KollectTargetSet` |
| Status conditions | `SetConditions` with evaluated types, health aggregation | **Adopt** — `Ready`/`Synced`/`Degraded` + `observedGeneration` |
| Application status size | Summaries + revision, not full manifest in status | **Adopt** — reinforces ADR-0103 |

### kube-state-metrics

| Pattern | Finding | Kollect stance |
| --- | --- | --- |
| CustomResourceStateMetrics | Config-driven GVK → Prometheus from informer paths | **Adopt (early)** — metrics testable from Phase 0/1 |
| Generic informers | Dynamic GVK registration, path-based label extraction | **Adopt** — validates `KollectTarget` engine |
| No persistence | Metrics served from cache, not etcd | **Adopt** — no inventory payload in status |
| Metrics-only scope | Observability, not stakeholder docs | **Reject as sole solution** — Kollect also exports to Git/docs |

### Other cited projects (not primary clones)

- **krateo resources-ingester** — dynamic CRD discovery + informers → DB; informs engine, not CRD shape.
- **crust-gather / influxdata/sinker** — full-state gather / JSONPath mapping; ideas for aggregation.

### Rejected

| Project | Reason |
| --- | --- |
| **git-change-operator** | Single-star OSS, unmaintained; not worth CRD comparison — Git sink ergonomics validated via Gitea/testcontainers and Flux/Git patterns instead |

## Decision

Lean on OSS patterns rather than reinvent:

1. **Static config + reconciled workload split** (Flux).
2. **Backend registry** (ESO provider model, simplified).
3. **Tenancy boundary** (Argo AppProject → `KollectScope`).
4. **Event-driven informers** (KSM + controller-runtime).
5. **Status as summary** (all mature refs).
6. **Helm/CI/docs toolchain** (ESO + Flux release hygiene).
7. **Reject `KollectPublication` / Confluence doc-sync** — templating and CMS push belong in external
   CI consuming Git or Kafka/Postgres export ([ADR-0702](0702-doc-sync-templating.md)).
8. **Add Postgres + Kafka sinks** as first-class export targets ([ADR-0402](0402-sink-backends-database-kafka.md)).

Kollect's unique value is the **combination** plus **stakeholder-facing export** (Git, HTTP, Postgres,
Kafka) with **multi-cluster aggregation** ([ADR-0501](0501-multi-cluster-fleet.md)) without
per-cluster export noise.

## Consequences

### Positive

- Reduces design risk; every major choice has a production precedent.
- Clear "not invented here" boundaries speed implementation reviews.
- Dropping git-change-operator avoids chasing dead prior art.

### Negative

- Mixing patterns from GitOps (Flux/Argo), secrets (ESO), and metrics (KSM) requires careful
  documentation so users understand Kollect is an **inventory export** operator, not GitOps or a CMS.

## Open questions

- **RESOLVED :** Multi-tenant — **both** deployment models: default cluster-scoped manager
  with namespaced `KollectScope` tenancy, plus optional per-team installs via Helm `watchNamespaces[]`
  and `tenantMode` (ESO scoped-controller pattern). Phase 1 priority — [ADR-0203](0203-namespaced-multi-tenancy.md).
- **RESOLVED :** Helm release sample — Flux `HelmRelease` summary profile default;
  values profile gated + redacted ([ADR-0303](0303-helm-release-inventory.md)).
