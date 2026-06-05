# Architecture Decision Records

Decisions for [kollect](https://github.com/konih/kollect) are recorded as numbered ADRs.
Each ADR follows: **Context → Decision → Consequences → Open questions** (where applicable).

| ADR | Title |
| --- | --- |
| [0001](0001-kubebuilder-v4.md) | Kubebuilder v4 + controller-runtime |
| [0003](0003-cel-jsonpath-extraction.md) | CEL and JSONPath attribute extraction |
| [0004](0004-crd-model.md) | CRD model (prefixed kinds, static vs reconciled) |
| [0006](0006-etcd-limit.md) | Data storage and etcd size limit |
| [0011](0011-doc-sync-templating.md) | Doc-sync / Confluence publication (**Rejected**) |
| [0012](0012-prometheus-metrics-stub.md) | Operator metrics (no Prometheus export sink) |
| [0013](0013-prior-art.md) | Prior art and OSS reference patterns |
| [0014](0014-event-driven-informers.md) | Event-driven dynamic informers |
| [0015](0015-static-vs-reconciled.md) | Static config vs reconciled CRDs |
| [0020](0020-error-taxonomy.md) | Error taxonomy and reconcile behavior |
| [0021](0021-mkdocs-github-pages.md) | MkDocs Material for documentation site |
| [0022](0022-multi-cluster-sync-rfc.md) | Multi-cluster sync topology (RFC, Proposed) |
| [0023](0023-lean-queue-transport.md) | Lean queue transport for hub fan-in (Accepted) |
| [0024](0024-inventory-api-auth.md) | Inventory HTTP API authentication (Accepted) |
| [0025](0025-sink-backends-database-kafka.md) | Postgres and Kafka export sinks (Accepted) |
| [0026](0026-performance-scalability.md) | Performance and scalability (Accepted) |
| [0027](0027-helm-release-inventory.md) | Helm release inventory sample (Accepted) |
| [0028](0028-hub-cluster-auth-istio-pattern.md) | Hub cluster authentication — Istio remote-secret pattern (Accepted) |

See also [ARCHITECTURE.md](../ARCHITECTURE.md), [REQUIREMENTS.md](../REQUIREMENTS.md), and
[PERFORMANCE.md](../PERFORMANCE.md).
