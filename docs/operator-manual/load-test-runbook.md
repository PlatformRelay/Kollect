# Load test runbook (100k design proof)

> **Status: PLANNED — not yet executed (AR-02).** Maintainer validation on **public cloud**
> (GKE target) is the gate for an honest **100k collected rows/cluster** claim. **Do not** run
> 100k on `ubuntu-latest` GitHub Actions runners.

## Scope

Every public scale tier states **workload shape**, **execution layer**, and **last evidence** (or
plainly planned / unverified / disabled). Re-enable Active wording for Nightly 10k only after
`ubuntu-latest-8-cores` runners exist **and** a green artifact is published.

| Tier | Workload shape | Execution layer | Last evidence / status |
| --- | --- | --- | --- |
| CI default | ≤500 synthetic objects | envtest (`task test`) | ✅ Active — every PR / local `task test` |
| CI extended | ≤2,000 synthetic objects | envtest opt-in (`KOLECT_LOAD_TEST=1 task load-test`) | ✅ Opt-in — **synthetic extraction**; ≠ in-cluster 10k collection/export/soak proof |
| Nightly 10k | 10,000 synthetic objects | `task load-test:10k` / scale envtest on `ubuntu-latest-8-cores` | ⬜ **Disabled / opt-in / unverified** — `load-test-10k` and `scale-envtest-10k` in `.github/workflows/e2e-nightly.yaml` run only on `workflow_dispatch` with `run_scale_jobs=true` because 8-core runners are unavailable; **no current green SHA** |
| Laptop / L4.5 lab | Bounded schedule (e.g. `quick+sinks`) | Single-host existing cluster (Kind / K3s / Talos) | Named pin only — see [local lab runbook](local-lab-runbook.md) and [lab evidence bundle](lab-evidence-bundle.md); **does not** satisfy the 100k / two-cluster gate |
| **Design proof** | **100,000** collected rows | **2× public cloud clusters** (GKE target) | ⬜ **Planned / unexecuted (AR-02)** — no SHA / date / hardware yet |

**Bounded `task load-test` (≤2k) is not in-cluster 10k proof.** Nightly 10k CI is **not** Active
while 8-core jobs stay disabled. **100k = manual cloud gate only** — laptop / lab READY WITH
CONDITIONS evidence does not close AR-02.

### Evidence index (publication)

| Claim | Required artifact | Current state |
| --- | --- | --- |
| CI ≤500 | PR / `task test` logs | Active |
| Opt-in ≤2k | Local / CI with `KOLECT_LOAD_TEST=1` | Opt-in synthetic |
| Nightly 10k | Green `load-test-10k` / `scale-envtest-10k` run (SHA + date + runner) | **Unverified** — jobs disabled |
| Laptop L4.5 | Redacted [lab evidence bundle](lab-evidence-bundle.md) for a named pin | Bounded single-host only |
| 100k / two-cluster | Completed soak per this runbook (metrics + SHA + hardware) | **Unexecuted** |

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

Generator lives under [`hack/loadtest/100k/`](https://github.com/platformrelay/kollect/tree/main/hack/loadtest/100k):

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
helm upgrade --install kollect oci://ghcr.io/platformrelay/kollect/charts/kollect \
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
task perf-report   # local → agent-context/PERF-SNAPSHOT.md; CI → artifacts/perf-snapshot.md (artifact)
```

### Log keys

Search operator logs for: `export failed`, `PayloadTooLarge`, `debounced`,
`export payload exceeds spill warn`. Watch `kollect_collect_dispatch_backpressure_total` for
dispatch queue saturation (no dedicated log line — metric only).

## Explicit non-goals

- **No** 100k job in `.github/workflows/` on `ubuntu-latest`
- **No** GKE execution in CI — maintainer runs manually when ready
- **No** treating laptop / Talos L4.5 evidence as the 100k / two-cluster claim
- **No** equating bounded `task load-test` synthetic extraction with Nightly 10k or in-cluster soak

## Related

- [Local lab runbook](local-lab-runbook.md) — existing-cluster / laptop L4.5 path (not this 100k gate)
- [Lab evidence bundle and redaction](lab-evidence-bundle.md)
- [Scaling and fleet](performance.md)
- [ADR-0603](../adr/0603-performance-scalability.md)
