# ADR-0601: Operator metrics — no Prometheus export sink

> Operator metrics live on `/metrics`; `prometheus` is **not** a `KollectSink.type`.

**Theme:** 06 · Observability & ops · **Status:** Current

## Context

Phase 4 may add kube-state-metrics-style custom resource metrics. Phase 1 ships **operator**
metrics per [ADR-0602](0602-error-taxonomy.md). Inventory export uses Git, object storage, Postgres,
and Kafka sinks ([ADR-0402](0402-sink-backends-database-kafka.md)) — **not** a `KollectSink` of type
`prometheus`.

## Decision

1. **Operator metrics (Phase 1):** expose cardinality-safe gauges and histograms on the controller
   `/metrics` endpoint — including `kollect_collect_items_total`, `kollect_collected_objects`,
   `kollect_export_duration_seconds`, and reconcile counters.
2. **No Prometheus export sink:** `prometheus` is **not** a valid `KollectSink.spec.type`. Do not
   register a prometheus sink in the export registry; avoids confusion with scrape endpoints.
3. **KSM-style domain metrics and target/inventory scope:** Phase 4 spike landed on operator
   `/metrics` ([ADR-0304](0304-custom-resource-aggregation-rfc.md)); richer target/inventory labels
   and `metricsScope` are **Exploring** in [ADR-0604](0604-target-scoped-prometheus-metrics.md).
   Scalar attribute gauges remain RFC-only ([Prometheus attribute metrics](../rfc/prometheus-attribute-metrics.md)).

## Consequences

- Portals scrape **operator** `/metrics` for health and export latency; inventory payloads go to configured sinks.
- Helm chart optionally ships `ServiceMonitor` + `PrometheusRule` when Prometheus Operator is present (see [operator manual metrics](../operator-manual/metrics.md)).
- Phase 4 work must extend this ADR rather than invent a parallel metrics model.

## See also

- [ADR-0304: Custom-resource metrics and aggregation](0304-custom-resource-aggregation-rfc.md)
- [ADR-0604: Target- and inventory-scoped Prometheus metrics](0604-target-scoped-prometheus-metrics.md)
- [ADR-0605: OpenTelemetry tracing](0605-opentelemetry-tracing.md)
- [RFC: Prometheus attribute metrics](../rfc/prometheus-attribute-metrics.md)
- [ADR-0602: Error taxonomy and metrics](0602-error-taxonomy.md)
- [ADR-0404: Inventory HTTP auth](0404-inventory-api-auth.md)
