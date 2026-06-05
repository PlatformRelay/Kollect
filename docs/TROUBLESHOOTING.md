# Troubleshooting

Central hub for diagnosing **Kollect** export, collection, and sink issues. Start with the condition
catalog below, then follow links to symptom-specific guides.

!!! tip "First checks"
    Run `kubectl describe` on the sink and inventory when export stalls — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

!!! note "Related guides"
    Symptom Q&A: [FAQ](FAQ.md). Step-by-step install and upgrade:
    [Operator manual](OPERATOR-MANUAL.md). Per-scenario walkthroughs:
    [Examples](examples/README.md).

## Condition catalog

Kollect follows Kubernetes condition conventions (`Ready`, `Synced`, `Degraded`) with sink-specific
types on static kinds. See [ADR-0602](adr/0602-error-taxonomy.md) for reconcile behavior.

### By kind

| Kind | Conditions | When to inspect |
| --- | --- | --- |
| [`KollectSink`](crds/kollectsink.md) | `ConnectionVerified`, `TLSInsecure`, `Degraded` | Before export; credential or endpoint problems |
| [`KollectTarget`](crds/kollecttarget.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Collection stalled or scope denied |
| [`KollectInventory`](crds/kollectinventory.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Export not running or payload errors |
| [`KollectConnectionTest`](crds/kollectconnectiontest.md) | `ConnectionVerified`, `Ready` | Audited composite probes |

Static kinds (`KollectProfile`, `KollectScope`) do **not** set `Ready` — admission webhooks and
events surface validation errors instead.

### Cross-object conditions

| Condition | Object | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | `KollectSink` | Last connectivity **probe** succeeded (credentials, TLS, network) |
| `SinkReachable` | `KollectInventory`, `KollectTarget` | Export pipeline resolved and can reach the referenced sink |
| `Synced` | `KollectInventory`, `KollectTarget` | Last export or collection cycle completed successfully |
| `Degraded` | Reconciled kinds | Hard block — fix `reason` before expecting progress |

A sink can show `ConnectionVerified=True` while inventory shows `SinkReachable=False` if the **name
or namespace** in `sinkRefs` is wrong — fix the reference, not just credentials.

### Common `Degraded` reasons

| Reason | Typical object | Cause | Fix |
| --- | --- | --- | --- |
| `SinkNotFound` | Inventory, Target | Typo or wrong namespace in `sinkRefs` | Match exact sink name in **same namespace** |
| `SinkUnreachable` | Inventory, Target | `ConnectionVerified=False` on sink | Fix Secret, DSN, network; re-probe sink |
| `ScopeSinkDenied` | Inventory | Sink not in `KollectScope` allow-list | Add sink to `spec.sinkRefs` on scope |
| `ScopeGVKDenied` | Target | GVK blocked by scope | Update `KollectScope.spec.allowedGVKs` |
| `ScopeNamespaceDenied` | Target | Workload namespace blocked | Add to `allowedNamespaces` |
| `ProfileNotFound` | Target | Missing `KollectProfile` | Apply profile in same namespace as target |
| `PayloadTooLarge` | Inventory | Exceeds `maxExportBytes` | Split targets or trim attributes |
| `ExportTerminal` | Inventory | Non-retryable sink error | Fix sink config; check operator logs |
| `Suspended` | Target, Inventory | `spec.suspend: true` | Set `suspend: false` |
| `Progressing` | Inventory | Transient network or 429 | Usually self-heals; inspect metrics |

Full per-kind tables: [KollectInventory](crds/kollectinventory.md#status-conditions),
[KollectTarget](crds/kollecttarget.md#status-conditions),
[KollectSink](crds/kollectsink.md#status-conditions).

## Symptom → cause quick reference

| Symptom | Likely cause | Next step |
| --- | --- | --- |
| Export never runs | `SinkReachable=False` (`SinkNotFound` / `SinkUnreachable`) | [FAQ — export never runs](FAQ.md#export-never-runs--what-should-i-check) |
| `ConnectionVerified=False` | Missing Secret, bad DSN, TLS failure | [Connection test](examples/connection-test.md) |
| Empty `status.itemCount` | Selector mismatch, suspended target, scope denied | [Deployment inventory — Troubleshooting](examples/deployment-inventory.md#troubleshooting) |
| Namespace skipped | Watch label or `OptIn` without `enabled` | [Annotations and labels](ANNOTATIONS-LABELS.md) |
| Postgres rows stale | Upsert-only drift or export error | [Postgres state store](examples/postgres-state-store.md#troubleshooting) |
| Hub spoke not merging | Transport or registration misconfig | [Hub mode](examples/hub-mode.md) |
| CR stopped working after upgrade | Pre-beta schema change | [FAQ — pre-beta](FAQ.md#why-did-my-cr-stop-working-after-an-upgrade) |

## Diagnostic commands

```sh
# Pipeline status (short names)
kubectl get kprof,ksink,ktgt,kinv -n <namespace>
kubectl describe kollectsink <name> -n <namespace>
kubectl describe kollectinventory <name> -n <namespace>

# Wait for sink probe
kubectl wait --for=condition=ConnectionVerified kollectsink/<name> \
  -n <namespace> --timeout=60s

# Re-probe without editing spec
kubectl annotate kollectsink <name> -n <namespace> \
  kollect.dev/test-connection=true --overwrite

# Operator logs
kubectl -n kollect-system logs deployment/kollect-controller-manager -f --tail=200
```

More shortcuts: [Command reference](COMMAND-REFERENCE.md).

## Example troubleshooting guides

| Scenario | Guide |
| --- | --- |
| First inventory pipeline on kind | [Deployment inventory](examples/deployment-inventory.md#troubleshooting) |
| Postgres DSN and delete reconciliation | [Postgres state store](examples/postgres-state-store.md#troubleshooting) |
| Helm / Argo Application attributes | [Helm release inventory](examples/helm-release-inventory.md#troubleshooting) |
| Sink connectivity probes | [Connection test](examples/connection-test.md) |
| Multi-tenant watch scope | [Multi-tenant watch namespaces](examples/multi-tenant-watch-namespaces.md) |
| Spoke → shared sink | [Spoke cluster inventory](examples/spoke-cluster-inventory.md) |
| Hub aggregation | [Hub mode](examples/hub-mode.md) |

## When to escalate

!!! warning "Pre-beta API"
    `v1alpha1` fields may change without conversion webhook. Check [ROADMAP](ROADMAP.md) before
    production use.

1. Collect `kubectl describe` output for sink, target, and inventory.
2. Capture operator logs (sanitize Secrets before sharing).
3. Note Helm `mode`, `tenantMode`, and `watchNamespaces` values.
4. Open a GitHub issue with repro steps and condition JSON from `status.conditions`.

## Related

- [FAQ](FAQ.md) · [Error taxonomy](adr/0602-error-taxonomy.md)
- [CR reference](CR-REFERENCE.md) · [Performance tuning](PERFORMANCE.md)
- [Operator manual](OPERATOR-MANUAL.md) · [Best practices](BEST-PRACTICES.md)
