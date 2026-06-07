# FAQ

Symptom-oriented answers for platform operators running **Kollect**. For step-by-step install and
upgrade, see [Operator manual](OPERATOR-MANUAL.md). For pipeline walkthroughs, see
[Examples](examples/README.md).

!!! tip "First checks"
    When export stalls, run `kubectl describe` on the sink and inventory — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

## Installation and upgrades

### Why do CRD schema changes not apply on `helm upgrade`?

Helm installs CRDs from `crds/` on first install but **does not upgrade them** on `helm upgrade`.
Kollect documents an explicit two-step path: `kubectl apply -f dist/install-crds.yaml`, then
`helm upgrade` ([ADR-0704](adr/0704-helm-chart-crd-lifecycle.md),
[Operator manual — Upgrade](operator-manual/upgrading.md)).

### Should I delete CRDs to fix a schema mismatch?

**No.** Deleting a CRD garbage-collects all custom resources. Apply the new CRD bundle instead;
never delete CRDs in production.

### What is the recommended per-team install?

```yaml
tenantMode: true
watchNamespaces:
  - team-a
mode: single
```

See [Operator manual — Watch scope](OPERATOR-MANUAL.md#watch-scope) and
[Multi-tenant watch scope](examples/multi-tenant-watch-namespaces.md).

## Same-namespace references

### Why does my inventory show `SinkNotFound` or `SinkReachable=False`?

`KollectInventory.spec.sinkRefs` must name `KollectSink` objects in the **same namespace** as the
Inventory. Cross-namespace sink refs are not supported for namespaced inventory
([ADR-0201](adr/0201-crd-model.md), [ADR-0201](adr/0201-crd-model.md)).

```sh
kubectl get kollectsink -n <inventory-namespace>
kubectl describe kollectinventory <name> -n <inventory-namespace>
```

The same rule applies to `KollectTarget.spec.profileRef` → `KollectProfile` in the target namespace,
and `KollectConnectionTest.spec.sinkRef` → sink in the test namespace.

!!! warning "Same-namespace sink refs"
    Create sinks in the same namespace as `KollectInventory` before expecting export. Cluster-wide
    rollup uses `KollectClusterInventory` with `spec.sinkNamespace` instead.

### I moved the sink to another namespace — why did export stop?

Update `sinkRefs` on the Inventory to names in the **new** namespace, or recreate the Inventory in
the sink namespace. The operator does not follow cross-namespace sink references for namespaced
inventory.

## SinkReachable and connection conditions

### What is the difference between `ConnectionVerified` and `SinkReachable`?

| Condition | Object | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | `KollectSink` | Last connectivity **probe** succeeded (credentials, TLS, network) |
| `SinkReachable` | `KollectInventory` / `KollectTarget` | Export pipeline can resolve and reach the referenced sink |
| `Synced` | `KollectInventory` / `KollectTarget` | Last export cycle completed successfully |

A sink can show `ConnectionVerified=True` while inventory shows `SinkReachable=False` if the
**name or namespace** in `sinkRefs` is wrong — fix the reference, not just credentials.

### How do I re-test sink connectivity without editing the CR?

Annotate the sink for a one-shot probe ([ADR-0403](adr/0403-connection-test.md)):

```sh
kubectl annotate kollectsink <name> -n <namespace> kollect.dev/test-connection=true --overwrite
kubectl wait --for=condition=ConnectionVerified kollectsink/<name> -n <namespace> --timeout=60s
```

Production manifests should keep `spec.connectionTest: false` and use the annotation for ad-hoc tests.

### Export never runs — what should I check?

| Symptom | Likely cause |
| --- | --- |
| `SinkReachable=False`, reason `SinkNotFound` | `sinkRefs` name or namespace mismatch |
| `SinkReachable=False`, reason `SinkUnreachable` | Backend down, bad DSN, or TLS failure — check `ConnectionVerified` on the sink |
| `ConnectionVerified=False` | Missing `secretRef`, wrong Secret key, or unreachable endpoint |
| `Synced=False` | Prior export failed — see manager logs and `Degraded` condition |
| Empty `status.itemCount` | No resources match target selector, target suspended, or scope denied |

Detailed table: [Deployment inventory — Troubleshooting](examples/deployment-inventory.md#troubleshooting).

## Pre-beta expectations

### Is Kollect safe for production today?

!!! warning "Pre-beta API"
    APIs and defaults may change until the first release candidate. `v1alpha1` has **no conversion
    webhook** — schema changes may require CRD re-apply and CR updates
    ([ADR-0206](adr/0206-api-versioning-conversion.md), [ROADMAP](ROADMAP.md)).

Evaluate against your risk tolerance. Use pinned chart and image versions; read **Unreleased**
notes in `CHANGELOG.md` before upgrading.

### Why did my CR stop working after an upgrade?

Pre-beta CRD fields can change without conversion. After upgrading CRDs (`install-crds.yaml`),
validate sample manifests and `kubectl explain` for renamed or removed fields. Breaking changes use
`feat!:` or `BREAKING CHANGE:` in commit messages ([CONTRIBUTING.md](../CONTRIBUTING.md)).

### Is the export JSON format stable?

Sink payloads and Read API responses are moving toward a versioned envelope — today many exports
emit a bare JSON array ([ADR-0405](adr/0405-export-data-contract.md)). Plan downstream consumers for
possible wrapper fields before `v1.0`.

## Multi-cluster fleet

### How do I inventory many clusters?

**Default path:** run one Kollect operator per cluster with `mode: single` and export to a **shared sink**
(Postgres, Kafka, NATS, Git) with **`spec.cluster`** set on inventory. The sink backend merges rows by
cluster id — no central hub tier
([ADR-0501](adr/0501-multi-cluster-fleet.md), [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md)).

Walkthrough: [Multi-cluster fleet](examples/multi-cluster-fleet.md).

### Is there a `KollectHub` CRD?

**No.** There is no hub/spoke Helm mode — only single-cluster deployment on the operator image
([ADR-0201](adr/0201-crd-model.md)). Register spokes with namespaced
`
### Hub transport is `inprocess` — is that production-ready?

!!! warning "Pre-beta hub transport"
    Hub ingest and spoke push paths are still maturing. `transport.type: inprocess` is the only
    default until external backends pass integration proof. Validate in your environment before
    fleet rollout (ADR-0502).

## Performance and scope

### My operator uses too much memory — what can I tune?

Restrict `watchNamespaces`, use `tenantMode`, narrow `KollectTarget` selectors, and increase
`exportMinInterval` on inventories. See [Performance tuning](PERFORMANCE.md) and
[ADR-0603](adr/0603-performance-scalability.md).

### A namespace is skipped even though a target exists

Check `kollect.dev/namespace-watch: disabled` on the namespace, `kollect.dev/watch: disabled` on
resources, `watchMode: OptIn` without `enabled` labels, or `KollectScope` deny rules
([ADR-0205](adr/0205-watch-labels.md)).

## Related

- [Operator manual](OPERATOR-MANUAL.md)
- [CR reference](CR-REFERENCE.md) · [Error taxonomy](adr/0602-error-taxonomy.md)
- [Connection test](adr/0403-connection-test.md) · [Examples](examples/README.md)
