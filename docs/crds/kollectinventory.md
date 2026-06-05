# KollectInventory

**Scope:** Namespace · **Reconciled:** Yes · **Short name:** `kinv`

Aggregates all `KollectTarget` objects in the same namespace and exports namespace payload to
configured sinks. Export debouncing is **per inventory**
([ADR-0032](../adr/0032-platform-architecture-pivot.md), [DATA-FLOWS.md](../DATA-FLOWS.md)).

## Spec fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `spec.sinkRefs[]` | list | No | — | `KollectSink` names in same namespace |
| `spec.exportMinInterval` | duration | No | **30s** | Min gap between identical exports; bypass on checksum or generation change |
| `spec.maxExportBytes` | int64 | No | global cap | Max marshalled payload size |
| `spec.suspend` | bool | No | false | Pause reconciliation |
| `spec.httpEndpoint.enabled` | bool | No | false | Per-CR HTTP debug (operator gate also required) |
| `spec.httpEndpoint.port` | int32 | No | 8082 | Listen port when HTTP enabled |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| `SinkReachable` | Pre/post export | Sink probe or last export outcome (`ExportSucceeded` / `ExportFailed`) |
| `Degraded` | Scope violation | No collect/export — hard degrade |
| `Ready` | Healthy path | Inventory aggregating and exporting |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollectinventories`, `kollecttargets`, `kollectsinks` | Aggregation + export |
| `update`, `patch` | `kollectinventories/status` | Controller writes status |

## Samples

- [`config/samples/kollect_v1alpha1_kollectinventory.yaml`](../../config/samples/kollect_v1alpha1_kollectinventory.yaml)
- Walkthrough: [Deployment inventory](../examples/deployment-inventory.md)

## Failure modes

> **TODO:** Document `ScopeSinkDenied`, payload over `maxExportBytes`, sink terminal errors, debounce
> `RequeueAfter` behavior, and optional HTTP path when feature-gated.
