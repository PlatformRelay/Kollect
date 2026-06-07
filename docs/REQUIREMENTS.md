# Requirements & assumptions

> First-principles requirements for Kollect — **what** the operator must do and **how well**, independent
> of any specific design. ADRs record *how*; this document records *what* and *why*. When an ADR and
> this document disagree, reconcile or flag the drift.

**Status:** living · **Audience:** architects, contributors, and reviewers evaluating or extending Kollect ·
**Not a tutorial** — start with [Understand the basics](UNDERSTAND-THE-BASICS.md) and
[Architecture](ARCHITECTURE.md); locked design choices live in [Platform decisions](PLATFORM-DECISIONS.md).

**Build order, not a release train** — see [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md).

---

## 1. Problem statement

Platform and application teams need **versioned, stakeholder-facing inventory** of what runs in their
Kubernetes clusters — *which* resources exist, with *which* attributes (chart versions, images, sync
status, certificate expiry, …), aggregated into a queryable, auditable form.

From first principles, the existing options each fail one requirement:

| Option | Why it is insufficient |
| --- | --- |
| Query `kube-apiserver` directly | Unbounded list/watch load; no history; couples portals to cluster availability and RBAC |
| Store inventory in CRD `.status` | etcd object-size limit (~1.5 MB); destabilizes apiserver at scale |
| Hardcoded collector schemas | Break whenever a new CRD/attribute is needed; no per-team extensibility |
| kube-state-metrics only | Metrics are observability, not diffable stakeholder inventory |
| Per-cluster Git commits | O(N) commit/export storms across a fleet; noise without aggregation |

**Kollect's thesis:** watch user-defined GVKs via shared informers → extract attributes via
CEL/JSONPath → aggregate in memory → **debounce** → export to **pluggable durable sinks**, so
consumers read **export data**, never the live API at scale.

## 2. Users and assumptions

| Persona | Need |
| --- | --- |
| **Application team** | Inventory of their own namespace's workloads, owned in their namespace |
| **Platform team** | Cross-namespace / cross-cluster rollup with tenancy guardrails |
| **Portal / automation** | A stable, queryable, durable read surface (SQL, object store, or stream) |
| **Auditor** | Diffable, point-in-time history of what was deployed |

Operating assumptions (binding unless revisited):

- **A1 — No external adopters on `v1alpha1`.** Breaking API changes are acceptable pre-beta.
- **A2 — Event-driven, not polling.** Collection reacts to informer events ([ADR-0301](adr/0301-event-driven-informers.md)).
- **A3 — Status is a summary, never a payload store** ([ADR-0103](adr/0103-etcd-limit.md)).
- **A4 — Single responsibility.** Kollect collects and exports; it does **not** render or publish docs/CMS ([ADR-0702](adr/0702-doc-sync-templating.md)).
- **A5 — The in-memory snapshot per inventory is canonical**; every sink is a projection of it ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)).
- **A6 — Internal/self-signed CAs are normal**; TLS trust is a first-class sink concern, not a bolt-on.

## 3. Functional requirements

IDs are stable handles for discussion (`FR-<area>-<n>`).

### 3.1 Configuration & API (FR-API)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-API-1 | Inventory is configured by CRDs, not code: extraction schema (`KollectProfile`), resource selection (`KollectTarget`), aggregation/export (`KollectInventory`), backend (`KollectSink`), tenancy (`KollectScope`) | [ADR-0201](adr/0201-crd-model.md) |
| FR-API-2 | Namespaced-by-default tenancy; cluster-scoped variants for platform-wide use | [ADR-0201](adr/0201-crd-model.md), [ADR-0203](adr/0203-namespaced-multi-tenancy.md) |
| FR-API-3 | Config kinds (`Profile`, `Scope`) are static (no controller); work kinds are reconciled | [ADR-0202](adr/0202-static-vs-reconciled.md) |
| FR-API-4 | Invalid CEL/JSONPath and unknown sink types are rejected at admission, not at runtime | [ADR-0201](adr/0201-crd-model.md), [ADR-0302](adr/0302-cel-jsonpath-extraction.md) |
| FR-API-5 | Every reconciled kind supports `spec.suspend` and a manual-trigger annotation | [ADR-0201](adr/0201-crd-model.md) |

### 3.2 Collection & extraction (FR-COL)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-COL-1 | Watch arbitrary GVKs declared by profiles via **one shared dynamic informer per GVK** | [ADR-0301](adr/0301-event-driven-informers.md) |
| FR-COL-2 | Extract named attributes via JSONPath (incl. `[*]` array wildcard) and `cel:`-prefixed CEL | [ADR-0302](adr/0302-cel-jsonpath-extraction.md) |
| FR-COL-3 | Scope watches by namespace/label selector and name lists to bound memory | [ADR-0301](adr/0301-event-driven-informers.md) |
| FR-COL-4 | Watch opt-in/opt-out via `kollect.dev/watch` labels and `watchMode: All\|OptIn` | [ADR-0205](adr/0205-watch-labels.md) |
| FR-COL-5 | Never extract `Secret.data` (incl. Helm `data.release`) without explicit opt-in; redact sensitive keys | [ADR-0303](adr/0303-helm-release-inventory.md) |
| FR-COL-6 | Ship tested sample profiles + contract tests (Deployment, Argo `Application`, cert-manager `Certificate`, …) | [ADR-0301](adr/0301-event-driven-informers.md), [ADR-0303](adr/0303-helm-release-inventory.md) |

