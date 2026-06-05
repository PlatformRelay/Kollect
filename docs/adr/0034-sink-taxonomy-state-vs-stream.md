# ADR-0034: Sink taxonomy — state stores vs event emitters; hub as optional tier

## Status

Accepted (2026-06-05)

## Context

Earlier ADRs listed Postgres and Kafka as co-equal "primary sinks"
([ADR-0025](0025-sink-backends-database-kafka.md)) and positioned a **hub** merge tier as the
multi-cluster answer ([ADR-0022](0022-multi-cluster-sync-rfc.md), [ADR-0023](0023-lean-queue-transport.md)).
Critical review surfaced three problems:

1. **Postgres and Kafka are not the same kind of thing.** Postgres answers *"what is deployed now?"*
   (queryable state); Kafka answers *"what changed?"* (an event log whose consumer builds its own
   view). Listing them side-by-side conflates a **destination** with a **pipe**.
2. **The hub re-implements what the backend already does.** For a shared Postgres, the upsert primary
   key `(cluster, ns, name, uid)` *is* the cross-cluster merge; for Kafka, partitioning *is* the
   sharding. A separate hub merge library + transport + push-auth is redundant for these backends.
3. **The current Postgres sink only upserts — it never deletes.** When a resource disappears its row
   persists, so the inventory drifts stale. Delete reconciliation is the genuinely hard, shared work
   — and snapshot-shaped sinks (Git/object store/HTTP) get deletes for free.

## Decision

### 1. Classify sinks by role, not by vendor

| Role | Backends | Answers | Deletes |
| --- | --- | --- | --- |
| **Snapshot store** | Git, **S3/GCS Parquet**, HTTP | current state, written whole each cycle | **free** (absent = deleted) |
| **Relational SoR** | Postgres | current state, rich SQL/joins for portals | requires **delete reconciliation** |
| **Event emitter** | **NATS JetStream** (lean default), **Kafka/Redpanda** (enterprise opt-in) | change stream for downstream integration | tombstone (consumer-owned) |

The **in-memory snapshot per `KollectInventory` is the canonical artifact.** Every sink is a
projection of it. Snapshot stores serialize it directly; relational/event sinks derive from it via a
shared diff step.

### 2. S3/GCS Parquet snapshot sink (queryable without a database)

Write one Parquet file per inventory per export, partitioned:

```
s3://bucket/inventory/cluster=<c>/ns=<ns>/name=<inv>/generation=<g>.parquet
```

- "Current inventory" = the **latest generation** per partition (documented view/macro).
- **Queryable by DuckDB / Athena** with **no database server** — predicate pushdown + partition
  pruning read only needed byte ranges.
- **Deletes are correct by construction** — absent from the latest snapshot = gone.
- Arbitrary profile attributes serialize to a JSON/struct column (DuckDB queries JSON natively).
- Frequent exports → many small files: rely on **`exportMinInterval`** (default 30s) and document a
  periodic compaction job. ACID update-in-place (Iceberg/DuckLake) is **out of scope** — kollect
  overwrites whole snapshots, so no table catalog/metadata DB is required.

This is the recommended **"small/medium platform wants queryable inventory without running a DB"**
option, complementing Git (audit) and Postgres (rich relational portal).

### 3. Postgres gains delete reconciliation

The Postgres sink must diff the current snapshot against the prior export per `(cluster, inventory)`
and **delete rows** for resources no longer present (or write a `deleted_at` tombstone). Upsert-only
is a correctness bug.

### 4. Event emit: NATS default, Kafka opt-in, unified with transport

- **NATS JetStream** is the **lean default** event backbone — single binary, sub-ms, no JVM/ZooKeeper;
  best fit for kollect's low-volume, high-fan-out change events.
- **Kafka (and Redpanda via the Kafka API)** is the **enterprise opt-in** — chosen when an org already
  operates Kafka and wants its connector/schema-registry ecosystem. One `kafka` driver covers both.
- The **Kafka sink (ADR-0025) and the hub transport (ADR-0023) collapse into one event-emitter
  abstraction.** Because multi-cluster fan-in is now *direct to a shared sink* (see §5), a spoke
  publishing to a shared NATS/Kafka subject **is** the fan-in — there is no separate transport.

### 5. Hub demoted to an optional aggregation tier

Direct **shared-sink fan-in** is the default multi-cluster topology: each operator exports to a shared
backend with `spec.cluster` set; the backend's key/PK provides the merge.

The **hub tier is opt-in**, justified only by constraints a shared backend cannot meet:

| Use the hub when | Why |
| --- | --- |
| **Git is the multi-cluster SoR** | Direct Git fan-in = N commits per change (rejected anti-pattern); needs aggregation |
| **Network isolation** | Spokes cannot reach a central DB/broker; one hub ingress is firewall/mTLS-friendly |
| **Credential centralization** | One DB/broker credential at the hub vs N spokes holding write creds |
| **Schema decoupling** | Spokes speak the stable report schema; hub owns DB schema/migrations |

Otherwise, no hub. This supersedes the "hub is the multi-cluster answer" framing in ADR-0022/0023.

## Consequences

### Positive

- Honest model: **state stores vs event emitters**, not redundant twin primaries.
- Parquet snapshot gives queryable inventory with **no server** and **correct deletes**.
- One event-emitter abstraction (NATS/Kafka) for both emit and optional hub fan-in — less surface.
- Most multi-cluster installs need **no hub** and **no transport auth** (ADR-0028 becomes opt-in).

### Negative

- New Parquet writer dependency + attribute→column mapping; compaction guidance needed.
- Postgres delete-reconciliation is new logic (diff vs last export).
- NATS is a new system for Kafka-only shops; mitigated by Kafka opt-in.
- Sink enum gains `nats`; object-store sinks gain a `format: parquet` mode — codegen + webhook + tests.

## Supersedes / amends

- [ADR-0025](0025-sink-backends-database-kafka.md) — sinks classified by role; Parquet added; Postgres deletes; NATS added.
- [ADR-0022](0022-multi-cluster-sync-rfc.md) — hub demoted to optional; direct shared-sink fan-in is default.
- [ADR-0023](0023-lean-queue-transport.md) — NATS elevated to lean default; transport unified with event sink.
- [ADR-0032](0032-platform-architecture-pivot.md) — sink/portal narrative refined.

## Open questions

- **OPEN:** Parquet attribute schema — single JSON column vs typed columns per profile attribute?
- **OPEN:** Compaction — operator-run job, external (S3 Tables auto-compaction), or documented only?
- **OPEN:** NATS KV vs JetStream stream for the emit channel — KV listing cost on large buckets.
