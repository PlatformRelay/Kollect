# Example: Postgres state store

`config/samples/kollect_v1alpha1_kollectsink_postgres.yaml` — namespaced sink, DSN via `postgres.databaseRef`.

Delete reconciliation required ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)).

Hub: sinks in `hub.exportNamespace` per [Hub mode](hub-mode.md).
