# ADR-0012: Prometheus metrics and export sink (stub)

## Status

Accepted (stub, 2026-06-05)

## Context

Phase 4 may add kube-state-metrics-style custom resource metrics. Phase 1 ships **operator**
metrics per [ADR-0020](0020-error-taxonomy.md). Inventory export uses Git, object storage, Postgres,
and Kafka sinks ([ADR-0025](0025-sink-backends-database-kafka.md)) — **not** a `KollectSink` of type
`prometheus`.

## Decision

1. **Operator metrics (Phase 1):** expose cardinality-safe gauges and histograms on the controller
   `/metrics` endpoint — including `kollect_collect_items_total`, `kollect_collected_objects`,
   `kollect_export_duration_seconds`, and reconcile counters.
2. **No Prometheus export sink:** `prometheus` is **not** a valid `KollectSink.spec.type`. Do not
   register a prometheus sink in the export registry; avoids confusion with scrape endpoints.
3. **Full kube-state-metrics-style `CustomResourceStateMetrics` config:** deferred to Phase 4; optional
   `KollectProfile.spec.metrics` field reserved in design docs only (emitted via operator metrics path).

## Consequences

- Portals scrape **operator** `/metrics` for health and export latency; inventory payloads go to configured sinks.
- Phase 4 work must extend this ADR rather than invent a parallel metrics model.

## See also

- [ADR-0020: Error taxonomy and metrics](0020-error-taxonomy.md)
- [ADR-0024: Inventory HTTP auth](0024-inventory-api-auth.md)
