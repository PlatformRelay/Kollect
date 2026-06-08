# Kollect roadmap

Phased delivery plan for [Kollect](https://github.com/konih/kollect) — a Kubernetes inventory
operator that watches arbitrary GVKs, aggregates extracted attributes, and exports to **role-based
pluggable sinks** — **`KollectSnapshotSink`** (Git, GitLab, S3, GCS), **`KollectDatabaseSink`**
(Postgres, MongoDB), and **`KollectEventSink`** (Kafka) — with optional HTTP for debug. The
in-memory snapshot is canonical; every sink is a projection ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)).

**Build order, not releases** — see [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md), ADR-0703 (archived).

!!! warning "Pre-beta"
    Kollect is not GA. API shapes and sink backends may change until the project reaches
    beta-quality overall. Check status marks (✅ / 🚧 / ⬜) before relying on a feature in production.

!!! info "Phases vs releases"
    Phases describe **implementation order**, not semver milestones. Items may land out of phase
    when dependencies allow; deferred (🔮) items are explicitly not on the near-term path.

**Last updated:** 2026-06-08 (**`v0.5.0`** shipped — sink config + export tranche; Read API freeze still ⬜;
see [RELEASE.md](RELEASE.md#versioning-policy))

!!! tip "Versioning"
    Semver milestones (0.2 → 0.10) track **release tranches**, not build phases. Phases 0–4 below
    describe **implementation order**. See [RELEASE.md — Versioning policy](RELEASE.md#versioning-policy).

## Top priority — Full resource export + pruning (ADR-0306)

**#1 build item.** Full-resource export lets a profile snapshot an entire target object (minus noise)
instead of hand-authoring every attribute — the foundation for audit/drift snapshots, exploratory
profiles, and GitOps debugging. It precedes the Fleet UI, Read API freeze, and remaining sink work.

See [ADR-0306](adr/0306-full-resource-export-pruning.md) — **Accepted; Phase 1 ✅ on `main`**
(post-**`v0.5.0`** tag — listed under Unreleased in [CHANGELOG.md](../CHANGELOG.md) until next release).

| Scope item | Status |
| --- | --- |
| `spec.export` block on `KollectProfile` / `KollectClusterProfile` (`mode`, `as`, `include`, `dedupeIdentity`) | ✅ |
| Collector serializes pruned informer object when `export.mode: Resource` | ✅ |
| Built-in defaults pruning (`prune.defaults`: managedFields, resourceVersion, generation, last-applied-config) | ✅ |
| Argo-style `prune.jsonPointers` (RFC 6901) + JSONPath subset `prune.jsonPaths` | ✅ |
| `prune.scrubKeys` merged with operator scrubKeys + integration with scrub/redaction stack ([ADR-0303](adr/0303-helm-release-inventory.md), [ADR-0104](adr/0104-security-model.md)) | ✅ |
| Admission guard: Secret/sensitive kinds require `kollect.dev/allow-full-resource-export` annotation | ✅ |
| Size governance honored — full-object rows count toward `maxExportBytes` ([ADR-0405](adr/0405-export-data-contract.md)) | ✅ |
| Docs, `config/samples/` (deployment-snapshot, argo-application-snapshot), unit + envtest coverage | ✅ |
| Phase 2: `prune.cel`, `prune.preset`, jqPathExpressions alias, nested-object metrics, scope-level `allowResourceExport` | ⬜ |

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
  P2[Phase 2<br/>Multi-cluster fleet]
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
| **1** | Collection + Sink | MVP: namespaced CRDs, export to role-based sinks (state store / event emitter); optional HTTP |
| **2** | Multi-cluster | N operators → shared sink (`spec.cluster`); fleet model per ADR-0501 |
| **3** | Governance | `KollectScope`, cluster inventory APIs, S3/GCS hardening |
| **4** | Metrics + aggregation | kube-state-metrics-style config, richer rollups |

See [ARCHITECTURE.md](ARCHITECTURE.md), [REQUIREMENTS.md](REQUIREMENTS.md),
[adr/README.md](adr/README.md), and [planned features](roadmap/planned-features.md) for design detail.

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
| Helm `values.schema.json` + unittest in CI | ✅ |
| Helm docs generation (`helm-docs`) | ✅ |
| Core documentation + MkDocs (GitHub Pages) | ✅ |
| CR reference guide (`docs/crds/`, failure modes) | ✅ |
| Data flows (`DATA-FLOWS.md`) | ✅ |
| Architecture Decision Records (46, thematic `0Txx` ranges) | ✅ |
| ADR-0603 performance & scalability | ✅ |
| `docs/development/guidelines.md`, `SECURITY.md`, `CONTRIBUTING.md` | ✅ |
| Validating webhook — Profile CEL/JSONPath | ✅ |
| Validating webhook — Profile Secret.data guard | ✅ |
| Validating webhook — Sink type enum | ✅ |
| Prometheus custom metrics (early) | ✅ |
| Connection test infrastructure | ✅ ([ADR-0403](adr/0403-connection-test.md)) |
| Namespaced `KollectProfile` API | ✅ ([ADR-0204](adr/0204-namespaced-profiles.md)) |
| Golden OpenAPI contract tests (`test/schema/`, 7 kinds) | ✅ |
| Kind smoke / operator deploy | ✅ |
| Release pipeline (SBOM, signing) | ✅ through **`v0.5.0`** on GHCR + chart ([RELEASE.md](RELEASE.md)) |
| Public demo Git inventory repo | ✅ |

**Counts:** ✅ 23 · 🚧 0 · ⬜ 0

---

## Phase 1 — Collection + Sink + HTTP

| Item | Status |
| --- | --- |
| CEL + JSONPath attribute extractor | ✅ |
| Dynamic informer engine (per Profile GVK) | ✅ |
| In-memory collection store + namespace aggregation | ✅ |
| `KollectTarget` controller | ✅ |
| `KollectInventory` controller (namespaced rollup + export) | ✅ |
| Event-driven path: informer changes → inventory export | 🚧 |
| Sink registry (factory by `type`) | ✅ |
| Git sink with custom CA TLS | ✅ |
| GitLab sink (`tls.caSecretRef` for internal CA) | ✅ REST client + MR wire + feature-branch push |
| S3 sink | 🚧 (MinIO integration; nightly + PR `test-integration`) |
| S3/GCS **Parquet** snapshot export (`format: parquet`) | 🚧 S3 shipped v0.4; GCS JSON default ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)) |
| `spec.pathTemplate` on snapshot sinks | ✅ [ADR-0407](adr/0407-git-object-store-layout.md) |
| **Git readability tranche** — YAML default + `layout` block (`document`/`perResource`/`split`), path templates, prune | ✅ [ADR-0419](adr/0419-git-export-serialization-layout.md) |
| Git **per-resource manifest tree** (auto from `export.mode: Resource`) | ✅ on `main` post-**`v0.5.0`** [ADR-0419](adr/0419-git-export-serialization-layout.md) + [ADR-0306](adr/0306-full-resource-export-pruning.md) |
| **Sink config layering** — cross-cutting `serialization` / `provisioning` / `options` ([ADR-0416](adr/0416-sink-config-layering.md)) | ✅ **`v0.5.0`** |
| **`status.preview`** on family sinks (resolved paths + sample snippet) | ✅ on `main` post-**`v0.5.0`** [ADR-0416](adr/0416-sink-config-layering.md) |
| Postgres sink (`type: postgres`) | ✅ |
| MongoDB sink (`type: mongodb`) | ✅ on `main` post-**`v0.5.0`** [ADR-0417](adr/0417-mongodb-database-sink.md) |
| Postgres **delete reconciliation** (stale-row fix) | ✅ [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md) |
| Kafka export sink (`type: kafka`) | ✅ |
| **NATS JetStream** emitter (`type: nats`, lean default) | ✅ [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md) |
| Postgres/Kafka testcontainers in CI | ✅ |
| SAR / RBAC scope degradation | ✅ |
| Typed reconcile errors + circuit breakers | 🚧 |
| Parallel reconcile workers (`MaxConcurrentReconciles`) | ✅ |
| Workqueue depth + reconcile latency metrics | ✅ |
| pprof server (feature-gated `:6060`) | ✅ |
| `task bench` / `task load-test` (bounded scale tests) | ✅ |
| Secondary watches (Profile → Targets, Sink → Inventories) | ✅ |
| Finalizers | ✅ `v0.1.0-rc.3` — inventory, target, cluster rollup |
| Read-only HTTP `GET /v1alpha1/inventory` (+ OpenAPI; SSE watch) | 🚧 |
| Inventory HTTP auth: TokenReview + SAR (K8s bearer) | ✅ |
| `--inventory-auth-mode=kubernetes` (default) | ✅ |
| Full Prometheus metrics per [ADR-0602](adr/0602-error-taxonomy.md) | ✅ |
| Sample profiles: Deployment, Service, Ingress | ✅ |
| Sample profile: Helm release summary (**Argo `Application` primary**) | ✅ |
| Argo `Application` contract test (`internal/collect/`) | ✅ |
| Sample profile: Helm release summary (Flux `HelmRelease` secondary) | ✅ |
| Helm values profile + operator scrub | ✅ |
| `helm:` decode for `helm.sh/v1` Secret releases | ✅ `v0.1.0-rc.3` |
| Sample: generic CRD (`cert-manager.io/Certificate` + contract test) | ✅ |
| Sample contract tests in CI | 🚧 |
| Integration tests (testcontainers) in CI | ✅ |
| End-to-end: install → collect → export → HTTP | ✅ (kind smoke + tier-0 PR gate) |
| `spec.suspend` on reconciled kinds | ✅ |
| **Multi-tenant (ASAP):** `watchNamespaces` / `tenantMode` Helm + `--watch-namespaces` | ✅ |
| **Multi-tenant:** `KollectScope` webhook + reconciler enforcement + sample | ✅ |
| **Multi-tenant e2e:** dynamic `kollect-tenant-a` / `kollect-tenant-b` isolation | ✅ |
| Inventory namespace isolation unit tests | ✅ |
| Sink family CRDs (`KollectSnapshotSink`, `KollectEventSink`, `KollectDatabaseSink`; `KollectSink` removed) | ✅ `v0.2.0-rc.1` [ADR-0414](adr/0414-sink-family-crds.md) |

**Counts:** ✅ 44 · 🚧 6 · ⬜ 0

---

## Phase 2 — Multi-cluster fleet

Multi-cluster support must **not** block single-cluster installs. **Fleet model:** deploy one
**single-mode operator per cluster**; export to a **shared sink** (Postgres, Git) with
`spec.cluster` row partitioning — no hub/spoke runtime tier ([ADR-0501](adr/0501-multi-cluster-fleet.md)).

| Item | Status |
| --- | --- |
| Multi-cluster fleet ADR (N operators → shared sink) | ✅ [ADR-0501](adr/0501-multi-cluster-fleet.md) |
| `spec.cluster` on inventory / export payloads | ✅ |
| Per-cluster Helm release + ServiceMonitor scrape | ✅ documented |
| Hub/spoke runtime (`mode: hub`, transport, ingest) | ❌ **Removed** v0.3 — see archive ADR retcon |
| Queue transport (Redis/NATS/Kafka between operators) | ❌ **Removed** with hub tier |
| Cross-cluster sink auth (mTLS, workload identity) | 🔮 Deferred — sink-specific |

**Counts:** ✅ 3 · ❌ 2 (removed) · 🔮 1

---

## Phase 3 — Governance + backends

| Item | Status |
| --- | --- |
| `KollectScope` reconciler-time enforcement | ✅ |
| `KollectScope` admission webhook | ✅ |
| `KollectClusterScope` (platform teams) | 🔮 |
| `KollectClusterTarget` API + webhook | ✅ |
| `KollectClusterProfile` API + webhook (no controller) | ✅ |
| `KollectClusterInventory` API + webhook | ✅ |
| `KollectClusterTarget` controller (engine + namespaceSelector) | ✅ |
| `KollectClusterInventory` controller (rollup + export to sinks) | ✅ |
| `KollectClusterSink` / namespaced sink split | 🔮 |
| GCS sink | ✅ |
| S3/GCS object-store CI gate (integration + nightly) | ✅ |
| Generic CRD proof (`cert-manager.io/Certificate` e2e) | ✅ |
| `KollectReceiver` / `KollectTargetSet` (design only) | 🔮 |

### Phase 3 exit criteria (before Phase 4 aggregation)

| Criterion | Status |
| --- | --- |
| Hub ingest → Postgres **and** Kafka parallel export | ✅ |
| `KollectClusterInventory` rollup + export to namespaced sinks | ✅ |
| `KollectClusterTarget` engine end-to-end | ✅ |
| `KollectClusterProfile` stub + profileRef resolution | ✅ |
| Generic CRD proof (`cert-manager.io/Certificate`) | ✅ |
| GitLab sink enterprise path (MR/API) | ✅ feature-branch push + REST MR client |
| S3/GCS production CI gate | ✅ PR integration + nightly |
| Scope at platform boundary (multitenant e2e) | ✅ |
| Release `workflow_dispatch` (cosign/SBOM/chart) | ✅ `v0.1.0-rc` – **`v0.5.0`** |
| E2E asserts export (Target Ready, sink conditions, git SHA) | ✅ `68667ca6` — export asserts + multitenant + cert-manager |
| No `KollectPublication` | ✅ ADR-0702 honored |

**Counts:** ✅ 20 · 🔮 3

---

## Phase 4 — Metrics + aggregation

| Item | Status |
| --- | --- |
| kube-state-metrics-style custom resource metrics config | ✅ [ADR-0304](adr/0304-custom-resource-aggregation-rfc.md) — `KollectProfile.spec.metrics` spike + admission validation |
| Collect engine → `RecordCustomResourceSeries` on target snapshot | ✅ configured paths or auto-sum fallback + `object_count` per profile/GVK |
| `spec.metrics[].labels` → `kollect_custom_resource_labeled_series` | ✅ per-label-tuple sums on target snapshot |
| Hub spoke merge metrics | ❌ Removed with hub tier — use per-cluster `/metrics` + `spec.cluster` |
| Cardinality-safe operator metrics (counts, export latency) | ✅ ADR-0602 catalog complete |
| Target/inventory-scoped domain metrics (`metricsScope`, Tier B/C) | 🔮 Parked [ADR-0604](adr/0604-target-scoped-prometheus-metrics.md) |
| OpenTelemetry tracing | 🔮 Parked [ADR-0605](adr/0605-opentelemetry-tracing.md) — not planned v0.x |
| Cross-target dedupe spike (`internal/aggregate/`) | ✅ row identity, `DedupeByResourceUID`, `ExportCoalesce` checksum skip |
| Advanced cross-target / cross-cluster aggregation (controller wire) | ✅ `KollectClusterInventory` — `MergeRows` + `ExportCoalesce` |
| `task perf-report` optional CI gate | ✅ `ci.yaml` job + preflight note |

**Counts:** ✅ 7 · 🔮 2 · ❌ 1

---

## Read API + UI console (planned — [ADR-0408](adr/0408-read-api-ui-architecture.md))

A read-only web console (searchable inventory catalog, export/freshness health, multi-cluster rollup,
attribute drift over time) is the priority adoption lever before the **v0.10 presentation gate**. The
UI depends only on a **versioned Read API** with a **pluggable backing store** (memory → Postgres →
Parquet), so the same SPA serves a zero-infra console and a scale portal — and never reads the live
cluster API.

!!! note "`v0.5.0` was not the Read API freeze"
    **`v0.5.0`** shipped **sink config layering** ([ADR-0416](adr/0416-sink-config-layering.md)) plus export/git
    hardening on `main` post-tag (ADR-0306, ADR-0419, MongoDB). **Read API contract freeze** remains ⬜ —
    UI milestones stay in the **v0.5–v0.10** band ([RELEASE.md](RELEASE.md#versioning-policy)).

| Milestone | Item | Status |
| --- | --- | --- |
| **v0.5.x** | Harden + freeze the Read API as the UI contract (filters, `schemaVersion`, OpenAPI) | ⬜ |
| **v0.6.x** | Memory `InventoryReader` adapter + `ui/` scaffold hardening | 🚧 early adopter preview on `main` |
| **v0.7.x** | Read-only SPA on **memory adapter**: catalog, search/filter, freshness/health | 🚧 mock MVP + docs; production gate |
| **v0.8.x – v0.9.x** | **Fleet console** portal — read-side fleet server on **Postgres/Parquet**; multi-cluster picker; **drift-over-time**; optional `kollect-server` split | ⬜ [ADR-0418](adr/0418-fleet-console-read-plane.md) |
| **v0.10.0** | Presentation-ready demo (UI + docs + stable soak) | ⬜ |

### Fleet console (multi-cluster read plane — [ADR-0418](adr/0418-fleet-console-read-plane.md))

In production Kollect is a **fleet**: N single-mode operators fan into a **shared sink**
([ADR-0501](adr/0501-multi-cluster-fleet.md)), so the thing worth visualizing is the fleet, not one
operator's in-memory store. The console therefore evolves from a single-cluster view into a
**read-only fleet console**: a standalone server consumes the existing event stream
([ADR-0402](adr/0402-sink-backends-database-kafka.md)) — the per-`(cluster, namespace)` inventory
envelope every cluster already emits — materializes a fleet read model, and serves the **existing Read
API contract extended with a `cluster` dimension** plus a `/v1alpha1/clusters` roster. It is a pure
read consumer: **no hub tier** ([ADR-0501](adr/0501-multi-cluster-fleet.md) holds), no kube-apiserver
writes, and the browser never holds bus or database credentials.

| Item | Status | Notes |
| --- | --- | --- |
| `InventoryReader` interface + `memoryFleet` adapter | ⬜ | v0.6 — fulfils [ADR-0408](adr/0408-read-api-ui-architecture.md) OQ-11 |
| Read-side fleet server (event consumer → fleet read model) | ⬜ | v0.6–v0.7 · [ADR-0418](adr/0418-fleet-console-read-plane.md) |
| Read API `cluster` dimension + `/v1alpha1/clusters` (additive OpenAPI) | ⬜ | v0.6 · [ADR-0411](adr/0411-read-api-extensions-for-ui.md) |
| SPA fleet overview + `cluster` column/filter + cluster picker | ⬜ | v0.7 |
| `postgresFleet` adapter + consume-to-database upsert (history/drift) | ⬜ | v0.8 |
| Cold-start rehydrate / compacted-topic replay; "rebuilding" banner | ⬜ | v0.8 |
| Drift-over-time / "what changed" views | ⬜ | v0.9 |
| `kollect-fleet-server` chart + oauth2-proxy overlay | ⬜ | v0.9 |

**Honors:** [ADR-0501](adr/0501-multi-cluster-fleet.md) (no hub), [FR-READ-1](REQUIREMENTS.md) (read
model, never the live API), [ADR-0702](adr/0702-doc-sync-templating.md) (single responsibility — useful
"actions" belong to a separate publisher component, not cluster writes).

---

## Performance and scalability

Cross-cutting NFRs accepted in [ADR-0603](adr/0603-performance-scalability.md). Tuning guide:
[PERFORMANCE.md](PERFORMANCE.md).

### Scale targets

| Target | Value | ADR |
| --- | --- | --- |
| Watched objects per spoke (baseline) | **10,000+** | [ADR-0603](adr/0603-performance-scalability.md) |
| Giant single cluster | 1000+ nodes, 10k+ resources | [ADR-0603](adr/0603-performance-scalability.md) |
| Hub spoke count | many spokes (see [ADR-0603](adr/0603-performance-scalability.md)) | [ADR-0501](adr/0501-multi-cluster-fleet.md) |
| Spoke working set (typical profiles) | ≤512 MiB at 10k rows | [ADR-0603](adr/0603-performance-scalability.md) |
| Hub merge complexity | O(total rows), sharded | [ADR-0501](adr/0501-multi-cluster-fleet.md) |

### Developer perf tooling

| Item | Status |
| --- | --- |
| Metrics catalog + PromQL hints in PERFORMANCE.md | ✅ |
| `task perf-report` + `hack/perf-report.sh` | ✅ |
| `artifacts/bench/` from `task bench` | ✅ |
| CI upload of bench artifacts (nightly) | ✅ nightly `e2e-bench` job |
| `task perf-report` PR CI job | ✅ non-blocking `ci.yaml` job (artifact upload) |
| `--collect-dispatch-workers` / queue size (PERF-03) | ✅ v0.4 |

**Counts:** ✅ 6

### Operator tuning and tests

| Item | Status |
| --- | --- |
| Scale target documented (10k validated; 100k design) | ✅ [ADR-0603](adr/0603-performance-scalability.md) |
| Fleet model documented (N operators → shared sink) | ✅ [ADR-0501](adr/0501-multi-cluster-fleet.md) |
| Bounded test tiers (500 default / 2000 opt-in load) | ✅ |
| `task bench` (Go benchmarks, `-short`) | ✅ |
| `task load-test` (`KOLECT_LOAD_TEST=1`, `-tags=load`) | ✅ |
| `--max-concurrent-reconciles-*` flags + Helm values | ✅ |
| **`spec.exportMinInterval`** per Inventory (default 30s) | ✅ |
| **Per-sink `exportMinInterval`** on `sinkRefs[]` + `status.sinkExports[]` | ✅ [ADR-0413](adr/0413-export-interval-scheduling.md) |
| `--reconcile-rate-limit` flag | ✅ |
| `--informer-resync-period` flag | ⬜ |
| pprof on `:6060` (feature gate) | ✅ |
| `kollect_workqueue_depth` / `kollect_reconcile_duration_seconds` metrics | ✅ |
| `kollect_informer_objects` / `kollect_export_bytes_total` metrics | ✅ |
| `BenchmarkExtract` in `internal/collect/` | ✅ |
| envtest synthetic scale harness (cap 500) | ✅ |
| Load test package (`test/load/`, `-tags=load`) | ✅ |

**Counts:** ✅ 17 · ⬜ 1

---

## Rejected

| Item | Rationale |
| --- | --- |
| `KollectPublication` (Confluence, Go templates, doc-sync) | Out of scope — external CI over Git/Kafka export ([ADR-0702](adr/0702-doc-sync-templating.md)) |
| `KollectSink.type: prometheus` | Operator `/metrics` only — not an inventory export sink ([ADR-0601](adr/0601-prometheus-metrics-stub.md)) |

## Deferred

| Item | When |
| --- | --- |
| `KollectClusterSink` + namespaced `KollectSink` split | Phase 3 — cluster-scoped sinks + `KollectScope.sinkRefs` until then ([ADR-0204](adr/0204-namespaced-profiles.md)) |
| Kafka as **required** hub transport | Pluggable optional backend only; `inprocess` default (ADR-0502 (archived)) |
| `KollectReceiver`, `KollectTargetSet` implementation | Reserved for future phases |
| oauth2-proxy sidecar (OIDC browser auth) | Optional Helm sidecar (`oauth2Proxy.enabled: false`); K8s bearer auth is primary — [ADR-0404](adr/0404-inventory-api-auth.md) |
| Hub federated mTLS | ADR-0503 deferred — push TokenReview default |
| Queue transport TLS/ACL production hardening | Beyond `cluster_id` wire metadata |

## Resolved questions

- ✅ **Hub ingest SAR shape** — `create` on `kollectremoteclusters` locked (ADR-0503 (archived))
- ✅ **SinkReachable** on Inventory/Target — implemented with `Synced` export conditions ([ADR-0403](adr/0403-connection-test.md))

See [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md) for locked vs still-open items.

## Breaking changes

### Namespaced `KollectInventory` (2026-06-05)

`KollectInventory` is **namespaced**. Each team owns an inventory object in their namespace that
aggregates `KollectTarget`s in the same namespace. Platform-wide rollup uses
`KollectClusterInventory` (cluster-scoped rollup + export shipped).

Migration: replace cluster-scoped inventory manifests with namespaced equivalents; update RBAC to
namespace scope where appropriate.

### Namespaced `KollectProfile` (2026-06-05)

`KollectProfile` is **namespaced**. Each `KollectTarget.spec.profileRef` resolves a profile in the
**same namespace** as the Target. Platform-wide shared schemas use `KollectClusterProfile`
(cluster-scoped API shipped; controller pending).

Migration: re-apply profile manifests into each team namespace (or use GitOps templating). Remove
cluster-scoped profile objects before upgrade.

### Namespaced `KollectSink` (2026-06-05)

`KollectSink` is **namespaced** (breaking — was cluster-scoped). Each `KollectInventory.spec.sinkRefs`
entry resolves a sink in the **same namespace** as the Inventory. Cross-namespace sink refs are
forbidden (webhook rejects `namespace/name`). Platform-shared backends are reserved for
`KollectClusterSink` (not yet implemented).

Migration: re-apply sink manifests into each team namespace alongside profiles and inventories.
Remove cluster-scoped sink objects before upgrade. Update `KollectScope.spec.sinkRefs` allowlists
to names in the scope namespace.

## GitLab sink — merge request workflow

Scaffold (`553117cc`) reuses the shared **HTTPS git push** path: `internal/sink/gitlab` resolves
`spec.endpoint` + `tls.caSecretRef` / `caBundle`, then delegates to `internal/sink/git.Export`
(direct push to the default branch). Connection probe runs `git ls-remote` with custom CA trust.

**Partial** — CRD + validation + export wire + REST client + feature-branch git push landed:

| Gap | Status |
| --- | --- |
| **CRD fields** | ✅ `spec.gitlab.mergeRequest` (mode `direct` \| `merge_request`), `targetBranch`, `branchPrefix`, `titleTemplate`, `autoMerge` |
| **Branch + push** | ✅ `merge_request` mode clones `targetBranch`, pushes feature branch via `git.ExportWithBranch` |
| **GitLab REST API v4** | ✅ `RESTClient` list/create MR; `EnsureMergeRequest` after export when token + `merge_request` mode |
| **Token scopes** | ✅ document `write_repository` + `api` in sink CR reference |
| **Export integration** | ✅ `Backend.Export` pushes feature branch then calls `EnsureMergeRequest` |
| **Integration test** | ✅ httptest MR client unit tests + file-remote feature-branch export test |
| **Hub/cluster sinks** | Same contract applies to `KollectClusterSink` when implemented (Phase 3) |

**Default:** `direct` mode pushes to the default branch. `merge_request` mode opens/updates an MR via
GitLab API v4 when `secretRef` provides an API token (`token` or `password` key).

## CI and end-to-end testing

| Item | Status |
| --- | --- |
| PR CI: gitleaks, verify, lint, unit tests, build | ✅ |
| PR CI: integration (testcontainers) | ✅ |
| PR CI: Helm lint + unittest | ✅ |
| Manual e2e workflow (`workflow_dispatch`, kind smoke parity) | ✅ |
| Nightly kind smoke (Helm + samples + cert-manager CRD + HTTP probe) | ✅ |
| Full e2e: conditions, Git export SHA, HTTP body, multitenant | ✅ |
| Object store sinks (S3/GCS MinIO) in PR integration + nightly | ✅ |
| Release workflow (`workflow_dispatch`) | ✅ Tags `v0.1.0-rc.*` – **`v0.5.0`** ([RELEASE.md](RELEASE.md)) |

## Architecture decisions (2026-06-05)

Full locked table: **[PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md)**.

| Decision | Status |
| --- | --- |
| Single-cluster MVP is the default install | Accepted |
| Namespaced inventory is the hub input contract | Accepted |
| **`KollectProfile` namespaced**; `KollectClusterProfile` reserved | Accepted ([ADR-0204](adr/0204-namespaced-profiles.md)) |
| **`KollectScope` Phase 1** — webhook + reconciler enforcement | Accepted ([ADR-0203](adr/0203-namespaced-multi-tenancy.md)) |
| **No `KollectHub` CRD** — Helm `mode: hub\|spoke` | Accepted (ADR-0703 (archived)) |
| **Namespaced `KollectSink`**; `KollectClusterSink` reserved | Accepted (ADR-0703 (archived)) |
| **Role-based sinks** — state stores (Git/object store, Postgres) vs event emitters (NATS default, Kafka opt-in); no single "primary"; HTTP debug optional | Accepted ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)) |
| **`KollectConnectionTest` CR** + **`spec.ttlSecondsAfterFinished`** default **300s** | Accepted (ADR-0703 (archived)) |
| **`spec.exportMinInterval`** default **30s** (not global debounce flag) | Accepted (ADR-0703 (archived)) |
| HTTP **`GET /v1alpha1/inventory`** + **`openapi/v1alpha1/inventory.yaml`** when enabled | Accepted ([ADR-0103](adr/0103-etcd-limit.md), [ADR-0404](adr/0404-inventory-api-auth.md)) |
| Inventory SAR: **`get`/`list`** on `kollectinventories`; TokenReview cache **30s** | Accepted ([ADR-0404](adr/0404-inventory-api-auth.md)) |
| **`maxExportBytes`** global + per-Inventory override (webhook capped) | Accepted ([ADR-0103](adr/0103-etcd-limit.md)) |
| Postgres PK **`(inventory_namespace, inventory_name, target_name, source_uid)`** | Accepted ([ADR-0402](adr/0402-sink-backends-database-kafka.md)) |
| **`kollect_sink_errors_total{reason}`** + export histogram buckets (ADR-0602) | Accepted |
| Hub shard: **`hash(clusterName) % shardCount`** via Helm/env — **no `KollectHub` CRD** | Accepted (ADR-0703 (archived)) |
| Hub federated mTLS | **Deferred** (ADR-0503 (archived)) |
| **`KollectClusterInventory`** + **`KollectClusterTarget`** rollup (no `inventoryRef` hack) | Accepted (ADR-0703 (archived)) |
| Same image **`mode: hub\|spoke`** | Accepted ([ADR-0501](adr/0501-multi-cluster-fleet.md)) |
| Transport: **`inprocess` only default**; Redis/NATS/Kafka explicit opt-in | Accepted (ADR-0502 (archived)) |
| Transport backend rule: no merge without integration/e2e proof | Accepted |
| Connection test: **`KollectConnectionTest` CR** + sink probes; prod `connectionTest: false` | Accepted (ADR-0703 (archived)) |
| Helm sample: **Argo `Application` primary** + contract test | Accepted ([ADR-0303](adr/0303-helm-release-inventory.md)) |
| Generic CRD sample: **`cert-manager.io/Certificate`** + contract test | Accepted |
| Default install: **`tenantMode: true`** per-team | Accepted ([ADR-0203](adr/0203-namespaced-multi-tenancy.md)) |
| Shared informer per GVK | Accepted ([ADR-0301](adr/0301-event-driven-informers.md)) |
| Postgres (relational SoR) + Kafka (event emitter) as first-class sinks; in-memory snapshot canonical, sinks are projections | Accepted ([ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md), [ADR-0402](adr/0402-sink-backends-database-kafka.md)) |
| Doc-sync / `KollectPublication` | Rejected ([ADR-0702](adr/0702-doc-sync-templating.md)) |
| **Read API + read-only UI console** — versioned API, pluggable backing store (memory→Postgres→Parquet); SPA reads the read model, never live API | Accepted, planned **v0.5–v0.10** ([ADR-0408](adr/0408-read-api-ui-architecture.md)) |
| Inventory HTTP auth: **K8s TokenReview + SAR**; `--inventory-auth-mode=kubernetes` default | Accepted |
| oauth2-proxy: **optional** Helm sidecar for OIDC browsers; not primary auth | Accepted |
| Git, object storage, and agent mesh documented as alternatives | Accepted |
| Extreme scale: many clusters, 10k+ objects/spoke, hub shard not O(n²) | Accepted ([ADR-0501](adr/0501-multi-cluster-fleet.md), [ADR-0603](adr/0603-performance-scalability.md)) |
| Hub cluster auth: **Istio remote-secret registration + push TokenReview** | Accepted (ADR-0503 (archived)) |
| Namespaced `KollectProfile`; `profileRef` same namespace | Accepted ([ADR-0204](adr/0204-namespaced-profiles.md)) |
| **`KollectClusterSink` deferred Phase 3** | Deferred |

## Further reading

- [Planned features (backlog and Exploring specs)](roadmap/planned-features.md)
- [ADR and RFC process](development/adr-rfc-process.md)
- [Platform decisions (2026-06-05)](PLATFORM-DECISIONS.md)
- [Product requirements](REQUIREMENTS.md)
- [Architecture](ARCHITECTURE.md)
- [Helm chart README](../charts/kollect/README.md) — inventory HTTP auth
- [ADR-0201: CRD model](adr/0201-crd-model.md)
- [ADR-0103: etcd limit + HTTP API](adr/0103-etcd-limit.md)
- [ADR-0301: Event-driven informers](adr/0301-event-driven-informers.md)
- [ADR-0501: Multi-cluster RFC](adr/0501-multi-cluster-fleet.md)
- ADR-0502: Lean queue transport (archived)
- [ADR-0404: Inventory API auth](adr/0404-inventory-api-auth.md)
- [ADR-0702: Doc-sync rejected](adr/0702-doc-sync-templating.md)
- [ADR-0402: Postgres and Kafka sinks](adr/0402-sink-backends-database-kafka.md)
- [ADR-0603: Performance and scalability](adr/0603-performance-scalability.md)
- [PERFORMANCE.md](PERFORMANCE.md) — tuning guide and metrics catalog
