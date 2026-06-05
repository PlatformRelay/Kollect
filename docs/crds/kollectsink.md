# KollectSink

**Scope:** Namespace · **Reconciled:** Connection probe only · **Short name:** —

Static export backend configuration. Primary sinks: **Postgres** and **Kafka**; Git for audit
([ADR-0025](../adr/0025-sink-backends-database-kafka.md), [ADR-0032](../adr/0032-platform-architecture-pivot.md)).

## Spec fields

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `spec.type` | enum | Yes | `git`, `gitlab`, `s3`, `gcs`, `postgres`, `kafka` |
| `spec.endpoint` | string | No | Backend-specific destination URL or bucket |
| `spec.secretRef` | object | No | Secret with credentials (`name`, optional `namespace`) |
| `spec.tls` | object | No | `insecureSkipVerify`, `caSecretRef`, `caBundle` (max 64 KiB) |
| `spec.connectionTest` | bool | No | Probe on create/update when true |
| `spec.cluster` | string | No | Cluster label for multi-cluster exports |
| `spec.postgres` | object | No | `databaseRef`, `table`, `schema` |
| `spec.kafka` | object | No | `brokers[]`, `topic`, optional `secretRef` |

## Status conditions

| Type | When set | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | Probe completes | Sink endpoint reachable (or probe failed) |
| `TLSInsecure` | TLS skip verify | Warning when verification disabled |

## RBAC

| Verb | Resource | Notes |
| --- | --- | --- |
| `get`, `list`, `watch` | `kollectsinks` | Inventory reconciler reads sink refs |
| `create`, `update`, `patch`, `delete` | `kollectsinks` | Sink admins in team namespace |

## Samples

- [`config/samples/kollect_v1alpha1_kollectsink_postgres.yaml`](../../config/samples/kollect_v1alpha1_kollectsink_postgres.yaml)
- [`config/samples/kollect_v1alpha1_kollectsink_kafka.yaml`](../../config/samples/kollect_v1alpha1_kollectsink_kafka.yaml)
- [`config/samples/kollect_v1alpha1_kollectsink.yaml`](../../config/samples/kollect_v1alpha1_kollectsink.yaml) — Git

## Failure modes

> **TODO:** Document `ConnectionVerified=False` reasons, secret missing keys, postgres DSN errors, and
> scope deny when sink not in `KollectScope.spec.sinkRefs`.
