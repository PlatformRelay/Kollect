# Planned features

Forward-looking capabilities that complement [ROADMAP.md](../ROADMAP.md): deferred work (🔮),
phased ⬜ backlog, and items that need a design pass before implementation. For **build-order status**
(Phase 0–4, shipped vs in progress), see the phase tables in the roadmap. For locked decisions, see
[ADR index](../adr/README.md).

!!! info "Not a commitment"
    Items here are **intent and backlog**, not release promises. Status may change as ADRs land or
    scope is rejected. Pre-beta APIs may shift until the first release candidate.

**Last updated:** 2026-06-07

## Status legend

| Status | Meaning |
| --- | --- |
| **Planned** | Accepted direction; implementation not started (⬜ on roadmap where phased) |
| **Spec needed** | Problem space agreed; ADR or RFC required before code |
| **Exploring** | Active design spike or RFC in review |
| **Deferred** | Accepted but explicitly not on the near-term path (🔮 on roadmap) |
| **Sample / docs** | Reference implementation or walkthrough, not operator code |

## Read API & UI

### Read API contract freeze (v0.5.x)

| | |
| --- | --- |
| **Status** | Planned |
| **Roadmap** | [Read API + UI console](../ROADMAP.md#read-api-ui-console-planned-adr-0408) · Phase 1 HTTP 🚧 |
| **Summary** | Harden and **freeze the Read API** as the UI contract: list/filter/search, envelope `schemaVersion`, OpenAPI (`openapi/v1alpha1/inventory.yaml`), and auth parity with [ADR-0404](../adr/0404-inventory-api-auth.md). Planned for **v0.5** band (not v0.1/v0.2). |
| **Related ADRs** | [ADR-0408](../adr/0408-read-api-ui-architecture.md) · [ADR-0411](../adr/0411-read-api-extensions-for-ui.md) · [ADR-0405](../adr/0405-export-data-contract.md) · [ADR-0103](../adr/0103-etcd-limit.md) |

---

### Inventory UI — memory adapter (v0.7.x)

| | |
| --- | --- |
| **Status** | In progress (early adopter preview on `main`) |
| **Roadmap** | [Read API + UI console](../ROADMAP.md#read-api-ui-console-planned-adr-0408) |
| **Summary** | Read-only **SPA** (`ui/`, separate `kollect-ui` image) on the **memory `InventoryReader`** adapter: searchable catalog, export/freshness health, SSE watch — zero extra infra, feature-gated on the operator. **Backend for v0.3.x** shipped sink families ([ADR-0414](../adr/0414-sink-family-crds.md)); UI docs and mock MVP are available for early adopters ahead of the **v0.7** milestone. |
| **Related ADRs** | [ADR-0408](../adr/0408-read-api-ui-architecture.md) · [ADR-0409](../adr/0409-kollect-ui-deployment.md) · [ADR-0410](../adr/0410-ui-engineering-and-quality-gates.md) · [ADR-0412](../adr/0412-mock-read-api-for-ui-development.md) |

---

### Portal UI — Postgres/Parquet adapter + drift (v0.8.x – v0.9.x)

| | |
| --- | --- |
| **Status** | Planned |
| **Roadmap** | [Read API + UI console](../ROADMAP.md#read-api-ui-console-planned-adr-0408) |
| **Summary** | **Portal mode** on Postgres/Parquet backing stores: multi-cluster rollup, **attribute drift over time**, poll-based freshness; optional **`kollect-server`** split so the portal does not couple to the controller process. |
| **Related ADRs** | [ADR-0408](../adr/0408-read-api-ui-architecture.md) · [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) · [ADR-0411](../adr/0411-read-api-extensions-for-ui.md) |

---

### oauth2-proxy OIDC browser auth (optional Helm sidecar)

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | [Deferred](../ROADMAP.md#deferred) |
| **Summary** | Optional **oauth2-proxy** sidecar (`oauth2Proxy.enabled: false`) for browser OIDC at ingress. Primary auth remains Kubernetes **TokenReview + SAR** on the Read/inventory HTTP API ([ADR-0404](../adr/0404-inventory-api-auth.md)). |
| **Related ADRs** | [ADR-0404](../adr/0404-inventory-api-auth.md) · [ADR-0409](../adr/0409-kollect-ui-deployment.md) |

---

## Sinks & export

### Sink family CRDs (ADR-0414)

| | |
| --- | --- |
| **Status** | Shipped (`v0.2.0-rc.1`) |
| **Roadmap** | Phase 1 ✅ |
| **Summary** | **`KollectSnapshotSink`**, **`KollectEventSink`**, **`KollectDatabaseSink`** replace monolithic **`KollectSink`**; family reconcilers, validating webhooks, and e2e bootstrap. |
| **Related ADRs** | [ADR-0414](../adr/0414-sink-family-crds.md) |

---

### S3/GCS Parquet snapshot sink

| | |
| --- | --- |
| **Status** | Planned |
| **Roadmap** | Phase 1 ⬜ |
| **Summary** | **Parquet snapshot layout** on S3 and GCS (JSON snapshot export is shipped today) for DuckDB/Athena-style queries without running Postgres — whole-file snapshots per export generation, partition paths, and documented "latest generation" views. |
| **Related ADRs** | [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) § Parquet snapshot · [ADR-0407](../adr/0407-git-object-store-layout.md) |

---

### BigQuery sink

| | |
| --- | --- |
| **Status** | Spec needed |
| **Summary** | **`KollectSink.type: bigquery`** (or equivalent) as a **relational / analytics** projection of the canonical snapshot — batch load or streaming insert of inventory rows for SQL dashboards and fleet analytics. |
| **Open design** | Role vs Postgres ([ADR-0402](../adr/0402-sink-backends-database-kafka.md)), delete reconciliation strategy, partition/clustering keys, and credential model (Workload Identity vs service account key in `secretRef`). Fits theme **04** numbering when accepted. |
| **Related ADRs** | [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) · [ADR-0402](../adr/0402-sink-backends-database-kafka.md) · [ADR-0406](../adr/0406-sink-registry.md) |

---

### Azure Blob Storage sink

| | |
| --- | --- |
| **Status** | Spec needed |
| **Summary** | **`KollectSink.type: azureblob`** (name TBD) as a **snapshot store** peer to S3/GCS — same canonical JSON (and future Parquet) contract, Azure-specific auth (`secretRef`, managed identity patterns). |
| **Open design** | Shared object-store backend abstraction vs separate package; connection test probe shape ([ADR-0403](../adr/0403-connection-test.md)); path template parity with [ADR-0407](../adr/0407-git-object-store-layout.md). |
| **Related ADRs** | [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) · [ADR-0406](../adr/0406-sink-registry.md) · [ADR-0403](../adr/0403-connection-test.md) |

---

### GCS and NATS connection test probes

| | |
| --- | --- |
| **Status** | Shipped (Phase 1) |
| **Roadmap** | Phase 1 ✅ |
| **Summary** | **`KollectConnectionTest`** and family sink annotation probes for **`gcs`** (`KollectSnapshotSink`) and **`nats`** (`KollectEventSink`) — alongside Git, Postgres, Kafka, S3, and GitLab probes. |
| **Related ADRs** | [ADR-0403](../adr/0403-connection-test.md) · [ADR-0414](../adr/0414-sink-family-crds.md) · [KollectSnapshotSink](../crds/kollectsnapshotsink.md) · [KollectEventSink](../crds/kollecteventsink.md) |

---

### Export data contract — versioned envelope

| | |
| --- | --- |
| **Status** | Exploring |
| **Summary** | Ship a **versioned envelope** on sink JSON and Read API responses (`schemaVersion` in body, stable ordering per [ADR-0405](../adr/0405-export-data-contract.md)) so portals and golden tests detect breaking contract changes independently of CRD `apiVersion`. |
| **Related ADRs** | [ADR-0405](../adr/0405-export-data-contract.md) · [ADR-0206](../adr/0206-api-versioning-conversion.md) · [ADR-0408](../adr/0408-read-api-ui-architecture.md) |

---

### KollectClusterSink — cluster-scoped sink API

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | Phase 3 🔮 · [Deferred](../ROADMAP.md#deferred) |
| **Summary** | **`KollectClusterSink`** for platform-shared backends referenced from `KollectClusterInventory` and future `KollectClusterScope.sinkRefs`. Namespaced `KollectSink` is the team default today ([ADR-0201](../adr/0201-crd-model.md)). |
| **Related ADRs** | [ADR-0204](../adr/0204-namespaced-profiles.md) · [ADR-0201](../adr/0201-crd-model.md) |

---

## Collection & samples

### Helm values profile + operator export scrub

| | |
| --- | --- |
| **Status** | Shipped (Phase 1) |
| **Roadmap** | Phase 1 ✅ |
| **Summary** | **`helm-release-values-redacted`** sample profile and operator **`scrubKeys[]`** extraction-time redaction so full Helm values inventory is safe without leaking secrets ([ADR-0303](../adr/0303-helm-release-inventory.md)). |
| **Related ADRs** | [ADR-0303](../adr/0303-helm-release-inventory.md) · [ADR-0104](../adr/0104-security-model.md) |

---

### `helm:` decode for `helm.sh/v1` Secret releases

| | |
| --- | --- |
| **Status** | Shipped (`v0.1.0-rc.3`) |
| **Roadmap** | Phase 1 ✅ |
| **Summary** | Gated **`helm:`** decode path for plain **`helm.sh/v1`** release Secrets (Flux `HelmRelease` secondary sample exists; Argo `Application` is primary). |
| **Related ADRs** | [ADR-0303](../adr/0303-helm-release-inventory.md) |

---

### Target collection filtering — `resourceRules` and CEL `matchPolicy`

| | |
| --- | --- |
| **Status** | Planned |
| **Summary** | Phase 2 **`resourceRules[]`** (OR-union GVK/label rules on Target) and Phase 3 per-rule **CEL `matchPolicy`** evaluated before store insert — platform deny via Scope, team intent via Target ([ADR-0207](../adr/0207-target-collection-filtering.md)). |
| **Related ADRs** | [ADR-0207](../adr/0207-target-collection-filtering.md) · [ADR-0205](../adr/0205-watch-labels.md) |

---

### Sample project — Git sink → Confluence (external CI)

| | |
| --- | --- |
| **Status** | Sample / docs |
| **Summary** | End-to-end **reference project** showing Kollect exporting inventory to a Git snapshot sink, then an **external** pipeline (CI job or small tool) rendering Markdown/HTML and publishing to Confluence or another wiki. |
| **Why external** | In-operator doc-sync and `KollectPublication` are **out of scope** — Kollect collects and exports; templating and CMS credentials stay outside the cluster ([ADR-0702](../adr/0702-doc-sync-templating.md)). |
| **Deliverable** | Standalone sample repo (manifests + CI template + optional render script), linked from [Examples](../examples/README.md). |
| **Related ADRs** | [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) · [ADR-0407](../adr/0407-git-object-store-layout.md) · [ADR-0702](../adr/0702-doc-sync-templating.md) |

---

## API & tenancy

### Finalizers on reconciled kinds

| | |
| --- | --- |
| **Status** | Shipped (`v0.1.0-rc.3`) |
| **Roadmap** | Phase 1 ✅ |
| **Summary** | **Finalizers** on `KollectTarget`, `KollectInventory`, and cluster rollup kinds — deletion waits for store teardown, in-flight exports, and hub report cleanup. |
| **Related ADRs** | [ADR-0201](../adr/0201-crd-model.md) · [ADR-0202](../adr/0202-static-vs-reconciled.md) |

---

### API v1beta1 + conversion webhook

| | |
| --- | --- |
| **Status** | Exploring |
| **Summary** | Introduce **`v1beta1`** as storage version with a **conversion webhook** (`v1alpha1 ↔ v1beta1`) at the **v0.10 presentation gate** (or post) — until then `v1alpha1` breaks freely. |
| **Related ADRs** | [ADR-0206](../adr/0206-api-versioning-conversion.md) · [ADR-0201](../adr/0201-crd-model.md) |

---

### KollectClusterScope — platform team scope CRD

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | Phase 3 🔮 |
| **Summary** | Cluster-scoped **`KollectClusterScope`** for platform teams to cap GVKs, namespaces, and (future) sink allowlists across tenant namespaces — complement to namespaced `KollectScope`. |
| **Related ADRs** | [ADR-0203](../adr/0203-namespaced-multi-tenancy.md) · [ADR-0207](../adr/0207-target-collection-filtering.md) |

---

### KollectReceiver and KollectTargetSet

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | Phase 3 🔮 · [Deferred](../ROADMAP.md#deferred) |
| **Summary** | Reserved CRDs: **`KollectReceiver`** (webhook-triggered collection) and **`KollectTargetSet`** (generator-style target grouping). Design-only placeholders; no controller until a concrete use case lands. |
| **Related ADRs** | [ADR-0201](../adr/0201-crd-model.md) · [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) |

---

### KollectCollectionRule CRD

| | |
| --- | --- |
| **Status** | Deferred |
| **Summary** | Standalone **`KollectCollectionRule`** CRD deferred until inline **`resourceRules[]`** on Target ([ADR-0207](../adr/0207-target-collection-filtering.md)) proves insufficient for reuse across many targets. |
| **Related ADRs** | [ADR-0207](../adr/0207-target-collection-filtering.md) |

---

## Multi-cluster & transport

### Hub federated mTLS

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | [Deferred](../ROADMAP.md#deferred) |
| **Summary** | **Cancelled** — hub/spoke tier removed; multi-cluster uses shared-sink fan-in ([ADR-0501](../adr/0501-multi-cluster-fleet.md)). |
| **Related ADRs** | [ADR-0501](../adr/0501-multi-cluster-fleet.md) |

---

### Queue transport TLS/ACL production hardening

| | |
| --- | --- |
| **Status** | Deferred |
| **Roadmap** | Phase 2 🚧 (TLS shipped; ACL allowlist stub) · [Deferred](../ROADMAP.md#deferred) |
| **Summary** | Production-grade **TLS/ACL hardening** for Redis/NATS/Kafka hub transport backends — beyond `cluster_id` wire metadata and dev/test defaults (ADR-0502). |
| **Related ADRs** | ADR-0502 · [ADR-0501](../adr/0501-multi-cluster-fleet.md) |

---

## Observability & performance

### Prometheus metrics scoped to targets / inventory rows

| | |
| --- | --- |
| **Status** | Exploring |
| **Summary** | Richer **domain metrics** derived from collected resources and target/inventory boundaries — beyond operator health counters on `/metrics`. Target-scoped collection gauges, inventory rollup gauges, optional per-target domain series (`metricsScope`), and **spoke-local Prometheus scrape** in multi-cluster installs. |
| **Spec** | [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) — three metric tiers (operator / collection boundary / KSM-style domain); affirms no `KollectSink.type: prometheus` ([ADR-0601](../adr/0601-prometheus-metrics-stub.md)); ships `KollectProfile.spec.metricsScope` (`profile` \| `target`); scrape `/metrics` on **each spoke** (not hub-only federation). Verification matrix: unit, envtest, integration, e2e (kind/nightly). |
| **Related ADRs** | [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) · [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) · [ADR-0601](../adr/0601-prometheus-metrics-stub.md) · [ADR-0602](../adr/0602-error-taxonomy.md) · [ADR-0603](../adr/0603-performance-scalability.md) · [ADR-0706](../adr/0706-testing-merge-gate-architecture.md) |

---

### Prometheus metrics from collected attribute values

| | |
| --- | --- |
| **Status** | Proposed (Exploring) — spec only, no implementation yet |
| **Summary** | Export **scalar numeric values** from CEL/JSONPath attributes as Prometheus gauges/counters on `/metrics` — complementing ADR-0304 **sum** series. Label vs value rules and cardinality guardrails. |
| **Spec** | [RFC: Prometheus attribute metrics](../rfc/prometheus-attribute-metrics.md) — revisit after ADR-0604 Tier B/C lands. |
| **Related** | [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) · [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) · [ADR-0302](../adr/0302-cel-jsonpath-extraction.md) |

---

### OpenTelemetry tracing

| | |
| --- | --- |
| **Status** | Exploring |
| **Summary** | Distributed tracing for reconcile loops, collection refresh batches, sink export attempts, and hub ingest (OTel SDK, OTLP export). Complements structured logs and Prometheus metrics. |
| **Spec** | [ADR-0605](../adr/0605-opentelemetry-tracing.md) — span map (`kollect.reconcile`, `kollect.collect.refresh`, `kollect.export`, `kollect.hub.ingest`); operator flags/env; default off in Helm. |
| **Validation home** | Exercised in **kollect-lab** (local companion environment for integration demos) alongside [kind local lab](../examples/kind-local-lab.md) — not required for default Helm install. |
| **Related ADRs** | [ADR-0605](../adr/0605-opentelemetry-tracing.md) · [ADR-0602](../adr/0602-error-taxonomy.md) · [ADR-0603](../adr/0603-performance-scalability.md) · ADR-0504 |

---

### `--informer-resync-period` operator flag

| | |
| --- | --- |
| **Status** | Planned |
| **Roadmap** | [Performance and scalability](../ROADMAP.md#operator-tuning-and-tests) ⬜ |
| **Summary** | Configurable **informer resync period** flag (and Helm value) for operators who need periodic full relist beyond event-driven reconcile — default remains conservative for large fleets ([ADR-0301](../adr/0301-event-driven-informers.md)). |
| **Related ADRs** | [ADR-0301](../adr/0301-event-driven-informers.md) · [ADR-0603](../adr/0603-performance-scalability.md) |

---

## Tooling & release

### Helm chart docs generation (`helm-docs`)

| | |
| --- | --- |
| **Status** | Done |
| **Roadmap** | Phase 0 ✅ |
| **Summary** | Automate **`helm-docs`** generation for `charts/kollect/README.md` from `values.yaml` comments — keep chart reference in sync with values schema ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md)). |
| **Related ADRs** | [ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md) |

---

### Release supply chain attestations (post-rc)

| | |
| --- | --- |
| **Status** | Planned |
| **Summary** | Post-release-candidate hardening: **cosign attestations**, Helm chart signing, OpenSSF scorecard — documented in [ADR-0705](../adr/0705-release-supply-chain.md), deferred until after first rc to avoid maintainer self-approval friction. Local `task release-dry-run` passes today; GH Actions `workflow_dispatch` rc remains 🚧 on roadmap. |
| **Related ADRs** | [ADR-0705](../adr/0705-release-supply-chain.md) · [RELEASE.md](../RELEASE.md) |

---

## How items graduate

1. **Spec needed** → draft an [RFC](../rfc/README.md) or ADR in **Exploring** status ([ADR/RFC process](../development/adr-rfc-process.md)).
2. **Accepted ADR** → track implementation on [ROADMAP.md](../ROADMAP.md) with phase and status marks.
3. **Rejected** → move to ROADMAP *Rejected* or ADR **Dropped** with rationale (see [ADR-0702](../adr/0702-doc-sync-templating.md)).

## See also

- [Roadmap (phased delivery)](../ROADMAP.md)
- [Platform decisions](../PLATFORM-DECISIONS.md)
- [Sink taxonomy](../adr/0401-sink-taxonomy-state-vs-stream.md)
- [ADR and RFC process](../development/adr-rfc-process.md)
