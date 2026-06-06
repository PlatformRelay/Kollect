# RFC: Prometheus metrics from collected attribute values

**Status:** Proposed (Exploring) · **Author:** @konih · **Created:** 2026-06-06

> Spec-only exploration — **no implementation** in the ADR-0604 ship window. Revisit when Tier C
> domain metrics ([ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md)) stabilizes.

## Summary

Extend operator `/metrics` export so **numeric (and optionally other cardinality-safe) values**
extracted via CEL/JSONPath in `KollectProfile.spec.attributes` can become **first-class Prometheus
series** — gauges and counters — not only KSM-style **aggregated sums** over a GVK.

Today [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) emits `kollect_custom_resource_series`
as **sums** across objects matching a profile path. Platform teams also want direct export of scalar
fields (e.g. cert `notAfter` epoch seconds, replica `availableReplicas`, Argo `health_status` as a
bounded enum mapped to numeric codes) without hand-rolling PromQL over inventory export sinks.

## Goals / non-goals

| Goals | Non-goals |
| --- | --- |
| Profile-defined gauge/counter from **numeric** attribute paths | `KollectSink.type: prometheus` — reaffirm [ADR-0601](../adr/0601-prometheus-metrics-stub.md) |
| Clear **label vs value** rules (what becomes a label key vs the sample value) | Unbounded string labels (pod names, UIDs, free-text messages) |
| Cardinality guardrails aligned with [ADR-0603](../adr/0603-performance-scalability.md) | Histogram/summary from arbitrary attribute distributions in v1 of this RFC |
| Reuse existing extraction engine ([ADR-0302](../adr/0302-cel-jsonpath-extraction.md)) | A second informer loop or push gateway |
| `metricsScope` parity with [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) (`profile` \| `target`) | Per-object series keyed by resource name/UID |

## Background

