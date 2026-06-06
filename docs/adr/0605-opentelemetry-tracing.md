# ADR-0605: OpenTelemetry tracing for reconcile and export paths

> Optional distributed tracing (OTel SDK, OTLP export) for controller reconcile, collection, sink
> export, and hub ingest — complementary to Prometheus metrics and structured logs.

**Theme:** 06 · Observability & ops · **Status:** Exploring

## Context

Kollect observability today is **metrics-first** ([ADR-0601](0601-prometheus-metrics-stub.md),
[ADR-0602](0602-error-taxonomy.md), [ADR-0603](0603-performance-scalability.md)) with
controller-runtime structured logging. Reconcile paths span multiple subsystems:

| Subsystem | Entry points | External I/O |
| --- | --- | --- |
| **Reconcile** | `KollectTarget`, `KollectInventory`, `KollectClusterTarget`, `KollectClusterInventory`, `KollectConnectionTest`, `KollectRemoteCluster` | Kubernetes API (status patches, SAR) |
| **Collect** | Dynamic informer callbacks → `internal/collect/engine.go` | API server list/watch (via informer) |
| **Export** | Inventory / cluster-inventory reconcilers → `internal/export`, `internal/sink` | Git, S3/GCS, Postgres, Kafka, NATS |
| **Hub ingest** | `internal/hub/ingest.go` HTTP handler, `internal/hub/consumer.go` transport subscriber | Queue / HTTP push from spokes |

Cross-cutting latency (slow Postgres export vs informer backlog vs hub merge) is hard to diagnose
from counters alone. [Planned features](../roadmap/planned-features.md) targets **OpenTelemetry
tracing** validated in **kollect-lab** (local companion environment alongside
[kind local lab](../examples/kind-local-lab.md)) — **not** enabled in default Helm values.

This ADR defines span boundaries, naming, configuration, and validation before production defaults
ship.

## Decision

### 1. Scope — in-process spans first

**In scope (v1):**

- Manual spans around controller `Reconcile` functions (via shared helper wrapping
  `reconcile_metrics.go` pattern).
- Child spans for **collect** batches triggered from informer handlers (one span per target refresh
  batch, not per object).
- Child spans for **export** attempts (one span per `(inventory, sink)` export try including
  debounce skip as a short span with attribute `export.skipped=true`).
- Hub **ingest** HTTP handler and transport **consumer** merge handler spans; child span for
  optional hub export fan-out.

**Out of scope (v1 — non-goals):**

- W3C trace context propagation into Git commits, SQL statements, or object-store PUT headers.
- Automatic gRPC/http client instrumentation of every client-go REST call (too noisy; use metrics
  `kollect_informer_objects` / API throttling logs instead).
- Tracing inside validating webhooks (keep webhook latency out of collection trace trees).
- CRD-based tracing configuration (`KollectTraceConfig`) — operator flags/env only until requirements
  stabilize.
- Default-on tracing in production Helm chart.

### 2. Trace provider and export

- **SDK:** OpenTelemetry Go SDK (traces only in v1; logs/metrics correlation deferred).
- **Exporter:** OTLP over gRPC (primary) and HTTP/protobuf (fallback) — standard
  `OTEL_EXPORTER_OTLP_*` env vars supported.
- **Resource attributes:** `service.name`, `service.version` (image tag / `--version`), `k8s.pod.name`,
  `k8s.namespace.name`, `kollect.mode` (`single` | `hub` | `spoke` from [ADR-0504](0504-operator-runtime-modes-ha-leader-election.md)).
- **Idempotent setup:** `internal/telemetry/tracer.go` (new package) initializes once from
  `Manager` setup in `cmd/main.go`; noop tracer when disabled (zero overhead on hot path aside from
  one atomic check).

### 3. Operator configuration (flags + env)

| Flag | Env override | Default | Notes |
| --- | --- | --- | --- |
| `--tracing-enabled` | `KOLLECT_TRACING_ENABLED` | `false` | Master gate |
| `--otel-exporter-otlp-endpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` | _(empty)_ | Required when tracing enabled |
| `--otel-service-name` | `OTEL_SERVICE_NAME` | `kollect-controller` | |
| `--otel-traces-sampler` | `OTEL_TRACES_SAMPLER` | `parentbased_traceidratio` | |
| `--otel-traces-sampler-arg` | `OTEL_TRACES_SAMPLER_ARG` | `0.01` | 100% in kollect-lab overlay |

Helm: `tracing.enabled: false` with documented `values-lab.yaml` overlay for kollect-lab (otel-collector
sidecar or cluster-level collector endpoint).

**Fail-open:** misconfigured endpoint logs an error at startup; operator continues with tracing
disabled rather than crash-looping.

### 4. Span naming and attributes

