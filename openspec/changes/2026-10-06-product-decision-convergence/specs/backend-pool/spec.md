# Spec Delta

## ADDED Requirements

### Requirement: BEP-1 Deleting a family sink evicts its pooled backend immediately

When a `KollectSnapshotSink`, `KollectDatabaseSink` or `KollectEventSink` is deleted, the process
pool SHALL drop the cached backend entry for that sink (by object UID, and by namespace/name when
the entry is keyed that way) and Close it, instead of holding it for the idle TTL. Eviction is
best-effort and idempotent: it SHALL NOT re-export, retract, or reconcile anything, and SHALL NOT
fail when no entry is pooled.

#### Scenario: Deleted sink's backend is closed promptly

- **WHEN** a sink's backend is pooled (an export ran) and the sink object is then deleted
- **THEN** the pool no longer holds an entry for that sink's UID and the backend's Close has run
- **AND** no connection to the sink's backend endpoint outlives the deletion beyond the eviction call

#### Scenario: Eviction without a pooled entry is a no-op

- **WHEN** a sink is deleted while its backend is not pooled (never exported, or already TTL-pruned)
- **THEN** nothing fails and nothing is logged as an error

#### Scenario: Live sinks are untouched

- **WHEN** a sink that was never deleted is reconciled or its spec changes
- **THEN** its pooled entry is not evicted by the delete hook (spec-hash changes are handled by the existing pool swap on acquire)

### Requirement: BEP-2 The TTL remains the backstop, not the contract

The idle-entry TTL SHALL keep its meaning (bound the pool and age out entries for sinks that
stopped being reconciled for any reason). The delete hook SHALL NOT change the TTL, the pooling
of live sinks, or the acquire-time spec-hash swap.

#### Scenario: TTL still ages out stale entries

- **WHEN** an entry is idle longer than `backendPoolTTL`
- **THEN** the next acquire prunes it, as today
