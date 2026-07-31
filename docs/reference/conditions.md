# Conditions and status

Conditions describe the latest observed generation; use them with Events and controller logs.

| Condition | Meaning |
| --- | --- |
| `Ready` | The resource is configured and able to perform its primary role. |
| `ConnectionVerified` | An explicit sink connectivity probe succeeded. |
| `SinkReachable` | The inventory can currently resolve and use its referenced sink. |
| `Synced` | The latest observed inventory was exported successfully. |
| `Degraded` | A terminal or partial failure needs attention. |

`KollectInventory.status.sinkExports[]` records timestamps, checksums, and conditions per sink.
This distinguishes partial fan-out from total failure. Full collected payloads never live in CR
status.

```sh
kubectl get kinv -A
kubectl describe kinv -n default team-inventory
kubectl get events -n default --sort-by=.lastTimestamp
```

The [troubleshooting guide](../operator-manual/troubleshooting.md) maps common reasons to fixes.
