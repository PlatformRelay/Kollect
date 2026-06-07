# Annotations and labels

Lookup for **Kollect**-owned metadata keys on Kubernetes objects and Helm-managed operator resources.
Constants live in `api/v1alpha1/constants.go`.

!!! tip "Assumptions"
    This reference assumes you understand watch scope and sink probes. For context, read
    [Understand the basics](UNDERSTAND-THE-BASICS.md) and [ADR-0205](adr/0205-watch-labels.md).

## Watch scope

Control which namespaces and resources the collection engine watches
([ADR-0205](adr/0205-watch-labels.md)).

| Key | Type | On | Values | Effect |
| --- | --- | --- | --- | --- |
| `kollect.dev/watch` | Label | Namespace, namespaced resource | `enabled`, `disabled` | Opt in or out a namespace or single resource |
| `kollect.dev/namespace-watch` | Annotation | Namespace | `enabled`, `disabled` | Applies to **all resources** in the namespace unless a resource label overrides |

### Precedence (`ShouldCollect`)

1. Resource label `kollect.dev/watch: disabled` — **always skip** (wins over everything).
2. Resource label `kollect.dev/watch: enabled` — **collect** (overrides namespace disabled).
3. Namespace label `kollect.dev/watch: disabled` or annotation `kollect.dev/namespace-watch: disabled` — skip all resources in namespace (unless step 2).
4. `KollectTarget.spec.watchMode: OptIn` — require namespace or resource `enabled`; otherwise skip.
5. `watchMode: All` (default) — collect when selectors match and no opt-out applies.

!!! note "Interaction with `watchMode`"
    Under `OptIn`, only explicitly `enabled` namespaces or resources are collected. Under `All`,
    matching selectors are collected except where `disabled` applies. See
    [Multi-tenant watch scope](examples/multi-tenant-watch-namespaces.md).

### Examples

**Opt-out a noisy namespace** (default `All` mode):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kube-system
  annotations:
    kollect.dev/namespace-watch: disabled
```

**Opt-in cluster** (`watchMode: OptIn` on the target):

```sh
kubectl label namespace team-a kollect.dev/watch=enabled --overwrite
```

**Opt-out one Deployment** in an otherwise watched namespace:

```yaml
metadata:
  labels:
    kollect.dev/watch: disabled
```

## Connection test

| Key | Type | On | Values | Effect |
| --- | --- | --- | --- | --- |
| `kollect.dev/test-connection` | Annotation | `KollectSink` | `"true"` | One-shot connectivity probe; sets `ConnectionVerified` on status |

Equivalent to `spec.connectionTest: true` on the sink CR ([ADR-0403](adr/0403-connection-test.md)).
The reconciler removes the annotation after a successful probe (kept when the probe fails).

!!! warning "Production probes"
    Keep `spec.connectionTest: false` in Git-managed manifests. Use the annotation for ad-hoc
    re-tests:

```sh
kubectl annotate kollectsink <name> -n <namespace> \
  kollect.dev/test-connection=true --overwrite
kubectl wait --for=condition=ConnectionVerified kollectsink/<name> \
  -n <namespace> --timeout=60s
```

See [Connection test example](examples/connection-test.md).

## Multi-cluster identity

Fleet installs distinguish clusters via **`spec.cluster`** on inventory and export rows
([ADR-0501](adr/0501-multi-cluster-fleet.md)). Remote-cluster registration labels and hub ingest
headers are **not** used in the default architecture.


## Profile and export metadata

| Key | Type | On | Values | Effect |
| --- | --- | --- | --- | --- |
| `kollect.dev/allow-secret-extraction` | Annotation | `KollectProfile`, `KollectClusterProfile` | `"true"` | Admission allows CEL/JSONPath paths into `Secret.data` |
| `kollect.dev/collectedGeneration` | Annotation | Exported source objects (metadata) | `"<n>"` | Records source `metadata.generation` for staleness detection |
| `kollect.dev/requestedAt` | Annotation | Reconciled Kollect CRs | RFC3339 timestamp | Manual reconcile trigger ([ADR-0201](adr/0201-crd-model.md)) |

!!! warning "Secret extraction"
    Profiles that read `Secret.data` require explicit opt-in via `kollect.dev/allow-secret-extraction: "true"`.
    Prefer indirect references (e.g. cert-manager status) when possible
    ([KollectClusterProfile](crds/kollectclusterprofile.md)).

## Export payload spill

Not an annotation — operator policy for marshalled inventory size
([ADR-0103](adr/0103-etcd-limit.md), [KollectSink spill](crds/kollectsink.md#export-payload-spill)):

| Signal | When | Meaning |
| --- | --- | --- |
| Log `export payload exceeds spill warn threshold` | Payload ≥ **1 MiB** | Approaching mandatory object-store spill |
| `kollect_export_spill_warn_total` | Payload ≥ **1 MiB** | Counter — tune targets or add S3/GCS before hard block |
| Inventory `Degraded` `SpillRequired` | Payload > **1 MiB**, no `s3`/`gcs` in `sinkRefs` | Add object-store sink or reduce payload |
| Inventory `Degraded` `PayloadTooLarge` | Payload > **`maxExportBytes`** (~1.5 MiB default) | Split targets, trim attributes, or raise cap within global limit |
| `kollect_sink_errors_total{reason="spill_required"}` | Spill gate blocked export | Same remediation as `SpillRequired` |

`KollectSink.spec.pathTemplate` controls where spill payloads land in Git/S3/GCS (not related to watch
labels above).

## Tenant and example labels

Sample manifests and e2e fixtures use conventional labels — not enforced by the operator unless
referenced in target selectors:

| Key | Example value | Usage |
| --- | --- | --- |
| `kollect.dev/tenant` | `platform`, `team-a` | Tenant isolation in samples and cluster rollup selectors |
| `kollect.dev/collect-certificates` | `enabled` | cert-manager example target selector |

## Helm chart labels

Standard labels on operator Deployments, Services, and webhooks
(`charts/kollect/templates/_helpers.tpl`):

| Label | Value | Meaning |
| --- | --- | --- |
| `helm.sh/chart` | `kollect-<version>` | Chart identity |
| `app.kubernetes.io/name` | `kollect` | Application name |
| `app.kubernetes.io/version` | Chart `AppVersion` | Operator version |
| `app.kubernetes.io/managed-by` | Helm release service | Managed by Helm |
| `control-plane` | `controller-manager` | Selector for manager pods |

List operator pods:

```sh
kubectl get pods -n kollect-system -l app.kubernetes.io/name=kollect
```

## Related

- [ADR-0205: Watch labels](adr/0205-watch-labels.md)
- [ADR-0403: Connection test](adr/0403-connection-test.md)
- ADR-0503: Hub cluster auth
- [Operator manual — Watch scope](OPERATOR-MANUAL.md#watch-scope)
- [Troubleshooting](TROUBLESHOOTING.md) · [Best practices](BEST-PRACTICES.md)
