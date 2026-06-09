# Example: Kafka event sink

!!! note "Apply separately"
    `config/samples/kollect_v1alpha1_kollecteventsink_kafka.yaml` is **not** in the default kustomization.
    Apply it explicitly after a Kafka broker is reachable from the operator namespace.

`config/samples/kollect_v1alpha1_kollecteventsink_kafka.yaml` — not in default kustomization.

!!! tip "Event + state pairing"
    Pair Kafka with Postgres via `databaseSinkRefs` and `eventSinkRefs` when portals need queryable
    state and downstream systems need change notifications.

## Sample manifest

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectEventSink
metadata:
  name: kafka-inventory-demo
  namespace: default
spec:
  type: kafka
  connectionTest: false
  kafka:
    brokers:
      - kafka.kollect-system.svc:9092
    topic: inventory.changes
```

| Field | Purpose |
| --- | --- |
| `spec.kafka.brokers` | Broker list (`host:port`); falls back to `spec.endpoint` |
| `spec.kafka.topic` | Topic for aggregated export envelopes (required) |
| `spec.secretRef` | Optional SASL credentials (`username`/`password` keys) |
| `spec.cluster` | Cluster label embedded in each event envelope |

Event emitter role ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)).

## Message contract

Each export publishes one JSON envelope per inventory snapshot — same shape as NATS
([ADR-0405](../adr/0405-export-data-contract.md)):

```json
{
  "schemaVersion": "kollect.dev/v1alpha1",
  "timestamp": "2026-01-15T12:00:00Z",
  "cluster": "prod-west",
  "namespace": "team-a",
  "payload": [/* canonical Item rows */]
}
```

Delivery is **at-least-once**. Consumers should key deduplication on
`{cluster, namespace, timestamp, payload checksum}`.

## Walkthrough

1. Ensure a Kafka broker is reachable from the operator (in-cluster Service or external endpoint).
2. Create a Secret when SASL is required:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: kafka-credentials
     namespace: default
   stringData:
     username: "<kafka-user>"
     password: "<kafka-password>"
   ```

3. Apply the `KollectEventSink` sample, then reference it from a `KollectInventory`:

   ```yaml
   spec:
     eventSinkRefs:
       - kafka-inventory-demo
   ```

4. Consume with `kcat` (or your preferred client):

   ```bash
   kcat -b kafka.kollect-system.svc:9092 -t inventory.changes -C -o beginning -e
   ```

NATS alternative: [NATS event sink](nats-event-sink.md) ·
`config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml`.

## See also

- [KollectEventSink](../crds/kollecteventsink.md)
- [ADR-0402: Postgres and Kafka sink backends](../adr/0402-sink-backends-database-kafka.md)
- [Connection test](connection-test.md)
