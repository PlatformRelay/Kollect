# KollectInventory

**Scope:** Namespace · **Reconciled:** Yes · **Short name:** `kinv`

!!! note "Payload location"
    Full export payloads live in sinks — `status` holds counts, conditions, and metadata refs only
    ([ADR-0103](../adr/0103-etcd-limit.md)). Query Postgres or Git for the authoritative snapshot.

## What it is for

A `KollectInventory` aggregates all collected rows from `KollectTarget` objects in the **same
namespace** and exports the marshalled JSON payload to one or more `KollectSink` backends. The
in-memory store updates immediately on every watch event; sink writes coalesce **per sink ref** —
each backend may use a different effective `exportMinInterval`
([ADR-0413](../adr/0413-export-interval-scheduling.md)).

Postgres and Kafka are the **primary** portal integration path; Git suits small single-cluster
installs. Full payloads live in sinks; `status` holds counts, conditions, and export metadata only
([ADR-0103](../adr/0103-etcd-limit.md)).

## How it fits the pipeline

```mermaid
flowchart LR
  Target[KollectTarget]
  Store[(Collect store)]
  Inv[KollectInventory]
  Scope[KollectScope]
  Sink[KollectSink]

  Target --> Store
  Store --> Inv
  Scope -.->|sink allow-list| Inv
  Inv -->|debounced export| Sink
```

| Relationship | Rule |
| --- | --- |
| Targets | All active targets in namespace contribute rows |
| Sinks | `spec.sinkRefs[]` — names in same namespace |
| Scope | When present, every sink must be listed in `scope.sinkRefs` |

Debouncing state machine: [DATA-FLOWS.md §1](../DATA-FLOWS.md#1-export-debouncing).

!!! info "Effective interval precedence"
    For each `sinkRefs` entry: **ref override** → **`KollectSink.spec.exportMinInterval`** →
    **`spec.exportMinInterval`** (default **30s**) → clamped to **`KollectScope.spec.minExportInterval`**
    floor when scope exists. Material checksum or `metadata.generation` changes bypass debounce **per
    sink**. See [ADR-0413](../adr/0413-export-interval-scheduling.md).

## Spec fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `spec.sinkRefs[]` | list | No | — | Sink names (string) or `{ name, exportMinInterval? }` objects — max 20 |
| `spec.exportMinInterval` | duration | No | **30s** | Default min gap for refs without override; bypass on checksum or generation change |
| `spec.maxExportBytes` | int64 | No | global cap | Max marshalled payload size |
| `spec.suspend` | bool | No | false | Pause reconciliation |
| `spec.httpEndpoint.enabled` | bool | No | false | Per-CR HTTP debug (operator gate also required) |
| `spec.httpEndpoint.port` | int32 | No | 8082 | Listen port when HTTP enabled |

## Sample usage

```sh
# Prerequisites: profile, target, sink in default namespace
kubectl apply -f config/samples/kollect_v1alpha1_kollectprofile.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollectsink_postgres.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollecttarget.yaml
kubectl apply -f config/samples/kollect_v1alpha1_kollectinventory.yaml

kubectl get kinv -n default team-inventory -w
kubectl describe kinv team-inventory -n default
```

Git-backed walkthrough (swap postgres sink for git sample):

```sh
kubectl apply -k config/samples/
kubectl get kinv,ktgt,ksink -A
```

Force faster export after spec change (generation bump):

```sh
kubectl patch kinv team-inventory -n default --type=merge \
  -p '{"spec":{"exportMinInterval":"10s"}}'
```

## Status conditions

| Type | When set | Meaning | Remediation |
| --- | --- | --- | --- |
| `Ready=True` | Healthy | Aggregating and exporting | None |
| `Synced=True` | Export OK | All sinks exported on last reconcile (or debounced with no failures) | Check `status.lastExportTime` |
| `Synced=False` `PartiallySynced` | Mixed cadence | Some sinks exported; others debounced on identical payload | Normal for dual-cadence fan-out — inspect `status.sinkExports[]` |
| `Synced=False` | Transient export error | `reason`: `Progressing` | Wait for retry/backoff |
| `Degraded=True` | Hard block | Scope, size, or terminal export | See reasons below |
| `SinkReachable=True/False` | Pre/post export | Sink probe or last export outcome | Fix [KollectSink](kollectsink.md) |

### Per-sink status (`status.sinkExports[]`)

When `spec.sinkRefs` lists multiple sinks, each entry mirrors export observation:

| Field | Meaning |
| --- | --- |
| `name` | Sink ref name (matches `spec.sinkRefs[].name`) |
| `lastExportTime` | Last successful export to this sink |
| `lastChecksum` | Payload fingerprint from last export |
| `conditions[]` | Per-sink `Synced` — `reason=Debounced` when interval not elapsed |

Aggregate `status.lastExportTime` is the **max** of per-sink times (backward compatible). Read API
`/status` prefers `sinkExports` when present ([ADR-0413](../adr/0413-export-interval-scheduling.md)).

### Common `Degraded` reasons

| Reason | Cause | Fix |
| --- | --- | --- |
| `ScopeSinkDenied` | Sink not in scope | Add to `KollectScope.spec.sinkRefs` |
| `ScopeLookupFailed` | Cannot read scope | RBAC / API error |
| `SinkNotFound` | Bad `sinkRefs` entry | Correct sink name |
| `SinkUnreachable` | `ConnectionVerified=False` | Fix sink credentials / network |
| `PayloadTooLarge` | Exceeds `maxExportBytes` | Split targets, raise cap within global limit, or trim attributes |
| `ExportTerminal` | Non-retryable sink error | Fix sink config; check operator logs |
| `Progressing` | Transient network/429 | Usually self-heals; inspect `kollect_sink_errors_total` |

## RBAC

| Actor | Verbs | Resource | Notes |
| --- | --- | --- | --- |
| Team admins | `create`, `update`, `patch`, `delete` | `kollectinventories` | Configure export |
| Developers | `get`, `list`, `watch` | `kollectinventories` | Read status / counts |
| Operator | `get`, `list`, `watch` | `kollectinventories`, `kollecttargets`, `kollectsinks`, `kollectscopes` | Aggregate + export |
| Operator | `get`, `list`, `watch` | `secrets` | Sink credential resolution |
| Operator | `update`, `patch` | `kollectinventories/status` | Conditions and export metadata |

HTTP inventory read path (when enabled) requires caller SAR `get` on `kollectinventories` —
[ADR-0404](../adr/0404-inventory-api-auth.md).

## Common failure modes

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `itemCount` 0 | No matching targets or suspended targets | Check `ktgt` status; deploy matching workloads |
| Exports every 30s identical payload | Debounce working as designed | Lower `exportMinInterval` only if needed |
| No export for minutes | Debounced identical checksum | Change inventory material (deploy patch) or wait interval |
| Postgres empty table | Export not implemented / sink error | `kubectl logs -n kollect-system deploy/kollect-controller-manager` |
| `RequeueAfter` in logs | Debounce wait | Normal — see [DATA-FLOWS](../DATA-FLOWS.md) timing example |
| HTTP endpoint unreachable | Feature gate off | Enable Helm `featureGates.inventoryHttp` **and** `spec.httpEndpoint.enabled` |

## See also

- [KollectTarget](kollecttarget.md) · [KollectSink](kollectsink.md) · [KollectScope](kollectscope.md)
- [DATA-FLOWS.md](../DATA-FLOWS.md)
- [examples/deployment-inventory.md](../examples/deployment-inventory.md)
- [ADR-0602](../adr/0602-error-taxonomy.md) — error classes
