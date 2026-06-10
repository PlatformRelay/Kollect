# KollectEventSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kevt`

Platform-shared backends: publish a `KollectEventSink` in `kollect-system` and reference it from a
`KollectClusterInventory` sink ref by `name` + `namespace` — there is no cluster-scoped sink kind
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

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

## Example

A Kafka event sink that emits one message per aggregated export
([`config/samples/kollect_v1alpha1_kollecteventsink_kafka.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollecteventsink_kafka.yaml)):

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

A NATS JetStream variant
([`config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml`](https://github.com/konih/kollect/blob/main/config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml);
full consumer walkthrough in the [NATS event sink example](../examples/nats-event-sink.md)):

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
  nats:
    url: nats://nats.kollect-system.svc:4222
    subject: inventory.events
    stream: kollect_events
```

NATS messages carry a versioned `EventEnvelope` (`schemaVersion`, `timestamp`, `cluster`,
`namespace`, `payload`) with JetStream `Nats-Msg-Id` dedupe — see
`test/schema/golden/nats-event-envelope.json`.

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to render a side-effect-free preview under
`status.preview` ([ADR-0416](../adr/0416-sink-config-layering.md) §8): for `kafka` it surfaces the
destination topic. Removing the annotation clears `status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model.
