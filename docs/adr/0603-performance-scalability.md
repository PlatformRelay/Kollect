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

| Tier | Scope | Collected rows | Clusters | Test tier |
| --- | --- | --- | --- | --- |
| **CI / dev** | Synthetic envtest | ≤500 | 1 | `task test` |
| **Opt-in load** | Synthetic | ≤2,000 | 1 | `KOLECT_LOAD_TEST=1 task load-test` |
| **Baseline production** | Single cluster | **10,000+** (validated) | 1 | Metrics + pprof; nightly optional |
| **Design target** | Single cluster | **100,000** | 1 | Manual / perf-report; delivery **v0.5+** ([gap analysis](../PERFORMANCE.md)) |
| **Fleet** | Shared Postgres/Git sink | 10k–100k × N operators | **many** | One ServiceMonitor per cluster release; correlate via `spec.cluster` |

**Memory bounds (per operator):**

- Collection store: O(collected rows × attribute width); target **≤512 MiB** working set at 10k
  objects with typical Deployment/Service profiles (measure via pprof and Prometheus RSS).
- Informer cache: prefer namespace-scoped dynamic informers when all targets for a GVR agree; cluster-wide
  watch only when required — document RSS delta in runbooks when cluster-wide scope is unavoidable.
- Export payload: coalesce via **`KollectInventory.spec.exportMinInterval`** (default **30s**);
  spill to object storage when payload exceeds inline limits ([ADR-0103](0103-etcd-limit.md),
  [ADR-0201](0201-crd-model.md)).

**Fleet path:**

- Each cluster operator exports **full inventory snapshots** (or partitioned exports when
  `pathTemplate` lands) to shared backends — no central merge tier.
- Cross-cluster correlation uses **sink row metadata** (`spec.cluster`), not hub merge counters.

## Decision

1. **Controller options:** Expose `MaxConcurrentReconciles` per reconciler
   (`KollectTarget`, `KollectInventory`, cluster rollup kinds) via operator flags with documented defaults.
2. **Workqueue:** Use controller-runtime default exponential failure rate limiting unless
   `--reconcile-rate-limit` overrides the base delay. Approximate queue depth with an in-flight
   reconcile gauge (`kollect_workqueue_depth`).
3. **Metrics:** Reconcile duration histogram, informer indexer size gauge, export byte
   counter, export debounce counter alongside existing export latency histogram. Catalog in
   [PERFORMANCE.md](../PERFORMANCE.md) with PromQL hints.
4. **Export debounce:** Per **`KollectInventory.spec.exportMinInterval`** (default **30s**)
   ([ADR-0201](0201-crd-model.md)).
5. **Informers:** Scope dynamic informers to a single namespace when all targets for a GVR agree;
   otherwise watch all namespaces and filter by `namespaceSelector` at dispatch. Paginate initial
   `List` where client-go allows.
6. **Dispatch pool:** GVR-indexed worker pool for collect engine refresh (shipped v0.3); tunable
   workers/queue deferred to v0.4+ ([PERF-SNAPSHOT](../PERFORMANCE.md)).
7. **Profiling:** Optional `--enable-pprof` on `:6060`; disabled in production Helm values.
8. **Tests:** `go test -bench` for extraction; optional `load`-tagged test gated by
   `KOLECT_LOAD_TEST=1`. Results written to `artifacts/bench/` for local regression tracking.
9. **100k claim gate:** Do not advertise 100k/cluster production readiness until export
   partitioning, Postgres bulk upsert, and perf-report tiers pass ([SCALABILITY gap — local agent-context]).

## Consequences

- Operators can scale reconcile throughput without rebuilding images.
- In-flight gauge is an approximation, not a substitute for controller-runtime's internal queue metrics.
- Multi-namespace targets still use cluster-wide informer caches when scopes differ — document as
  known RSS cost in operator runbooks.
- 10k baseline is **validated in CI tiers**; 100k is a **design target** with honest delivery timeline.
- Fleet observability = **scrape `/metrics` on each cluster operator** — no hub federation tier.

## References

- [ADR-0301](0301-event-driven-informers.md) — event-driven collection
- [ADR-0602](0602-error-taxonomy.md) — error classes and requeue behavior
- [ADR-0501](0501-multi-cluster-fleet.md) — multi-cluster fleet model
- [PERFORMANCE.md](../PERFORMANCE.md) — tuning guide and metrics catalog
