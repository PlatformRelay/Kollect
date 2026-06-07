# ADR-0605: OpenTelemetry tracing for reconcile and export paths

> **Status: Parked (Deferred)** — design reference only; **no implementation** in v0.x.
> Prometheus metrics and structured logs remain the observability source of truth
> ([ADR-0601](0601-prometheus-metrics-stub.md), [ADR-0602](0602-error-taxonomy.md)).

**Theme:** 06 · Observability & ops · **Status:** Parked

## Context

Kollect observability today is **metrics-first** with controller-runtime structured logging.
Reconcile paths span target/inventory controllers, the collect engine, and sink export — all
**in-process** in a single operator binary (`mode: single`).

Cross-cutting latency is diagnosable today via:

- Prometheus: `kollect_reconcile_duration_seconds`, `kollect_export_duration_seconds`, error counters
- Structured logs with reconcile keys and `error_class` ([ADR-0602](0602-error-taxonomy.md))
- CR `status.sinkExports[]` and inventory conditions for per-export drill-down

**Hub/spoke tracing scope is obsolete** — `internal/hub/` was removed; there is no hub ingest,
queue transport between operators, or cross-pod trace propagation to design for.

Distributed tracing (OTel SDK, OTLP export) was explored for reconcile, collection, and export
spans. **No tracing code ships:** no `internal/telemetry/`, no operator flags, no Helm
`tracing.*`, no kollect-lab overlay. Continuing to elaborate sampling policies without spans
is **cargo-cult complexity** relative to trace volume (effectively zero).

## Decision (parked)

**Do not implement OpenTelemetry in v0.x** unless a concrete trigger is documented (maintainer
decision): mandated corporate OTLP backend, multi-binary SaaS portal needing `trace_id` on failed
exports, or paying users requesting trace correlation beyond metrics/logs.

If reopened post-v0.8, scope would be **in-process only**:

| Span name | Scope |
| --- | --- |
| `kollect.reconcile` | Per reconcile request |
| `kollect.collect.refresh` | Per target refresh batch |
| `kollect.export` | Per `(inventory, sink)` export attempt |

**Explicit non-goals (unchanged):** Prometheus export sink, CRD-based trace config, default-on
Helm tracing, W3C propagation into Git/SQL/object-store headers, webhook spans.

Configuration sketch (not implemented): `--tracing-enabled`, `OTEL_EXPORTER_OTLP_*`, Helm
`tracing.enabled: false` default, kollect-lab `always_on` for smoke tests.

## Consequences

### Positive (of parking)

- Solo-maintainer bandwidth stays on Prometheus Tier A hardening and Read API/UI tranches.
- Public docs stop promising OTEL delivery on the v0.3–0.7 path.
- Revisit when product shape includes **multiple services** or mandated OTLP.

### Negative

- No automatic causal chains for slow export sub-steps — operators use logs + metrics + status.
- ADR body retained as **design reference**; implementation deferred indefinitely.

## Reopen triggers

Document in maintainer backlog before any implementation:

1. Platform team requires OTLP and provides managed collector.
2. Portal UI needs `trace_id` on failed exports ([ADR-0408](0408-read-api-ui-architecture.md)).
3. Split read API / operator binaries need cross-service latency visibility.

## See also

- [ADR-0601: Operator metrics — no Prometheus export sink](0601-prometheus-metrics-stub.md)
- [ADR-0602: Error taxonomy and reconcile behavior](0602-error-taxonomy.md)
- [ADR-0603: Performance and scalability](0603-performance-scalability.md)
- [Planned features — OpenTelemetry tracing](../roadmap/planned-features.md#opentelemetry-tracing) (Deferred)
