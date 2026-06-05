# ADR-0026: Performance and scalability

## Status

Accepted (revised 2026-06-05 — extreme scale + agent observability)

## Context

kollect watches arbitrary GVKs, aggregates attributes in memory, and exports on inventory
reconcile. Installations span **giant single clusters** (1000s of nodes, **10k+ watched resources
per cluster as baseline**) and **100+ cluster** hub deployments ([ADR-0022](0022-multi-cluster-sync-rfc.md)).

Performance bottlenecks must surface **early** — via operator metrics, bounded benchmarks, and
**agent-readable reports** ([ADR-0027](0027-agent-observability-feedback.md)) — before hub sharding
and spoke transport choices lock in.

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
  objects with typical Deployment/Service profiles (measure via `task perf-report` + pprof).
- Informer cache: prefer namespace-scoped dynamic informers when all targets for a GVR agree; cluster-wide
  watch only when required — document RSS delta in PERF-SNAPSHOT.
- Export payload: coalesce via `--export-debounce`; spill to object storage when payload exceeds hub
  gRPC/queue limits ([ADR-0006](0006-etcd-limit.md)).

**Hub path (100+ clusters):**

- Hub consumers process **summarized spoke snapshots** — not full object mirrors.
- Merge complexity **O(total rows)**, not O(spokes²).
- Horizontal scale: `MaxConcurrentReconciles` on hub + queue partition count.

## Decision

1. **Controller options:** Expose `MaxConcurrentReconciles` per reconciler
   (`KollectTarget`, `KollectInventory`, `KollectHub`) via operator flags with documented defaults.
2. **Workqueue:** Use controller-runtime default exponential failure rate limiting unless
   `--reconcile-rate-limit` overrides the base delay. Approximate queue depth with an in-flight
   reconcile gauge (`kollect_workqueue_depth`).
3. **Metrics:** Add reconcile duration histogram, informer indexer size gauge, and export byte
   counter alongside existing export latency histogram. Catalog in [PERFORMANCE.md](../PERFORMANCE.md)
   with PromQL hints for agents.
4. **Export debounce:** Make inventory export debounce configurable via `--export-debounce`.
5. **Informers:** Scope dynamic informers to a single namespace when all targets for a GVR agree;
   otherwise watch all namespaces and filter by `namespaceSelector` at dispatch. Paginate initial
   `List` where client-go allows.
6. **Profiling:** Optional `--enable-pprof` on `:6060`; disabled in production Helm values.
7. **Tests:** `go test -bench` for extraction; optional `load`-tagged test gated by
   `KOLECT_LOAD_TEST=1`. Results written to `artifacts/bench/` for `task perf-report`.
8. **Agent observability:** `task perf-report` + local `agent-context/PERF-SNAPSHOT.md` per
   [ADR-0027](0027-agent-observability-feedback.md).

## Consequences

- Operators can scale reconcile throughput without rebuilding images.
- In-flight gauge is an approximation, not a substitute for controller-runtime's internal queue metrics.
- Multi-namespace targets still use cluster-wide informer caches when scopes differ — document as
  known RSS cost in perf snapshots.
- 10k baseline is a **design target**, not default CI volume — agents use tier labels to avoid
  running giant tests locally.
- Hub at 100+ clusters requires Phase 2 sharding proof before claiming production readiness.

## References

- [ADR-0014](0014-event-driven-informers.md) — event-driven collection
- [ADR-0020](0020-error-taxonomy.md) — error classes and requeue behavior
- [ADR-0022](0022-multi-cluster-sync-rfc.md) — multi-cluster scale path
- [ADR-0027](0027-agent-observability-feedback.md) — agent perf feedback loop
- [PERFORMANCE.md](../PERFORMANCE.md) — tuning guide and metrics catalog
