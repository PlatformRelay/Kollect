# Common errors — operator guide

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
| `ExportShardWarning=True` | ≥ ~1,800 rows in one namespace aggregate | Condition + `increase(kollect_export_shard_warn_total[1h])` | Split inventories **before** hard cap | See [scaling and fleet](scaling-and-fleet.md) |
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
- [FAQ](../FAQ.md) — installation, same-namespace refs, connection conditions
- [Load test runbook](load-test-runbook.md) — scale diagnosis matrix and pprof
- [Scaling and fleet](scaling-and-fleet.md) — export sharding and multi-cluster shared sinks
- [Deployment inventory troubleshooting](../examples/deployment-inventory.md#troubleshooting) — first-check table for namespaced pipelines
