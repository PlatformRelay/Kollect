# Troubleshooting

Central hub for diagnosing **Kollect** collection, export, and sink failures — what operators see
and what to do about it. For error-class semantics and reconcile behavior, see
[ADR-0602: Error taxonomy](../adr/0602-error-taxonomy.md). Step-by-step install and upgrade live in
the [Operator manual](index.md); per-scenario walkthroughs in [Examples](../examples/README.md).

!!! tip "First checks"
    When export stalls, run `kubectl describe` on the sink and inventory — `ConnectionVerified`,
    `SinkReachable`, and `Synced` conditions usually pinpoint credential, namespace, or selector
    issues before diving into controller logs.

!!! note "No hub/spoke runtime"
    There is **no** `KollectHub` CRD — hub/spoke ingest was removed in v0.3. Multi-cluster uses
    **N single-mode operators**, each writing a cluster-partitioned record to a shared sink with
    `spec.cluster` set. There are **no** hub-specific conditions, metrics, or failure modes in
    current releases ([ADR-0501](../adr/0501-multi-cluster-fleet.md),
    [Multi-cluster fleet](../concepts/multi-cluster.md)).

## How errors surface

Kollect reports failures through four channels. Start with **conditions** on the CR that owns the
pipeline; use metrics and logs when status is stale or ambiguous. Kollect follows Kubernetes
condition conventions (`Ready`, `Synced`, `Degraded`) with sink-specific types on the family sinks.

### Conditions

| Condition | Typical objects | Meaning |
| --- | --- | --- |
| `Ready` | `KollectTarget`, `KollectInventory`, `KollectClusterInventory` | Pipeline healthy enough to collect or export |
| `Synced` | Inventory / cluster inventory / target | Last export **or collection** cycle outcome (aggregate across sinks) |
| `PartiallySynced` | Inventory (`Ready` or `Synced` **reason**) | Some sinks exported; others debounced or failed |
| `Degraded` | Target, inventory, sink family CRs | Terminal misconfig or export gate — **spec change required** |
| `ExportShardWarning` | `KollectInventory` | Namespace aggregate ≥ ~1,800 rows — split before cap |
| `SinkReachable` | Inventory / target | Sink ref resolves and backend reachable |
| `ConnectionVerified` | `KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink` | Last connectivity probe succeeded (credentials, TLS, network) |

Per-sink detail lives in `status.sinkExports[]` — each entry has its own `Synced` condition and
`lastExportTime`. Condition semantics in brief: [Conditions and status](../reference/conditions.md).

A sink can show `ConnectionVerified=True` while inventory shows `SinkReachable=False` if the **name
or namespace** in a `*SinkRefs` entry is wrong — fix the reference, not just credentials.

### Conditions by kind

