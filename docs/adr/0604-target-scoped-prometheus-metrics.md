# ADR-0604: Target- and inventory-scoped Prometheus metrics

> Domain and collection metrics keyed by `KollectTarget` / `KollectInventory` boundaries on the operator
> `/metrics` scrape path — without a `KollectSink.type: prometheus` export sink.

**Theme:** 06 · Observability & ops · **Status:** Parked

!!! note "Implementation deferred"
    Tier A operator metrics ship today ([ADR-0601](0601-prometheus-metrics-stub.md)). Tier B/C
    target/inventory labels and `metricsScope` **are not implemented** — no CRD field, profile-only
    labels on `kollect_collected_objects`. Revisit when per-target alerting is a proven user need.
    Fleet installs scrape **each cluster operator** ([ADR-0501](0501-multi-cluster-fleet.md)) — no
    hub federation tier.

## Context

Phase 1 ships **operator health** metrics on `/metrics` ([ADR-0601](0601-prometheus-metrics-stub.md),
[ADR-0602](0602-error-taxonomy.md)). Phase 4 adds **KSM-style domain series** from
`KollectProfile.spec.metrics` on the same endpoint ([ADR-0304](0304-custom-resource-aggregation-rfc.md)).
Inventory payloads still export only to Git, object storage, Postgres, and Kafka sinks
([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).

[Planned features](../roadmap/planned-features.md) calls for richer metrics at **target** and
**inventory** boundaries. Today the collection engine updates gauges with **profile** labels only:

| Metric | Current labels | Gap |
| --- | --- | --- |
| `kollect_collected_objects` | `profile`, `gvk` | Multiple `KollectTarget` objects sharing a profile **overwrite** the same series (last writer wins). |
| `kollect_custom_resource_series` | `profile`, `gvk`, `series` | Domain sums are emitted per target refresh but stored under profile keys — target attribution is lost. |
| `kollect_inventory_items_total` | _(none)_ | Process-global gauge; cannot distinguish inventories in multi-tenant installs. |
| `kollect_collect_items_total` | _(none)_ | Store-wide size only — correct for capacity, not for per-inventory alerting. |

ADR-0601 **rejects** `KollectSink.type: prometheus` so inventory is never pushed to a remote
Prometheus as an export artifact. Platform teams still need **scrape-native** series aligned with
`KollectTarget` / `KollectInventory` CR boundaries for alerting (e.g. “target X has zero collected
objects”, “inventory Y export stale”). This ADR reconciles that need with the single-endpoint model
and cardinality guardrails in [ADR-0603](0603-performance-scalability.md) and
[PERFORMANCE.md](../PERFORMANCE.md).

Future **scalar attribute export** (gauge/counter from individual numeric fields) is spec-only in
[RFC: Prometheus metrics from collected attribute values](../rfc/prometheus-attribute-metrics.md) —
out of scope for the first ADR-0604 implementation pass but informs Tier C label and cardinality
rules below.

## Decision

### 1. One scrape endpoint — no Prometheus export sink

- **Affirm [ADR-0601](0601-prometheus-metrics-stub.md):** all Prometheus series are served from the
  controller **`/metrics`** endpoint (TLS + TokenReview/SAR). No `prometheus` sink type, no
  Pushgateway sidecar, no remote-write from export reconcilers.
- **Affirm [ADR-0304](0304-custom-resource-aggregation-rfc.md) engine path:** KSM-style series are
  computed in the collection engine (`internal/collect/metrics_snapshot.go`), not a second informer
  loop.

### 2. Three metric tiers (relationship to Phase 1 / Phase 4)

| Tier | Purpose | Label keys | Examples | Cardinality driver |
| --- | --- | --- | --- | --- |
| **A — Operator internals** | Health, queues, sink failures | `controller`, `sink_type`, `error_class`, … | `kollect_reconcile_total`, `kollect_export_duration_seconds` | Fixed small enums — **unchanged** from Phase 1 |
| **B — Collection boundary** | Per-target / per-inventory collection state | `target_namespace`, `target_name`, `profile`, `gvk`; `inventory_namespace`, `inventory_name` | `kollect_collected_objects`, `kollect_inventory_snapshot_items` | O(targets × gvks) + O(inventories) |
| **C — Domain (KSM-style)** | Aggregated numeric attributes from CR fields | Tier B keys + `series` + optional bounded attribute labels | `kollect_custom_resource_series`, `kollect_custom_resource_labeled_series` | O(targets × series × label tuples) — **admission-bounded** |

Tier A metrics **must not** gain `target_*` or `inventory_*` labels. Export latency and sink error
counters stay keyed by `sink_type` only ([ADR-0602](0602-error-taxonomy.md)) — use inventory
**conditions** and `status.sinkExports[]` for per-inventory export drill-down, not high-cardinality
histogram labels.

**Tier C′ (future):** per-field scalar gauges/counters from extracted attributes — see
[RFC: Prometheus metrics from collected attribute values](../rfc/prometheus-attribute-metrics.md);
not shipped with the first ADR-0604 milestone.

### 3. Target-scoped collection metrics (Tier B)

Replace profile-only semantics on `kollect_collected_objects`:

```
kollect_collected_objects{target_namespace, target_name, profile, gvk}
```

- **Type:** gauge
- **Value:** object count in the in-memory store for that target/GVK tuple
  (`Store.CountForTarget` — already computed in `internal/collect/engine.go`).
- **Pre-beta migration:** add labels to the existing metric name (same PromQL prefix). Document the
  breaking label-set change in [operator manual metrics](../operator-manual/metrics.md) release notes.

**New** target readiness gauge (derived from `KollectTarget.status.conditions`, not from informer
cache):

```
kollect_target_ready{target_namespace, target_name, profile}
```

- **Type:** gauge — `1` when `Ready=True`, `0` otherwise
- **Cardinality:** one series per `KollectTarget` CR

### 4. Domain metrics — `metricsScope` on profile (Tier C)

**Ship** optional `KollectProfile.spec.metricsScope` (and the same field on
`KollectClusterProfile`). This is a **profile-level knob only** — no per-target override in v1.

| `metricsScope` | Emission key | Series labels |
| --- | --- | --- |
| `profile` (default) | Sum across all targets using the profile in the operator watch scope | `profile`, `gvk`, `series` (+ attribute labels) |
| `target` | One snapshot per target refresh | `target_namespace`, `target_name`, `profile`, `gvk`, `series` (+ attribute labels) |

**API sketch:**

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: cert-domain-metrics
spec:
  metricsScope: target   # optional — profile | target; default profile
  attributes:
    - name: notAfterEpoch
      path: "{.status.notAfter}"
      # ...
  metrics:
    - name: cert_expiry_sum
      path: notAfterEpoch
      labels:
        - issuer
```

- **Validation:** webhook rejects unknown enum values; default `profile` when omitted preserves
  current aggregate behavior for shared profiles.
- **Labeled series:** `kollect_custom_resource_labeled_series` follows the same scope rule; attribute
  label keys remain admission-bounded (max 5 keys per `MetricSpec`, max distinct tuples documented in
  PERFORMANCE.md).
- **`object_count` series:** always emitted for the chosen scope (per ADR-0304 spike).

**Forbidden by default:** `name`, `namespace`, or `uid` as Prometheus labels on domain metrics unless
listed explicitly in `MetricSpec.labels` **and** admission confirms the attribute is low-cardinality
(e.g. `phase`, `sync_status` — not pod names).

### 5. Inventory-scoped rollup metrics (Tier B)

Replace the process-global `kollect_inventory_items_total` gauge with inventory-keyed series:

```
kollect_inventory_snapshot_items{inventory_namespace, inventory_name}
```

- **Type:** gauge — item count in the last aggregated snapshot for that inventory reconcile
- **Set from:** `KollectInventory` reconciler after aggregation (same count written to status /
  Read API snapshot)

Optional companion gauges (same label pair):

| Metric | Meaning |
| --- | --- |
| `kollect_inventory_targets_selected` | Targets matched by inventory selector |
| `kollect_inventory_targets_ready` | Subset with `Ready=True` |
| `kollect_inventory_targets_degraded` | Subset with `Degraded=True` |

**Cluster rollup:** mirror with `kollect_cluster_inventory_snapshot_items{inventory_namespace, inventory_name}` for `KollectClusterInventory` when cluster-scoped rollups ship.

**Deprecation:** remove unlabeled `kollect_inventory_items_total` after one release with both gauges
(pre-beta: may hard-cut).

### 6. Multi-cluster scrape topology — per cluster operator

**Fleet model:** Deploy one operator per cluster (`mode: single`). Prometheus scrapes **`/metrics`
on each cluster Helm release** where collection runs ([ADR-0501](0501-multi-cluster-fleet.md),
[operator manual](../operator-manual/metrics.md)).

| Install | What `/metrics` exposes |
| --- | --- |
| **Single cluster** | Tier A (+ Tier B/C when ADR-0604 ships) for that watch scope |
| **Fleet** | Same per cluster — correlate alerts with `spec.cluster` on export rows, not hub counters |

Hub/spoke federation and `kollect_hub_*` counters were **removed** with the hub tier (v0.3).

### 7. Cardinality rules

| Rule | Enforcement |
| --- | --- |
| Target labels | One series per **CR** (`target_namespace` + `target_name`), not per collected object |
| Domain attribute labels | Webhook: max 5 label keys per `MetricSpec`; reject `name`/`namespace` unless allow-listed |
| Operator budget | Document **soft cap** 10k active series per operator instance at baseline tier ([ADR-0603](0603-performance-scalability.md)); split profiles or reduce targets when exceeded |
| HA | Only the **leader** pod runs collection reconcilers ([ADR-0504](0706-testing-merge-gate-architecture.md)) — `/metrics` on non-leader replicas expose Tier A only (controller-runtime defaults) unless documented otherwise |

Add a `kollect_metrics_series_estimated` internal gauge (optional implementation detail) for
operators to alert on cardinality growth — not required for first ship.

### 8. Scrape configuration

No new Service type. Extend existing Helm `metrics.serviceMonitor` per **spoke** install
([operator manual](../operator-manual/metrics.md)):

```yaml
# Example: drop domain labeled series in large fleets (keep Tier A + B)
metricRelabelings:
  - sourceLabels: [__name__]
    regex: kollect_custom_resource_labeled_series
    action: drop
```

Document in PERFORMANCE.md:

- Scrape interval ≥ 30s when Tier C enabled (aligns with default export debounce).
- `honorLabels: true` not required — kollect does not expose `target` Kubernetes labels on metrics.
- Prometheus Operator `PrometheusRule` alerts should prefer Tier B for tenant alerts (`kollect_target_ready == 0`) and Tier A for operator health.
- Multi-cluster: one `ServiceMonitor` per cluster operator release.

### 9. Verification

ADR-0604 metrics must be **well tested** across the pyramid defined in
[ADR-0706](0706-testing-merge-gate-architecture.md). Implementation is not complete until all rows
below have passing tests.

| Layer | Tier | What it proves | Where / how |
| --- | --- | --- | --- |
| **L0 — Unit** | A–C | Gauge label sets and values from mock store / snapshot builder; catalog drift gate | `internal/metrics/metrics_catalog_test.go`, `internal/collect/metrics_snapshot*_test.go`, table-driven Prom registry assertions |
| **L1 — envtest** | B, C | Reconcilers update target/inventory gauges after CR status changes; `metricsScope` changes emission labels | `internal/controller/*_test.go` (Ginkgo + envtest); webhook rejects invalid `metricsScope` |
| **L2 — Golden / contract** | B, C | OpenAPI / CRD validation for `metricsScope`; sample profiles decode | `test/schema/`, `config/samples/`; extend samples with `metricsScope: target` |
| **L3 — Integration** | A–C | `/metrics` handler returns expected series after in-process reconcile with real registry | New or extended `-tags=integration` test scraping `:8080/metrics` text exposition (opt-in CI tier per ADR-0706) |
| **L4 — E2E** | B, C | Helm install + live scrape asserts on kind | `hack/kind/e2e/smoke.sh`, `hack/kind/e2e/bootstrap-samples.sh`, `config/samples/e2e/`; add Prometheus scrape step (port-forward or in-cluster Prometheus from `hack/kind/dev/setup.sh` pattern) asserting `kollect_collected_objects{target_*}` and inventory gauges; nightly `e2e-nightly.yaml` |

**`metricsScope` test expectations:**

- With default `profile`, two targets sharing a profile produce **one** sum series per `(profile, gvk, series)`.
- With `metricsScope: target`, the same setup produces **distinct** series per `(target_namespace, target_name, …)`.
- Switching scope on an existing profile updates labels on next refresh (no stale target labels on profile-scoped series).

**E2E patterns to follow:** multitenant fixtures in `test/e2e/fixtures/multitenant/` (multiple targets),
`hack/e2e/cert-manager.sh` (generic CRD collection → object count), and kind sample bootstrap in
`hack/kind/e2e/bootstrap-samples.sh`.

## Consequences

### Positive

- Target and inventory alerts match CR boundaries users already manage with `kubectl`.
- Resolves last-writer-wins bug on shared profiles without inventing a Prometheus export sink.
- Tier model keeps Phase 1 PromQL dashboards stable while Phase 4 domain metrics opt in via profile config.
- Spoke-local scrape matches where collection runs — no hub federation complexity in v1.

### Negative

- Label-set change on `kollect_collected_objects` breaks existing dashboards pre-1.0.
- Per-target domain scope multiplies Tier C cardinality by target count — requires `metricsScope` default `profile` and admission guardrails.
- Inventory gauges add O(inventories) series — acceptable for namespaced tenancy, still bounded by CR count.
- Multi-cluster operators must scrape **each spoke** — more `ServiceMonitor` wiring than a single hub federate.

## Open questions

- **Read API parity:** expose the same snapshot counts via Read API health fields for UI without
  duplicating Prometheus ([ADR-0408](0408-read-api-ui-architecture.md)).
- **Estimated series gauge:** ship as Tier A metric or internal pprof-only debug?
- **Hub domain series:** `kollect_hub_custom_resource_series` shape and merge semantics when hub-only
  alerting is requested — depends on scrape-at-spoke remaining primary.
- **Attribute value metrics (Tier C′):** promote [RFC: Prometheus attribute metrics](../rfc/prometheus-attribute-metrics.md)
  when scalar field export is prioritized.

## See also

- [Planned features — Prometheus metrics scoped to targets](../roadmap/planned-features.md#prometheus-metrics-scoped-to-targets-inventory-rows)
- [RFC: Prometheus metrics from collected attribute values](../rfc/prometheus-attribute-metrics.md)
- [ADR-0601: Operator metrics — no Prometheus export sink](0601-prometheus-metrics-stub.md)
- [ADR-0304: Custom-resource metrics and richer aggregation](0304-custom-resource-aggregation-rfc.md)
- [ADR-0602: Error taxonomy and reconcile behavior](0602-error-taxonomy.md)
- [ADR-0603: Performance and scalability](0603-performance-scalability.md)
- [ADR-0706: Testing and merge-gate architecture](0706-testing-merge-gate-architecture.md)
- [Operator metrics](../operator-manual/metrics.md)
- [PERFORMANCE.md](../PERFORMANCE.md)
