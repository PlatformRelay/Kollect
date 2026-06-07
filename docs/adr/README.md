# Architecture Decision Records

Architecture decisions for [Kollect](https://github.com/konih/kollect). Numbers are grouped by
**theme** (`0Txx`) so the index reads as a system overview — foundations → API → collection →
export → fleet → observability → project engineering.

**Status:** `Current` · `Exploring` (RFC / spike) · `Parked` · `Dropped`

Each ADR uses **Context / Decision / Consequences** — written for readers who need the design,
not the project timeline.

New readers: [REQUIREMENTS.md](../REQUIREMENTS.md) → theme **02** (CRD model) → **03** (collection) → **04** (sinks).

## 01 · Foundations

| ADR | Title | Status |
| --- | --- | --- |
| [0101](0101-kubebuilder-v4.md) | Kubebuilder v4 + controller-runtime | Current |
| [0102](0102-prior-art.md) | Prior art and OSS reference patterns | Current |
| [0103](0103-etcd-limit.md) | Data storage and the etcd size limit | Current |
| [0104](0104-security-model.md) | Security model — secrets, TLS, RBAC, redaction | Current |
| [0105](0105-webhook-serving-cert-management.md) | Webhook serving and certificate management | Current |

## 02 · API & tenancy

| ADR | Title | Status |
| --- | --- | --- |
| [0201](0201-crd-model.md) | CRD model — prefixed kinds, static vs reconciled | Current |
| [0202](0202-static-vs-reconciled.md) | Static config vs reconciled CRDs | Current |
| [0203](0203-namespaced-multi-tenancy.md) | Namespaced multi-tenancy and operator watch scope | Current |
| [0204](0204-namespaced-profiles.md) | Namespaced `KollectProfile` | Current |
| [0205](0205-watch-labels.md) | Watch opt-in / opt-out labels | Current |
| [0206](0206-api-versioning-conversion.md) | API versioning and conversion strategy | Exploring |
| [0207](0207-target-collection-filtering.md) | Target collection filtering | Current |

## 03 · Collection & extraction

| ADR | Title | Status |
| --- | --- | --- |
| [0301](0301-event-driven-informers.md) | Event-driven dynamic informers (one per GVK) | Current |
| [0302](0302-cel-jsonpath-extraction.md) | CEL and JSONPath attribute extraction | Current |
| [0303](0303-helm-release-inventory.md) | Helm / Argo release inventory sample + redaction | Current |
| [0304](0304-custom-resource-aggregation-rfc.md) | Custom-resource metrics and richer aggregation | Exploring |
| [0305](0305-aggregation-dedupe.md) | Aggregation, row identity, and dedupe semantics | Current |

## 04 · Export, sinks, read API & UI

| ADR | Title | Status |
| --- | --- | --- |
| [0401](0401-sink-taxonomy-state-vs-stream.md) | Sink taxonomy — state stores vs event emitters | Current |
| [0402](0402-sink-backends-database-kafka.md) | Postgres and Kafka sink backends | Current |
| [0403](0403-connection-test.md) | Connection test — sink probes + `KollectConnectionTest` CR | Current |
| [0404](0404-inventory-api-auth.md) | Inventory HTTP API authentication | Current |
| [0405](0405-export-data-contract.md) | Export data contract and schema versioning | Current |
| [0406](0406-sink-registry.md) | Sink registry and the `Backend` interface | Current |
| [0407](0407-git-object-store-layout.md) | Git / object-store export layout and workflow | Current |
| [0408](0408-read-api-ui-architecture.md) | Read API and UI architecture (pluggable backing store) | Current |
| [0409](0409-kollect-ui-deployment.md) | Kollect UI deployment — separate static SPA image | Current |
| [0410](0410-ui-engineering-and-quality-gates.md) | UI engineering and quality gates | Current |
| [0411](0411-read-api-extensions-for-ui.md) | Read API extensions for UI | Current |
| [0412](0412-mock-read-api-for-ui-development.md) | Mock Read API for UI development | Current |
| [0413](0413-export-interval-scheduling.md) | Per-sink export interval scheduling | Current |
| [0414](0414-sink-family-crds.md) | Sink family CRDs (`KollectSnapshotSink`, etc.) | Current |
| [0415](0415-git-sink-commit-ergonomics.md) | Git sink commit ergonomics | Current |
| [0416](0416-sink-config-layering.md) | Sink configuration layering | Current |
| [0417](0417-mongodb-database-sink.md) | MongoDB database sink | Current |
| [0418](0418-fleet-console-read-plane.md) | Fleet console read plane (event-fed, `cluster` dimension) | Exploring |

## 05 · Multi-cluster fleet

| ADR | Title | Status |
| --- | --- | --- |
| [0501](0501-multi-cluster-fleet.md) | Multi-cluster fleet — shared sink fan-in | Current |

## 06 · Observability & ops

| ADR | Title | Status |
| --- | --- | --- |
| [0601](0601-prometheus-metrics-stub.md) | Operator metrics (no Prometheus export sink) | Current |
| [0602](0602-error-taxonomy.md) | Error taxonomy and reconcile behavior | Current |
| [0603](0603-performance-scalability.md) | Performance and scalability targets | Current |
| [0604](0604-target-scoped-prometheus-metrics.md) | Target- and inventory-scoped Prometheus metrics | Exploring |
| [0605](0605-opentelemetry-tracing.md) | OpenTelemetry tracing | Exploring |

## 07 · Project engineering

| ADR | Title | Status |
| --- | --- | --- |
| [0701](0701-mkdocs-github-pages.md) | MkDocs Material documentation site | Current |
| [0702](0702-doc-sync-templating.md) | Doc-sync / Confluence publication | Dropped |
| [0704](0704-helm-chart-crd-lifecycle.md) | Helm chart and CRD lifecycle | Current |
| [0705](0705-release-supply-chain.md) | Release engineering and supply chain | Current |
| [0706](0706-testing-merge-gate-architecture.md) | Testing and merge-gate architecture | Current |

---

See [ARCHITECTURE.md](../ARCHITECTURE.md), [PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md),
[development/adr-rfc-process.md](../development/adr-rfc-process.md).
