# ADR-0402: Sink backends — Postgres and Kafka

> Postgres (queryable state of record) and Kafka (event stream) backends, and how to test them.

**Theme:** 04 · Export & sinks · **Status:** Current
[ADR-0401](0401-sink-taxonomy-state-vs-stream.md) — Postgres and Kafka are **not** co-equal primaries
(state store vs event emitter); Postgres needs delete reconciliation; NATS is the lean event default;
S3/GCS object-store sinks support Parquet serialization.

## Context

Kollect exports aggregated inventory to pluggable **family sink** backends
([ADR-0201](0201-crd-model.md), [ADR-0414](0414-sink-family-crds.md), sink registry). Git and object
storage are in flight; the user's platform also needs:

- **Durable queryable storage** (Postgres) for portals and SQL analytics
- **Event streaming** (Kafka) for downstream consumers (audit, fan-out, shared-sink fleet pipelines)

Doc-sync / Confluence is **out of scope** ([ADR-0702](0702-doc-sync-templating.md)). Database and
Kafka sinks are **first-class export targets** alongside Git, S3, and GCS — not deferred to a
separate "documentation" phase.

Operator **Prometheus metrics** remain on the controller `/metrics` endpoint ([ADR-0601](0601-prometheus-metrics-stub.md));
they are not a family sink export type.

## Decision

### Sink types

Extend the family-sink `spec.type` allow-lists (webhook + Go registry):

| Family CRD | `type` | Library (preferred) | Role |
| --- | --- | --- | --- |
| `KollectDatabaseSink` | `postgres` | `github.com/jackc/pgx/v5` (or `database/sql` + driver) | Upsert keyed rows and/or append JSON events |
| `KollectEventSink` | `kafka` | `github.com/IBM/sarama` or `github.com/twmb/franz-go` | Publish inventory change messages to a topic |

Existing snapshot types unchanged: `git`, `gitlab`, `s3`, `gcs` on `KollectSnapshotSink`.

### Data shape

**Default contract:** one JSON document per inventory export cycle (aggregated namespace snapshot),
stable key ordering ([engineering guidelines](https://github.com/platformrelay/kollect/blob/main/docs/development/guidelines.md)).

| Backend | Mode | Notes |
| --- | --- | --- |
| **Postgres** | **Upsert** keyed rows; **primary key** **`(inventory_namespace, inventory_name, target_name, source_uid)`** plus `cluster`, `gvk`, `generation`, `payload` jsonb, `exported_at` | Schema/table configurable on sink spec; add `cluster` column for multi-cluster shared-sink fan-in ([ADR-0501](0501-multi-cluster-fleet.md)) |
| **Postgres** | **Append-only events** table (optional alternate `spec.postgres.mode`) | Full snapshot JSON per row |
| **Kafka** | **Message per export** (aggregated) | Key = **`{inventory_ns}/{inventory_name}`**; with fleet shared sink: **`{cluster}:{inventory_ns}/{inventory_name}`** for partition locality |
| **Kafka** | **Optional:** finer-grained change events later | Phase 1 value = JSON row batch + metadata (`generation`, `checksum`); at-least-once |

### Family sink spec (sketch)

**Postgres (`KollectDatabaseSink`)**

- `secretRef` — connection string or `username`/`password`/`host` keys (never inline secrets)
- `database`, `schema`, `table` (required)
- `tls` — same CA patterns as Git ([ADR-0201](0201-crd-model.md))
- `mode`: `upsert` | `append` (default `upsert`)

**Kafka (`KollectEventSink`)**

- `brokers[]`, `topic` (required)
- `secretRef` — optional SASL/SCRAM or TLS client certs
- `headers` — optional static map; operator adds `kollect.dev/cluster`, `namespace`, `gvk` when known
- `compression`, `acks` — sensible defaults; advanced fields optional

### End-to-end testing (merge gate)

Before marking either backend **done** in [ROADMAP.md](../ROADMAP.md):

- **testcontainers-go** — Postgres official image; **Redpanda** or Kafka-compatible image for broker
- Integration tests: apply sample CRs → export → assert row count / consumed message headers+body
- CI job runs integration package (same bar as Git/S3 sinks)

### Architecture

```mermaid
flowchart LR
  Profile[KollectProfile]
  Target[KollectTarget]
  Inv[KollectInventory]
  Snap[KollectSnapshotSink]
  DB[KollectDatabaseSink]
  Evt[KollectEventSink]
  Profile --> Target
  Target --> Inv
  Inv --> Snap
  Inv --> DB
  Inv --> Evt
  Snap --> Git[Git / GitLab]
  Snap --> Obj[S3 / GCS]
  DB --> PG[(Postgres)]
  Evt --> KF[Kafka topic]
```

## Consequences

### Positive

- Fits team Kafka usage without inventing an in-operator fleet transport ([ADR-0501](0501-multi-cluster-fleet.md)).
- SQL backends enable portal queries without cloning Git repos.
- Clear separation from rejected doc-sync ([ADR-0702](0702-doc-sync-templating.md)).

### Negative

- Postgres schema migrations are operator-external unless we ship opinionated DDL in docs.
- Kafka ordering/idempotency is consumer's responsibility; document at-least-once export semantics.
- Two more backends to test and harden (connection test, circuit breaker per [ADR-0602](0602-error-taxonomy.md)).

### Export observability (Phase 1)

- **`kollect_sink_errors_total{reason}`** — separate from generic reconcile error counter
  ([ADR-0602](0602-error-taxonomy.md)).
- **`kollect_export_duration_seconds`** default histogram buckets (seconds):
  `.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10` — override via manager flag if load tests require.
- **Export debounce:** **`KollectInventory.spec.exportMinInterval`** — default **30s**; material-change
  bypass ([ADR-0201](0201-crd-model.md)).
- **Connection test GC:** **`KollectConnectionTest.spec.ttlSecondsAfterFinished`** — default **300**.

## Open questions

- **RESOLVED :** **Skip SQLite** — Postgres testcontainers sufficient for dev/CI.
- **RESOLVED :** Kafka key includes **`cluster`** prefix when multi-cluster shared-sink fan-in is active
  ([ADR-0501](0501-multi-cluster-fleet.md)).
- **RESOLVED :** Postgres upsert PK
  **`(inventory_namespace, inventory_name, target_name, source_uid)`**.

## See also

- [ADR-0201: CRD model](0201-crd-model.md)
- [ADR-0414: Sink family CRDs](0414-sink-family-crds.md)
- [ADR-0602: Error taxonomy](0602-error-taxonomy.md)
- [REQUIREMENTS.md](../REQUIREMENTS.md)
