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
each failure mode needs predictable behavior.

## Decision

Typed error taxonomy (see `GUIDELINES.md`):

| Class | Examples | Reconcile behavior | Status |
| --- | --- | --- | --- |
| `ErrTransient` | Network blip, 429, conflict, sink timeout | Requeue with exponential backoff + jitter | `Synced=False`, `Reason=Progressing` |
| `ErrTerminal` | Invalid CEL/JSONPath, unknown GVK, bad sink type | **No requeue** | `Degraded=True` + Warning Event |
| `ErrForbidden` | SAR denied for namespace/cluster list | Degrade scope, continue partial collection | Per-target `skipped:forbidden` |

Additional rules:

- Wrap with `%w`; never ignore errors (`errcheck`, `errorlint`, `wrapcheck`).
- No `panic` in reconcilers; entrypoint `recover` → requeue + Event.
- Context deadlines on all external calls; never log secrets or full payloads.
- Circuit breaker (`gobreaker`) per sink/doc backend for `ErrTransient` storms.

Conditions follow Kubernetes conventions: `Ready`, `Synced`, `Degraded` with `observedGeneration`.

## Consequences

### Positive

- Predictable operator behavior under RBAC degradation and sink outages.
- Testable: table-driven tests per error class.
- Aligns with Argo/Flux condition semantics familiar to platform engineers.

### Negative

- Requires discipline in backend implementations to return typed errors, not raw HTTP strings.
- `ErrForbidden` partial success complicates "single Ready condition" — may need per-target sub-conditions
  in status summaries.

## Open questions

- **OPEN:** Standardize condition **Reason** enum in CRD validation or keep free-form strings?
- **OPEN:** Export Prometheus counter `kollect_reconcile_errors_total{kind,class}` in Phase 0 or Phase 1?
