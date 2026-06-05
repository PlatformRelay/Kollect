# Examples

!!! tip "Prerequisites"
    These walkthroughs assume a running Kollect operator and `kubectl` access. Start with
    [QUICKSTART.md](../QUICKSTART.md) or [Kind local lab](kind-local-lab.md) if you have not
    installed the controller yet.

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
| [Wide-scope kind demo](../../hack/demo/kind-wide-scope/README.md) | Sales-pitch showcase — Trivy, certs, Git export, UI reveal |
| [Wide-scope demo ROADMAP](../../hack/demo/kind-wide-scope/ROADMAP.md) | Personas, churn presets, early-adopter checklist |
| [UI local development](ui-local-development.md) | Mock vs live Read API for kollect-ui |

!!! note "Samples not in default kustomization"
    NATS (`kollectsink_nats.yaml`) and some cluster-scoped samples are documented but not included in
    `kubectl apply -k config/samples/`. Apply those files individually when following their guides.

!!! info "S3/GCS Parquet — not sampled yet"
    JSON object export is shipped for `type: s3` and `type: gcs`. DuckDB-queryable **Parquet** snapshot
    mode is planned per [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) — there is no
    kustomized sample under `config/samples/` yet. Use the JSON sinks in
    [postgres-state-store.md](postgres-state-store.md) or deployment-inventory for end-to-end export
    until a Parquet walkthrough lands.
