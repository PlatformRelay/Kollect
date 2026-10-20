# Design

## Context

See `proposal.md` *Why*. Three defects produce the symptom, and one more becomes visible once
"debounced" means "synced":

1. The aggregate maps a debounce to `Synced=False/PartiallySynced`
   (`internal/controller/per_sink_export.go:336-358`).
2. The per-sink condition is `False/Debounced`, and its message carries a countdown.
3. `status.sinkExports[]` is rebuilt from empty entries every pass, so a debounced pass erases
   `lastExportTime` and `lastChecksum` and recreates the condition with a new
   `lastTransitionTime`.
4. The coalesce baseline (`perSinkCoalesceTracker`) is updated only on success and never cleared
   on failure. After a success with C1 and a failed export of C2, a payload back at C1 is
   skipped as unchanged although the last attempt failed. Today that reads `Synced=False`; under
   SDC-1 it would read `Synced=True`, which would be wrong.

## Option chosen

Decision brief `agent-context/decisions/2026-10-20-synced-debounce-reason.md` (local), run through
two independent legs; both chose A. Pending operator validation.

| Option | Aggregate when no sink failed | Churn | Surface |
| --- | --- | --- | --- |
| **A (chosen)** | `True/Exported`, one message for export and debounce passes | none in the aggregate | none new |
| B | `True/Exported` or `True/UpToDate` | reason alternates each interval | new reason |
| C | unchanged, docs only | flip stays | none |
| D | `True/Exported` plus a new `ExportDebounced` condition | the new condition flips each interval | new condition type |

A loses one bit in the aggregate: whether the last pass exported or skipped. The per-sink reason
and `lastExportTime` keep it.

## Behaviour

### Per-sink condition (SDC-1, SDC-3)

| Sink outcome in the pass | `Synced` | Reason | Message |
| --- | --- | --- | --- |
| exported | `True` | `Exported` | `export completed` (unchanged) |
| debounced, interval > 0 | `True` | `Debounced` | `payload unchanged since the last export; re-exported every <interval>` |
| debounced, interval `0s` | `True` | `Debounced` | `payload unchanged since the last export; exports only on change (interval 0s)` |
| failed | `False` | `ExportFailed` / `SpillRequired` | unchanged |

`<interval>` is the effective interval as `time.Duration.String()`. Nothing time-dependent goes
into the message. Both reconcilers and the namespaced preview path
(`previewAllSinksDebounced`) use one helper for the debounced row, so the three call sites cannot
drift.

### Aggregate (SDC-2, SDC-3)

`aggregateInventorySync(conditions, generation, exported, debounced, failed)` keeps its
signature. Let `upToDate = exported + debounced`, `bound = upToDate + failed`.

| Case | `Synced` | Reason | Message |
| --- | --- | --- | --- |
| `failed > 0 && exported > 0` | `False` | `PartiallySynced` | unchanged (`%d/%d sinks exported; %d failed`) |
| `failed > 0` | `False` | `Progressing` | unchanged (`%d sink(s) failed export`) |
| otherwise | `True` | `Exported` | `%d/%d sink(s) up to date` (`upToDate`, `bound`) |

The total-failure path in `applyInventoryExportOutcome` and the cluster equivalent, which set
`ExportTerminal` or `SpillRequired`, are unchanged. The debounced-plus-failed row stays `False`
(SDC-3); `Ready` stays as it is (proposal non-goals).

The `KollectClusterInventory` `ExportSucceeded` message in the no-failure branch becomes
`exported %d item(s); %d/%d sink(s) up to date`. Its reason and status are unchanged.

### Per-sink entry carry-over (SDC-5)

`carryOverLastExportPaths` becomes `carryOverSinkExport(status, previous, exportKey)`: before the
pass decides anything for the sink, the new entry is seeded from the previous entry with the
same name: `LastExportTime`, `LastChecksum`, `LastExportPaths` and `Conditions` (deep-copied).
Then:

- debounced: the `Synced` condition is set as above; `SetStatusCondition` keeps
  `lastTransitionTime` because the status stays `True`;
- exported: `LastExportTime`, `LastChecksum`, `LastExportPaths` are replaced as today;
- failed: only the condition changes (to `False`, which moves `lastTransitionTime`).

