# Example: NATS event sink

!!! note "Apply separately"
    `config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml` is **not** in the default kustomization.
    Apply it explicitly after creating a NATS JetStream endpoint and credentials Secret.

`config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml` — not in default kustomization.

!!! tip "Event + state pairing"
    Event emitters complement relational sinks — pair NATS with Postgres via `databaseSinkRefs` and
    `eventSinkRefs` when portals need queryable state and downstream systems need change notifications.

## Sample manifest

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectEventSink
metadata:
  name: nats-inventory-demo
  namespace: default
spec:
  type: nats
  cluster: prod-west
  connectionTest: false
  secretRef:
    name: nats-credentials
  nats:
    url: nats://nats.kollect-system.svc:4222
    subject: inventory.events
    stream: kollect_events
```

| Field | Purpose |
| --- | --- |
| `spec.nats.url` | NATS server (`nats://host:4222`); falls back to `spec.endpoint` |
| `spec.nats.subject` | JetStream publish subject (required) |
| `spec.nats.stream` | Stream name (default `kollect_events`; dots sanitized to `_`) |
| `spec.secretRef` | Optional `token` or `username`/`password` keys |
| `spec.cluster` | Cluster label embedded in each event envelope |

Event emitter role ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Pair with Postgres via
`databaseSinkRefs` + `eventSinkRefs` on the same `KollectInventory`.

## Message contract

Each export publishes one JSON envelope to JetStream:

```json
{
  "schemaVersion": "kollect.dev/v1alpha1",
  "timestamp": "2026-01-15T12:00:00Z",
  "cluster": "prod-west",
  "namespace": "team-a",
  "payload": [/* canonical Item rows */]
}
```

Delivery is **at-least-once**. The backend sets `Nats-Msg-Id` to a content hash of
`{cluster}/{namespace}/{payload}` so JetStream deduplicates identical replays within the
stream's duplicate-detection window.

Golden fixture: `test/schema/golden/nats-event-envelope.json`.

## Consumer walkthrough

1. Ensure a JetStream-enabled NATS server is reachable from the operator.
2. Create a Secret with credentials when auth is required:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: nats-credentials
     namespace: default
   stringData:
     token: "<nats-token>"
   ```

3. Apply the `KollectEventSink` sample, then reference it from a `KollectInventory`:

   ```yaml
   spec:
     eventSinkRefs:
       - nats-inventory-demo
   ```

4. Consume with the NATS CLI (replace stream/subject names to match your spec):

   ```bash
   nats stream add kollect_events --subjects "inventory.events" --storage file --retention limits
   nats consumer add kollect_events inventory-reader --filter inventory.events --ack explicit
   nats consumer next kollect_events inventory-reader
   ```

   Or with `nats.go`:

   ```go
   js, _ := jetstream.New(nc)
   cons, _ := js.CreateOrUpdateConsumer(ctx, "kollect_events", jetstream.ConsumerConfig{
       FilterSubject: "inventory.events",
       AckPolicy:     jetstream.AckExplicitPolicy,
   })
   cons.Consume(func(msg jetstream.Msg) {
       // msg.Data() is the EventEnvelope JSON
       _ = msg.Ack()
   })
   ```

Kafka alternative: [Kafka event sink](kafka-event-sink.md) ·
`config/samples/kollect_v1alpha1_kollecteventsink_kafka.yaml`.
