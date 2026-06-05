# Architecture Decision Records

Decisions for [kollect](https://github.com/konih/kollect), kept as **living working documents**.
Numbers are grouped by **theme** (`0Txx`), so reading top-to-bottom is also a tour of the system's
aspects — not a strict chronology. Each ADR is **current-state-first**: the decision as it stands
today up top, with the exploration captured in an *Evolution* section.

**Status vocabulary:** `Current` (in effect) · `Exploring` (RFC / spike) · `Parked` (deferred) ·
`Dropped` (decided against).

New to the project? Read in theme order; for the *why*, start with [REQUIREMENTS.md](../REQUIREMENTS.md).

## 01 · Foundations — language, framework, storage limits

| ADR | Title | Status |
| --- | --- | --- |
| [0101](0101-kubebuilder-v4.md) | Kubebuilder v4 + controller-runtime | Current |
| [0102](0102-prior-art.md) | Prior art and OSS reference patterns | Current (living) |
| [0103](0103-etcd-limit.md) | Data storage and the etcd size limit | Current |
| [0104](0104-security-model.md) | Security model — secrets, TLS, RBAC, redaction | Current |

## 02 · API & tenancy — the CRD model and how teams are isolated

| ADR | Title | Status |
| --- | --- | --- |
| [0201](0201-crd-model.md) | CRD model — prefixed kinds, static vs reconciled | Current |
| [0202](0202-static-vs-reconciled.md) | Static config vs reconciled CRDs | Current |
| [0203](0203-namespaced-multi-tenancy.md) | Namespaced multi-tenancy and operator watch scope | Current |
| [0204](0204-namespaced-profiles.md) | Namespaced `KollectProfile` | Current |
| [0205](0205-watch-labels.md) | Watch opt-in / opt-out labels | Current |
| [0206](0206-api-versioning-conversion.md) | API versioning and conversion strategy | Exploring |

## 03 · Collection & extraction — turning live objects into rows

| ADR | Title | Status |
| --- | --- | --- |
| [0301](0301-event-driven-informers.md) | Event-driven dynamic informers (one per GVK) | Current |
| [0302](0302-cel-jsonpath-extraction.md) | CEL and JSONPath attribute extraction | Current |
| [0303](0303-helm-release-inventory.md) | Helm / Argo release inventory sample + redaction | Current |
| [0304](0304-custom-resource-aggregation-rfc.md) | Custom-resource metrics and richer aggregation | Exploring |
| [0305](0305-aggregation-dedupe.md) | Aggregation, row identity, and dedupe semantics | Current |

## 04 · Export & sinks — where inventory lands and how it's read

| ADR | Title | Status |
| --- | --- | --- |
| [0401](0401-sink-taxonomy-state-vs-stream.md) | Sink taxonomy — state stores vs event emitters | Current |
| [0402](0402-sink-backends-database-kafka.md) | Postgres and Kafka sink backends | Current |
| [0403](0403-connection-test.md) | Connection test — sink probes + `KollectConnectionTest` CR | Current |
| [0404](0404-inventory-api-auth.md) | Inventory HTTP API authentication | Current |
| [0405](0405-export-data-contract.md) | Export data contract and schema versioning | Current |
| [0406](0406-sink-registry.md) | Sink registry and the `Backend` interface | Current |
| [0407](0407-git-object-store-layout.md) | Git / object-store export layout and workflow | Current |
| [0408](0408-read-api-ui-architecture.md) | Read API and UI architecture (pluggable backing store) | Exploring |

## 05 · Multi-cluster — fan-in, transport, and the optional hub

| ADR | Title | Status |
| --- | --- | --- |
| [0501](0501-multi-cluster-sync-rfc.md) | Multi-cluster sync topology (shared-sink default; hub optional) | Current |
| [0502](0502-lean-queue-transport.md) | Event-emitter transport (NATS default, Kafka opt-in) | Current |
| [0503](0503-hub-cluster-auth-istio-pattern.md) | Hub cluster authentication — push-first | Current |

## 06 · Observability & ops — metrics, errors, performance

| ADR | Title | Status |
| --- | --- | --- |
| [0601](0601-prometheus-metrics-stub.md) | Operator metrics (no Prometheus export sink) | Current |
| [0602](0602-error-taxonomy.md) | Error taxonomy and reconcile behavior | Current |
| [0603](0603-performance-scalability.md) | Performance and scalability targets | Current |

## 07 · Project & meta — docs, scope guardrails, the big pivot

| ADR | Title | Status |
| --- | --- | --- |
| [0701](0701-mkdocs-github-pages.md) | MkDocs Material documentation site | Current |
| [0702](0702-doc-sync-templating.md) | Doc-sync / Confluence publication | Dropped |
| [0703](0703-platform-architecture-pivot.md) | Platform architecture pivot — decision log | Current (log) |
| [0704](0704-helm-chart-crd-lifecycle.md) | Helm chart and CRD lifecycle | Current |
| [0705](0705-release-supply-chain.md) | Release engineering and supply chain | Current |

---

> **Numbering note (2026-06-05):** ADRs were renumbered from a flat chronological sequence into these
> thematic ranges to make the corpus readable as a project overview. Pre-beta, with no external
> adopters, we treat ADRs as working docs and optimize for clarity over historical immutability.
> Old number → new number mapping lives in the git history of this change.

See also [ARCHITECTURE.md](../ARCHITECTURE.md), [REQUIREMENTS.md](../REQUIREMENTS.md),
[PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md), and [PERFORMANCE.md](../PERFORMANCE.md).