| Kind | Conditions | When to inspect |
| --- | --- | --- |
| Family sinks ([`KollectSnapshotSink`](../crds/kollectsnapshotsink.md), [`KollectDatabaseSink`](../crds/kollectdatabasesink.md), [`KollectEventSink`](../crds/kollecteventsink.md)) | `ConnectionVerified`, `TLSInsecure`, `Degraded` | Before export; credential or endpoint problems |
| [`KollectTarget`](../crds/kollecttarget.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Collection stalled or scope denied |
| [`KollectInventory`](../crds/kollectinventory.md) | `Ready`, `Synced`, `Degraded`, `SinkReachable` | Export not running or payload errors |
| [`KollectConnectionTest`](../crds/kollectconnectiontest.md) | `ConnectionVerified`, `Ready` | Audited composite probes |

Static kinds (`KollectProfile`, `KollectScope`) do **not** set `Ready` — admission webhooks and
events surface validation errors instead.

Full per-kind tables: [KollectInventory](../crds/kollectinventory.md#status-conditions),
[KollectTarget](../crds/kollecttarget.md#status-conditions),
[KollectClusterTarget](../crds/kollectclustertarget.md#status-conditions),
[KollectSnapshotSink](../crds/kollectsnapshotsink.md#status).

### Common `Degraded` reasons

| Reason | Typical object | Cause | Fix |
| --- | --- | --- | --- |
| `SinkNotFound` | Inventory, Target | Typo or wrong namespace in `*SinkRefs` | Match exact sink name in **same namespace** |
| `SinkUnreachable` | Inventory, Target | `ConnectionVerified=False` on sink | Fix Secret, DSN, network; re-probe sink |
| `ScopeSinkDenied` | Inventory | Sink not in `KollectScope` allow-list | Add sink to scope allow-list refs |
| `ScopeGVKDenied` | Target, ClusterTarget | GVK blocked by scope | Update `allowedGVKs` on `KollectScope` (Target) or `KollectClusterScope` (ClusterTarget) |
| `ScopeNamespaceDenied` | Target, ClusterTarget | Workload namespace blocked; on ClusterTarget, `profileRef.namespace` outside `allowedStaticRefNamespaces` | Add to `allowedNamespaces`, or permit the profile namespace |
| `ProfileNotFound` | Target | Missing `KollectProfile` | Apply profile in same namespace as target |
| `PayloadTooLarge` | Inventory | Exceeds `maxExportBytes` | Split targets or trim attributes |
| `ExportTerminal` | Inventory | Non-retryable sink error | Fix sink config; check operator logs |
| `Suspended` | Target, Inventory | `spec.suspend: true` | Set `suspend: false` |
| `Progressing` | Inventory | Transient network or 429 | Usually self-heals; inspect metrics |

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

## Diagnostic commands

```sh
# Pipeline status (short names — family sinks: ksnap/kdb/kevt)
kubectl get kprof,ksnap,kdb,kevt,ktgt,kinv -n <namespace>
kubectl describe kollectsnapshotsink <name> -n <namespace>
kubectl describe kollectdatabasesink <name> -n <namespace>
kubectl describe kollecttarget <name> -n <namespace>
kubectl describe kollectinventory <name> -n <namespace>
kubectl get events -n <namespace> --field-selector involvedObject.name=<name>

# Wait for sink probe (pick the family kind you installed)
kubectl wait --for=condition=ConnectionVerified kollectdatabasesink/<name> \
  -n <namespace> --timeout=60s

# Re-probe without editing spec
kubectl annotate kollectdatabasesink <name> -n <namespace> \
  kollect.dev/test-connection=true --overwrite

# Operator logs
kubectl -n kollect-system logs deployment/kollect-controller-manager -f --tail=200
```

Production sinks should keep `spec.connectionTest: false` and use the annotation for ad-hoc probes
([ADR-0403](../adr/0403-connection-test.md)). More shortcuts:
[Command reference](../COMMAND-REFERENCE.md).

## Error classes (ADR-0602)

| Class | Meaning | Reconcile | Self-heal? |
| --- | --- | --- | --- |
| **Transient** | Network blip, 429, API conflict, sink timeout, circuit breaker open | Requeue with backoff; `Synced=False`, reason `Progressing` | **Yes** — when root cause clears |
| **Terminal** | Bad config, invalid extraction path, auth permanently wrong, payload over cap | **No requeue**; `Degraded=True` + Warning event | **No** — fix spec/credentials, then observe new generation |
| **Forbidden** | SAR/RBAC denied for list/watch on a namespace/GVK | Degrade scope; partial collection; metric `error_class=forbidden` | **Partial** — grant RBAC or narrow selectors |

Details, examples, and circuit-breaker rules: [ADR-0602](../adr/0602-error-taxonomy.md).

## Catalog by symptom

Each row: what you see → likely cause → how to confirm → fix → escalate when stuck.

### Scope and tenancy

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Target `Degraded`, reason `ScopeGVKDenied` | `KollectScope` allow-list excludes profile GVK | Event reason; scope CR in same namespace | Add GVK to scope or remove scope binding | Platform team owns scope policy |
| Inventory `Degraded`, reason `ScopeSinkDenied` | Scope disallows snapshot/database/event family ref | `kubectl describe kollectinventory`; scope spec | Use allowed sink family/type or widen scope | Same |
| Target `Degraded`, reason `ScopeNamespaceDenied` | Target/intent namespaces outside scope | Event + target spec `namespaceSelector` | Fix selectors or scope `allowedNamespaces` | Same |
| Target `Degraded`, reason `Forbidden` | SAR denied for list in workload namespace | `kollect_reconcile_errors_total{error_class="forbidden"}`; target message cites namespace/GVK | Grant operator Role/ClusterRole list/watch on GVR; or narrow target to permitted namespaces | RBAC audit / cluster admin |
| Namespace empty in inventory (or `status.itemCount` is 0) although a target exists | `namespaceSelector` mismatch, watch opt-out, `spec.suspend: true`, or scope deny | Target `Ready`; `status.itemCount=0`; labels `kollect.dev/watch` | Align selector with workloads; check [watch labels](../adr/0205-watch-labels.md) and [Annotations and labels](../ANNOTATIONS-LABELS.md) | If SAR OK but still empty — profile/GVK issue |

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
| `ExportShardWarning=True` | ≥ ~1,800 rows in one namespace aggregate | Condition + `increase(kollect_export_shard_warn_total[1h])` | Split inventories **before** hard cap | See [Performance and scalability](performance.md) |
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
| Postgres rows stale while status looks healthy | Upsert-only drift, or an export error on that ref | Compare `lastExportTime` on the database entry in `status.sinkExports[]` | Follow [Postgres state store — Troubleshooting](../examples/postgres-state-store.md#troubleshooting) | DBA for server-side limits |
| S3/GCS 403 | IAM, wrong bucket, signature | Export logs; `terminal`/`forbidden` | Fix credentials and bucket policy | Cloud IAM review |
| `sink circuit breaker open` in logs | 5 consecutive transient failures per sink key | `transient` errors then silence ~30s | Fix backend; breaker self-closes after timeout | Backend SLA breach |
| `SinkReachable=False`, `SinkNotFound` | Wrong sink name or cross-namespace ref in `*SinkRefs` | Inventory message; `kubectl get kollect*sink -n <inv-ns>` | Fix ref name; create sink in inventory namespace | — |
| `SinkReachable=False`, `SinkUnreachable` | Backend down despite CR present; bad DSN or TLS failure | Sink `ConnectionVerified`; probe annotation | Fix network/credentials first | — |
| `Synced=False` with nothing else obvious | A prior export attempt failed | Manager logs plus the `Degraded` condition on the inventory | Fix the reported sink error; the next cycle re-exports | — |

### Export — debounce (not a failure)

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| `Ready=True`, reason `PartiallySynced` | Per-sink `exportMinInterval`; unchanged payload checksum | `status.sinkExports[].conditions` reason `Debounced`; **no** `kollect_sink_errors_total` spike | Expected — wait for interval; tighten interval only if SLA requires fresher data | Mistaking debounce for outage |
| `Synced=False`, reason `PartiallySynced`, all sinks debounced | All sinks within cadence window | All per-sink `Debounced`; `kollect_export_debounced_total` up | Normal for dual-cadence (e.g. Postgres 30s + Git 1h) | — |
| Stale data in Postgres but `Synced=True` on Git ref | Different intervals per ref | Compare `lastExportTime` per `sinkExports` entry | Set ref-level `exportMinInterval` intentionally | — |

### Connection test

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Sink `ConnectionVerified=False`, `ConnectionTestFailed` | TLS verify fail, missing `secretRef`, wrong Secret key, wrong endpoint | `kubectl describe` sink; `kollect_sink_connection_test_total{result="failure"}` | Fix `secretRef`, CA bundle, URL; one-shot: annotation `kollect.dev/test-connection=true` | Corporate TLS inspection |
| `KollectConnectionTest` stuck false | One-shot CR probe failed | `kubectl describe kollectconnectiontest` | Same as sink probe; check `spec.sinkRef` family field | — |
| `TLSInsecure=True` on sink | Explicit insecure TLS (non-default) | Condition on sink | Prefer proper CA; document exception per security policy | Security review |

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
| A working CR stopped reconciling right after an upgrade | Pre-beta CRD schema change without conversion | `kubectl explain` the new CRD; compare against the applied spec | Re-apply `install-crds.yaml`, then fix renamed or removed fields — see [Upgrading](upgrading.md) | — |

### Multi-sink partial success

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| `Ready=True`, `PartiallySynced`; Postgres OK, Git failed | Independent per-sink export | `status.sinkExports[]` — mixed `Exported` / `ExportFailed` | Fix failing sink only; successful sinks stay current | — |
| `Synced=False`, `PartiallySynced`; some failed | One backend terminal while others OK | Failed count in condition message | Terminal sink needs spec/cred fix; others self-heal | — |
| Aggregate `Synced=False`, all per-sink failed | Shared payload gate (spill) before export | Inventory-level `Degraded` + spill reasons | Fix size/sharding first | — |

### Multi-cluster fleet

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| Fleet rows in the shared sink are missing a cluster | Wrong or empty `spec.cluster` on that cluster's inventory | Compare the cluster partition in the sink backend against `spec.cluster` | Set a unique `spec.cluster` per cluster — see [Multi-cluster fleet](../examples/multi-cluster-fleet.md) | — |

### Resources (brief)

| Symptom | Likely causes | Identify | Handle | Escalate |
| --- | --- | --- | --- | --- |
| OOMKilled controller | Large collect store, cluster-wide informers | Pod status; `kollect_collect_items_total`; RSS | `resourcesProfile: large`; namespace-scope targets; shard inventories | [Load test runbook](load-test-runbook.md) |
| CPU throttle | High dispatch/reconcile load | pprof; `kollect_reconcile_duration_seconds` p95 | Raise limits; tune workers; reduce churn | — |
| etcd / API slow | Status write rate, large fleets | Apiserver metrics; many inventories | Longer export intervals; fewer status transitions | Platform cluster health |

## PromQL cheat sheet

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

## Log patterns

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

## Common questions

### Why do CRD schema changes not apply on `helm upgrade`?

Helm installs CRDs from `crds/` on first install but **does not upgrade them** on `helm upgrade`.
Kollect documents an explicit two-step path: `kubectl apply -f dist/install-crds.yaml`, then
`helm upgrade` ([ADR-0704](../adr/0704-helm-chart-crd-lifecycle.md),
[Operator manual — Upgrade](upgrading.md)).

### Should I delete CRDs to fix a schema mismatch?

**No.** Deleting a CRD garbage-collects all custom resources — including the ones you are trying to
save. Apply the release CRD bundle before upgrading the controller and follow
[Upgrading](upgrading.md); never delete CRDs in production.

### What is the recommended per-team install?

```yaml
# kollect-doc: ignore Helm values, not a kollect CR
tenantMode: true
watchNamespaces:
  - team-a
mode: single
```

See [Operator manual — Watch scope](index.md#watch-scope) and
[Multi-tenant watch scope](../examples/multi-tenant-watch-namespaces.md).

### Why does my inventory show `SinkNotFound` or `SinkReachable=False`?

`KollectInventory` binds sinks via typed lists — `spec.snapshotSinkRefs`,
`spec.databaseSinkRefs`, and/or `spec.eventSinkRefs` — naming family sink objects in the **same
namespace** as the Inventory. Cross-namespace sink refs are not supported for namespaced inventory
([ADR-0201](../adr/0201-crd-model.md), [ADR-0414](../adr/0414-sink-family-crds.md)). List the sinks
in the inventory namespace with the [diagnostic commands](#diagnostic-commands) above.

The same rule applies to `KollectTarget.spec.profileRef` → `KollectProfile` in the target namespace,
and `KollectConnectionTest.spec.sinkRef` → a family sink in the test namespace.

!!! warning "Same-namespace sink refs"
    Create family sinks in the same namespace as `KollectInventory` before expecting export.
    Cluster-wide rollup uses `KollectClusterInventory` with `spec.sinkNamespace` instead.

### I moved the sink to another namespace — why did export stop?

Update the matching `*SinkRefs` list on the Inventory to names in the **new** namespace, or recreate
the Inventory in the sink namespace. The operator does not follow cross-namespace sink references for
namespaced inventory; namespaced inventories resolve family sinks in their own namespace only.

### How do I re-test sink connectivity without editing the CR?

Annotate the family sink with `kollect.dev/test-connection=true` for a one-shot probe, then wait for
`ConnectionVerified` — the exact invocation is in [Diagnostic commands](#diagnostic-commands) above
([ADR-0403](../adr/0403-connection-test.md)). Production manifests should keep
`spec.connectionTest: false` and use the annotation for ad-hoc tests.

### Does `exportMinInterval` delay exports after a change?

**No.** The interval debounces re-export of an **identical payload** only; it coalesces changes since
the last successful export. A material change (payload checksum or `metadata.generation` bump)
exports immediately per sink, regardless of the configured interval. Set `exportMinInterval: 0s` for
*material-change only* semantics (typical for Kafka/NATS event sinks): instant export on change,
identical payloads never re-sent. `0` and sub-second durations are valid (cap: 24h), but wake-ups
floor at 1s, so anything below `1s` behaves like `0s`. Effective-interval precedence and full detail:
[Export pipeline and debouncing](../concepts/export-pipeline.md#debounce-and-interval-precedence)
and [ADR-0413](../adr/0413-export-interval-scheduling.md).

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

### How do I inventory many clusters?

**Default path:** run one Kollect operator per cluster with `mode: single` and export to a **shared sink**
(Postgres, Kafka, NATS, Git) with **`spec.cluster`** set on inventory. The sink backend merges rows by
cluster id — no central hub tier
([ADR-0501](../adr/0501-multi-cluster-fleet.md), [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md)).

Walkthrough: [Multi-cluster fleet](../examples/multi-cluster-fleet.md); concept:
[Multi-cluster fleet](../concepts/multi-cluster.md).

### My operator uses too much memory — what can I tune?

Restrict `watchNamespaces`, use `tenantMode`, narrow `KollectTarget` selectors, and increase
`exportMinInterval` on inventories. See [Performance and scalability](performance.md) and
[ADR-0603](../adr/0603-performance-scalability.md).

### Why is a namespace skipped even though a target exists?

Check `kollect.dev/namespace-watch: disabled` on the namespace, `kollect.dev/watch: disabled` on
resources, `watchMode: OptIn` without `enabled` labels, or `KollectScope` deny rules
([ADR-0205](../adr/0205-watch-labels.md), [Annotations and labels](../ANNOTATIONS-LABELS.md)).

## Example troubleshooting guides

| Scenario | Guide |
| --- | --- |
| First inventory pipeline on kind | [Your first inventory](../getting-started/first-inventory.md#if-it-didnt-work) |
| Postgres DSN and delete reconciliation | [Postgres state store](../examples/postgres-state-store.md#troubleshooting) |
| Helm / Argo Application attributes | [Helm release inventory](../examples/helm-release-inventory.md#troubleshooting) |
| Sink connectivity probes | [Connection test](../examples/connection-test.md) |
| Multi-tenant watch scope | [Multi-tenant watch namespaces](../examples/multi-tenant-watch-namespaces.md) |
| Fleet rows, shared sinks, and `spec.cluster` partitioning | [Multi-cluster fleet](../examples/multi-cluster-fleet.md) |

## When to escalate

!!! warning "Pre-beta API"
    `v1alpha1` fields may change without conversion webhook. Check [ROADMAP](../ROADMAP.md) before
    production use.

1. Collect `kubectl describe` output for sink, target, and inventory.
2. Capture operator logs (sanitize Secrets before sharing).
3. Note Helm `mode`, `tenantMode`, and `watchNamespaces` values.
4. Open a GitHub issue with repro steps and condition JSON from `status.conditions`.

## See also

- [ADR-0602: Error taxonomy](../adr/0602-error-taxonomy.md) — class definitions and reconcile rules
- [Conditions and status](../reference/conditions.md) — condition reference
- [Operator metrics](metrics.md) — full metric catalog and Prometheus Operator setup
- [Operator manual](index.md) · [Production checklist](production-checklist.md)
- [Custom resource reference](../crds/index.md) · [Connection test](../adr/0403-connection-test.md)
- [Performance and scalability](performance.md) — export sharding and multi-cluster shared sinks
- [Load test runbook](load-test-runbook.md) — scale diagnosis matrix and pprof
- [Examples](../examples/README.md) · [Your first inventory](../getting-started/first-inventory.md#if-it-didnt-work)
