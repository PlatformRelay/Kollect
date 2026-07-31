# Examples

Each example starts from a running operator, gives you manifests to apply, and ends with an
observable result. Begin with the Git path; add other sinks only when their delivery model is what
your consumer needs.

## Collect

1. [Your first inventory](../getting-started/first-inventory.md) — Deployment images to a Git diff.
2. [Helm and Argo release inventory](helm-release-inventory.md) — extract release metadata.
3. [Cluster-scoped rollup](cluster-rollup.md) — aggregate across selected namespaces.
4. [Multi-tenant watch scope](multi-tenant-watch-namespaces.md) — opt teams and namespaces in.
5. [Team-owned operator](team-operator.md) — install with minimal, namespaced RBAC.

## Export

6. [Git snapshot sink](git-snapshot-sink.md) — export a diffable snapshot.
7. [S3 and GCS object storage](object-store-sink.md) — write durable bucket objects.
8. [Postgres state store](postgres-state-store.md) — query current inventory relationally.
9. [MongoDB sink](mongodb-sink.md) — maintain document-oriented current state.
10. [BigQuery sink](bigquery-sink.md) — export analytics-ready inventory.
11. [Kafka event sink](kafka-event-sink.md) — publish changes for stream consumers.
12. [NATS event sink](nats-event-sink.md) — emit changes through JetStream.
13. [Multi-sink fan-out](multi-sink-fanout.md) — drive independent destinations from one inventory.
14. [Connection tests](connection-test.md) — probe a sink without exporting inventory.

## Fleet and automation

15. [Multi-cluster fleet](multi-cluster-fleet.md) — partition several cluster writers in a shared sink.
16. [Pipeline CLI](../guides/pipeline-cli.md) — collect from CI/CD without running the operator.

The [custom-resource reference](../crds/index.md) documents every supported field. For a failed
example, use the page's troubleshooting section and the canonical
[troubleshooting guide](../operator-manual/troubleshooting.md).
