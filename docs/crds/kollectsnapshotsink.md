# KollectSnapshotSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `ksnap`

Cluster-scoped variant: **`KollectClusterSnapshotSink`** (`kcsnap`).

## What it is for

A `KollectSnapshotSink` configures **snapshot-store** export backends — Git, GitLab, S3, GCS, Azure
Blob, and HTTP/webhook ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Inventories reference
snapshot sinks via `KollectInventory.spec.snapshotSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `git`, `gitlab`, `s3`, `gcs`, `azureblob`, `http` |
| `spec.endpoint` | Repository URL, bucket URI, or webhook URL |
| `spec.git` / `spec.gitlab` / `spec.objectStore` / `spec.http` | Type-specific settings |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model.
