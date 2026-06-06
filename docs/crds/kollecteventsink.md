# KollectEventSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kevt`

Cluster-scoped variant: **`KollectClusterEventSink`** (`kcevt`).

## What it is for

A `KollectEventSink` configures **stream/event** export backends — Kafka and NATS JetStream
([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Inventories reference event sinks via
`KollectInventory.spec.eventSinkRefs`.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `kafka`, `nats` |
| `spec.kafka` / `spec.nats` | Broker and topic/subject settings |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model.
