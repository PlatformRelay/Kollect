# ADR-0020: Error taxonomy and reconcile behavior

## Status

Accepted

## Context

Robust operators must classify errors so the workqueue does not spin on permanent failures or
silently swallow transient ones. OSS precedents:

- **controller-runtime** defaults: return error → requeue with rate limit; `RequeueAfter` for delays.
- **Flux** controllers set **Ready/Stalled** conditions with distinct reasons; terminal misconfig
  stops reconciliation until spec changes.
- **external-secrets** ExternalSecret status records `SecretSyncedError` with provider error messages
  (sanitized).
- **Argo CD** Application conditions distinguish `ComparisonError`, `SyncError`, and permission issues.

kollect touches the API server, arbitrary GVKs, SAR boundaries, and external sinks/doc backends —
each failure mode needs predictable behavior. Observability must be **testable from day one**, not
bolted on after Phase 4.

## Decision

Typed error taxonomy (see `GUIDELINES.md`):

| Class | Examples | Reconcile behavior | Status |
| --- | --- | --- | --- |
| `ErrTransient` | Network blip, 429, conflict, sink timeout | Requeue with exponential backoff + jitter | `Synced=False`, `Reason=Progressing` |
| `ErrTerminal` | Invalid CEL/JSONPath (if not caught by webhook), unknown GVK, bad sink type | **No requeue** | `Degraded=True` + Warning Event |
| `ErrForbidden` | SAR denied for namespace/cluster list | Degrade scope, continue partial collection | Per-target `skipped:forbidden` |

Additional rules:

- Wrap with `%w`; never ignore errors (`errcheck`, `errorlint`, `wrapcheck`).
- No `panic` in reconcilers; entrypoint `recover` → requeue + Event.
- Context deadlines on all external calls; never log secrets or full payloads.
- Circuit breaker (`gobreaker`) per sink/doc backend for `ErrTransient` storms.

Conditions follow Kubernetes conventions: `Ready`, `Synced`, `Degraded` with `observedGeneration`.

### CRD enums (sink type and condition reasons)

Use **OpenAPI enums** (and Go constants) for:

- `KollectSink.spec.type` — `git`, `gitlab`, `s3`, `gcs`, `postgres`, `kafka` (extensible via webhook
  allow-list when adding backends). **Not** `prometheus` — operator metrics use `/metrics` only
  ([ADR-0012](0012-prometheus-metrics-stub.md)).
- Condition **`reason`** fields on reconciled kinds — e.g. `Progressing`, `InvalidProfile`,
  `SinkUnreachable`, `Forbidden`, `ConnectionTestSucceeded`, `ConnectionTestFailed`

Free-form operator strings are allowed only in **message** fields, not in `reason` or `type`.

### Prometheus metrics (Phase 0/1)

Export and **test** counters/histograms including at minimum:

| Metric | Labels |
| --- | --- |
| `kollect_reconcile_total` | `controller`, `result` |
| `kollect_reconcile_errors_total` | `kind`, `error_class` (`transient`, `terminal`, `forbidden`) |
| `kollect_export_duration_seconds` | `sink_type` |
| `kollect_collected_objects` | `profile`, `gvk` |
| `kollect_connection_test_total` | `sink_type`, `result` |

Metrics register on the controller-runtime metrics registry; unit tests assert label cardinality
and increment on table-driven reconcile cases.

## Consequences

### Positive

- Predictable operator behavior under RBAC degradation and sink outages.
- Testable: table-driven tests per error class + metrics assertions.
- Aligns with Argo/Flux condition semantics familiar to platform engineers.
- Enum reasons improve `kubectl` UX and portal filtering.

### Negative

- Requires discipline in backend implementations to return typed errors, not raw HTTP strings.
- `ErrForbidden` partial success complicates "single Ready condition" — may need per-target sub-conditions
  in status summaries.
- Enum evolution needs CRD versioning when adding reasons.

## Open questions

- **OPEN:** Separate `kollect_sink_errors_total{reason}` or fold into reconcile errors?
- **OPEN:** Histogram buckets for export duration — cluster-size dependent defaults?
