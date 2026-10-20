# Conditions and status

Conditions describe the latest observed generation; use them with Events and controller logs.

| Condition | Meaning |
| --- | --- |
| `Ready` | The resource is configured and able to perform its primary role. |
| `ConnectionVerified` | An explicit sink connectivity probe succeeded. |
| `SinkReachable` | The inventory can currently resolve and use its referenced sink. |
| `Synced` | Every bound sink holds the latest observed inventory. `True` (`Exported`, message `N/N sink(s) up to date`) when no sink failed in the last export pass. `False` when a sink failed: reason `PartiallySynced` when some sinks exported and some failed, otherwise `Progressing`, `ExportTerminal` or `SpillRequired`. |
| `ExportSucceeded` | Set on `KollectClusterInventory` per export pass: `True` (`Exported`, message `exported N item(s); N/N sink(s) up to date`) when no sink failed; `True` (`PartiallySynced`) when some sinks exported and some failed; `False` when the export failed. Cluster inventories only. |
| `TLSInsecure` | Set on a family sink while `spec.tls.insecureSkipVerify` is `true`; removed when the field is unset. Present only when TLS certificate verification is disabled — dev use, never production. |
| `Degraded` | A terminal or partial failure needs attention. |

**A debounced sink is up to date.** Each `status.sinkExports[]` entry carries its own `Synced`
condition. When the payload, the inventory and the sink object are unchanged since the sink's last
successful export and its `exportMinInterval` has not elapsed (or is `0s`), the operator skips the
write and sets that entry to `Synced=True`, reason `Debounced`. Such a sink counts toward `N/N sink(s) up to date` in
the aggregate `Synced` and in `ExportSucceeded`, exactly like a sink that exported in the pass.
Use the per-sink reason or `kollect_export_debounced_total` to see debouncing.

**`KollectTarget` `Ready.lastTransitionTime` is not a flap signal.** The `Ready` message restates
the live collected count, and the controller treats a changed message as a transition. A busy
Target whose count moves therefore gets a fresh `lastTransitionTime` on every refresh — as often as
`--target-count-resync` (default `60s`) — while `Ready` never leaves `True`. Do not alert on it as
if the Target had flapped, and do not read an *old* `lastTransitionTime` as "collection has
stalled": it only means the count has not moved. Use `status.collectedCountUpdatedAt` and the
`Degraded` condition instead.

`KollectInventory.status.sinkExports[]` records timestamps, checksums, and conditions per sink.
This distinguishes partial fan-out from total failure. Full collected payloads never live in CR
status.

```sh
kubectl get kinv -A
kubectl describe kinv -n default team-inventory
kubectl get events -n default --sort-by=.lastTimestamp
```

The [troubleshooting guide](../operator-manual/troubleshooting.md) maps common reasons to fixes.