### 3.3 Aggregation & export (FR-EXP)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-EXP-1 | Aggregate target rows into a per-namespace `KollectInventory` snapshot | [ADR-0201](adr/0201-crd-model.md) |
| FR-EXP-2 | Coalesce identical exports via `spec.exportMinInterval` (default 30s); material changes bypass | [ADR-0201](adr/0201-crd-model.md), [ADR-0603](adr/0603-performance-scalability.md) |
| FR-EXP-3 | Deterministic, stable-ordered serialization (diffable Git, golden tests) | [ADR-0103](adr/0103-etcd-limit.md) |
| FR-EXP-4 | Pluggable sinks by **role**: snapshot store (Git, S3/GCS Parquet, HTTP), relational SoR (Postgres), event emitter (NATS, Kafka) | [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md), [ADR-0402](adr/0402-sink-backends-database-kafka.md) |
| FR-EXP-5 | Resource deletions are reflected in sinks (snapshot stores free; Postgres/Kafka via reconcile) | [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md) |
| FR-EXP-6 | First-class sink connectivity testing (`KollectConnectionTest` CR + sink probe) | [ADR-0403](adr/0403-connection-test.md) |
| FR-EXP-7 | Custom CA / self-signed TLS trust for Git/GitLab/Postgres sinks (`caSecretRef` / `caBundle`) | [ADR-0201](adr/0201-crd-model.md) |

### 3.4 Read path (FR-READ)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-READ-1 | Primary scalable read = sink export (SQL/object store/stream), **not** the live API | [ADR-0103](adr/0103-etcd-limit.md) |
| FR-READ-2 | Optional read-only HTTP inventory API, **feature-gated off by default**, for debug/small installs | [ADR-0103](adr/0103-etcd-limit.md) |
| FR-READ-3 | When HTTP is enabled, authenticate via Kubernetes TokenReview + SubjectAccessReview | [ADR-0404](adr/0404-inventory-api-auth.md) |

### 3.5 Multi-cluster (FR-MC)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-MC-1 | Default multi-cluster = direct shared-sink fan-in (`spec.cluster`); backend key/PK merges | [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md), [ADR-0501](adr/0501-multi-cluster-fleet.md) |
| FR-MC-2 | Each cluster runs an independent operator (`mode: single`); no hub tier | [ADR-0501](adr/0501-multi-cluster-fleet.md) |
| FR-MC-3 | Operators stay lightweight: debounced export, bounded in-memory collect store | [ADR-0501](adr/0501-multi-cluster-fleet.md), [ADR-0603](adr/0603-performance-scalability.md) |

### 3.6 Observability (FR-OBS)

| ID | Requirement | Reference |
| --- | --- | --- |
| FR-OBS-1 | Operator Prometheus metrics on `/metrics` (reconcile, export, sink errors, collection counts) | [ADR-0601](adr/0601-prometheus-metrics-stub.md), [ADR-0602](adr/0602-error-taxonomy.md) |
| FR-OBS-2 | Typed error taxonomy drives requeue behavior and conditions (`Ready`/`Synced`/`Degraded`) | [ADR-0602](adr/0602-error-taxonomy.md) |
| FR-OBS-3 | Operators can tell **why** export failed from conditions, events, and metrics | [ADR-0602](adr/0602-error-taxonomy.md), [ADR-0403](adr/0403-connection-test.md) |
| FR-OBS-4 | `prometheus` is **not** a sink type; domain (KSM-style) metrics emit from the collection engine | [ADR-0601](adr/0601-prometheus-metrics-stub.md), [ADR-0304](adr/0304-custom-resource-aggregation-rfc.md) |

## 4. Non-functional requirements

### 4.1 Performance & scale (NFR-PERF) — [ADR-0603](adr/0603-performance-scalability.md)

| ID | Target |
| --- | --- |
| NFR-PERF-1 | Baseline single spoke: **10,000+** watched objects; collection store working set **≤512 MiB** |
| NFR-PERF-2 | Giant cluster: 1000+ nodes — scoped informers + paginated list mandatory |
| NFR-PERF-3 | Hub fleet: **100–500+** clusters; merge cost **O(total rows)**, never O(spokes²) |
| NFR-PERF-4 | One shared informer per GVK; memory scales with objects × GVKs, not with target count |
| NFR-PERF-5 | Export load bounded by debounce; spill oversized payloads to object store, never etcd |
| NFR-PERF-6 | Tunable `MaxConcurrentReconciles`; observable queue depth |

### 4.2 Reliability & correctness (NFR-REL)

| ID | Requirement |
| --- | --- |
| NFR-REL-1 | Idempotent, level-based reconcile; safe to repeat |
| NFR-REL-2 | At-least-once export; sinks idempotent on `(cluster, ns, name, uid)` |
| NFR-REL-3 | No reconcile spin on terminal errors; exponential backoff + jitter on transient |
| NFR-REL-4 | Degrade (not crash) under partial RBAC; record `skipped:forbidden` |
| NFR-REL-5 | Pod restart loses only in-memory cache; rebuilt from informer resync |

