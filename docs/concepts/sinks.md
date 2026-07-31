# Sink roles: state, snapshots, and streams

All sinks project the same canonical inventory; no backend is privileged.

| Family | Typical backends | Use it for |
| --- | --- | --- |
| `KollectSnapshotSink` | Git, GitLab, S3, GCS | durable snapshots, audit, and diffs |
| `KollectDatabaseSink` | Postgres, MongoDB, BigQuery | current-state queries and analytics |
| `KollectEventSink` | Kafka, NATS | downstream reactions and change streams |

Choose Git first when evaluating Kollect: the result is inspectable without another data service.
Add database or event sinks when a consumer needs their query or delivery semantics. Credentials
belong in referenced Secrets, and each backend reports connectivity and export status separately.

See [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) and the
[sink CRD references](../crds/index.md).
