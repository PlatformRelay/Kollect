# Example: NATS event sink

!!! note "Apply separately"
    `config/samples/kollect_v1alpha1_kollectsink_nats.yaml` is **not** in the default kustomization.
    Apply it explicitly after creating a NATS JetStream endpoint and credentials Secret.

`config/samples/kollect_v1alpha1_kollectsink_nats.yaml` — not in default kustomization.

!!! tip "Event + state pairing"
    Event emitters complement relational sinks — pair NATS with Postgres in `sinkRefs` when portals
    need queryable state and downstream systems need change notifications.

Event emitter role ([ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)). Pair with Postgres in `sinkRefs`.

Kafka alternative: `kollect_v1alpha1_kollectsink_kafka.yaml`.
