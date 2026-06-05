# KollectConnectionTest

**Scope:** Namespace · **Reconciled:** Yes · **Short name:** `kconntest`

One-shot audited connectivity probe for a `KollectSink`. Supplements sink annotation probes
([ADR-0030](../adr/0030-connection-test.md), [ADR-0032](../adr/0032-platform-architecture-pivot.md)).

## Spec fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `spec.sinkRef` | string | Yes | — | `KollectSink` name in same namespace |
| `spec.profileRef` | string | No | — | Reserved for composite probes |
| `spec.ownerSink` | bool | No | true | Set ownerReference to sink |
| `spec.ttlSecondsAfterFinished` | int32 | No | **300** | Delete CR after completion + TTL |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | Probe completes | Success or failure with reason |

## Status fields

| Field | Description |
| --- | --- |
| `status.completed` | True after probe finishes |
| `status.completedAt` | Timestamp for TTL cleanup |
| `status.latencyMs` | Last probe duration |
| `status.observedGeneration` | Generation last reconciled |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch`, `create`, `delete` | `kollectconnectiontests` | Users trigger probes |
| `update`, `patch` | `kollectconnectiontests/status` | Controller writes outcome |

## Samples

- [`config/samples/kollect_v1alpha1_kollectconnectiontest.yaml`](../../config/samples/kollect_v1alpha1_kollectconnectiontest.yaml)

## Failure modes

> **TODO:** Document re-probe on spec change, TTL deletion, sink not found, and terminal probe errors.
