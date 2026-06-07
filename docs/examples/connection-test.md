# Example: Connection test

!!! tip "Opt out in production"
    `spec.connectionTest` defaults to **true** on family sink CRDs. Set `connectionTest: false` when
    automatic probes on every spec change are undesirable. Use the `kollect.dev/test-connection`
    annotation or a `KollectConnectionTest` CR for on-demand probes.

Kollect offers **three** ways to verify sink connectivity before relying on export
([ADR-0403](../adr/0403-connection-test.md), [ADR-0414](../adr/0414-sink-family-crds.md),
[ADR-0201](../adr/0201-crd-model.md)).

## Overview

| Mechanism | Best for | Writes to |
| --- | --- | --- |
| `spec.connectionTest` (default **true**) | Automatic probe on create/update | Family sink `status` |
| `kollect.dev/test-connection` annotation | On-demand prod re-test | Family sink `status` |
| `KollectConnectionTest` CR | Audited / CI pipelines | `KollectConnectionTest.status` |

**Default:** `spec.connectionTest: true` (CRD OpenAPI default). Set `false` to opt out.

Family sink CRDs ([ADR-0414](../adr/0414-sink-family-crds.md)):

| CRD | Role | Wired probe types |
| --- | --- | --- |
| `KollectSnapshotSink` | Snapshot store | `git`, `gitlab`, `s3`, `gcs` |
| `KollectDatabaseSink` | Relational SoR | `postgres` |
| `KollectEventSink` | Event emitter | `kafka`, `nats` |

Stub backends (`azureblob`, `http`, `bigquery`) pass admission but return *not implemented* at probe
time until shipped.

## Sink probe — `spec.connectionTest`

`config/samples/kollect_v1alpha1_kollectdatabasesink.yaml` sets `connectionTest: true`.

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectdatabasesink.yaml
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/postgres-inventory-demo \
  -n default --timeout=60s
kubectl describe kdb postgres-inventory-demo -n default
```

Git snapshot sink sample (`config/samples/kollect_v1alpha1_kollectsnapshotsink.yaml`):

```sh
kubectl wait --for=condition=ConnectionVerified kollectsnapshotsink/git-inventory-demo \
  -n default --timeout=60s
```

Each family has a **narrow reconciler** whose sole job is connection-test status — not collection or
export ([ADR-0403](../adr/0403-connection-test.md)).

## Annotation re-test

Trigger a one-shot probe without editing `spec`:

```sh
kubectl annotate kollectdatabasesink postgres-inventory-demo -n default \
  kollect.dev/test-connection=true --overwrite
```

```sh
kubectl annotate kollectsnapshotsink git-inventory-demo -n default \
  kollect.dev/test-connection=true --overwrite
```

When `spec.connectionTest: false`, the annotation is the supported way to re-probe in production.
After a successful annotation-only probe, the reconciler clears the annotation (kept when
`spec.connectionTest: true`).

## KollectConnectionTest CR

`config/samples/kollect_v1alpha1_kollectconnectiontest.yaml` — `spec.sinkRef` names exactly one
family sink:

```yaml
spec:
  sinkRef:
    databaseSinkRef: postgres-inventory-demo
  ownerSink: true
```

Snapshot or event probes use `snapshotSinkRef` or `eventSinkRef` instead.

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectconnectiontest.yaml
kubectl wait --for=condition=ConnectionVerified kollectconnectiontest/postgres-sink-probe \
  -n default --timeout=120s
kubectl get kconntest postgres-sink-probe -n default -o wide
```

Default `spec.ttlSecondsAfterFinished`: **300** (CR auto-deleted after probe completes + TTL).

Re-run after fixing credentials: delete and re-apply the CR, or patch `spec` to bump generation.

## Status conditions

On family sink CRDs:

| Condition | Meaning |
| --- | --- |
| **`ConnectionVerified`** `True` | Last probe succeeded |
| **`ConnectionVerified`** `False` | Probe failed (`ConnectionTestFailed`, `SecretResolveFailed`, …) |
| **`Degraded`** `True` | Set alongside failed probe |
| **`TLSInsecure`** `True` | `insecureSkipVerify` enabled (dev warning) |

Operator metric: `kollect_sink_connection_test_total{type,result}`.

## Pipeline reachability

`ConnectionVerified` on a family sink proves **credentials and network to the backend**. End-to-end
export health belongs on reconciled objects:

| Condition | Object | Meaning |
| --- | --- | --- |
| **`SinkReachable`** | `KollectInventory`, `KollectTarget` | Family sink resolution before export |
| **`Synced`** | `KollectInventory` | Export succeeded for a sink ref |

See [Deployment inventory](deployment-inventory.md) for the full Profile → Target → Inventory →
family sink path.

## Related

- [KollectConnectionTest](../crds/kollectconnectiontest.md)
- [KollectSnapshotSink](../crds/kollectsnapshotsink.md) · [KollectDatabaseSink](../crds/kollectdatabasesink.md) · [KollectEventSink](../crds/kollecteventsink.md)
- [Postgres state store](postgres-state-store.md)
- [ADR-0403](../adr/0403-connection-test.md) · [ADR-0414](../adr/0414-sink-family-crds.md)
