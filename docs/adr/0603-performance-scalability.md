# ADR-0603: Performance and scalability

> Scale targets and the knobs to meet them: 10k+ objects/spoke, 100+ clusters, O(rows) hub merge.

**Theme:** 06 · Observability & ops · **Status:** Current

## Context

Kollect watches arbitrary GVKs, aggregates attributes in memory, and exports on inventory
reconcile. Installations span **giant single clusters** (1000s of nodes, **10k+ watched resources
per cluster as baseline**) and **100+ cluster** hub deployments ([ADR-0501](0501-multi-cluster-sync-rfc.md)).

Performance bottlenecks must surface **early** — via operator metrics and bounded benchmarks —
before hub sharding and spoke transport choices lock in.

Large clusters need tunable controller parallelism, observable queue pressure, bounded sink churn,
optional profiling without coupling to Prometheus scrape paths, and **explicit memory bounds per
spoke**.

## Scale targets

| Tier | Scope | Objects (watched) | Clusters | Test tier |
| --- | --- | --- | --- | --- |
| **Baseline** | Single spoke / dev | 10,000+ | 1 | `task test` ≤500 synthetic; `task load-test` ≤2000 |
| **Large spoke** | Production cluster | 50,000+ (stretch) | 1 | Manual / nightly only |
| **Hub path** | Platform rollup | 10k × N spokes (summarized) | **100+** | Hub merge benchmarks; no full mesh in CI |
| **Giant cluster** | Node-heavy | 1000+ nodes, 10k+ resources | 1 | Scoped informers + pagination mandatory |

**Memory bounds (spoke):**

- Collection store: O(collected rows × attribute width); target **≤512 MiB** working set at 10k
  objects with typical Deployment/Service profiles (measure via pprof and Prometheus RSS).
- Informer cache: prefer namespace-scoped dynamic informers when all targets for a GVR agree; cluster-wide
  watch only when required — document RSS delta in runbooks when cluster-wide scope is unavoidable.
- Export payload: coalesce via **`KollectInventory.spec.exportMinInterval`** (default **30s**);
  spill to object storage when payload exceeds hub gRPC/queue limits ([ADR-0103](0103-etcd-limit.md),
  [ADR-0703](0703-platform-architecture-pivot.md)).

**Hub path (100+ clusters):**

- Hub consumers process **summarized spoke snapshots** — not full object mirrors.
- Merge complexity **O(total rows)**, not O(spokes²).
- Horizontal scale: `MaxConcurrentReconciles` on hub + queue partition count.

## Decision

1. **Controller options:** Expose `MaxConcurrentReconciles` per reconciler
   (`KollectTarget`, `KollectInventory`, hub mode) via operator flags with documented defaults.
2. **Workqueue:** Use controller-runtime default exponential failure rate limiting unless
   `--reconcile-rate-limit` overrides the base delay. Approximate queue depth with an in-flight
   reconcile gauge (`kollect_workqueue_depth`).
3. **Metrics:** Add reconcile duration histogram, informer indexer size gauge, and export byte
   counter alongside existing export latency histogram. Catalog in [PERFORMANCE.md](../PERFORMANCE.md)
   with PromQL hints for operators.
4. **Export debounce:** Per **`KollectInventory.spec.exportMinInterval`** (default **30s**); legacy
   `--export-debounce` flag deprecated once field is wired ([ADR-0703](0703-platform-architecture-pivot.md)).
5. **Informers:** Scope dynamic informers to a single namespace when all targets for a GVR agree;
   otherwise watch all namespaces and filter by `namespaceSelector` at dispatch. Paginate initial
   `List` where client-go allows.
6. **Profiling:** Optional `--enable-pprof` on `:6060`; disabled in production Helm values.
7. **Tests:** `go test -bench` for extraction; optional `load`-tagged test gated by
   `KOLECT_LOAD_TEST=1`. Results written to `artifacts/bench/` for local regression tracking.

## Consequences

- Operators can scale reconcile throughput without rebuilding images.
- In-flight gauge is an approximation, not a substitute for controller-runtime's internal queue metrics.
- Multi-namespace targets still use cluster-wide informer caches when scopes differ — document as
  known RSS cost in operator runbooks.
- 10k baseline is a **design target**, not default CI volume — use bounded test tiers locally and in CI.
- Hub at 100+ clusters requires Phase 2 sharding proof before claiming production readiness.

## References

- [ADR-0301](0301-event-driven-informers.md) — event-driven collection
- [ADR-0602](0602-error-taxonomy.md) — error classes and requeue behavior
- [ADR-0501](0501-multi-cluster-sync-rfc.md) — multi-cluster scale path
- [PERFORMANCE.md](../PERFORMANCE.md) — tuning guide and metrics catalog
