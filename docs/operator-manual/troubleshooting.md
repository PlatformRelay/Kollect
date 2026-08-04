# Troubleshooting

Central hub for diagnosing **Kollect** export, collection, and sink issues. Start with the condition
catalog below, then follow links to symptom-specific guides.

!!! tip "First checks"
    Run `kubectl describe` on the sink and inventory when export stalls — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

!!! note "Related guides"
    Symptom Q&A: [FAQ](troubleshooting.md). Step-by-step install and upgrade:
    [Operator manual](index.md). Per-scenario walkthroughs:
    [Examples](../examples/README.md).

## Condition catalog

Kollect follows Kubernetes condition conventions (`Ready`, `Synced`, `Degraded`) with sink-specific
types on static kinds. See [ADR-0602](../adr/0602-error-taxonomy.md) for reconcile behavior.

### By kind

| Kind | Conditions | When to inspect |
| --- | --- | --- |
| Family sinks ([`KollectSnapshotSink`](../crds/kollectsnapshotsink.md), [`KollectDatabaseSink`](../crds/kollectdatabasesink.md), [`KollectEventSink`](../crds/kollecteventsink.md)) | `ConnectionVerified`, `TLSInsecure`, `Degraded` | Before export; credential or endpoint problems |
| [`KollectTarget`](../crds/kollecttarget.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Collection stalled or scope denied |
| [`KollectInventory`](../crds/kollectinventory.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Export not running or payload errors |
| [`KollectConnectionTest`](../crds/kollectconnectiontest.md) | `ConnectionVerified`, `Ready` | Audited composite probes |

Static kinds (`KollectProfile`, `KollectScope`) do **not** set `Ready` — admission webhooks and
events surface validation errors instead.

### Cross-object conditions

| Condition | Object | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | Family sinks (`KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink`) | Last connectivity **probe** succeeded (credentials, TLS, network) |
| `SinkReachable` | `KollectInventory`, `KollectTarget` | Export pipeline resolved and can reach the referenced sink |
| `Synced` | `KollectInventory`, `KollectTarget` | Last export or collection cycle completed successfully |
| `Degraded` | Reconciled kinds | Hard block — fix `reason` before expecting progress |

A sink can show `ConnectionVerified=True` while inventory shows `SinkReachable=False` if the **name
or namespace** in a `*SinkRefs` entry is wrong — fix the reference, not just credentials.

### Common `Degraded` reasons

| Reason | Typical object | Cause | Fix |
| --- | --- | --- | --- |
| `SinkNotFound` | Inventory, Target | Typo or wrong namespace in `*SinkRefs` | Match exact sink name in **same namespace** |
| `SinkUnreachable` | Inventory, Target | `ConnectionVerified=False` on sink | Fix Secret, DSN, network; re-probe sink |
| `ScopeSinkDenied` | Inventory | Sink not in `KollectScope` allow-list | Add sink to scope allow-list refs |
| `ScopeGVKDenied` | Target | GVK blocked by scope | Update `KollectScope.spec.allowedGVKs` |
| `ScopeNamespaceDenied` | Target | Workload namespace blocked | Add to `allowedNamespaces` |
| `ProfileNotFound` | Target | Missing `KollectProfile` | Apply profile in same namespace as target |
| `PayloadTooLarge` | Inventory | Exceeds `maxExportBytes` | Split targets or trim attributes |
| `ExportTerminal` | Inventory | Non-retryable sink error | Fix sink config; check operator logs |
| `Suspended` | Target, Inventory | `spec.suspend: true` | Set `suspend: false` |
| `Progressing` | Inventory | Transient network or 429 | Usually self-heals; inspect metrics |

Full per-kind tables: [KollectInventory](../crds/kollectinventory.md#status-conditions),
[KollectTarget](../crds/kollecttarget.md#status-conditions),
[KollectSnapshotSink](../crds/kollectsnapshotsink.md#status).

## Symptom → cause quick reference

| Symptom | Likely cause | Next step |
| --- | --- | --- |
| Export never runs | `SinkReachable=False` (`SinkNotFound` / `SinkUnreachable`) | Resolve the family sink in the inventory namespace and inspect Events. |
| `ConnectionVerified=False` | Missing Secret, bad DSN, TLS failure | [Connection test](../examples/connection-test.md) |
| Empty `status.itemCount` | Selector mismatch, suspended target, scope denied | [First inventory](../getting-started/first-inventory.md#if-it-didnt-work) |
| Namespace skipped | Watch label or `OptIn` without `enabled` | [Annotations and labels](../ANNOTATIONS-LABELS.md) |
| Postgres rows stale | Upsert-only drift or export error | [Postgres state store](../examples/postgres-state-store.md#troubleshooting) |
| Fleet rows missing a cluster | Wrong or empty `spec.cluster` on inventory | [Multi-cluster fleet](../examples/multi-cluster-fleet.md) |
| CR stopped working after upgrade | Pre-beta schema change | [Upgrading](upgrading.md) |

## Diagnostic commands

```sh
# Pipeline status (short names — family sinks: ksnap/kdb/kevt)
kubectl get kprof,ksnap,kdb,kevt,ktgt,kinv -n <namespace>
kubectl describe kollectsnapshotsink <name> -n <namespace>
kubectl describe kollectdatabasesink <name> -n <namespace>
kubectl describe kollectinventory <name> -n <namespace>

# Wait for sink probe (pick the family kind you installed)
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/<name> \
  -n <namespace> --timeout=60s

# Re-probe without editing spec
kubectl annotate kollectdatabasesink <name> -n <namespace> \
  kollect.dev/test-connection=true --overwrite

# Operator logs
kubectl -n kollect-system logs deployment/kollect-controller-manager -f --tail=200
```

More shortcuts: [Command reference](../COMMAND-REFERENCE.md).

## Example troubleshooting guides

| Scenario | Guide |
| --- | --- |
| First inventory pipeline on kind | [First inventory](../getting-started/first-inventory.md#if-it-didnt-work) |
| Postgres DSN and delete reconciliation | [Postgres state store](../examples/postgres-state-store.md#troubleshooting) |
| Helm / Argo Application attributes | [Helm release inventory](../examples/helm-release-inventory.md#troubleshooting) |
| Sink connectivity probes | [Connection test](../examples/connection-test.md) |
| Multi-tenant watch scope | [Multi-tenant watch namespaces](../examples/multi-tenant-watch-namespaces.md) |
| Fleet rows / shared sink | [Multi-cluster fleet](../examples/multi-cluster-fleet.md) |
| Missing `spec.cluster` partitioning | [Multi-cluster fleet](../examples/multi-cluster-fleet.md) |

## When to escalate

!!! warning "Pre-beta API"
    `v1alpha1` fields may change without conversion webhook. Check [ROADMAP](../ROADMAP.md) before
    production use.

1. Collect `kubectl describe` output for sink, target, and inventory.
2. Capture operator logs (sanitize Secrets before sharing).
3. Note Helm `mode`, `tenantMode`, and `watchNamespaces` values.
4. Open a GitHub issue with repro steps and condition JSON from `status.conditions`.

## Common questions

**Should I delete CRDs to fix an upgrade?** No. Apply the release CRDs before upgrading the
controller; deleting them also deletes custom resources. Follow [Upgrading](upgrading.md).

**Why did moving a sink break export?** Namespaced inventories resolve family sinks in their own
namespace. Move or recreate the reference together, or use an appropriate cluster resource.

**Does `exportMinInterval` delay every change?** It coalesces changes since the last successful
export. The effective interval follows the precedence described in
[Export pipeline and debouncing](../concepts/export-pipeline.md).

**Is there a fleet hub?** No. Each cluster operator writes a cluster-partitioned record to a shared
sink; see [Multi-cluster fleet](../concepts/multi-cluster.md).

## FAQ links

- [Error taxonomy](../adr/0602-error-taxonomy.md)
- [CR reference](../crds/index.md) · [Performance tuning](performance.md)
- [Operator manual](index.md) · [Production checklist](production-checklist.md)

---

<!-- Consolidated from the former docs/FAQ.md page. -->

Symptom-oriented answers for platform operators running **Kollect**. For step-by-step install and
upgrade, see [Operator manual](index.md). For pipeline walkthroughs, see
[Examples](../examples/README.md).

!!! tip "First checks"
    When export stalls, run `kubectl describe` on the sink and inventory — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

## Installation and upgrades

### Why do CRD schema changes not apply on `helm upgrade`?

Helm installs CRDs from `crds/` on first install but **does not upgrade them** on `helm upgrade`.
Kollect documents an explicit two-step path: `kubectl apply -f dist/install-crds.yaml`, then
`helm upgrade` ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md),
[Operator manual — Upgrade](upgrading.md)).

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

See [Operator manual — Watch scope](index.md#watch-scope) and
[Multi-tenant watch scope](../examples/multi-tenant-watch-namespaces.md).

## Same-namespace references

### Why does my inventory show `SinkNotFound` or `SinkReachable=False`?

`KollectInventory` binds sinks via typed lists — `spec.snapshotSinkRefs`,
`spec.databaseSinkRefs`, and/or `spec.eventSinkRefs` — naming family sink objects in the **same
namespace** as the Inventory. Cross-namespace sink refs are not supported for namespaced inventory
([ADR-0201](../adr/0201-crd-model.md), [ADR-0414](../adr/0414-sink-family-crds.md)).

```sh
kubectl get ksnap,kdb,kevt -n <inventory-namespace>
kubectl describe kollectinventory <name> -n <inventory-namespace>
```

The same rule applies to `KollectTarget.spec.profileRef` → `KollectProfile` in the target namespace,
and `KollectConnectionTest.spec.sinkRef` → a family sink in the test namespace.

!!! warning "Same-namespace sink refs"
    Create family sinks in the same namespace as `KollectInventory` before expecting export.
    Cluster-wide rollup uses `KollectClusterInventory` with `spec.sinkNamespace` instead.

### I moved the sink to another namespace — why did export stop?

Update the matching `*SinkRefs` list on the Inventory to names in the **new** namespace, or recreate
the Inventory in the sink namespace. The operator does not follow cross-namespace sink references for
namespaced inventory.

## SinkReachable and connection conditions

### What is the difference between `ConnectionVerified` and `SinkReachable`?

| Condition | Object | Meaning |
| --- | --- | --- |
| `ConnectionVerified` | Family sinks (`KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink`) | Last connectivity **probe** succeeded (credentials, TLS, network) |
| `SinkReachable` | `KollectInventory` / `KollectTarget` | Export pipeline can resolve and reach the referenced sink |
| `Synced` | `KollectInventory` / `KollectTarget` | Last export cycle completed successfully |

A sink can show `ConnectionVerified=True` while inventory shows `SinkReachable=False` if the
**name or namespace** in a `*SinkRefs` entry is wrong — fix the reference, not just credentials.

### How do I re-test sink connectivity without editing the CR?

Annotate the family sink for a one-shot probe ([ADR-0403](../adr/0403-connection-test.md)):

```sh
kubectl annotate kollectdatabasesink <name> -n <namespace> kollect.dev/test-connection=true --overwrite
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/<name> -n <namespace> --timeout=60s
```

Production manifests should keep `spec.connectionTest: false` and use the annotation for ad-hoc tests.

### Export never runs — what should I check?

| Symptom | Likely cause |
| --- | --- |
| `SinkReachable=False`, reason `SinkNotFound` | `*SinkRefs` name or namespace mismatch |
| `SinkReachable=False`, reason `SinkUnreachable` | Backend down, bad DSN, or TLS failure — check `ConnectionVerified` on the sink |
| `ConnectionVerified=False` | Missing `secretRef`, wrong Secret key, or unreachable endpoint |
| `Synced=False` | Prior export failed — see manager logs and `Degraded` condition |
| Empty `status.itemCount` | No resources match target selector, target suspended, or scope denied |

Detailed table: [Deployment inventory — Troubleshooting](../getting-started/first-inventory.md#if-it-didnt-work).

### Does `exportMinInterval` delay exports after a change?

**No.** The interval debounces re-export of an **identical payload** only. A material change
(payload checksum or `metadata.generation` bump) exports immediately per sink, regardless of the
configured interval. Set `exportMinInterval: 0s` for *material-change only* semantics (typical for
Kafka/NATS event sinks): instant export on change, identical payloads never re-sent. `0` and
sub-second durations are valid (cap: 24h), but wake-ups floor at 1s, so anything below `1s` behaves
like `0s`. See [DATA-FLOWS §1](../concepts/export-pipeline.md#debounce-and-interval-precedence) and
[ADR-0413](../adr/0413-export-interval-scheduling.md).

### How do I confirm a multipart YAML export set is complete (not torn or stale)?

A Git/GitLab snapshot that exceeds `maxExportBytes` is sharded into several parts written one after
another. The human-readable YAML data files carry no envelope metadata, so completeness is confirmed
from the **per-set manifest sidecar** at `inventory/<namespace>/<name>.manifest.json`
([ADR-0405](../adr/0405-export-data-contract.md)). A single-part export writes **no** sidecar and is
complete on its own.

Read the sidecar and apply two checks:

```bash
# 1) Fetch the manifest for the export set.
cat inventory/default/partitioned-inventory.manifest.json
# {
#   "kind": "KollectExportSetManifest",
#   "generation": 7,
#   "partTotal": 3,
#   "parts": [1, 2, 3],
#   "paths": ["default/team-a/Deployment/api.yaml", "…/web.yaml", "…/worker.yaml"]
# }

# 2) COMPLETE-check: every path in "paths" must exist on disk. Any missing path = a torn set
#    (a part failed to persist). This prints the paths that are MISSING (empty output = complete):
jq -r '.paths[]' inventory/default/partitioned-inventory.manifest.json \
  | while read -r p; do [ -f "$p" ] || echo "MISSING: $p"; done
```

- **Torn set** — a path in `paths` is missing on disk: an earlier part failed before the set finished.
  The manifest is only written on the successful final part, so **no manifest at all** beside part files
  is itself a torn set (the run never completed).
- **Stale set** — the manifest's `generation` does not match the generation you expect (from the
  inventory's `status` / the commit metadata). A torn re-export can leave a prior-generation manifest in
  place; compare `generation` before trusting the set.

`generation` is uniform across a healthy set, `partTotal` is the expected part count, and `parts` lists
the per-part identifiers a complete set must hold. Writer/verifier split: the **controller only writes**
the manifest (it does not read it back), so torn-set detection is the **consumer's** responsibility —
run the checks above before trusting a set. `layout.VerifySet` is a helper you can vendor into a Go
consumer to apply exactly this rule; the operator equivalent is the `jq` snippet above.

## Pre-beta expectations

### Is Kollect safe for production today?

!!! warning "Pre-beta API"
    APIs and defaults may change until the first release candidate. `v1alpha1` has **no conversion
    webhook** — schema changes may require CRD re-apply and CR updates
    ([ADR-0206](../adr/0206-api-versioning-conversion.md), [ROADMAP](../ROADMAP.md)).

Evaluate against your risk tolerance. Use pinned chart and image versions; read **Unreleased**
notes in `CHANGELOG.md` before upgrading.

### Why did my CR stop working after an upgrade?

Pre-beta CRD fields can change without conversion. After upgrading CRDs (`install-crds.yaml`),
validate sample manifests and `kubectl explain` for renamed or removed fields. Breaking changes use
`feat!:` or `BREAKING CHANGE:` in commit messages ([CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md)).

### Is the export JSON format stable?

Sink payloads and Read API responses are moving toward a versioned envelope — today many exports
emit a bare JSON array ([ADR-0405](../adr/0405-export-data-contract.md)). Plan downstream consumers for
possible wrapper fields before `v1.0`.

## Multi-cluster fleet

### How do I inventory many clusters?

**Default path:** run one Kollect operator per cluster with `mode: single` and export to a **shared sink**
(Postgres, Kafka, NATS, Git) with **`spec.cluster`** set on inventory. The sink backend merges rows by
cluster id — no central hub tier
([ADR-0501](../adr/0501-multi-cluster-fleet.md), [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)).

Walkthrough: [Multi-cluster fleet](../examples/multi-cluster-fleet.md).

### Is there a `KollectHub` CRD?

**No.** Hub/spoke runtime was removed in v0.3. Multi-cluster uses **N single-mode operators** exporting
to a shared sink with `spec.cluster` ([ADR-0501](../adr/0501-multi-cluster-fleet.md)).

## Performance and scope

### My operator uses too much memory — what can I tune?

Restrict `watchNamespaces`, use `tenantMode`, narrow `KollectTarget` selectors, and increase
`exportMinInterval` on inventories. See [Performance tuning](performance.md) and
[ADR-0603](../adr/0603-performance-scalability.md).

### A namespace is skipped even though a target exists

Check `kollect.dev/namespace-watch: disabled` on the namespace, `kollect.dev/watch: disabled` on
resources, `watchMode: OptIn` without `enabled` labels, or `KollectScope` deny rules
([ADR-0205](../adr/0205-watch-labels.md)).

## Related

- [Operator manual](index.md)
- [Common errors](troubleshooting.md) — full catalog: conditions, metrics, and fixes
- [CR reference](../crds/index.md) · [Error taxonomy](../adr/0602-error-taxonomy.md)
- [Connection test](../adr/0403-connection-test.md) · [Examples](../examples/README.md)

---

<!-- Consolidated from the former docs/operator-manual/troubleshooting.md page. -->

Symptom-oriented catalog for production failures in **Kollect** reconcilers, collection, and export.
For error-class semantics and reconcile behavior, see [ADR-0602: Error taxonomy](../adr/0602-error-taxonomy.md)
— this page focuses on **what operators see** and **what to do**.

!!! note "No hub/spoke runtime"
    Hub/spoke ingest was removed in v0.3. Multi-cluster uses **N single-mode operators** exporting to a
    shared sink with `spec.cluster`. There are **no** hub-specific conditions, metrics, or failure modes
    in current releases.

## 1. How errors surface

Kollect reports failures through four channels. Start with **conditions** on the CR that owns the
pipeline; use metrics and logs when status is stale or ambiguous.

### Conditions

| Condition | Typical objects | Meaning |
| --- | --- | --- |
| `Ready` | `KollectTarget`, `KollectInventory`, `KollectClusterInventory` | Pipeline healthy enough to collect or export |
| `Synced` | Inventory / cluster inventory | Last export cycle outcome (aggregate across sinks) |
| `PartiallySynced` | Inventory (`Ready` or `Synced` **reason**) | Some sinks exported; others debounced or failed |
| `Degraded` | Target, inventory, sink family CRs | Terminal misconfig or export gate — **spec change required** |
| `ExportShardWarning` | `KollectInventory` | Namespace aggregate ≥ ~1,800 rows — split before cap |
| `SinkReachable` | Inventory / target | Sink ref resolves and backend reachable |
| `ConnectionVerified` | `KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink` | Last connectivity probe succeeded |

Per-sink detail lives in `status.sinkExports[]` — each entry has its own `Synced` condition and
`lastExportTime`.

```sh
kubectl describe kollectinventory <name> -n <ns>
kubectl describe kollecttarget <name> -n <ns>
kubectl describe kollectsnapshotsink <name> -n <ns>   # or databasesink / eventsink
kubectl get events -n <ns> --field-selector involvedObject.name=<name>
```

### Events

Warning events carry stable **reason** enums (not free-form types). Common reasons:
`ScopeGVKDenied`, `PayloadTooLarge`, `ExportFailed`, `Progressing`, `ConnectionTestFailed`,
`ReconcilePanic`.

### Metrics

| Metric | Labels | Use |
| --- | --- | --- |
| `kollect_reconcile_errors_total` | `kind`, `error_class` | Reconcile failures: `transient`, `terminal`, `forbidden` |
| `kollect_sink_errors_total` | `reason` | Export failures — **separate** from reconcile errors |
| `kollect_sink_connection_test_total` | `type`, `result` | Probe outcomes per sink family |
| `kollect_export_duration_seconds` | `sink_type` | Slow exports (Git clone, Postgres bulk, etc.) |
| `kollect_workqueue_depth` | `controller` | Reconcile backlog / conflict storms |

Sink error `reason` values include: `transient`, `terminal`, `forbidden`, `payload_too_large`,
`spill_required`, `unknown`.

Full catalog: [Operator metrics](metrics.md).

### Inventory `status.sinkExports[]`

Each bound sink (`snapshotSinkRefs`, `databaseSinkRefs`, `eventSinkRefs`) gets a status slice entry:

| Per-sink `Synced` | Operator read |
| --- | --- |
| `True`, reason `Exported` | Last attempt succeeded |
| `False`, reason `Debounced` | **Not a failure** — cadence/coalesce skipped write |
| `False`, reason `ExportFailed` | Export attempt failed — read `message` |

Read API mirrors this as `debounced` vs `degraded` per sink ([metrics note](metrics.md)).

---

## 2. Error classes (ADR-0602)

| Class | Meaning | Reconcile | Self-heal? |
| --- | --- | --- | --- |
| **Transient** | Network blip, 429, API conflict, sink timeout, circuit breaker open | Requeue with backoff; `Synced=False`, reason `Progressing` | **Yes** — when root cause clears |
| **Terminal** | Bad config, invalid extraction path, auth permanently wrong, payload over cap | **No requeue**; `Degraded=True` + Warning event | **No** — fix spec/credentials, then observe new generation |
| **Forbidden** | SAR/RBAC denied for list/watch on a namespace/GVK | Degrade scope; partial collection; metric `error_class=forbidden` | **Partial** — grant RBAC or narrow selectors |

Details, examples, and circuit-breaker rules: [ADR-0602](../adr/0602-error-taxonomy.md).

---

## 3. Catalog by symptom

Each row: what you see → likely cause → how to confirm → fix → escalate when stuck.

### Scope and tenancy

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Target `Degraded`, reason `ScopeGVKDenied` | `KollectScope` allow-list excludes profile GVK | Event reason; scope CR in same namespace | Add GVK to scope or remove scope binding | Platform team owns scope policy |
| Inventory `Degraded`, reason `ScopeSinkDenied` | Scope disallows snapshot/database/event family ref | `kubectl describe kollectinventory`; scope spec | Use allowed sink family/type or widen scope | Same |
| Target `Degraded`, reason `ScopeNamespaceDenied` | Target/intent namespaces outside scope | Event + target spec `namespaceSelector` | Fix selectors or scope `allowedNamespaces` | Same |
| Target `Degraded`, reason `Forbidden` | SAR denied for list in workload namespace | `kollect_reconcile_errors_total{error_class="forbidden"}`; target message cites namespace/GVK | Grant operator Role/ClusterRole list/watch on GVR; or narrow target to permitted namespaces | RBAC audit / cluster admin |
| Namespace empty in inventory but target exists | `namespaceSelector` mismatch, watch opt-out, or scope deny | Target `Ready`; `status.itemCount=0`; labels `kollect.dev/watch` | Align selector with workloads; check [watch labels](../adr/0205-watch-labels.md) | If SAR OK but still empty — profile/GVK issue |

### Collection

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Target `Degraded`, `ProfileNotFound` | Wrong `profileRef` or cross-namespace ref | `kubectl get kollectprofile -n <target-ns>` | Create profile in **same namespace** as target | — |
| Target `Degraded`, `InformerRegistrationFailed` | Unknown/uninstalled GVK, CRD missing | Target message; apiserver discovery | Install CRD/API; fix `KollectProfile.spec.targetGVK` | Vendor CRD not on cluster |
| Target `Degraded`, `AccessCheckFailed` | SAR API error (not denial) during list pre-check | Logs: `access check failed`; transient error metric | Fix apiserver connectivity; check operator pod network | Sustained apiserver errors |
| Target `Degraded`, `Forbidden` (collection) | List denied for namespace | `error_class=forbidden`; engine marks forbidden scope | Fix RBAC or reduce target scope | — |
| Partial/empty attributes | CEL/JSONPath eval error on object | Logs: `extract attributes` (no secret values logged) | Fix attribute paths in profile; test with `kubectl explain` sample | Webhook should catch most invalid paths at admission |
| Growing `kollect_watch_map_list_errors_total` | List failure registering watch map handler | PromQL increase; controller logs on inventory/target | Fix RBAC for mapped GVR; check apiserver load | API server degradation |

### Export — payload size

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Inventory `Degraded`, `PayloadTooLarge` | Monolithic export &gt; ~1.5 MiB (`maxExportBytes`) | Condition message with byte counts; `kollect_sink_errors_total{reason="payload_too_large"}` | **Shard**: multiple `KollectInventory` per namespace (&lt;~2k rows each) | Architecture review for 10k+ row namespaces |
| Inventory `Degraded`, `SpillRequired` | Large payload needs object-store spill, none configured | Reason `SpillRequired`; `spill_required` metric | Add `KollectSnapshotSink` type `s3` or `gcs` to inventory refs | — |
| `ExportShardWarning=True` | ≥ ~1,800 rows in one namespace aggregate | Condition + `increase(kollect_export_shard_warn_total[1h])` | Split inventories **before** hard cap | See [scaling and fleet](performance.md) |
| `kollect_export_spill_warn_total` increasing | Payload ≥ 1 MiB warn threshold | Metric + log `export payload exceeds spill warn threshold` | Shard or tune `spec.maxExportBytes` (within global cap) | — |

### Export — sink backends

Family CRDs: **`KollectSnapshotSink`** (git/gitlab/s3/gcs), **`KollectDatabaseSink`** (postgres),
**`KollectEventSink`** (kafka/nats). Inventory refs must use the matching family field
(`snapshotSinkRefs`, etc.) in the **same namespace**.

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Git push failures (NFF) | Remote ahead of operator; concurrent writers | Logs `git push`; `terminal` or `transient` on sink errors | `pushPolicy: Commit` retries merge+push; ensure single writer per branch/path | Protected branch / hook rejects — platform Git admin |
| Git auth `terminal` | Bad token, expired credential, 401/403 | Sink `ConnectionVerified=False`; `ConnectionTestFailed` event | Rotate `secretRef`; re-annotate `kollect.dev/test-connection=true` | IdP / Git provider outage |
| Postgres connection failures | DSN, network policy, TLS, pool timeout | Database sink conditions; `transient` sink errors; connection test metric | Verify Secret keys, egress NetworkPolicy, server reachable | DBA for server-side limits |
| S3/GCS 403 | IAM, wrong bucket, signature | Export logs; `terminal`/`forbidden` | Fix credentials and bucket policy | Cloud IAM review |
| `sink circuit breaker open` in logs | 5 consecutive transient failures per sink key | `transient` errors then silence ~30s | Fix backend; breaker self-closes after timeout | Backend SLA breach |
| `SinkReachable=False`, `SinkNotFound` | Wrong sink name or cross-namespace ref | Inventory message; `kubectl get kollect*sink -n <inv-ns>` | Fix ref name; create sink in inventory namespace | — |
| `SinkReachable=False`, `SinkUnreachable` | Backend down despite CR present | Sink `ConnectionVerified`; probe annotation | Fix network/credentials first | — |

### Export — debounce (not a failure)

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| `Ready=True`, reason `PartiallySynced` | Per-sink `exportMinInterval`; unchanged payload checksum | `status.sinkExports[].conditions` reason `Debounced`; **no** `kollect_sink_errors_total` spike | Expected — wait for interval; tighten interval only if SLA requires fresher data | Mistaking debounce for outage |
| `Synced=False`, reason `PartiallySynced`, all sinks debounced | All sinks within cadence window | All per-sink `Debounced`; `kollect_export_debounced_total` up | Normal for dual-cadence (e.g. Postgres 30s + Git 1h) | — |
| Stale data in Postgres but `Synced=True` on Git ref | Different intervals per ref | Compare `lastExportTime` per `sinkExports` entry | Set ref-level `exportMinInterval` intentionally | — |

### Connection test

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Sink `ConnectionVerified=False`, `ConnectionTestFailed` | TLS verify fail, bad Secret, wrong endpoint | `kubectl describe` sink; `kollect_sink_connection_test_total{result="failure"}` | Fix `secretRef`, CA bundle, URL; one-shot: annotation `kollect.dev/test-connection=true` | Corporate TLS inspection |
| `KollectConnectionTest` stuck false | One-shot CR probe failed | `kubectl describe kollectconnectiontest` | Same as sink probe; check `spec.sinkRef` family field | — |
| `TLSInsecure=True` on sink | Explicit insecure TLS (non-default) | Condition on sink | Prefer proper CA; document exception per security policy | Security review |

Production sinks should keep `spec.connectionTest: false` and use annotation for ad-hoc probes
([ADR-0403](../adr/0403-connection-test.md)).

### Reconcile and workqueue

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Slow inventory updates, no `Degraded` | Optimistic-lock conflicts, high churn | `kollect_workqueue_depth` sustained; conflict requeues in logs | Raise `maxConcurrentReconciles`; increase `exportMinInterval`; shard inventories | etcd/apiserver slow |
| Event `ReconcilePanic` | Unexpected panic (should not crash pod) | Event reason; log `reconcile panic recovered` | Upgrade to fixed release; file bug with stack from logs | Repeat panics on same controller |
| `kollect_collect_dispatch_backpressure_total` rising | Dispatch queue saturated — informer events blocking on enqueue | Metric + dispatch queue depth | Increase `collect.dispatchWorkers` / `dispatchQueueSize` | CPU throttle on controller |
| Status update lag | Many inventories, frequent export | Reconcile duration p95; etcd metrics | Debounce, sharding, fewer sinks per inventory | [Load test runbook](load-test-runbook.md) |

### Webhook vs runtime validation

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| `kubectl apply` rejected | CEL validation on CRD, validating webhook | Admission error message (no object created) | Fix spec before create | — |
| CR accepted but `Degraded` at runtime | GVK/CRD absent on cluster, scope enforced only at reconcile, SAR not checked at admission | Compare admission vs `kubectl describe` conditions | Install CRDs; fix runtime-only constraints | Gap between webhook and runtime — upstream issue |
| Scope ceiling on cluster targets | `KollectClusterScope` webhook deny | Forbidden on apply | Adjust cluster target GVKs to allowed set | — |

### Multi-sink partial success

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| `Ready=True`, `PartiallySynced`; Postgres OK, Git failed | Independent per-sink export | `status.sinkExports[]` — mixed `Exported` / `ExportFailed` | Fix failing sink only; successful sinks stay current | — |
| `Synced=False`, `PartiallySynced`; some failed | One backend terminal while others OK | Failed count in condition message | Terminal sink needs spec/cred fix; others self-heal | — |
| Aggregate `Synced=False`, all per-sink failed | Shared payload gate (spill) before export | Inventory-level `Degraded` + spill reasons | Fix size/sharding first | — |

### Resources (brief)

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| OOMKilled controller | Large collect store, cluster-wide informers | Pod status; `kollect_collect_items_total`; RSS | `resourcesProfile: large`; namespace-scope targets; shard inventories | [Load test runbook](load-test-runbook.md) |
| CPU throttle | High dispatch/reconcile load | pprof; `kollect_reconcile_duration_seconds` p95 | Raise limits; tune workers; reduce churn | — |
| etcd / API slow | Status write rate, large fleets | Apiserver metrics; many inventories | Longer export intervals; fewer status transitions | Platform cluster health |

---

## 4. PromQL cheat sheet

Tie queries to symptoms (adjust namespace/job labels for your scrape config):

```promql
# Sustained reconcile failures by class
sum(rate(kollect_reconcile_errors_total[5m])) by (error_class)

# Inventory export errors only
sum(rate(kollect_reconcile_errors_total{kind="KollectInventory"}[5m])) by (error_class)

# Export failure reasons (auth, size, transient, …)
sum(increase(kollect_sink_errors_total[15m])) by (reason)

# Slow exports — Git vs Postgres vs event
histogram_quantile(0.95, sum(rate(kollect_export_duration_seconds_bucket[5m])) by (le, sink_type))

# Debounce (expected) vs failure — debounce should NOT correlate with sink_errors
sum(rate(kollect_export_debounced_total[5m])) by (controller)

# Workqueue backlog — conflict storms or under-provisioned workers
max_over_time(kollect_workqueue_depth[10m])

# Collect store growth — OOM/sharding signal
kollect_collect_items_total

# Approaching export shard cap
increase(kollect_export_shard_warn_total[1h])
```

Default alert rules: [metrics.md — Default alerts](metrics.md#default-alerts-kollectrules).

---

## 5. Log patterns

Structured controller logs (`logr`). Grep operator pod logs (namespace typically `kollect-system`):

| Key / message fragment | Indicates |
| --- | --- |
| `error_class` | `transient` / `terminal` / `forbidden` on wrapped errors |
| `reason` | Spill gate, export failure, scope denial (stable enum) |
| `inventory`, `target` | Which CR pipeline |
| `sink` | Backend key during export |
| `access check failed` | SAR API error → target `AccessCheckFailed` |
| `extract attributes` | CEL/JSONPath failure on a resource |
| `export failed` | Sink export path |
| `export payload exceeds spill warn threshold` | Approaching 1 MiB — shard soon |
| `debounced` | Export skipped by interval/coalesce |
| `sink circuit breaker open` | Repeated transient sink failures |
| `reconcile panic recovered` | Panic converted to requeue (EC-P2-01) |
| `git push` / `git auth failed` | Snapshot sink transport |

**Never** expect secrets, tokens, or full payloads in logs.

```sh
kubectl logs -n kollect-system deploy/kollect-controller-manager --tail=500 \
  | rg 'export failed|PayloadTooLarge|debounced|access check failed|circuit breaker'
```

---

## 6. See also

- [ADR-0602: Error taxonomy](../adr/0602-error-taxonomy.md) — class definitions and reconcile rules
- [Operator metrics](metrics.md) — full metric catalog and Prometheus Operator setup
- [FAQ](troubleshooting.md) — installation, same-namespace refs, connection conditions
- [Load test runbook](load-test-runbook.md) — scale diagnosis matrix and pprof
- [Scaling and fleet](performance.md) — export sharding and multi-cluster shared sinks
- [Deployment inventory troubleshooting](../getting-started/first-inventory.md#if-it-didnt-work) — first-check table for namespaced pipelines
