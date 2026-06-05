# kollect roadmap

Phased delivery plan for [kollect](https://github.com/konih/kollect) — a Kubernetes inventory
operator that watches arbitrary GVKs, aggregates extracted attributes, and exports to pluggable
sinks (Git, object storage, Postgres, Kafka) with a read-only HTTP API for portals.

**Last updated:** 2026-06-05

## Status legend

| Mark | Meaning |
| --- | --- |
| ✅ | Done |
| 🚧 | In progress |
| ⬜ | Planned |
| 🔮 | Deferred |
| ❓ | Open decision |

## Phase overview

```mermaid
flowchart LR
  P0[Phase 0<br/>Bootstrap]
  P1[Phase 1<br/>Collection + Sink]
  P2[Phase 2<br/>Hub / multi-cluster]
  P3[Phase 3<br/>Governance + scope]
  P4[Phase 4<br/>Metrics + aggregation]
  P0 --> P1
  P1 --> P2
  P1 --> P3
  P2 --> P3
  P3 --> P4
```

| Phase | Focus | Summary |
| --- | --- | --- |
| **0** | Bootstrap | Scaffold, guidelines, ADRs, Helm, CI, webhooks, metrics, docs |
| **1** | Collection + Sink | Dynamic informers, CEL/JSONPath, namespaced inventory, Git/HTTP export |
| **2** | Multi-cluster | `KollectHub` CRD, spoke agents, pluggable lean queue fan-in |
| **3** | Governance | `KollectScope`, S3/GCS hardening, cluster inventory |
| **4** | Metrics + aggregation | kube-state-metrics-style config, richer rollups |

See [ARCHITECTURE.md](ARCHITECTURE.md), [REQUIREMENTS.md](REQUIREMENTS.md), and
[adr/README.md](adr/README.md) for design detail.

---

## Phase 0 — Bootstrap

| Item | Status |
| --- | --- |
| Kubebuilder v4 project scaffold | ✅ |
| MIT license | ✅ |
| CRDs: `KollectProfile`, `KollectSink`, `KollectTarget`, `KollectInventory` | ✅ |
| Taskfile, verify gate, golangci-lint, pre-commit, gitleaks | ✅ |
| CI: preflight, verify, lint, test, build, container image | ✅ |
| Helm chart (`charts/kollect/`) | ✅ |
| Helm docs / unittest / `values.schema.json` in CI | ⬜ |
| Core documentation + MkDocs (GitHub Pages) | ✅ |
| Architecture Decision Records (core set) | 🚧 |
| ADR-0026 performance & scalability | ✅ |
| `GUIDELINES.md`, `SECURITY.md`, `CONTRIBUTING.md` | ✅ |
| Validating webhook — Profile CEL/JSONPath | ✅ |
| Validating webhook — Profile Secret.data guard | ✅ |
| Validating webhook — Sink type enum | ⬜ |
| Prometheus custom metrics (early) | 🚧 |
| Connection test infrastructure | ✅ ([ADR-0030](adr/0030-connection-test.md)) |
| Namespaced `KollectProfile` API | ✅ ([ADR-0031](adr/0031-namespaced-profiles.md)) |
| Golden OpenAPI contract tests (`test/schema/`) | ⬜ |
| Kind smoke / operator deploy | ✅ |
| Release pipeline (SBOM, signing) | ⬜ |
| Public demo Git inventory repo | ✅ |

**Counts:** ✅ 15 · 🚧 2 · ⬜ 6

---

## Phase 1 — Collection + Sink + HTTP

| Item | Status |
| --- | --- |
| CEL + JSONPath attribute extractor | ✅ |
| Dynamic informer engine (per Profile GVK) | ✅ |
| In-memory collection store + namespace aggregation | ✅ |
| `KollectTarget` controller | ✅ |
| `KollectInventory` controller (namespaced rollup + export) | 🚧 |
| Event-driven path: informer changes → inventory export | 🚧 |
| Sink registry (factory by `type`) | ✅ |
| Git sink with custom CA TLS | ✅ |
| GitLab sink | ⬜ |
| S3 sink | 🚧 |
| Postgres sink (`type: postgres`) | ✅ |
| Kafka export sink (`type: kafka`) | ✅ |
| Postgres/Kafka testcontainers in CI | ✅ |
| SAR / RBAC scope degradation | ✅ |
| Typed reconcile errors + circuit breakers | ⬜ |
| Parallel reconcile workers (`MaxConcurrentReconciles`) | ✅ |
| Workqueue depth + reconcile latency metrics | ✅ |
| pprof server (feature-gated `:6060`) | ✅ |
| `task bench` / `task load-test` (bounded scale tests) | ✅ |
| Secondary watches (Profile/Sink changes) | ⬜ |
| Finalizers | ⬜ |
| Read-only HTTP `GET /inventory` (+ SSE watch) | ✅ |
| Inventory HTTP auth: TokenReview + SAR (K8s bearer) | ✅ |
| `--inventory-auth-mode=kubernetes` (default) | ✅ |
| Full Prometheus metrics per [ADR-0020](adr/0020-error-taxonomy.md) | ✅ |
| Sample profiles: Deployment, Service, Ingress | ✅ |
| Sample profile: Helm release summary (Flux `HelmRelease`) | ✅ |
| Helm values profile + operator scrub | ⬜ |
| `helm:` decode for `helm.sh/v1` Secret releases | ⬜ |
| Sample: generic CRD | ⬜ |
| Sample contract tests in CI | 🚧 |
| Integration tests (testcontainers) in CI | 🚧 |
| End-to-end: install → collect → export → HTTP | 🚧 |
| `spec.suspend` on reconciled kinds | ✅ |
| **Multi-tenant (ASAP):** `watchNamespaces` / `tenantMode` Helm + `--watch-namespaces` | 🚧 |
| **Multi-tenant:** `KollectScope` webhook + reconciler enforcement + sample | 🚧 |
| **Multi-tenant e2e:** dynamic `kollect-tenant-a` / `kollect-tenant-b` isolation | 🚧 |
| Inventory namespace isolation unit tests | 🚧 |

**Counts:** ✅ 15 · 🚧 9 · ⬜ 14

---

## Phase 2 — Hub / multi-cluster

Multi-cluster support must **not** block single-cluster installs. Design for **100+ clusters**
(60 is not the ceiling) and **giant spokes** (10k+ resources). Hub **shards and aggregates** —
never O(spokes²). See [ADR-0022](adr/0022-multi-cluster-sync-rfc.md) and
[ADR-0023](adr/0023-lean-queue-transport.md).

| Item | Status |
| --- | --- |
| Multi-cluster topology RFC | ✅ |
| Lean queue transport ADR (pluggable factory) | ✅ |
| `KollectHub` CRD (`spec.transport.type`) | ✅ |
| Spoke operator / agent snapshot reports (lightweight, delta) | ⬜ |
| Hub merge and deduplication (O(rows), sharded consumers) | ⬜ |
| Transport: in-process (dev/test default) | ✅ |
| Transport: Redis Streams (Phase 2 spike, explicit opt-in) | ✅ |
| Transport: NATS JetStream (config alternative) | 🚧 |
| Transport: Kafka backend (optional, integration-tested) | 🔮 |
| Cross-cluster authentication (Istio-style + push TokenReview) | ✅ |
| `KollectRemoteCluster` CRD (hub registration stub) | ✅ |
| Spoke HTTP push auth (`Bearer` + `X-Kollect-Cluster-Id`) | ✅ |
| Hub ingest HTTP stub (`POST /hub/v1alpha1/reports`) | ✅ |

**Counts:** ✅ 10 · 🚧 1 · ⬜ 4

---

## Phase 3 — Governance + backends

| Item | Status |
| --- | --- |
| `KollectScope` reconciler-time enforcement (Phase 1) | 🚧 |
| `KollectScope` admission webhook | 🚧 |
| `KollectClusterScope` (platform teams) | 🔮 |
| `KollectClusterInventory` (platform rollup) | ⬜ |
| `KollectClusterSink` / namespaced sink split | 🔮 |
| GCS sink | ✅ |
| S3 sink CI hardening | 🚧 |
| `KollectReceiver` / `KollectTargetSet` (design only) | 🔮 |

**Counts:** ✅ 2 · 🚧 3 · ⬜ 1 · 🔮 4

---

## Phase 4 — Metrics + aggregation

| Item | Status |
| --- | --- |
| kube-state-metrics-style custom resource metrics config | ⬜ |
| Cardinality-safe operator metrics (counts, export latency) | ✅ |
| Advanced cross-target / cross-cluster aggregation | ⬜ |

**Counts:** ✅ 1 · ⬜ 3

---

## Performance and scalability

Cross-cutting NFRs accepted in [ADR-0026](adr/0026-performance-scalability.md). Tuning guide:
[PERFORMANCE.md](PERFORMANCE.md).

### Scale targets

| Target | Value | ADR |
| --- | --- | --- |
| Watched objects per spoke (baseline) | **10,000+** | [ADR-0026](adr/0026-performance-scalability.md) |
| Giant single cluster | 1000+ nodes, 10k+ resources | [ADR-0026](adr/0026-performance-scalability.md) |
| Hub spoke count | **100+** (not capped at 60) | [ADR-0022](adr/0022-multi-cluster-sync-rfc.md) |
| Spoke working set (typical profiles) | ≤512 MiB at 10k rows | [ADR-0026](adr/0026-performance-scalability.md) |
| Hub merge complexity | O(total rows), sharded | [ADR-0022](adr/0022-multi-cluster-sync-rfc.md) |

### Developer perf tooling

| Item | Status |
| --- | --- |
| Metrics catalog + PromQL hints in PERFORMANCE.md | ✅ |
| `task perf-report` + `hack/perf-report.sh` | ✅ |
| `artifacts/bench/` from `task bench` | ✅ |
| CI upload of bench artifacts (nightly, optional) | ⬜ |

**Counts:** ✅ 3 · ⬜ 1

### Operator tuning and tests

| Item | Status |
| --- | --- |
| Scale target documented (10k+ objects per spoke) | ✅ |
| 100+ cluster hub path documented | ✅ |
| Bounded test tiers (500 default / 2000 opt-in load) | ✅ |
| `task bench` (Go benchmarks, `-short`) | ✅ |
| `task load-test` (`KOLECT_LOAD_TEST=1`, `-tags=load`) | ✅ |
| `--max-concurrent-reconciles-*` flags + Helm values | ✅ |
| `--export-debounce` / `--reconcile-rate-limit` flags | ✅ |
| `--informer-resync-period` flag | ⬜ |
| pprof on `:6060` (feature gate) | ✅ |
| `kollect_workqueue_depth` / `kollect_reconcile_duration_seconds` metrics | ✅ |
| `kollect_informer_objects` / `kollect_export_bytes_total` metrics | ✅ |
| `BenchmarkExtract` in `internal/collect/` | ✅ |
| envtest synthetic scale harness (cap 500) | ⬜ |
| Load test package (`test/load/`, `-tags=load`) | ✅ |

**Counts:** ✅ 14 · ⬜ 5

---

## Rejected

| Item | Rationale |
| --- | --- |
| `KollectPublication` (Confluence, Go templates, doc-sync) | Out of scope — external CI over Git/Kafka export ([ADR-0011](adr/0011-doc-sync-templating.md)) |
| `KollectSink.type: prometheus` | Operator `/metrics` only — not an inventory export sink ([ADR-0012](adr/0012-prometheus-metrics-stub.md)) |

## Deferred

| Item | When |
| --- | --- |
| `KollectClusterSink` + namespaced `KollectSink` split | Phase 3 — cluster-scoped sinks + `KollectScope.sinkRefs` until then ([ADR-0031](adr/0031-namespaced-profiles.md)) |
| Kafka as **required** hub transport | Pluggable optional backend only; `inprocess` default ([ADR-0023](adr/0023-lean-queue-transport.md)) |
| `KollectReceiver`, `KollectTargetSet` implementation | Reserved for future phases |
| oauth2-proxy sidecar (OIDC browser auth) | Optional Helm sidecar (`oauth2Proxy.enabled: false`); K8s bearer auth is primary — [ADR-0024](adr/0024-inventory-api-auth.md) |
| `KollectClusterSink` + namespaced sink split | Phase 3 — `KollectScope.sinkRefs` sufficient for Phase 1 ([ADR-0004](adr/0004-crd-model.md)) |

## Open questions

- **Hub ingest SAR shape** — `create` on `kollectremoteclusters` vs custom URL ([ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md))
- **SinkReachable** on Inventory/Target vs sink-only `ConnectionVerified` ([ADR-0030](adr/0030-connection-test.md))

See [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md) for locked vs still-open items.

## Breaking changes

### Namespaced `KollectInventory` (2026-06-05)

`KollectInventory` is **namespaced**. Each team owns an inventory object in their namespace that
aggregates `KollectTarget`s in the same namespace. Platform-wide rollup is reserved for
`KollectClusterInventory` (cluster-scoped, not yet implemented).

Migration: replace cluster-scoped inventory manifests with namespaced equivalents; update RBAC to
namespace scope where appropriate.

### Namespaced `KollectProfile` (2026-06-05)

`KollectProfile` is **namespaced**. Each `KollectTarget.spec.profileRef` resolves a profile in the
**same namespace** as the Target. Platform-wide shared schemas are reserved for
`KollectClusterProfile` (not yet implemented).

Migration: re-apply profile manifests into each team namespace (or use GitOps templating). Remove
cluster-scoped profile objects before upgrade.

## CI and end-to-end testing

| Item | Status |
| --- | --- |
| PR CI: gitleaks, verify, lint, unit tests, build | ✅ |
| Manual e2e workflow (`workflow_dispatch`) | ✅ |
| Nightly kind smoke (Helm install + sample CRs + HTTP probe) | 🚧 |
| Full e2e: conditions, Git export, HTTP body | 🚧 |
| Integration tests in CI (testcontainers) | 🚧 |

## Architecture decisions (2026-06-05)

Full locked table: **[PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md)**.

| Decision | Status |
| --- | --- |
| Single-cluster MVP is the default install | Accepted |
| Namespaced inventory is the hub input contract | Accepted |
| **`KollectProfile` namespaced**; `KollectClusterProfile` reserved | Accepted ([ADR-0031](adr/0031-namespaced-profiles.md)) |
| **`KollectScope` Phase 1** — webhook + reconciler enforcement | Accepted ([ADR-0016](adr/0016-namespaced-multi-tenancy.md)) |
| Hub-and-spoke via **`KollectHub` CRD last** (L1→L4 layering) | Accepted ([ADR-0022](adr/0022-multi-cluster-sync-rfc.md)) |
| Same image **`mode: hub\|spoke`** before CRD packaging | Accepted ([ADR-0022](adr/0022-multi-cluster-sync-rfc.md)) |
| Transport: **`inprocess` only default**; Redis/NATS/Kafka explicit opt-in | Accepted ([ADR-0023](adr/0023-lean-queue-transport.md)) |
| Transport backend rule: no merge without integration/e2e proof | Accepted |
| Connection test: no dedicated CR; prod `connectionTest: false` | Accepted ([ADR-0030](adr/0030-connection-test.md)) |
| Helm release: `lastAttemptedRevision` + `history[0]` contract test | Accepted ([ADR-0027](adr/0027-helm-release-inventory.md)) |
| Shared informer per GVK | Accepted ([ADR-0014](adr/0014-event-driven-informers.md)) |
| Postgres + Kafka as first-class export sinks | Accepted ([ADR-0025](adr/0025-sink-backends-database-kafka.md)) |
| Doc-sync / `KollectPublication` | Rejected ([ADR-0011](adr/0011-doc-sync-templating.md)) |
| Inventory HTTP auth: **K8s TokenReview + SAR**; `--inventory-auth-mode=kubernetes` default | Accepted |
| oauth2-proxy: **optional** Helm sidecar for OIDC browsers; not primary auth | Accepted |
| Git, object storage, and agent mesh documented as alternatives | Accepted |
| Extreme scale: 100+ clusters, 10k+ objects/spoke, hub shard not O(n²) | Accepted ([ADR-0022](adr/0022-multi-cluster-sync-rfc.md), [ADR-0026](adr/0026-performance-scalability.md)) |
| Hub cluster auth: **Istio remote-secret registration + push TokenReview** | Accepted ([ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md)) |
| Namespaced `KollectProfile`; `profileRef` same namespace | Accepted ([ADR-0031](adr/0031-namespaced-profiles.md)) |
| Connection test: no dedicated CR; `ConnectionVerified` on sink | Accepted ([ADR-0030](adr/0030-connection-test.md)) |
| **`KollectClusterSink` deferred Phase 3** | Deferred |

## Further reading

- [Platform decisions (2026-06-05)](PLATFORM-DECISIONS.md)
- [Product requirements](REQUIREMENTS.md)
- [Architecture](ARCHITECTURE.md)
- [Helm chart README](../charts/kollect/README.md) — inventory HTTP auth
- [ADR-0004: CRD model](adr/0004-crd-model.md)
- [ADR-0006: etcd limit + HTTP API](adr/0006-etcd-limit.md)
- [ADR-0014: Event-driven informers](adr/0014-event-driven-informers.md)
- [ADR-0022: Multi-cluster RFC](adr/0022-multi-cluster-sync-rfc.md)
- [ADR-0023: Lean queue transport](adr/0023-lean-queue-transport.md)
- [ADR-0024: Inventory API auth](adr/0024-inventory-api-auth.md)
- [ADR-0011: Doc-sync rejected](adr/0011-doc-sync-templating.md)
- [ADR-0025: Postgres and Kafka sinks](adr/0025-sink-backends-database-kafka.md)
- [ADR-0026: Performance and scalability](adr/0026-performance-scalability.md)
- [PERFORMANCE.md](PERFORMANCE.md) — tuning guide and metrics catalog
