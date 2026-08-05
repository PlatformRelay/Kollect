# Lab function-to-scenario and recovery matrix

Traceability from user-visible functions and failure modes to driving-range / lab scenario IDs
(`DR-*`), assertions, and evidence artifacts. Schedules live under `hack/lab/schedules/`; stubs under
`hack/lab/scenarios/`. Publishable rows must follow the [lab evidence bundle](lab-evidence-bundle.md)
(LAB-DOC-02). Runner flags: [local lab runbook](local-lab-runbook.md). Architecture:
[ADR-0707](../adr/0707-lab-harness.md).

**Honesty rule:** schedule exclusions, stub-only scenarios, emulator-only sinks, and anything that
needs the deferred **LAB-H08 failure injector** appear as **assurance gaps** — never as green
`PASS` coverage cells.

## Function to scenario coverage

| Function area | Requirements / ADRs | Positive scenario(s) | Negative / recovery scenario(s) | Evidence artifacts |
| --- | --- | --- | --- | --- |
| CRDs / admission (Profile, Target, Inventory, family sinks, Scope) | [FR-API-1](../REQUIREMENTS.md) · [ADR-0201](../adr/0201-crd-model.md) · [ADR-0414](../adr/0414-sink-family-crds.md) | `DR-0.1` install/pin · `DR-2.1` core CR apply | Invalid CEL/type → admission reject (Kind L4 + webhook tests; **lab gap** until H08 malformed apply) | `manifest.md`, `scenario-matrix.md` under `artifacts/lab/<RUN_ID>/` |
| Collection / extraction (JSONPath, CEL, watch labels) | [FR-COL-1](../REQUIREMENTS.md)–[FR-COL-6](../REQUIREMENTS.md) · [ADR-0301](../adr/0301-event-driven-informers.md) · [ADR-0302](../adr/0302-cel-jsonpath-extraction.md) | `DR-2.2` inventory converge · `DR-3.1` cert scrape | Malformed extraction → **assurance gap (H08)**; signals today: `Degraded` / extract errors in metrics ([FR-OBS-3](../REQUIREMENTS.md)) | `results.json`, redacted notes |
| Scope / tenancy | [FR-API-2](../REQUIREMENTS.md) · [ADR-0203](../adr/0203-namespaced-multi-tenancy.md) | `DR-2.4` namespaced path | `DR-3.3` scope deny (schedule-excluded from `quick+sinks` — **gap** until scheduled) | matrix row + limitations |
| Aggregation / partitioning / export interval | [FR-EXP-1](../REQUIREMENTS.md)–[FR-EXP-3](../REQUIREMENTS.md) · [ADR-0413](../adr/0413-export-interval-scheduling.md) | `DR-2.5` export cadence · `DR-2.8` dual-path sanity | Burst / backpressure → partial: idle footprint `DR-4.3`; full inject **gap (H08)** | metrics window + limitations |
| Snapshot sinks (`git`, `gitlab`, `s3`, `gcs`, `local`) | [FR-EXP-4](../REQUIREMENTS.md) · [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) | `DR-2b.11` github · `DR-2b.12` gitlab · `DR-2b.4` minio/S3 | Sink outage → **gap (H08)**; fidelity limits: [lab backend fidelity](lab-backend-fidelity.md) | serial-backend state + export notes |
| Database sinks (`postgres`, `mongodb`, `bigquery`) | [FR-EXP-4](../REQUIREMENTS.md) · [ADR-0402](../adr/0402-sink-backends-database-kafka.md) · [ADR-0420](../adr/0420-bigquery-database-sink.md) | `DR-2b.3` postgres | `DR-2b.7` mongodb · `DR-2b.10` bq-gcs **excluded** from `quick+sinks` (RAM/time) — **gap** | PASS_WITH_LIMITATION for emulators |
| Event sinks (`nats`, `kafka`/Redpanda) | [FR-EXP-4](../REQUIREMENTS.md) · [ADR-0401](../adr/0401-sink-taxonomy-state-vs-stream.md) | `DR-2b.5` nats | `DR-2b.8` redpanda **excluded** — **gap**; Redpanda ≠ managed Kafka | event counts (redacted) |
| Pipeline / multi-sink fan-out | [FR-EXP-9](../REQUIREMENTS.md) | thin coverage via `DR-2.8` | `DR-2b.9` fan-out **excluded** — **gap** | PartiallySynced conditions |
| Lifecycle (suspend, resume, HA failover) | [FR-API-5](../REQUIREMENTS.md) | `DR-1.1` HA · `DR-1.2` failover | Operator restart inject → **gap (H08)**; partial: `DR-1.1`/`DR-1.2` on multi-node lab | restart counts, Ready flaps |
| Observability (metrics, conditions, pprof) | [FR-OBS-1](../REQUIREMENTS.md)–[FR-OBS-3](../REQUIREMENTS.md) · [ADR-0601](../adr/0601-prometheus-metrics-stub.md) | `DR-4.3` idle footprint | `DR-4.4` on-demand pprof **excluded** — **gap** until triggered | `profiles/` index when captured |
| Security / NetPol / private sinks | [resolved-address policy](../security/resolved-address-policy.md) | `DR-2b.3`–`DR-2b.5` with `allowPrivateSinks` | `DR-1.5` NetPol **excluded** from `quick+sinks` — **gap** | limitations list |
| UI-mock / demo surface | DEMO-04 deferred | Kind demo path (not DR lab) | N/A — **assurance gap** for lab matrix (not claimed here) | — |
| Load / scale | [NFR-PERF-1](../REQUIREMENTS.md) · [load test runbook](load-test-runbook.md) | none in `quick+sinks` beyond idle | `DR-4.1` / `DR-4.2` Wave-4 **SKIPPED/excluded** — **gap**; never claim 100k from laptop lab | limitations + load-test-runbook |
| Workload spread / drain | multi-node ops | `DR-0.2` / `DR-0.3` topology | `DR-1.3` spread · `DR-1.4` drain **excluded** — **gap** | topology in manifest |
| Extra collection samples | [FR-COL-6](../REQUIREMENTS.md) | `DR-3.1` | `DR-3.2` Argo scrape **excluded** — **gap** | scrape counts |
| Deferred local/bare/Forgejo sinks | fidelity matrix | — | `DR-2b.1` · `DR-2b.2` · `DR-2b.6` **excluded** — **gap** | schedule exclusion reason |

