# Custom resource reference

Detailed reference for each kollect API kind. These pages document **spec fields**, **status
conditions**, **RBAC**, **samples**, and **failure modes** for operators and platform teams.

For architecture context see [ARCHITECTURE.md](ARCHITECTURE.md) and [DATA-FLOWS.md](DATA-FLOWS.md).
Locked product decisions: [PLATFORM-DECISIONS.md](PLATFORM-DECISIONS.md).

## Kinds

| Kind | Scope | Reconciled | Reference |
| --- | --- | --- | --- |
| `KollectProfile` | Namespace | No | [crds/kollectprofile.md](crds/kollectprofile.md) |
| `KollectSink` | Namespace | Probe only | [crds/kollectsink.md](crds/kollectsink.md) |
| `KollectTarget` | Namespace | Yes | [crds/kollecttarget.md](crds/kollecttarget.md) |
| `KollectInventory` | Namespace | Yes | [crds/kollectinventory.md](crds/kollectinventory.md) |
| `KollectScope` | Namespace | No | [crds/kollectscope.md](crds/kollectscope.md) |
| `KollectConnectionTest` | Namespace | Yes | [crds/kollectconnectiontest.md](crds/kollectconnectiontest.md) |
| `KollectClusterTarget` | Cluster | Webhook only (Phase 1) | [crds/kollectclustertarget.md](crds/kollectclustertarget.md) |

## Reserved kinds (stubs pending)

| Kind | Scope | Notes |
| --- | --- | --- |
| `KollectClusterProfile` | Cluster | Platform extraction schemas |
| `KollectClusterSink` | Cluster | Shared export backends |
| `KollectClusterInventory` | Cluster | Platform rollup — pairs with `KollectClusterTarget` |
| `KollectClusterScope` | Cluster | Platform policy boundary |
| `KollectRemoteCluster` | Namespace | Hub spoke registration ([ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md)) |

## Short names

| Kind | Short name |
| --- | --- |
| `KollectInventory` | `kinv` |
| `KollectTarget` | `ktgt` |
| `KollectClusterTarget` | `kctgt` |
| `KollectConnectionTest` | `kconntest` |
| `KollectScope` | `kscope` |
