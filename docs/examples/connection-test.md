# Example: Connection test

!!! tip "Production default"
    Set `spec.connectionTest: false` on sinks in production (Helm chart default). Use the
    `kollect.dev/test-connection` annotation or a `KollectConnectionTest` CR for on-demand probes.

Kollect offers **three** ways to verify sink connectivity before relying on export
([ADR-0403](../adr/0403-connection-test.md), [ADR-0703](../adr/0703-platform-architecture-pivot.md)).

## Overview

| Mechanism | Best for | Writes to |
| --- | --- | --- |
| `spec.connectionTest: true` | Samples and CI | `KollectSink.status` |
| `kollect.dev/test-connection` annotation | On-demand prod re-test | `KollectSink.status` |
| `KollectConnectionTest` CR | Audited / CI pipelines | `KollectConnectionTest.status` |

**Production default:** `spec.connectionTest: false` (Helm chart default).

## Sink probe — `spec.connectionTest`

`config/samples/kollect_v1alpha1_kollectsink_postgres.yaml` sets `connectionTest: true`.

```sh
kubectl wait --for=condition=ConnectionVerified kollectsink/postgres-inventory-demo \
  -n default --timeout=60s
```

Supported probe types: `git`, `gitlab`, `postgres`, `kafka`, `s3`.

## Annotation re-test

```sh
kubectl annotate kollectsink postgres-inventory-demo -n default \
  kollect.dev/test-connection=true --overwrite
```

## KollectConnectionTest CR

`config/samples/kollect_v1alpha1_kollectconnectiontest.yaml`:

```yaml
spec:
  sinkRef: postgres-inventory-demo
  ownerSink: true
```

```sh
kubectl apply -f config/samples/kollect_v1alpha1_kollectconnectiontest.yaml
kubectl wait --for=condition=ConnectionVerified kollectconnectiontest/postgres-sink-probe \
  -n default --timeout=120s
```

Default `spec.ttlSecondsAfterFinished`: **300** (CR auto-deleted after probe).

## Pipeline reachability

`ConnectionVerified` proves backend credentials/network. `SinkReachable` and `Synced` on
`KollectInventory` prove export path health — see
[Deployment inventory](deployment-inventory.md).

## Related

- [KollectConnectionTest](../crds/kollectconnectiontest.md)
- [Postgres state store](postgres-state-store.md)
- [ADR-0403](../adr/0403-connection-test.md)
