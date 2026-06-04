# Architecture Decision Records

Decisions for [kollect](https://github.com/konih/kollect) are recorded as numbered ADRs.
Each ADR follows: **Context → Decision → Consequences → Open questions** (where applicable).

| ADR | Title |
| --- | --- |
| [0001](0001-kubebuilder-v4.md) | Kubebuilder v4 + controller-runtime |
| [0004](0004-crd-model.md) | CRD model (prefixed kinds, static vs reconciled) |
| [0006](0006-etcd-limit.md) | Data storage and etcd size limit |
| [0013](0013-prior-art.md) | Prior art and OSS reference patterns |
| [0014](0014-event-driven-informers.md) | Event-driven dynamic informers |
| [0015](0015-static-vs-reconciled.md) | Static config vs reconciled CRDs |
| [0020](0020-error-taxonomy.md) | Error taxonomy and reconcile behavior |

See also [ARCHITECTURE.md](../ARCHITECTURE.md) for the end-to-end system view.