## Failure and recovery coverage

Injected failure modes required by LAB-DOC-05. Until **LAB-H08** lands, rows are **assurance gaps**
with the best *today* signal operators can use from conditions/metrics/troubleshooting — not green
coverage.

| Failure mode | Positive / related ID | Recovery / negative ID | Signals today | Recovery criteria | Status |
| --- | --- | --- | --- | --- | --- |
| Sink outage / backend down | `DR-2b.3`–`DR-2b.5`, `DR-2b.11`, `DR-2b.12` | H08 inject | `Synced=False` / circuit breaker / sink error metrics ([FR-OBS-2](../REQUIREMENTS.md)) | Export resumes; breaker closes; no silent data loss | **Assurance gap (H08)** |
| RBAC loss | `DR-2.1` / `DR-2.4` | H08 inject | Forbidden events; Ready false | RBAC restored; reconcile recovers | **Assurance gap (H08)** |
| Malformed extraction | `DR-2.2` | H08 inject | Admission or extract error conditions | Fix profile; inventory converges | **Assurance gap (H08)** — Kind webhook covers some admission cases offline |
| Operator restart | `DR-1.1`, `DR-1.2` | H08 crash inject | Leader election; brief Ready flap | Single leader; export continues | Partial via HA scenarios; crash inject **gap (H08)** |
| Burst / backpressure | `DR-4.3` | H08 load inject · `DR-4.1`/`DR-4.2` | Queue depth / throttle metrics | Drain after churn; no unbounded growth | Wave-4 **excluded** — **gap** |
| Oversized payload | export size governance ADRs | H08 inject | Size / prune errors ([ADR-0306](../adr/0306-full-resource-export-pruning.md)) | Payload within cap; export succeeds | **Assurance gap (H08)** |