Follow [OpenTelemetry semantic conventions](https://opentelemetry.io/docs/specs/semconv/) where they
apply. Kollect-specific attributes use the `kollect.*` prefix.

| Span name | Parent | Key attributes |
| --- | --- | --- |
| `kollect.reconcile` | root (per reconcile request) | `kollect.controller`, `k8s.namespace.name`, `k8s.object.name`, `kollect.result` (`success`/`failure`), `kollect.error_class` ([ADR-0602](0602-error-taxonomy.md)) |
| `kollect.collect.refresh` | `kollect.reconcile` or informer-driven root | `kollect.target.namespace`, `kollect.target.name`, `kollect.profile`, `kollect.gvk`, `kollect.objects.updated` |
| `kollect.export` | `kollect.reconcile` | `kollect.inventory.namespace`, `kollect.inventory.name`, `kollect.sink.type`, `kollect.export.skipped` (debounce), `kollect.export.bytes` |
| `kollect.sink.connect` | `kollect.reconcile` (connection test) | `kollect.sink.type`, `kollect.connection_test.result` |
| `kollect.hub.ingest` | root (HTTP/queue handler) | `kollect.hub.name`, `kollect.cluster.id`, `kollect.report.bytes`, `kollect.merge.rows_applied` |
| `kollect.hub.export` | `kollect.hub.ingest` | Same as `kollect.export` when hub fan-out runs |

**Events (not child spans)** for terminal validation failures inside export (e.g. payload too large) —
keeps span count stable under error storms.

**Span status:** set `Error` on returned Go error; `Ok` on success or benign `RequeueAfter` without
error.

### 5. Sampling defaults

| Environment | Sampler | Rationale |
| --- | --- | --- |
| Production (default Helm) | Tracing off | Avoid accidental OTLP traffic |
| Production (explicit enable) | `parentbased_traceidratio` @ **1%** | Balance cost vs coverage |
| kollect-lab / CI smoke | `always_on` or ratio **100%** | Deterministic integration assertions |

Always record spans when `kollect.error_class=terminal` if **tail-sampling** hook is added later —
open for Phase 2.

### 6. Relationship to metrics and logs

- **Metrics remain source of truth for SLOs** ([ADR-0603](0603-performance-scalability.md)) — tracing is
  diagnostic, not alerting-first.
- Span duration for `kollect.export` should correlate with
  `kollect_export_duration_seconds{sink_type}` histogram samples (different aggregation — document in
  runbooks, do not dual-write histograms from spans).
- **Log correlation (Phase 2):** inject `trace_id` / `span_id` into controller-runtime log values
  when span context present — not required for v1 ship.
- **Hub / spoke:** propagate `traceparent` on spoke **HTTP push** to hub ingest when both sides have
  tracing enabled (optional v1.1); queue transport may carry trace context in message headers per
  [ADR-0502](0502-lean-queue-transport.md) extension.

### 7. kollect-lab validation story

Validation lives in the **kollect-lab** companion layout (kind cluster + observability overlay), not
the default chart:

1. Deploy **OpenTelemetry Collector** (contrib) with `debug` or `otlphttp` exporter plus a
   `file`/`memory` exporter for assertions.
2. Install kollect with `tracing.enabled=true` and endpoint pointing at the collector.
3. Apply sample targets/inventories (`config/samples/` or wide-scope demo).
4. **Assert:** at least one `kollect.reconcile` span per controller kind, one `kollect.collect.refresh`
   after object churn, one `kollect.export` on inventory export, and hub spans when `mode: hub` /
   spoke push is exercised.
5. **Regression gate:** optional `KOLECT_TRACING_SMOKE=1` integration test in repo (skipped in default
   CI) mirroring kollect-lab — aligns with [ADR-0706](0706-testing-merge-gate-architecture.md) opt-in
   tier.

Document the overlay in `docs/examples/kind-local-lab.md` (or a sibling `kollect-lab-tracing.md`) when
implementation lands — keep collector manifests under `config/telemetry/` or `hack/lab/`.

### 8. Security and performance

- OTLP export uses **TLS** when endpoint requires it (`OTEL_EXPORTER_OTLP_CERTIFICATE` / system roots).
- Do **not** attach inventory row payloads, extracted attributes, or sink credentials to span
  attributes ([ADR-0104](0104-security-model.md)).
- Span creation on hot informer paths must be gated by sampling — unsampled traces should avoid
  attribute allocations (use `trace.WithSpanKind` + minimal attrs).
- Tracing overhead target: **&lt;5%** CPU at 10k objects with 1% sampling (validate in perf snapshot
  workflow per local agent observability guidelines).

## Consequences

### Positive

- End-to-end latency visibility across collect → aggregate → export → hub merge without new CRDs.
- Opt-in model preserves default Helm simplicity; kollect-lab proves wiring before SLO commitments.
- Span attributes align with existing error taxonomy and metric label vocabulary.

### Negative

- Manual span instrumentation must be maintained across new reconcilers and sink backends.
- Partial traces when sampling drops child spans — operators need runbook guidance.
- OTLP endpoint misconfiguration may silently disable tracing (fail-open trade-off).

## Open questions

- **Tail sampling:** adopt collector-side tail sampling for error-only retention vs in-process hooks?
- **Trace propagation on NATS/Kafka** ([ADR-0502](0502-lean-queue-transport.md)): header schema and
  compatibility with non-Go consumers.
- **Leader election:** only leader emits collect/reconcile traces — document gap on standby pods.
- **UI / Read API:** expose trace IDs on failed export status for portal drill-down
  ([ADR-0408](0408-read-api-ui-architecture.md)) — likely Phase 2.
- **Unified telemetry config:** merge with future `--enable-pprof` runbook into one observability
  section of Helm values.

## See also

- [Planned features — OpenTelemetry tracing](../roadmap/planned-features.md#opentelemetry-tracing)
- [ADR-0601: Operator metrics — no Prometheus export sink](0601-prometheus-metrics-stub.md)
- [ADR-0602: Error taxonomy and reconcile behavior](0602-error-taxonomy.md)
- [ADR-0603: Performance and scalability](0603-performance-scalability.md)
- [ADR-0502: Lean queue transport](0502-lean-queue-transport.md)
- [ADR-0504: Operator runtime modes, HA, and leader election](0504-operator-runtime-modes-ha-leader-election.md)
- [Kind local lab](../examples/kind-local-lab.md)
- [ADR and RFC process](../development/adr-rfc-process.md)