An entry for a sink no longer bound is dropped as today (entries are built from the current
bindings). `recordExportPaths` keeps its "empty write keeps the previous paths" rule.

The previous entry comes from `inv.Status.SinkExports` of the object being reconciled, which is
what `carryOverLastExportPaths` already reads.

### Baseline reset on failure (SDC-4)

`perSinkCoalesceTracker` gains `forget(invKey, sinkName)`, which deletes the entry. Each export
failure branch that follows a `shouldSkip == false` decision calls it: the per-sink job failure
(`kollectinventory_controller.go:437-443`, cluster `kollectclusterinventory_controller.go:359-364`). Failures before the skip check
(sink not resolved, partition error) also call it, since their payload was not delivered. With
no entry, `shouldSkip` returns false, so the next reconcile exports; `nextDue` returns 1 s, and
the existing failure requeue still applies. A pod restart already starts with an empty tracker,
so this adds no new state.

The reset changes behaviour only when the inputs return to those of the last successful export
after a failed one (payload back at C1, or a `requestedAt` value set back). Every other failure
is already retried, because its checksum, generation or `requestedAt` value differs from the
baseline. In the changed case the next reconcile exports again, which is the intended outcome:
the last attempt failed, so the sink is not known to be up to date.

### Steady state without writes (SDC-5)

With the four changes, a debounced pass with nothing changed produces a status equal to the
stored one: aggregate condition unchanged, per-sink entries equal (seeded from the previous ones,
same message, same `lastTransitionTime`), `itemCount`, `observedGeneration` and
`lastExportTime` unchanged. `Status().Update` of an unchanged object is not persisted, so
`resourceVersion` stays and the inventory's own watch does not fire again. No Event is recorded
on a pass with `ExportedCount == 0` (unchanged: `recordNormal` is guarded by it). The reconciler
does not skip the update call itself; equality is left to the API server, which keeps this change
small. Probe 0.1 checks the no-op behaviour in envtest.

The periodic re-export pass still changes `lastExportTime` and the per-sink reason
(`Debounced` → `Exported`), so it writes status once per interval, and emits `ExportSucceeded`
(proposal non-goals).

### Read API (SDC-6)

`exportStatusFromInventory` (`internal/inventory/status.go:167-180`) checks the reason first:
reason `Debounced` → `debounced` for either status; then `True` → `ok`, `False` → `degraded`,
other → `unknown`.

### Future pending state

No state holds back a changed payload (ADR-0413 §2). A later change that adds one (rate
limiting, a schedule) must report it with its own per-sink reason and must decide its aggregate
status explicitly; SDC-4's scenario "Changed payload inside the interval" guards that today's
code exports instead.

## Compatibility

| Observer | Before | After |
| --- | --- | --- |
| Aggregate `Synced`, all sinks debounced | `False/PartiallySynced` "N sink(s) debounced" | `True/Exported` "N/N sink(s) up to date" |
| Aggregate `Synced`, some exported, some debounced | `False/PartiallySynced` "x/y sinks debounced" | `True/Exported` |
| Aggregate `Synced`, all exported | `True/Exported` "exported to N sink(s)" | `True/Exported` "N/N sink(s) up to date" (message text changes) |
| Aggregate `Synced`, any failure | `False`, reasons as today | unchanged |
| Per-sink `Synced`, debounced | `False/Debounced`, countdown message | `True/Debounced`, static message |
| Per-sink `lastExportTime`, `lastChecksum` after a debounced or failed pass | erased | kept |
| Read API per-sink status | `debounced` | `debounced` |
| `kollect_export_debounced_total` | counts skips | unchanged |
| `Ready`, `Degraded`, Events | as today | unchanged |

An alert on `Synced=False` or on reason `PartiallySynced` stops firing for a debounce. Anything
that detected debouncing from the aggregate must read the per-sink reason or
`kollect_export_debounced_total` instead. Upgrade note in SDC-7. No CRD schema change: reasons
and messages are free strings.

## Risks

- A message text change can break a script matching "exported to N sink(s)". Mitigation: the
  upgrade note lists the new messages.
- Deep-copying the previous conditions carries forward a condition type that a future change
  stops setting. Only `Synced` is set per sink today; the copy keeps the list as is.