| Artifact | Relevance |
| --- | --- |
| [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) | KSM-style **sum** series from `spec.metrics[]` |
| [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) | Tier B/C label keys, `metricsScope`, scrape-at-spoke |
| [ADR-0302](../adr/0302-cel-jsonpath-extraction.md) | Attribute extraction paths reused for metric values |
| [Planned features — attribute metrics](../roadmap/planned-features.md#prometheus-metrics-from-collected-attribute-values) | Backlog entry |

**Relationship to ADR-0604 tiers:**

| Tier | This RFC |
| --- | --- |
| **A — Operator internals** | Out of scope |
| **B — Collection boundary** | Out of scope (counts/readiness stay as today) |
| **C — Domain (KSM-style sums)** | **Complements** — same `spec.metrics` config surface may grow, or a sibling `spec.attributeMetrics[]` block |
| **C′ — Attribute value export (proposed)** | **This RFC** — export the **scalar value** of a path per aggregation scope, not only sum/count |

When promoted, Tier C and C′ should share admission webhooks and cardinality budgets documented in
[PERFORMANCE.md](../PERFORMANCE.md).

## Proposal

### 1. Config surface (sketch)

Extend `KollectProfile` / `KollectClusterProfile` (exact field name TBD — `attributeMetrics` vs extended
`metrics[]` with a `kind` discriminator):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectProfile
metadata:
  name: cert-metrics
spec:
  metricsScope: target   # ADR-0604 — profile (default) | target
  attributeMetrics:
    - name: cert_not_after_seconds
      attribute: notAfterEpoch          # references spec.attributes[].name
      type: gauge                         # gauge | counter (counter only for monotonic counters)
      help: "Certificate notAfter as Unix seconds"
      labels:                           # optional — attribute names → Prometheus label keys
        - issuer
        - isCA
    - name: deployment_available_replicas
      attribute: availableReplicas
      type: gauge
```

**Rules:**

- `attribute` **must** reference an existing `spec.attributes[]` entry whose extracted type is
  **numeric** (`int`, `float`) or a **bounded enum** with an explicit numeric mapping in admission.
- **Value:** for `gauge`, emit the extracted number (last-write or sum policy per series kind — see §2).
- **Labels:** only keys listed in `labels[]`; each referenced attribute must pass low-cardinality
  admission (same rules as `spec.metrics[].labels` in ADR-0304).
- **`metricsScope`:** when `profile`, aggregate (e.g. sum or max — see open questions) across targets;
  when `target`, one metric family per target refresh with `target_namespace`, `target_name` labels
  ([ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md)).

**Metric name prefix:** `kollect_attribute_` or reuse `kollect_custom_resource_` with a `_value`
suffix — pick one at promotion time to avoid colliding with sum series.

### 2. Label vs value semantics

| Extracted shape | Prometheus role | Example |
| --- | --- | --- |
| Single numeric field | **Sample value** | `cert_not_after_seconds 1.734e9` |
| Low-cardinality string enum | **Label** (if listed) or **rejected** | `phase="Active"` → label only when allow-listed |
| High-cardinality string | **Forbidden** as label; do not emit | pod name, CN, serial number |
| Boolean | **Label** `true`/`false` or numeric 0/1 — **one convention** locked at promotion | `isCA="true"` |

**Forbidden by default:** `name`, `namespace`, `uid`, `resourceVersion` as labels — same as ADR-0604
Tier C.

### 3. Cardinality guardrails

| Guardrail | Enforcement |
| --- | --- |
| Max `attributeMetrics` entries per profile | Webhook cap (e.g. 20) |
| Max label keys per entry | 5 (align ADR-0304) |
| Series budget | Count toward ADR-0604 soft cap (~10k series / operator) |
| Counter type | Only when attribute is documented monotonic; otherwise gauge |
| Stale series | Clear gauges on object delete / target teardown (same as Tier B) |

### 4. Emission model

- Compute during collection engine refresh ([ADR-0301](../adr/0301-event-driven-informers.md)) — no
  separate scrape of inventory exports.
- **Spoke scrape:** metrics appear on the local operator `/metrics` ([ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md)
  scrape-at-spoke decision); hub does not federate these series in v1.
- Optional **enum → numeric** mapping in CRD for alert-friendly thresholds without string labels.

## Alternatives considered

| Option | Pros | Cons |
| --- | --- | --- |
| **Extend `spec.metrics[]` with `aggregation: sum\|value\|max`** | One config block | Overloads KSM sum semantics; harder validation |
| **Separate `attributeMetrics[]` (this RFC)** | Clear mental model | More CRD surface |
| **Prometheus recording rules over inventory export** | No operator change | Requires sink scrape; violates ADR-0601 spirit for domain alerts |
| **Only sums (status quo ADR-0304)** | Simple | Cannot expose per-field scalars (cert expiry timestamp) |

## Open questions

- **Aggregation when `metricsScope: profile`:** sum, max, min, or avg for gauge values across targets?
- **Multi-object same label tuple:** last-write vs sum vs expose `_count` companion gauge?
- **Histogram from numeric attributes:** defer or allow fixed-bucket profiles only?
- **Unified vs split CRD fields:** merge into `spec.metrics[]` with a `export: sum\|value` enum at promotion.
- **Testing:** mirror ADR-0604 verification matrix when implementation starts.

## Promotion

When accepted:

1. Update [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) and/or [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) with Tier C′ rules.
2. Set this RFC to **Superseded** with links.
3. Add CRD docs under `docs/crds/kollectprofile.md` and webhook validation.

## See also

- [ADR-0604: Target- and inventory-scoped Prometheus metrics](../adr/0604-target-scoped-prometheus-metrics.md)
- [ADR-0304: Custom-resource metrics and richer aggregation](../adr/0304-custom-resource-aggregation-rfc.md)
- [ADR-0601: Operator metrics — no Prometheus export sink](../adr/0601-prometheus-metrics-stub.md)
- [Planned features](../roadmap/planned-features.md)
- [RFC index](README.md)
