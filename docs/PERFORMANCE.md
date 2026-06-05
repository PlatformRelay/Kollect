# Performance and scalability

kollect is designed for **giant single clusters** (1000s of nodes, **10k+ watched resources
baseline**) and **100+ cluster** hub deployments. This guide summarizes tuning knobs from
[ADR-0026](adr/0026-performance-scalability.md) and the **agent observability loop** from
[ADR-0027](adr/0027-agent-observability-feedback.md).

## Scale tiers

| Tier | Watched objects | Clusters | How to validate |
| --- | --- | --- | --- |
| Dev / CI default | ≤500 synthetic | 1 | `task test` |
| Opt-in load | ≤2000 synthetic | 1 | `KOLECT_LOAD_TEST=1 task load-test` |
| Baseline production spoke | 10,000+ | 1 | Metrics + pprof; manual load |
| Hub platform | 10k × N (summarized) | **100+** | Hub merge benchmarks; sharded queue |
| Stretch spoke | 50,000+ | 1 | Scoped informers; object-store spillover |

**60 clusters is not the limit** — hub merge must stay **O(total rows)**, never O(spokes²).

## Controller parallelism

| Flag | Default | Controller |
| --- | --- | --- |
| `--max-concurrent-reconciles-target` | `5` | `KollectTarget` |
| `--max-concurrent-reconciles-inventory` | `3` | `KollectInventory` |
| `--max-concurrent-reconciles-hub` | `2` | `KollectHub` |

Raise concurrency when reconcile latency grows while CPU is underutilized. Lower it when
API server throttling or etcd watch pressure appears.

## Workqueue rate limiting

When `--reconcile-rate-limit` is **unset** (`0`), controller-runtime uses its default
exponential failure rate limiter (5ms base, 1000s cap). Set a positive duration (for example
`100ms`) to slow retries on terminal errors without changing success-path throughput.

`kollect_workqueue_depth` approximates queue pressure as **in-flight reconciles** per controller
(not the internal client-go queue length).

## Export debouncing

`--export-debounce` (default `30s`) coalesces identical inventory payloads per `KollectInventory`
before hitting external sinks. Lower for fresher Git/DB exports; raise to reduce sink API load.
At 100+ spokes, debouncing is **mandatory** on the hub path to avoid export storms.

## Collection engine

- **In-memory store:** O(n) memory in collected object count; one `RWMutex` guards nested maps.
  Target **≤512 MiB** RSS at 10k typical Deployment/Service rows (verify with pprof).
- **Informers:** When all active targets for a GVK resolve to **one** namespace via
  `spec.namespaceSelector`, the dynamic informer is scoped to that namespace. Otherwise the
  informer watches all namespaces and filters events at dispatch time (correctness over cache size).
- **Resync:** 12h informer resync is a correctness backstop, not a freshness driver.
- **Spoke → hub:** Push **summarized deltas**, not per-object streams ([ADR-0022](adr/0022-multi-cluster-sync-rfc.md)).

## Metrics catalog (for operators and agents)

Use these names when scraping `/metrics` or writing PromQL in issues and ADRs.

| Metric | Type | Labels | PromQL hint | Agent interpretation |
| --- | --- | --- | --- | --- |
| `kollect_workqueue_depth` | Gauge | `controller` | `max_over_time(kollect_workqueue_depth[5m])` | Sustained high values → raise `--max-concurrent-reconciles-*` or reduce reconcile work |
| `kollect_reconcile_duration_seconds` | Histogram | `controller` | `histogram_quantile(0.99, sum(rate(kollect_reconcile_duration_seconds_bucket[5m])) by (le, controller))` | p99 rising while depth low → slow API/sink; p99 rising with depth → under-provisioned workers |
| `kollect_informer_objects` | Gauge | `gvr` | `sum by (gvr) (kollect_informer_objects)` | Unexpected growth → extra GVR watches or cluster-wide scope; check namespace scoping |
| `kollect_export_bytes_total` | Counter | `sink_type` | `rate(kollect_export_bytes_total[5m])` | Spike → debounce too low or inventory churn; flat while stale → export path stuck |
| `kollect_export_duration_seconds` | Histogram | `sink_type` | `histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))` | Sink slowness (Git/Postgres/Kafka) — not collection |
| `kollect_reconcile_errors_total` | Counter | `controller`, `class` | `sum(rate(kollect_reconcile_errors_total[5m])) by (class)` | See [ADR-0020](adr/0020-error-taxonomy.md) error classes |

Additional runtime signals: Go `memstats` via pprof (`--enable-pprof`), API server `429` in operator logs.

## Profiling

`--enable-pprof` serves standard Go profiles on `--pprof-bind-address` (default `:6060`),
separate from Prometheus metrics (`:8080` / `:8443`). Helm sets `pprof.enabled: false` by default;
enable in dev overlays only.

## Benchmarks, load tests, and agent reports

```bash
task bench                    # writes artifacts/bench/*.txt
KOLECT_LOAD_TEST=1 task load-test
task perf-report              # JSON summary to stdout
task perf-report --format=markdown > agent-context/PERF-SNAPSHOT.md
```

`task perf-report` aggregates:

- Last build / test / bench durations (from task cache and `artifacts/bench/`)
- Latest benchmark ns/op and allocs/op for `BenchmarkExtract`
- Metric names + PromQL hints (this catalog)
- Active scale tier label (`dev` | `load` | `ci`)

**Agents:** read `agent-context/PERF-SNAPSHOT.md` (local, gitignored) before perf tasks; regenerate
after benchmark changes. CI may upload `artifacts/bench/` on nightly runs ([ADR-0027](adr/0027-agent-observability-feedback.md)).

Default `go test ./...` excludes `load`-tagged tests.

## Early bottleneck checklist

| Symptom | Likely cause | First action |
| --- | --- | --- |
| High `kollect_informer_objects`, high RSS | Cluster-wide informer for multi-namespace targets | Namespace-scope targets; split profiles |
| High `kollect_workqueue_depth` on `inventory` | Export or aggregation on hot path | Raise inventory workers; increase `--export-debounce` |
| High export bytes rate, low object churn | Missing payload dedupe | Verify debounce + content-hash skip |
| Bench regression in `BenchmarkExtract` | CEL/JSONPath hot path | Profile extractor; check attribute count |
| Hub OOM at many spokes | Full mirror in hub RAM | Sharded consumers; spoke summaries only ([ADR-0022](adr/0022-multi-cluster-sync-rfc.md)) |
