# Scaling and fleet operations

Guidance for operators running Kollect at **large cluster** scale (design target: **100,000
collected rows per cluster operator**) and **multi-cluster fleets** sharing Postgres, Git, or object
stores.

!!! warning "Honest scale claim"
    **100k/cluster is a design target**, not a blanket product guarantee. Proof requires a **manual
    cloud load test** ([load test runbook](load-test-runbook.md)) — not GitHub Actions runners.
    Until that gate passes, treat 100k as **architecture guidance** with mandatory export sharding.

## Collected rows vs export shards

| Term | Meaning |
| --- | --- |
| **Collected rows** | Items in the operator collect store (`kollect_collect_items_total`) |
| **Export shard** | One `KollectInventory` namespace aggregate — keep **<~2,000 rows** per shard |

Monolithic namespace inventories hit `PayloadTooLarge` above ~2,500 rows. Spread workloads across
**many namespaces**, each with its own `KollectInventory` — see
[`config/samples/kollect_v1alpha1_kollectinventory_sharded.yaml`](../../config/samples/kollect_v1alpha1_kollectinventory_sharded.yaml).

The operator sets `status.conditions[ExportShardWarning]` and increments
`kollect_export_shard_warn_total` when a namespace aggregate reaches **~1,800 rows**.

## Helm resource profiles

For large clusters, use the chart **`resourcesProfile: large`** preset (≥2 GiB request, ≥4 GiB
limit). Tune dispatch and reconcile flags per [PERFORMANCE.md](../PERFORMANCE.md).

```yaml
resourcesProfile: large
collect:
  dispatchWorkers: 8
  dispatchQueueSize: 1024
```

## Git audit @ 1h

Git snapshot sinks are for **audit cadence** (typically **1h** `exportMinInterval`), not portal
query. At scale:

1. **Shard exports** (<2k rows/inventory).
2. Set **1h** (or longer) per-ref interval on `snapshotSinkRefs`.
3. Use **`pathTemplate: clusters/{cluster}/…`** on snapshot sinks for fleet repos.
4. Operator **PERF-10** persistent mirror + **checksum fingerprint skip** avoid clone/push when
   payload is unchanged (env: `KOLLECT_GIT_MIRROR_DIR`).

## Shared Postgres fleet

Multiple cluster operators can upsert into **one Postgres sink** ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).
Each operator sets **`spec.cluster`** on database sinks; the backend primary key is
`(cluster, namespace, name, uid)`.

### Row growth

```
total_rows ≈ Σ (clusters × collected_rows_per_cluster)
```

Example: **200 clusters × 50k rows** ≈ **10M rows** — plan DBA review before sustained growth.

### When to partition (DBA)

| Signal | Action |
| --- | --- |
| Table **>~10M rows** or **>~100 GiB** | Partitioning review |
| Slow exports / vacuum pressure | Partition by **`cluster`** or **`exported_at` month** |
| Retention policy | Drop/archive old monthly partitions |

### Responsibilities

| Role | Owns |
| --- | --- |
| **Kollect operator** | Upsert semantics, `spec.cluster`, export debounce, row identity |
| **Platform / DBA** | Partition DDL, indexes, retention, connection pooling, backups |

The operator does **not** create Postgres partitions. Document expected table shape in your runbook;
use `kollect_export_duration_seconds` and sink error metrics for early warning.

### Index hints (DBA)

- Composite unique index aligned with upsert PK: `(cluster, namespace, name, uid)`
- Optional BRIN on `exported_at` for time-range portal queries
- Avoid unbounded JSONB bloat — keep attribute profiles lean ([REQUIREMENTS.md](../REQUIREMENTS.md))

## Related

- [Performance tuning](../PERFORMANCE.md)
- [Load test runbook](load-test-runbook.md)
- [Multi-cluster fleet example](../examples/multi-cluster-fleet.md)
- [ADR-0603](../adr/0603-performance-scalability.md)
