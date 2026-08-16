# Performance and scalability

Kollect is designed for **large single clusters** (1000s of nodes, **10k+ watched resources** as a
**design** baseline — not a published CI proof) and **multi-cluster fleets** — **N independent
single-mode operators** exporting to a **shared sink** partitioned by `spec.cluster`. There is
**no hub/spoke runtime tier** ([ADR-0501](../adr/0501-multi-cluster-fleet.md)). This guide
summarizes tuning knobs from [ADR-0603](../adr/0603-performance-scalability.md). Scale **claims**
must match [load-test-runbook.md](load-test-runbook.md) evidence status.

## Scale tiers

| Tier | Workload shape | Clusters | Execution layer | Evidence status |
| --- | --- | --- | --- | --- |
| Dev / CI default | ≤500 synthetic | 1 | envtest (`task test`) | ✅ Active — every PR |
| Extractor budget | 128 varied objects, in-process | 0 (no cluster) | `task extract-budget` | ✅ Active — micro-benchmark budget; **not a scale tier**, ≠ in-cluster 10k soak |
| Nightly 10k | **10,000** synthetic | 1 | `scale-envtest-10k` on `ubuntu-latest-8-cores` | ⬜ **Disabled / unverified** — job opt-in only (`run_scale_jobs`); no green SHA while runners unavailable |
| Baseline production | **10,000+** in-cluster | 1 | Metrics + pprof; manual load | ⬜ **Unverified** — no published SHA / date / hardware evidence yet |
| Laptop / L4.5 lab | Bounded schedule | 1 (single-host) | Existing cluster (Kind / K3s / Talos) | Named pin only — [local lab](local-lab-runbook.md) / [evidence bundle](lab-evidence-bundle.md); **does not** satisfy 100k / two-cluster gate |
| Design target | **100,000** | 1–2 cloud | Manual cloud soak ([load-test runbook](load-test-runbook.md)) | ⬜ **Planned / unexecuted (AR-02)** — needs export sharding + Postgres bulk upsert |
| Fleet | 10k–100k × **N** | **many** | One `ServiceMonitor` per cluster release | Architecture guidance — correlate by `spec.cluster` |

The **100,000-row design target** requires **mandatory export sharding** — one `KollectInventory`
per workload namespace (or smaller groups) so each export stays **below ~2,000 rows** (~1.5 MiB) —
plus `resourcesProfile: large` (request ≥2 GiB, limit ≥4 GiB). Fleet scale fans out across
operators; there is no central merge process to bottleneck ([ADR-0603](../adr/0603-performance-scalability.md)).

## Controller parallelism

| Flag | Default | Controller |
| --- | --- | --- |
| `--max-concurrent-reconciles-target` | `5` | `KollectTarget` |
| `--max-concurrent-reconciles-inventory` | `3` | `KollectInventory` |
| `--max-concurrent-reconciles-cluster-target` | `2` | `KollectClusterTarget` |
| `--max-concurrent-reconciles-cluster-inventory` | `2` | `KollectClusterInventory` |
| `--collect-dispatch-workers` | `4` | Collection informer dispatch pool |
| `--collect-dispatch-queue-size` | `512` | Dispatch queue depth before informer dispatch blocks (backpressure) |

Raise concurrency when reconcile latency grows while CPU is underutilized. Lower it when
API server throttling or etcd watch pressure appears.

## Workqueue rate limiting

When `--reconcile-rate-limit` is **unset** (`0`), controller-runtime uses its default
exponential failure rate limiter (5ms base, 1000s cap). Set a positive duration (for example
`100ms`) to slow retries on terminal errors without changing success-path throughput.

`kollect_workqueue_depth` approximates queue pressure as **in-flight reconciles** per controller
(not the internal client-go queue length).

## Reported collection scale

**`KollectTarget.status.collectedCount`** is the machine-readable number of resources a Target is
collecting, surfaced as the `COLLECTED` column of `kubectl get kollecttargets`. The `Ready`
condition message restates it as prose for backward compatibility only.

Objects entering or leaving a Target's matched set do **not** enqueue that Target, so the number is
refreshed by a periodic self-requeue: **`--target-count-resync`** (default **`60s`**, Helm
`controller.targetCountResync`).

**Budget the resync — it is not free.** Each pass costs **one live, cluster-wide, unpaginated
namespace `LIST`** — **two** when a `KollectScope` is enforced on the Target's namespace, because
the scope check resolves the filter status as well. At the default interval, **N** Targets therefore
cost **N** namespace `LIST`s per minute — **2N** under an enforced scope. The rest is cheap: the
engine skips the informer backfill when the Target's state is unchanged, and the status write is
skipped when the number did not move.

