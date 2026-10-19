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
([`config/samples/advanced/kollect_v1alpha1_kollecteventsink_kafka.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollecteventsink_kafka.yaml)):

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
([`config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/kollect_v1alpha1_kollecteventsink_nats.yaml);
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

NATS credentials go in the Secret named by `spec.secretRef` (`token`, or `username` and
`password`). A `spec.nats.url` or `spec.endpoint` that embeds them (`nats://user:pass@host`,
`nats://token@host`) is rejected at config validation, so the credential can never be echoed into
a status condition or Event.

## Secret rotation

Each export reads the metadata of the Secrets the sink references (`spec.secretRef`,
`spec.kafka.secretRef` or `spec.nats.secretRef`, and `spec.tls.caSecretRef`) from the API server;
a `spec.secretRef` counts even when the type-specific reference overrides it.
When one of them changed since the backend was built, that export builds a new producer or
connection from the current values. No restart, spec change or annotation is needed. Exports
already running finish on the old backend, which is closed after the last of them ends.

- **When it takes effect.** A Secret change alone does not trigger an export. With an export
  interval above `0s`, the next export comes within the interval. With `exportMinInterval: 0s`
  ("export only on material change", the usual setting for event sinks), it comes with the next
  material change, spec generation change or `kollect.dev/requestedAt` change.
- **Deleted or emptied Secret.** A missing Secret, or one with no non-empty value in its data,
  fails the export with a retryable error that names the Secret and its namespace
  (`secret "<name>" not found in namespace "<namespace>"` or `… has no data`) and never a value.
  No message is sent and the backend is dropped. Once the Secret is restored, the next retry
  exports again with no change to the sink; an inventory with `exportMinInterval: 0s` retries
  every 30 s.
- **Terminal errors** do not requeue, and no controller watches Secrets. After fixing the Secret,
  change any annotation on the sink, for example
  `kubectl annotate kollecteventsink <name> secret-fixed-at="$(date +%s)" --overwrite`.
- **Inventory deletion** does not need the Secret: events cannot be retracted, so cleanup reports
  `CleanupRetained` without reading the Secret or contacting the broker.

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
