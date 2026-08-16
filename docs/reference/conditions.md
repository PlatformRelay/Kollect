# Conditions and status

Conditions describe the latest observed generation; use them with Events and controller logs.

| Condition | Meaning |
| --- | --- |
| `Ready` | The resource is configured and able to perform its primary role. |
| `ConnectionVerified` | An explicit sink connectivity probe succeeded. |
| `SinkReachable` | The inventory can currently resolve and use its referenced sink. |
| `Synced` | The latest observed inventory was exported successfully. |
| `Degraded` | A terminal or partial failure needs attention. |

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
