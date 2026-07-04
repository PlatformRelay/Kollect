# Performance and scalability

Kollect is designed for **large single clusters** (1000s of nodes, **10k+ watched resources
baseline**) and **multi-cluster fleets** — **N independent single-mode operators** exporting to a
**shared sink** partitioned by `spec.cluster`. There is **no hub/spoke runtime tier**
([ADR-0501](adr/0501-multi-cluster-fleet.md)). This guide summarizes tuning knobs from
[ADR-0603](adr/0603-performance-scalability.md).

## Scale tiers

| Tier | Collected rows | Clusters | How to validate |
| --- | --- | --- | --- |
| Dev / CI default | ≤500 synthetic | 1 | `task test` |
| Opt-in load | ≤2,000 synthetic | 1 | `KOLECT_LOAD_TEST=1 task load-test` |
| Nightly load | **10,000** synthetic | 1 | `task load-test:10k` on 8-core runners |
| Baseline production | **10,000+** (validated) | 1 | Metrics + pprof; manual load |
| Design target | **100,000** | 1 | Manual / `perf-report`; needs export sharding + Postgres bulk upsert (claim gate v0.5+) |
| Fleet | 10k–100k × **N** | **many** | One `ServiceMonitor` per cluster release; correlate by `spec.cluster` |

The **100,000-row design target** requires **mandatory export sharding** — one `KollectInventory`
per workload namespace (or smaller groups) so each export stays **below ~2,000 rows** (~1.5 MiB) —
plus `resourcesProfile: large` (request ≥2 GiB, limit ≥4 GiB). Fleet scale fans out across
operators; there is no central merge process to bottleneck ([ADR-0603](adr/0603-performance-scalability.md)).

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

## Export debouncing

**`KollectInventory.spec.exportMinInterval`** (default **`30s`**) coalesces export to external sinks
per inventory. Material payload changes (generation/checksum bump) may export immediately inside the
min interval ([ADR-0201](adr/0201-crd-model.md)).
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
  partitioned by `spec.cluster` — no cross-cluster merge on the hot path ([ADR-0501](adr/0501-multi-cluster-fleet.md)).

## Metrics catalog

Full scrape setup, default PrometheusRule alerts, and the complete `kollect_*` metric reference live in
[Operator metrics](operator-manual/metrics.md). The table below highlights **performance and
scalability** signals — use it with the [bottleneck checklist](#early-bottleneck-checklist) below.

| Metric | Type | Labels | PromQL hint | What rising values imply |
| --- | --- | --- | --- | --- |
| `kollect_inventory_items_total` | Gauge | — | `kollect_inventory_items_total` | Stale while store grows → inventory reconcile or export lag |
| `kollect_collect_items_total` | Gauge | — | `kollect_collect_items_total` | RSS scales with store size at 10k+ objects |
| `kollect_collected_objects` | Gauge | `profile`, `gvk` | `sum by (profile, gvk) (kollect_collected_objects)` | Per-target cardinality; split profiles when labels explode |
| `kollect_reconcile_total` | Counter | `controller`, `result` | `sum(rate(kollect_reconcile_total[5m])) by (controller, result)` | Rising failure ratio → check error-class counters |
| `kollect_reconcile_errors_total` | Counter | `kind`, `error_class` | `sum(rate(kollect_reconcile_errors_total[5m])) by (error_class)` | See [ADR-0602](adr/0602-error-taxonomy.md) error classes |
| `kollect_sink_errors_total` | Counter | `reason` | `sum(rate(kollect_sink_errors_total[5m])) by (reason)` | Export failures — separate from reconcile errors ([ADR-0602](adr/0602-error-taxonomy.md)) |
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
| `kollect_informer_cluster_wide_scope` | Gauge | `group`, `version`, `resource` | `max by (group, version, resource) (kollect_informer_cluster_wide_scope)` | 1 = cluster-wide watch (RSS risk at scale) |

Additional runtime signals: Go `memstats` via pprof (`--enable-pprof`), API server `429` in operator logs.
See [Operator metrics](operator-manual/metrics.md) for profile-derived series, connection-test counters,
and example PromQL for alerting.

## Profiling

`--enable-pprof` serves standard Go profiles on `--pprof-bind-address` (default `:6060`),
separate from Prometheus metrics (`:8080` / `:8443`). Helm sets `pprof.enabled: false` by default;
enable in dev overlays only.

## Benchmarks and load tests

```bash
task bench                    # writes artifacts/bench/*.txt
KOLECT_LOAD_TEST=1 task load-test
```

For local perf summaries (`task perf-report`), see [DEVELOPMENT.md](DEVELOPMENT.md).

Default `go test ./...` excludes `load`-tagged tests.

## Early bottleneck checklist

| Symptom | Likely cause | First action |
| --- | --- | --- |
| High `kollect_informer_objects`, high RSS | Cluster-wide informer for multi-namespace targets | Namespace-scope targets; split profiles |
| High `kollect_workqueue_depth` on `inventory` | Export or aggregation on hot path | Raise inventory workers; increase `spec.exportMinInterval` |
| High export bytes rate, low object churn | Missing payload dedupe | Verify debounce + content-hash skip |
| Bench regression in `BenchmarkExtract` | CEL/JSONPath hot path | Profile extractor; check attribute count |
| High RSS on large clusters | Full in-memory collect store | Namespace-scoped targets; raise export interval ([ADR-0603](adr/0603-performance-scalability.md)) |
