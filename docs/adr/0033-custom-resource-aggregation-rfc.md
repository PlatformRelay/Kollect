# ADR-0033: Custom-resource metrics and aggregation (RFC stub)

## Status

Accepted (spike landed — Phase 4, 2026-06-05)

## Context

Phase 1–3 shipped **operator** Prometheus metrics on `/metrics` ([ADR-0020](0020-error-taxonomy.md),
[ADR-0012](0012-prometheus-metrics-stub.md)) and **inventory aggregation** via `KollectInventory` /
`KollectClusterInventory` with hub merge ([ADR-0022](0022-multi-cluster-sync-rfc.md)). Stakeholder
export uses Git, Postgres, Kafka, and object-store sinks — not a Prometheus export sink.

[kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) (KSM) exposes
**`CustomResourceStateMetrics`**: config-driven GVK → Prometheus series from informer cache paths.
That pattern complements kollect's existing operator metrics and is the primary Phase 4 deliverable
per [prior art](0013-prior-art.md) and [ROADMAP](../ROADMAP.md).

Phase 4 must also define **richer cross-target / cross-cluster aggregation** without duplicating
full inventory payloads in etcd ([ADR-0006](0006-etcd-limit.md)) or exploding label cardinality.

## Decision

### 1. KSM-style custom-resource metrics

- **Emit from the existing collection engine** (shared dynamic informers per GVK —
  [ADR-0014](0014-event-driven-informers.md)), not a second watch loop.
- **Config surface:** `KollectProfile.spec.metrics` (and `KollectClusterProfile.spec.metrics`) —
  companion `KollectMetricsProfile` CR **deferred** until cross-profile reuse is required.
- **Spike shape (2026-06-05):** `MetricSpec { name, path, labels? }` where `path` references an
  attribute name from `spec.attributes`; admission validates bounded label keys (max 5). Engine emits
  `kollect_custom_resource_series{profile,gvk,series}` and, when labels are configured,
  `kollect_custom_resource_labeled_series{profile,gvk,series,<attribute labels>}`; auto-sum of all
  numeric attributes remains the fallback when `spec.metrics` is empty.
- **Cardinality rules:** bounded label sets; no unbounded `name`/`namespace` labels unless explicitly
  opted in per metric; document max series per profile in [PERFORMANCE.md](../PERFORMANCE.md).
- **Serve on operator `/metrics`** alongside existing `kollect_*` counters — no `KollectSink.type:
  prometheus` ([ADR-0012](0012-prometheus-metrics-stub.md)).

### 2. Richer aggregation

- **Spoke:** `KollectInventory` remains the per-namespace rollup contract (debounced export —
  `spec.exportMinInterval`).
- **Hub:** `KollectClusterInventory` merges spoke summaries; aggregation rules stay **O(total rows)**.
- **Cross-target rollups (TBD):** optional `KollectTargetSet`-style grouping deferred; Phase 4 spike
  documents dedupe/checksum strategies for multi-target inventories sharing one sink.

### 3. Relationship to existing metrics

| Layer | Examples | Phase |
| --- | --- | --- |
| Operator health | `kollect_reconcile_*`, `kollect_workqueue_depth`, `kollect_sink_errors_total` | 0–1 ✅ |
| Collection / export | `kollect_collected_objects`, `kollect_export_duration_seconds` | 1 ✅ |
| Domain series from CR fields | KSM-style gauges per `spec.metrics` path | 4 🚧 (config + engine wire) |

## Consequences

### Positive

- Platform teams can alert on **domain** signals (e.g. cert expiry, Argo sync status) without
  scraping inventory export sinks.
- Reuses proven KSM config patterns; testable with table-driven metric assertions like Phase 1.

### Negative

- CRD/schema design for metrics config adds API surface and webhook validation work.
- Misconfigured high-cardinality paths can overwhelm Prometheus — needs guardrails in admission.

## Open questions

- **Companion CR:** revisit `KollectMetricsProfile` when platform teams need one metrics schema across many profiles.
- **Per-metric labels:** ✅ `kollect_custom_resource_labeled_series` emits attribute label values from `spec.metrics[].labels`.
- **Hub domain series:** `kollect_hub_merged_items_total` wired; federated spoke scrapes vs hub-only domain gauges TBD.
- **Dedupe:** content-hash skip vs generation-based skip for cross-target aggregation ([ROADMAP](../ROADMAP.md)).

## See also

- [ADR-0012: Operator metrics stub](0012-prometheus-metrics-stub.md)
- [ADR-0013: Prior art — kube-state-metrics](0013-prior-art.md)
- [ADR-0022: Multi-cluster sync](0022-multi-cluster-sync-rfc.md)
- [PERFORMANCE.md](../PERFORMANCE.md) — operator metrics catalog
