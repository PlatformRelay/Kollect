# Load test runbook (100k design proof)

> **Status: PLANNED — not yet executed.** Maintainer validation on **public cloud** (GKE target) is
> the gate for an honest **100k collected rows/cluster** claim. **Do not** run 100k on
> `ubuntu-latest` GitHub Actions runners.

## Scope

| Tier | Objects | Where | Status |
| --- | --- | --- | --- |
| CI default | 500 | envtest (`task test`) | ✅ Active |
| CI extended | 2,000 | `KOLECT_LOAD_TEST=1 task load-test` | ✅ Opt-in |
| Nightly | 10,000 | `ubuntu-latest-8-cores` workflow | ✅ Active |
| **Design proof** | **100,000** | **2× public cloud clusters** | ⬜ **Planned (GKE)** |

**10k nightly on GH runners is OK.** **100k = manual cloud gate only.**

## Prerequisites

1. **Two** Kubernetes clusters (GKE/EKS/AKS — maintainer plan: GKE).
2. Shared sink endpoints reachable from both clusters:
   - Postgres (primary query path)
   - Git snapshot @ **1h** cadence
   - S3 or GCS object store (optional spill path)
   - Optional NATS/Kafka event sink
3. Clone kollect @ release SHA; set `KOLLECT_SRC` to repo root.
4. Helm install with **`resourcesProfile: large`** on each cluster.
5. Namespace-scoped targets + **sharded inventories** (one per workload namespace).

## Manifest bundle

Generator lives under [`hack/loadtest/100k/`](../../hack/loadtest/100k/):

```bash
cd hack/loadtest/100k
./generate.sh --namespaces 50 --deployments-per-ns 2000
# → ~100k Deployments across 50 namespaces (adjust flags to hit collect-store row target)
kubectl apply -k manifests/
```

Tune `--namespaces` and `--deployments-per-ns` so `kollect_collect_items_total` approaches **100k**
after filters, not merely raw API object count.

## Step-by-step (per cluster)

### 1. Install operator

```bash
helm upgrade --install kollect oci://ghcr.io/konih/kollect/charts/kollect \
  --namespace kollect-system --create-namespace \
  -f hack/loadtest/100k/values-large.yaml
```

### 2. Apply sinks + sharded inventories

```bash
kubectl apply -f hack/loadtest/100k/sinks.yaml
kubectl apply -k hack/loadtest/100k/manifests/
```

Ensure each workload namespace has a `KollectInventory` with **<2k rows** per shard.

### 3. Soak

Run **≥4 hours** steady state. Record:

- `kollect_collect_items_total`
- `kollect_reconcile_duration_seconds` p95
- `kollect_export_duration_seconds` by sink type
- Pod RSS / CPU throttling

### 4. Shared sink verification

| Sink | Check |
| --- | --- |
| Postgres | `SELECT cluster, count(*) FROM … GROUP BY 1` — both clusters present |
| Git | Commit cadence ~1h; fingerprint skip reduces empty pushes |
| S3/GCS | Object count matches export generations |

## Diagnosis

When the load test shows bottlenecks, use this matrix:

| Symptom | Metrics / tools | Likely cause | Next action |
| --- | --- | --- | --- |
| CPU throttle | pprof, `kollect_reconcile_duration_seconds`, pod limits | dispatch pool, marshal | Raise `collect.dispatchWorkers`; add inventory sharding |
| OOM | RSS, `kollect_collect_items_total`, `kollect_informer_objects` | cluster-wide informers | Namespace-scope targets; `resourcesProfile: large` |
| Export timeout | `kollect_export_duration_seconds`, `kollect_sink_errors_total` | Postgres row loop / git clone | PERF-09 bulk; PERF-10 mirror + fingerprint skip |
| PayloadTooLarge | inventory `Degraded`, `kollect_export_spill_warn_total` | monolithic inventory | Multi-namespace inventories |
| etcd/API slow | apiserver metrics, status update rate | status churn | Longer `exportMinInterval`; debounce already shipped |

### PromQL snippets

```promql
# Collect store size
kollect_collect_items_total

# Reconcile latency p95
histogram_quantile(0.95, sum(rate(kollect_reconcile_duration_seconds_bucket[5m])) by (le, controller))

# Export latency by sink
histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))

# Sharding warning
increase(kollect_export_shard_warn_total[1h])
```

### pprof

```bash
kubectl port-forward -n kollect-system deploy/kollect-controller-manager 6060:6060
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Local perf snapshot

```bash
task perf-report   # → agent-context/PERF-SNAPSHOT.md (maintainer local)
```

### Log keys

Search operator logs for: `export failed`, `PayloadTooLarge`, `debounced`, `dispatch sync fallback`,
`export payload exceeds spill warn`.

## Explicit non-goals

- **No** 100k job in `.github/workflows/` on `ubuntu-latest`
- **No** GKE execution in CI — maintainer runs manually when ready

## Related

- [Scaling and fleet](scaling-and-fleet.md)
- [PERFORMANCE.md](../PERFORMANCE.md)
- [ADR-0603](../adr/0603-performance-scalability.md)