**`status.collectedCountUpdatedAt`** records when the number last *changed* — **not** when it was
last checked. A steady Target keeps an old timestamp while still being re-derived every resync, so
an old timestamp on its own does not mean the count is stale.

Judge liveness from the timestamp and the conditions together: a `Ready` Target with an old
timestamp has a count that genuinely has not moved, while a `Degraded` Target keeps its last known
count and the timestamp shows how old that measurement is. Lower the interval for a more responsive
count; raise it to cut reconcile volume on very large fleets.

## Export debouncing

**`KollectInventory.spec.exportMinInterval`** (default **`30s`**) coalesces export to external sinks
per inventory. Material payload changes (generation/checksum bump) may export immediately inside the
min interval ([ADR-0201](../adr/0201-crd-model.md)).
Lower the interval for fresher Postgres/Kafka exports; raise to reduce sink API load. Across a large
fleet writing to a shared sink, debouncing is **mandatory** to avoid export storms against the
backend.

## Collection engine

- **In-memory store:** O(n) memory in collected object count; one `RWMutex` guards nested maps.
  Target **≤512 MiB** RSS at 10k typical Deployment/Service rows (verify with pprof).
- **Informers:** When all active targets for a GVK resolve to **one** namespace via
  `spec.namespaceSelector`, the dynamic informer is scoped to that namespace. Otherwise the
  informer watches all namespaces and filters events at dispatch time (correctness over cache size).
- **Resync:** 12h informer resync is a correctness backstop, not a freshness driver.
- **Fleet exports:** Each operator writes its own debounced inventory snapshot to the shared sink,
  partitioned by `spec.cluster` — no cross-cluster merge on the hot path ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).

## Metrics catalog