Enforcement patterns: [guidelines § 1–2](development/guidelines.md#2-robustness-and-reliability).

### 4.3 Security (NFR-SEC)

| ID | Requirement |
| --- | --- |
| NFR-SEC-1 | Credentials only via `secretRef`; never in spec/status/logs |
| NFR-SEC-2 | Default verify TLS; `insecureSkipVerify` opt-in and surfaced in status |
| NFR-SEC-3 | Tenancy enforced by `KollectScope` (hard degrade) + SAR; least-privilege RBAC |
| NFR-SEC-4 | Sensitive-key redaction before export; no secret material in inventory |
| NFR-SEC-5 | Distroless nonroot image; minimal attack surface |

Enforcement: [guidelines § 3](development/guidelines.md#3-security),
[coding-standards.md § Security](development/coding-standards.md#security).

### 4.4 Operability (NFR-OPS)

| ID | Requirement |
| --- | --- |
| NFR-OPS-1 | Helm chart day one; `tenantMode` + `watchNamespaces` default per-team install |
| NFR-OPS-2 | Feature gates default to safe values (HTTP off, profiling off, `connectionTest` off in prod) |
| NFR-OPS-3 | Clear, sanitized, actionable condition/error messages |
| NFR-OPS-4 | No hard dependency on Kafka/NATS/Postgres for install or CI (`inprocess` defaults) |

### 4.5 Extensibility & compatibility (NFR-EXT)

| ID | Requirement |
| --- | --- |
| NFR-EXT-1 | New sink backends register via a factory; no vendor SDK in reconcilers |
| NFR-EXT-2 | New GVKs need no codegen — profile-driven |
| NFR-EXT-3 | A sink backend ships only when integration/e2e-testable (testcontainers or kind sidecar) |
| NFR-EXT-4 | CRD enums/conditions evolve via OpenAPI; pre-beta breaking changes allowed (A1) |

### 4.6 Testability (NFR-TEST)

| ID | Requirement |
| --- | --- |
| NFR-TEST-1 | Extraction + error classes covered by table-driven unit tests (no cluster) |
| NFR-TEST-2 | Samples double as contract/regression tests; breaking extraction fails CI |
| NFR-TEST-3 | Scheduled full-path e2e (install → apply samples → assert conditions/export) |
| NFR-TEST-4 | Codegen drift gate (`task verify`) green at every commit |

Enforcement: [guidelines § 4](development/guidelines.md#4-testing),
[testing.md](development/testing.md), [coding-standards.md § Testing](development/coding-standards.md#testing).

## 5. Explicit non-goals

| Non-goal | Rationale |
| --- | --- |
| In-operator doc/CMS rendering (Confluence, wiki, templating) | Single responsibility — external CI consumes exports ([ADR-0702](adr/0702-doc-sync-templating.md)) |
| `prometheus` as a `KollectSink.type` | Operator metrics use `/metrics`; avoids scrape/sink confusion ([ADR-0601](adr/0601-prometheus-metrics-stub.md)) |
| `KollectHub` CRD | Never shipped — hub tier removed ([ADR-0501](adr/0501-multi-cluster-fleet.md)) |
| Full inventory payload in CRD status | etcd limit ([ADR-0103](adr/0103-etcd-limit.md)) |
| Pairwise agent mesh beyond ~20 peers | Does not scale; use shared sink ([ADR-0501](adr/0501-multi-cluster-fleet.md)) |
| In-place ACID lakehouse updates (Iceberg/DuckLake) | Kollect overwrites whole snapshots; no catalog/metadata DB needed ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)) |

## 6. Resolved requirement questions (2026-06-05)

- **Payload spill:** object-store spill is **mandatory above 1 MiB** (warn at 1 MiB; hard cap
  ~1.5 MiB `maxExportBytes`) ([ADR-0103](adr/0103-etcd-limit.md)).
- **Delivery semantics:** **at-least-once + idempotent** (effectively-once for state); exactly-once is a
  non-goal (ADR-0502).
- **Parquet schema:** **hybrid** — typed identity columns + JSON `attributes` + a promoted hot-attribute
  allowlist ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)).
- **Cluster-scoped under OptIn:** honor a **target-level default opt-in**, per-object opt-out wins
  ([ADR-0205](adr/0205-watch-labels.md)).

## See also

- [Engineering guidelines](development/guidelines.md) — operator engineering principles (enforcement of NFRs)
- [coding-standards.md](development/coding-standards.md) — Go lint, testing, and CI gates
- [CONTRIBUTING.md](../CONTRIBUTING.md) — contribution process
- [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md) — coordinator brief (locked decisions)
- [ARCHITECTURE.md](ARCHITECTURE.md) — system overview
- [adr/README.md](adr/README.md) — decision records, grouped by theme
- [ROADMAP.md](ROADMAP.md) · [PERFORMANCE.md](PERFORMANCE.md)