## Registry ID index

Every ID from `hack/lab/schedules/quick+sinks.json` (implemented + excluded). Stub present means
`hack/lab/scenarios/<ID>.sh` exists (may still be dry-run/`BLOCKED` until live body lands).

| ID | In `quick+sinks` run list? | Stub? | Role (short) |
| --- | --- | --- | --- |
| `DR-0.1` | yes | yes | Product pin / install |
| `DR-0.2` | yes | yes | Topology / nodes |
| `DR-0.3` | yes | yes | Lab labels / isolation |
| `DR-1.1` | yes | yes | HA replicas |
| `DR-1.2` | yes | yes | Failover |
| `DR-1.3` | excluded | no | Workload spread — gap |
| `DR-1.4` | excluded | no | Worker drain — gap |
| `DR-1.5` | excluded | no | NetPol — gap |
| `DR-2.1` | yes | yes | Core CR apply |
| `DR-2.2` | yes | yes | Inventory converge |
| `DR-2.3` | excluded | no | Extended core — gap |
| `DR-2.4` | yes | yes | Namespaced tenancy path |
| `DR-2.5` | yes | yes | Export cadence |
| `DR-2.6` | excluded | no | Extended export — gap |
| `DR-2.7` | excluded | no | Extended export — gap |
| `DR-2.8` | yes | yes | Dual-path / sanity |
| `DR-2b.1` | excluded | no | local-fs — gap |
| `DR-2b.2` | excluded | no | bare-git — gap |
| `DR-2b.3` | yes | yes | Postgres |
| `DR-2b.4` | yes | yes | MinIO / S3 |
| `DR-2b.5` | yes | yes | NATS |
| `DR-2b.6` | excluded | no | Forgejo — gap |
| `DR-2b.7` | excluded | no | MongoDB — gap |
| `DR-2b.8` | excluded | no | Redpanda — gap |
| `DR-2b.9` | excluded | no | Fan-out — gap |
| `DR-2b.10` | excluded | no | BQ/GCS emulator — gap |
| `DR-2b.11` | yes | yes | GitHub remote |
| `DR-2b.12` | yes | yes | GitLab remote |
| `DR-3.1` | yes | yes | Cert scrape |
| `DR-3.2` | excluded | no | Argo scrape — gap |
| `DR-3.3` | excluded | no | Scope deny — gap |
| `DR-4.1` | excluded | no | Wave-4 load — gap |
| `DR-4.2` | excluded | no | Wave-4 load — gap |
| `DR-4.3` | yes | yes | Idle footprint |
| `DR-4.4` | excluded | no | On-demand pprof — gap |

## Assurance gaps

Explicit non-green list (do not collapse into a program `PASS`):

- **LAB-H08** failure injector not implemented — sink outage, RBAC loss, malformed extraction,
  crash restart, burst inject, oversized payload lack coded recovery scenarios.
- Schedule exclusions in `quick+sinks` (see index) remain **gaps** until a larger schedule
  (`full-lab-day` / `soak`) is implemented and no longer `BLOCKED`.
- Emulator / lab substitutes (`minio`, Forgejo, Redpanda, BQ emulator) never prove managed-cloud
  IAM — see [lab backend fidelity](lab-backend-fidelity.md).
- UI-mock / DEMO-04 and cloud **100k** gates are out of this matrix’s claim set.

## Related

- [Local lab runbook](local-lab-runbook.md)
- [Lab evidence bundle](lab-evidence-bundle.md)
- [Lab backend fidelity](lab-backend-fidelity.md)
- [ADR-0707: Lab harness](../adr/0707-lab-harness.md)
- [REQUIREMENTS](../REQUIREMENTS.md)