Full scrape setup, default PrometheusRule alerts, and the complete `kollect_*` metric reference live in
[Operator metrics](metrics.md). The table below highlights **performance and
scalability** signals — use it with the [bottleneck checklist](#early-bottleneck-checklist) below.

| Metric | Type | Labels | PromQL hint | What rising values imply |
| --- | --- | --- | --- | --- |
| `kollect_inventory_items_total` | Gauge | — | `kollect_inventory_items_total` | Stale while store grows → inventory reconcile or export lag |
| `kollect_collect_items_total` | Gauge | — | `kollect_collect_items_total` | RSS scales with store size at 10k+ objects |
| `kollect_collected_objects` | Gauge | `profile`, `gvk` | `sum by (profile, gvk) (kollect_collected_objects)` | Per-target cardinality; split profiles when labels explode |
| `kollect_reconcile_total` | Counter | `controller`, `result` | `sum(rate(kollect_reconcile_total[5m])) by (controller, result)` | Rising failure ratio → check error-class counters |
| `kollect_reconcile_errors_total` | Counter | `kind`, `error_class` | `sum(rate(kollect_reconcile_errors_total[5m])) by (error_class)` | See [ADR-0602](../adr/0602-error-taxonomy.md) error classes |
| `kollect_sink_errors_total` | Counter | `reason` | `sum(rate(kollect_sink_errors_total[5m])) by (reason)` | Export failures — separate from reconcile errors ([ADR-0602](../adr/0602-error-taxonomy.md)) |
| `kollect_sink_connection_test_total` | Counter | `type`, `result` | `sum(rate(kollect_sink_connection_test_total[5m])) by (type, result)` | Spikes on sink CR churn; sustained failure → creds/network |
| `kollect_workqueue_depth` | Gauge | `controller` | `max_over_time(kollect_workqueue_depth[5m])` | Sustained high values → raise `--max-concurrent-reconciles-*` or reduce reconcile work |
| `kollect_reconcile_duration_seconds` | Histogram | `controller` | `histogram_quantile(0.99, sum(rate(kollect_reconcile_duration_seconds_bucket[5m])) by (le, controller))` | p99 rising while depth low → slow API/sink; p99 rising with depth → under-provisioned workers |
| `kollect_informer_objects` | Gauge | `group`, `version`, `resource` | `sum by (group, version, resource) (kollect_informer_objects)` | Unexpected growth → extra GVR watches or cluster-wide scope; check namespace scoping |
| `kollect_export_bytes_total` | Counter | `sink_type` | `rate(kollect_export_bytes_total[5m])` | Spike → debounce too low or inventory churn; flat while stale → export path stuck |
| `kollect_export_duration_seconds` | Histogram | `sink_type` | `histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))` | Sink slowness (Git/Postgres/Kafka) — not collection |
| `kollect_export_debounced_total` | Counter | `controller` | `sum(rate(kollect_export_debounced_total[5m])) by (controller)` | Exports skipped by min interval — expected when debounce is tight |
| `kollect_namespace_fingerprint_cache_total` | Counter | `controller`, `result` | `sum(rate(kollect_namespace_fingerprint_cache_total[5m])) by (result)` | `hit` skips the namespace snapshot+fingerprint recompute (AR-10); low hit ratio under steady churn-free load → check Store mutation rate |
| `kollect_collect_dispatch_duration_seconds` | Histogram | — | `histogram_quantile(0.95, sum(rate(kollect_collect_dispatch_duration_seconds_bucket[5m])) by (le))` | Collection extract/upsert latency |
| `kollect_collect_dispatch_queue_depth` | Gauge | — | `max_over_time(kollect_collect_dispatch_queue_depth[5m])` | Sustained high → raise dispatch workers/queue |
| `kollect_collect_dispatch_backpressure_total` | Counter | — | `increase(kollect_collect_dispatch_backpressure_total[15m])` | Queue overflow — dispatch pool undersized |
| `kollect_informer_resync_dispatches_total` | Counter | `group`, `version`, `resource` | `sum(increase(kollect_informer_resync_dispatches_total[1h])) by (group, version, resource)` | Resync-driven dispatch volume |
| `kollect_collect_namespace_mismatch_total` | Counter | `group`, `version`, `resource` | `sum(rate(kollect_collect_namespace_mismatch_total[5m])) by (group, version, resource)` | Objects dropped outside a target's effective namespaces |
| `kollect_informer_cluster_wide_scope` | Gauge | `group`, `version`, `resource` | `max by (group, version, resource) (kollect_informer_cluster_wide_scope)` | 1 = cluster-wide watch (RSS risk at scale) |

Additional runtime signals: Go `memstats` via pprof (`--enable-pprof`), API server `429` in operator logs.
See [Operator metrics](metrics.md) for profile-derived series, connection-test counters,
and example PromQL for alerting.

## Profiling

`--enable-pprof` serves standard Go profiles on `--pprof-bind-address` (default `:6060`),
separate from Prometheus metrics (`:8080` / `:8443`). Helm sets `pprof.enabled: false` by default;
enable in dev overlays only.

## Benchmarks and the extractor budget

```bash
task bench                    # writes artifacts/bench/*.txt
task extract-budget           # fails a >25% B/op or allocs/op regression
```

`task extract-budget` runs `TestExtractHotPathBudget`, which drives the same workload as
`BenchmarkExtract` over 128 varied objects and checks ns/op, B/op and allocs/op against a recorded
baseline. **What it exercises:** `collect.Extractor.Extract`, single-threaded, in-process. **What
it does not:** API server, cluster, informers, sinks, controller, concurrency, export. It is not
cluster-scale evidence — see the tier table above.

### What the budget does and does not enforce

**Enforced at baseline +25%:** `B/op` and `allocs/op`. These are hardware-independent for a fixed
Go toolchain, so an allocation regression fails on any runner.

**Not enforced:** a >25% wall-clock gate. The `ns/op` ceiling is a coarse catastrophic-regression
net only — a ceiling loose enough to be safe on a shared CI runner is far too loose to catch 25%,
and a CPU-only regression at unchanged allocations will pass it. For a real latency floor, measure
your own hardware and pin `KOLECT_EXTRACT_MAX_NS_PER_OP` to that value x 1.25 (also `_BYTES_` /
`_ALLOCS_`).

For local perf summaries (`task perf-report`), see [contributor setup](../development/setup.md).

## Early bottleneck checklist

| Symptom | Likely cause | First action |
| --- | --- | --- |
| High `kollect_informer_objects`, high RSS | Cluster-wide informer for multi-namespace targets | Namespace-scope targets; split profiles |
| High `kollect_workqueue_depth` on `inventory` | Export or aggregation on hot path | Raise inventory workers; increase `spec.exportMinInterval` |
| High export bytes rate, low object churn | Missing payload dedupe | Verify debounce + content-hash skip |
| `TestExtractHotPathBudget` red / `BenchmarkExtract` regression | CEL/JSONPath hot path | Profile extractor; check attribute count and CEL complexity |
| High RSS on large clusters | Full in-memory collect store | Namespace-scoped targets; raise export interval ([ADR-0603](../adr/0603-performance-scalability.md)) |

## Fleet operations

Collected rows and export shards are different scaling dimensions. Keep each inventory bounded,
partition shared sink keys by `spec.cluster`, and monitor per-sink latency before increasing worker
concurrency. Git audit cadences can usually be slower than database freshness; configure intervals
per sink instead of forcing every backend onto the fastest cadence.

For relational fleets, include cluster in uniqueness constraints and indexes. Partition only after
query plans and measured row growth justify it.

---

<!-- Consolidated from the former docs/operator-manual/scaling-and-fleet.md page. -->

Guidance for operators running Kollect at **large cluster** scale (design target: **100,000
collected rows per cluster operator**) and **multi-cluster fleets** sharing Postgres, Git, or object
stores.

!!! warning "Honest scale claim"
    **100k/cluster is a design target**, not a blanket product guarantee. Proof requires a **manual
    cloud load test** ([load test runbook](load-test-runbook.md)) — not GitHub Actions runners and
    not laptop / L4.5 lab evidence. Until that **AR-02** gate passes, treat 100k as **architecture
    guidance** with mandatory export sharding. Nightly 10k CI is **disabled / unverified** while
    `ubuntu-latest-8-cores` jobs stay opt-in.

## Collected rows vs export shards

| Term | Meaning |
| --- | --- |
| **Collected rows** | Items in the operator collect store (`kollect_collect_items_total`) |
| **Export shard** | One `KollectInventory` namespace aggregate — keep **<~2,000 rows** per shard |

Monolithic namespace inventories hit `PayloadTooLarge` above ~2,500 rows. Spread workloads across
**many namespaces**, each with its own `KollectInventory` — see
[`config/samples/advanced/kollect_v1alpha1_kollectinventory_sharded.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectinventory_sharded.yaml).

The operator sets `status.conditions[ExportShardWarning]` and increments
`kollect_export_shard_warn_total` when a namespace aggregate reaches **~1,800 rows**.

## Helm resource profiles

For large clusters, use the chart **`resourcesProfile: large`** preset (≥2 GiB request, ≥4 GiB
limit). Tune dispatch and reconcile flags per [PERFORMANCE.md](performance.md).

```yaml
resourcesProfile: large
collect:
  dispatchWorkers: 8
  dispatchQueueSize: 1024
```

## Git audit @ 1h

Git snapshot sinks are for **audit cadence** (typically **1h** `exportMinInterval`), not portal
query. At scale:

1. **Shard exports** (<2k rows/inventory).
2. Set **1h** (or longer) per-ref interval on `snapshotSinkRefs`.
3. Use **`pathTemplate: clusters/{cluster}/…`** on snapshot sinks for fleet repos.
4. Operator **PERF-10** persistent mirror + **checksum fingerprint skip** avoid clone/push when
   payload is unchanged (env: `KOLLECT_GIT_MIRROR_DIR`).

## Shared Postgres fleet

Multiple cluster operators can upsert into **one Postgres sink** ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).
Each operator sets **`spec.cluster`** on database sinks; the backend primary key is
`(cluster, namespace, name, uid)`.

### Row growth

```
total_rows ≈ Σ (clusters × collected_rows_per_cluster)
```

Example: **200 clusters × 50k rows** ≈ **10M rows** — plan DBA review before sustained growth.

### When to partition (DBA)

| Signal | Action |
| --- | --- |
| Table **>~10M rows** or **>~100 GiB** | Partitioning review |
| Slow exports / vacuum pressure | Partition by **`cluster`** or **`exported_at` month** |
| Retention policy | Drop/archive old monthly partitions |

### Responsibilities

| Role | Owns |
| --- | --- |
| **Kollect operator** | Upsert semantics, `spec.cluster`, export debounce, row identity |
| **Platform / DBA** | Partition DDL, indexes, retention, connection pooling, backups |

The operator does **not** create Postgres partitions. Document expected table shape in your runbook;
use `kollect_export_duration_seconds` and sink error metrics for early warning.

### Index hints (DBA)

- Composite unique index aligned with upsert PK: `(cluster, namespace, name, uid)`
- Optional BRIN on `exported_at` for time-range portal queries
- Avoid unbounded JSONB bloat — keep attribute profiles lean ([REQUIREMENTS.md](../REQUIREMENTS.md))

## Related

- [Lab evidence bundle and redaction](lab-evidence-bundle.md)
- [Performance tuning](performance.md)
- [Load test runbook](load-test-runbook.md)
- [Multi-cluster fleet example](../examples/multi-cluster-fleet.md)
- [ADR-0603](../adr/0603-performance-scalability.md)
