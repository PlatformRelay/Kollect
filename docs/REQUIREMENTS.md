# kollect — product requirements

Binding requirements for kollect beyond engineering guidelines
([GUIDELINES.md](https://github.com/konih/kollect/blob/main/GUIDELINES.md)).
**Build order** is not a release train — see [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md).

## MVP (build first)

| Requirement | Rationale |
| --- | --- |
| **Namespaced tenancy** | `KollectProfile`, **`KollectSink`**, `KollectTarget`, `KollectInventory` in team namespace — [ADR-0032](adr/0032-platform-architecture-pivot.md) |
| **Collect → aggregate → export** | Deployment (or one GVK) through to **Postgres or Kafka** sink |
| **Shared informer per GVK** | Scalability — [ADR-0014](adr/0014-event-driven-informers.md) |
| **Default install: `tenantMode`** | Per-team Helm + `watchNamespaces` — [ADR-0016](adr/0016-namespaced-multi-tenancy.md) |
| **Export debouncing** | Coalesce sink writes under event load — [ADR-0032](adr/0032-platform-architecture-pivot.md) |
| **`KollectConnectionTest` CR** | Audited connectivity probes — [ADR-0032](adr/0032-platform-architecture-pivot.md) |

Implemented code for extended features (HTTP, hub, extra sinks) **remains**; MVP success is the export path above.

## Core engineering (parallel / after MVP)

| Requirement | Rationale |
| --- | --- |
| **Custom / self-signed CA TLS** for Git and GitLab sinks | Internal CAs — human-user-0 without `insecureSkipVerify` |
| **Validating webhooks early** | Reject invalid CEL/JSONPath and sink config at admission |
| **Helm chart day 1** | `helm template`, unittest, schema in CI |
| **Prometheus metrics early** | `/metrics` in CI — [ADR-0012](adr/0012-prometheus-metrics-stub.md) |
| **Connection test on Sink** | `connectionTest: false` prod default; annotation for ad-hoc — [ADR-0030](adr/0030-connection-test.md) |
| **Postgres + Kafka sinks** | Primary integration backends — [ADR-0025](adr/0025-sink-backends-database-kafka.md) |
| **`KollectScope` enforcement** | Webhook + reconciler — [ADR-0016](adr/0016-namespaced-multi-tenancy.md) |
| **Tested samples + contract tests** | Deployment, Argo `Application` helm sample + **contract test** — [ADR-0027](adr/0027-helm-release-inventory.md) |
| **Watch opt-in/out** | `watchMode: All` and `OptIn`; `kollect.dev/watch` labels — [ADR-0029](adr/0029-watch-labels.md) |

## Optional / debug

| Requirement | Rationale |
| --- | --- |
| **HTTP inventory API** | **Feature-gated, default off** — debug and small installs only; not portal scale path — [ADR-0032](adr/0032-platform-architecture-pivot.md), [ADR-0006](adr/0006-etcd-limit.md) |
| **Inventory HTTP auth** | When HTTP enabled: TokenReview + SAR — [ADR-0024](adr/0024-inventory-api-auth.md) |
| **Git sink samples** | Audit/diff and CI determinism — not primary portal narrative |

## Multi-cluster (build order — not MVP blocker)

| Requirement | Rationale |
| --- | --- |
| **Hub `mode: hub\|spoke`** | Same image; **no `KollectHub` CRD** — [ADR-0022](adr/0022-multi-cluster-sync-rfc.md), [ADR-0032](adr/0032-platform-architecture-pivot.md) |
| **`internal/hub/` merge** | Hub Postgres/Kafka as portal read path |
| **Lean queue transport** | `inprocess` only default — [ADR-0023](adr/0023-lean-queue-transport.md) |
| **Hub auth** | Push-first — [ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md) |

## Performance

See [ADR-0026](adr/0026-performance-scalability.md) and [PERFORMANCE.md](PERFORMANCE.md).

## Architecture principles

- **Postgres/Kafka primary** for portals; **Git** for audit; **HTTP** optional debug.
- **Schema clarity** — contract is export JSON/rows, not rendered docs.
- **Single responsibility** — no in-cluster doc-sync ([ADR-0011](adr/0011-doc-sync-templating.md)).
- **No adopters on v1alpha1** — breaking API changes acceptable until beta.

## Rejected / deferred

| Item | Rationale |
| --- | --- |
| `KollectPublication` | [ADR-0011](adr/0011-doc-sync-templating.md) |
| `KollectHub` CRD | Helm `mode: hub` — [ADR-0032](adr/0032-platform-architecture-pivot.md) |
| `KollectSink.type: prometheus` | [ADR-0012](adr/0012-prometheus-metrics-stub.md) |
| **`KollectClusterSink`** | Reserved until platform-shared sinks needed |
| JSONPath filters | After core export |

## See also

- [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md) — coordinator brief
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [ROADMAP.md](ROADMAP.md)
