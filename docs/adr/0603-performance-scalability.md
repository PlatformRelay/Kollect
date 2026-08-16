# ADR-0603: Performance and scalability

> Scale targets and tuning knobs for single-cluster operators and fleet installs via shared sinks.

**Theme:** 06 · Observability & ops · **Status:** Current

## Context

Kollect watches arbitrary GVKs, aggregates attributes in memory, and exports on inventory
reconcile. Installations span **large single clusters** (1000s of nodes, **100k collected rows
design target** per cluster) and **multi-cluster fleets** where **N independent operators** export
to a **shared sink** with `spec.cluster` partitioning ([ADR-0501](0501-multi-cluster-fleet.md)).

There is **no hub/spoke runtime tier** — each cluster runs `mode: single`. Performance bottlenecks
must surface early via operator metrics and bounded benchmarks before fleet-wide sink layouts lock in.

Large clusters need tunable controller parallelism, observable queue pressure, bounded sink churn,
optional profiling without coupling to Prometheus scrape paths, and **explicit memory bounds per
operator**.

## Scale targets

| Tier | Scope | Collected rows | Clusters | Test tier / evidence |
| --- | --- | --- | --- | --- |
| **CI / dev** | Synthetic envtest | ≤500 | 1 | `task test` — Active |
| **Extractor budget** | In-process, no cluster | n/a (micro-benchmark) | 1 | `task extract-budget` — hot-path ns/op + B/op budget; **not** a scale tier, ≠ in-cluster soak |
| **Nightly load** | Synthetic envtest | **10,000** | 1 | `scale-envtest-10k` on `ubuntu-latest-8-cores` — **disabled / opt-in** until runners exist ([load-test runbook](../operator-manual/load-test-runbook.md)) |
| **Baseline production** | Single cluster | **10,000+** | 1 | Metrics + pprof — **unverified** until a named SHA / hardware evidence is published |
| **Design target** | Single cluster | **100,000** | 1–2 cloud | Manual cloud soak — **unexecuted (AR-02)** |
| **Fleet** | Shared Postgres/Git sink | 10k–100k × N operators | **many** | One ServiceMonitor per cluster release |

**Memory bounds (per operator):**

- Collection store: O(collected rows × attribute width); **≤512 MiB** at 10k typical profiles;
  **≤400 MiB** store component at 100k when export-sharded.
- **Operator RSS @ 100k:** request **≥2 GiB**, limit **≥4 GiB** — Helm `resourcesProfile: large`.
- Informer cache: prefer namespace-scoped dynamic informers; `kollect_informer_cluster_wide_scope`
  alerts when a GVR watches all namespaces.
- Export: **mandatory multi-namespace sharding** (<~2k rows/inventory) or per-target envelopes;
  `maxExportBytes` default **1.5 MiB** ([ADR-0103](0103-etcd-limit.md)).

**Fleet path:**

- Each cluster operator exports inventory snapshots to shared backends — no central merge tier.
- Cross-cluster correlation uses **sink row metadata** (`spec.cluster`).

## Decision

1. **Controller options:** Expose `MaxConcurrentReconciles` per reconciler via operator flags.
2. **Workqueue:** Default exponential failure rate limiting; in-flight gauge `kollect_workqueue_depth`.
3. **Metrics:** Reconcile/export histograms, informer size, dispatch pool, resync rate — catalog in
   [operator-manual/performance.md](../operator-manual/performance.md).
4. **Export debounce:** Per `KollectInventory.spec.exportMinInterval` (default **30s**).
5. **Informers:** Namespace-scoped when targets agree; paginated initial `List` where allowed.
6. **Dispatch pool:** Tunable `--collect-dispatch-workers` / queue; enqueue wait, then block
   (backpressure) on the informer goroutine until queue capacity frees or ctx cancels — never
   processes inline, to keep worker concurrency the only path that does extract/access-check work.
7. **Resync / metrics sampling:** `--informer-resync-period`; `--collect-metrics-sample-interval`.
8. **Profiling:** Optional `--enable-pprof` on `:6060`; disabled in production Helm values.
9. **Tests:** `load`-tagged tests to **10k** (nightly when 8-core runners exist); 100k manual design proof only.
10. **100k claim gate:** Export sharding enforced + Postgres bulk upsert + **10k nightly green** (once re-enabled).

## Consequences

- Operators scale reconcile and dispatch throughput without rebuilding images.
- 100k/cluster requires **many inventories** — monolithic namespace rollups fail export caps.
- Fleet Postgres growth is **ops/DBA** (partition by cluster or month at >10M rows).
- 10k nightly CI is **disabled / unverified** while `ubuntu-latest-8-cores` jobs stay opt-in; 100k remains a design target until AR-02 cloud sign-off.

## References

- [ADR-0301](0301-event-driven-informers.md) — event-driven collection
- [ADR-0602](0602-error-taxonomy.md) — error classes and requeue behavior
- [ADR-0501](0501-multi-cluster-fleet.md) — multi-cluster fleet model
- [operator-manual/performance.md](../operator-manual/performance.md) — tuning guide and metrics catalog
