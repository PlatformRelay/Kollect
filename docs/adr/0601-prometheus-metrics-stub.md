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
3. **Full kube-state-metrics-style `CustomResourceStateMetrics` config:** deferred to Phase 4; optional
   `KollectProfile.spec.metrics` field reserved in design docs only (emitted via operator metrics path).

## Consequences

- Portals scrape **operator** `/metrics` for health and export latency; inventory payloads go to configured sinks.
- Phase 4 work must extend this ADR rather than invent a parallel metrics model.

## See also

- [ADR-0602: Error taxonomy and metrics](0602-error-taxonomy.md)
- [ADR-0404: Inventory HTTP auth](0404-inventory-api-auth.md)
