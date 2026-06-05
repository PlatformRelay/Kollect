# Performance tuning guide

This guide explains how to run **kollect** at scale (10,000+ watched objects), observe bottlenecks,
and run **bounded** load tests on a laptop or kind cluster. Design rationale and acceptance criteria
are in [ADR-0026: Performance and scalability](adr/0026-performance-scalability.md).

## Scale expectations

| Dimension | Target |
| --- | --- |
| Watched objects | 10,000+ (with namespace/label selectors) |
| Memory | Bounded via scoped informers and paginated lists |
| Reconcile | Parallel workers; no single-target stall of the whole queue |
| etcd | Status holds summaries only — not full inventory payloads ([ADR-0006](adr/0006-etcd-limit.md)) |

## Operator flags

Configure the manager via command-line flags or Helm `values.yaml` (controller manager container args).

| Flag | Default (planned) | Purpose |
| --- | --- | --- |
| `--max-concurrent-reconciles` | Controller-specific (e.g. 5) | Parallel reconcile workers per controller |
| `--informer-resync-period` | Long (e.g. 10h+) | Correctness backstop only — not a freshness knob |
| `--pprof-bind-address` | `:6060` when enabled | pprof HTTP endpoint (feature-gated) |
| `--enable-pprof` / feature gate | `false` | Opt-in profiling server |

Helm example (when wired):

```yaml
controllerManager:
  pprof:
    enabled: false
  maxConcurrentReconciles:
    target: 5
    inventory: 3
  informerResyncPeriod: 10h
```

!!! warning "Production pprof"
    Keep pprof disabled in production unless you need a short-lived debug window. Prefer
    `kubectl port-forward` to localhost rather than exposing `:6060` on a Service.

## Prometheus metrics

Scrape the operator `/metrics` endpoint (controller-runtime default, typically port 8443 or as
configured in Helm).

### Reconcile and queue

| Metric | Labels | How to use |
| --- | --- | --- |
| `kollect_reconcile_total` | `controller`, `result` | Reconcile rate; compare `success` vs `failure` |
| `kollect_reconcile_errors_total` | `kind`, `error_class` | Mix of `transient`, `terminal`, `forbidden` |
| `kollect_reconcile_duration_seconds` | `controller` | p95/p99 reconcile latency |
| `kollect_workqueue_depth` | `controller` | Backpressure — sustained high depth → increase workers or reduce work |
| `kollect_workqueue_latency_seconds` | `controller` | Time items wait before reconcile starts |

### Collection and cache

| Metric | Labels | How to use |
| --- | --- | --- |
| `kollect_collected_objects` | `profile`, `gvk` | Objects in collection store per profile |
| `kollect_collect_items_total` | — | Total in-memory store size |
| `kollect_informer_cache_objects` | `gvk` | Informer cache size — memory proxy |
| `kollect_export_duration_seconds` | `sink_type` | Sink latency; correlate with circuit breaker opens |

### Example PromQL

```promql
# Reconcile error rate (5m)
rate(kollect_reconcile_errors_total[5m])

# p95 reconcile duration by controller
histogram_quantile(0.95, sum(rate(kollect_reconcile_duration_seconds_bucket[5m])) by (le, controller))

# Workqueue backlog
kollect_workqueue_depth
```

Built-in controller-runtime workqueue metrics (prefix `workqueue_*`) may also appear on `/metrics`;
use them alongside `kollect_*` series when debugging queue stalls.

## pprof cookbook

1. Enable pprof (feature gate or `--enable-pprof`) and port-forward:

```sh
kubectl -n kollect-system port-forward deploy/kollect-controller-manager 6060:6060
```

2. Capture profiles while reproducing load:

```sh
# CPU (30s sample)
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile?seconds=30

# Heap
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap

# Goroutines (deadlock / leak hints)
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/goroutine
```

3. Compare heap profiles before/after scaling watched objects to find informer cache growth.

Common hot paths: CEL/JSONPath extraction, JSON marshal for export, workqueue `Get`/`Done` loops.

## Local and kind load tests (bounded)

**Do not** run 10k-object tests on a dev laptop by default. Use the opt-in tiers below.

### Micro-benchmarks (safe)

```sh
task bench
# equivalent:
go test -short -bench=. -benchmem ./internal/collect/...
```

Runs `BenchmarkExtract` and related benches with `-short`; suitable for CI and pre-push checks.

### Synthetic envtest load (opt-in)

```sh
KOLECT_LOAD_TEST=1 task load-test
# equivalent:
KOLECT_LOAD_TEST=1 go test -tags=load -count=1 -timeout=15m ./test/load/...
```

- Requires **`KOLECT_LOAD_TEST=1`** and build tag **`load`**.
- Hard cap **2000** synthetic objects (enforced in test harness).
- Not part of default `task test` or PR CI.

### kind smoke with many Deployments (manual, bounded)

For integration-style validation without envtest:

```sh
kind create cluster --name kollect-perf
task install:crds && task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-perf
task deploy:operator
kubectl apply -k config/samples/

# Create N deployments in one namespace (example N=200 — adjust down if laptop struggles)
for i in $(seq 1 200); do
  kubectl create deployment "load-$i" --image=nginx:alpine -n default --dry-run=client -o yaml | kubectl apply -f -
done

# Watch metrics and conditions
kubectl -n kollect-system port-forward svc/kollect-controller-manager-metrics-service 8443:8443
curl -k https://localhost:8443/metrics | rg kollect_
```

Tear down: `kind delete cluster --name kollect-perf`.

## Tuning checklist

1. **Scope watches** — tighten `namespaceSelector` and `labelSelector` on `KollectTarget`.
2. **Raise workers** — increase `--max-concurrent-reconciles` if queue depth stays high and CPU has headroom.
3. **Sink storms** — watch `kollect_export_duration_seconds` and circuit-breaker logs; fix backend before raising reconcile concurrency.
4. **SAR failures** — expect `error_class=forbidden` and partial collection; do not treat as global outage.
5. **Resync period** — keep long; shortening increases API list traffic without improving event latency.

## Further reading

- [ADR-0026: Performance and scalability](adr/0026-performance-scalability.md)
- [ADR-0014: Event-driven informers](adr/0014-event-driven-informers.md)
- [ADR-0020: Error taxonomy](adr/0020-error-taxonomy.md)
- [DEVELOPMENT.md](DEVELOPMENT.md) — `task bench`, `task load-test`, test pyramid
