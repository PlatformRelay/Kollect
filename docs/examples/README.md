# Examples

Walkthroughs backed by `config/samples/`. Apply defaults:

```sh
kubectl apply -k config/samples/
```

| Example | Topic |
| --- | --- |
| [Deployment inventory](deployment-inventory.md) | Profile → Target → Inventory → Postgres (Git optional) |
| [Helm / Argo release inventory](helm-release-inventory.md) | Argo CD `Application` (Flux secondary) |
| [Spoke cluster inventory](spoke-cluster-inventory.md) | Per-team Postgres/Git export |
| [Postgres state store](postgres-state-store.md) | Relational SoR + delete reconciliation |
| [NATS event sink](nats-event-sink.md) | JetStream events |
| [Hub mode](hub-mode.md) | `mode: hub`, no `KollectHub` CRD |
| [Cluster-scoped rollup](cluster-rollup.md) | Cluster CRDs + dedupe |
| [Multi-tenant watch scope](multi-tenant-watch-namespaces.md) | Scope + watchNamespaces |
| [Connection test](connection-test.md) | `KollectConnectionTest` workflow |
| [Cert-manager webhooks](cert-manager-webhook.md) | Webhook TLS install |
| [Kind local lab](kind-local-lab.md) | kind quickstart |

S3/GCS Parquet: [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) — no kustomized sample yet.
