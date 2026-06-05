# Performance and scalability

Kollect is designed for **giant single clusters** (1000s of nodes, **10k+ watched resources
baseline**) and **100+ cluster** hub deployments. This guide summarizes tuning knobs from
[ADR-0603](adr/0603-performance-scalability.md).

## Scale tiers

| Tier | Watched objects | Clusters | How to validate |
| --- | --- | --- | --- |
| Dev / CI default | ≤500 synthetic | 1 | `task test` |
| Opt-in load | ≤2000 synthetic | 1 | `KOLECT_LOAD_TEST=1 task load-test` |
| Baseline production spoke | 10,000+ | 1 | Metrics + pprof; manual load |
| Hub platform | 10k × N (summarized) | **100+** | Hub merge benchmarks; sharded queue |
| Stretch spoke | 50,000+ | 1 | Scoped informers; object-store spillover |

Hub scale targets are defined in [ADR-0603](adr/0603-performance-scalability.md) — hub merge must
stay **O(total rows)**, never O(spokes²).

## Controller parallelism

| Flag | Default | Controller |
| --- | --- | --- |
| `--max-concurrent-reconciles-target` | `5` | `KollectTarget` |
| `--max-concurrent-reconciles-inventory` | `3` | `KollectInventory` |
| `--max-concurrent-reconciles-hub` | `2` | Hub mode (`mode: hub`) |

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
min interval ([ADR-0703](adr/0703-platform-architecture-pivot.md)).
Lower the interval for fresher Postgres/Kafka exports; raise to reduce sink API load. At 100+ spokes,
debouncing is **mandatory** on the hub path to avoid export storms.

## Collection engine

- **In-memory store:** O(n) memory in collected object count; one `RWMutex` guards nested maps.
  Target **≤512 MiB** RSS at 10k typical Deployment/Service rows (verify with pprof).
- **Informers:** When all active targets for a GVK resolve to **one** namespace via
  `spec.namespaceSelector`, the dynamic informer is scoped to that namespace. Otherwise the
  informer watches all namespaces and filters events at dispatch time (correctness over cache size).
- **Resync:** 12h informer resync is a correctness backstop, not a freshness driver.
- **Spoke → hub:** Push **summarized deltas**, not per-object streams ([ADR-0501](adr/0501-multi-cluster-sync-rfc.md)).

## Metrics catalog

Use these names when scraping `/metrics` or writing PromQL in runbooks and issues.

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
| `kollect_hub_spoke_reports_total` | Counter | `hub`, `result` | `sum(rate(kollect_hub_spoke_reports_total[5m])) by (hub, result)` | Hub fan-in throughput; flat at zero → transport or spoke agent not wired |

Additional runtime signals: Go `memstats` via pprof (`--enable-pprof`), API server `429` in operator logs.

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
| Hub OOM at many spokes | Full mirror in hub RAM | Sharded consumers; spoke summaries only ([ADR-0501](adr/0501-multi-cluster-sync-rfc.md)) |
