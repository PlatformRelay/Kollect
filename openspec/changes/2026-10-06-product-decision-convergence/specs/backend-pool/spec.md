# Spec Delta

## ADDED Requirements

### Requirement: BEP-1 Deleting a family sink evicts its pooled backend immediately

When a `KollectSnapshotSink`, `KollectDatabaseSink` or `KollectEventSink` is deleted, the process
pool SHALL drop the cached backend entry for that sink's object UID and Close it, instead of
holding it for the idle TTL; when the delete event's object carries no UID, the eviction SHALL
fall back to the sink's namespace/name key. A backend whose build was still in flight when the
delete event landed SHALL be discarded instead of pooled for the deleted sink (delete-tombstone),
so eviction cannot be undone by the re-store race. Eviction is best-effort and idempotent: it
SHALL NOT re-export, retract, reconcile or clean up anything, and SHALL NOT fail when no entry
is pooled. An in-flight export that already holds the evicted backend may fail against it; the
sink no longer exists and no new acquire SHALL rebuild an entry for that sink's UID.

#### Scenario: Deleted sink's pooled entry is evicted and closed

- **WHEN** a sink's backend is pooled (an export ran) and the sink object is then deleted
- **THEN** the pool no longer holds an entry for that sink's UID and the pooled backend's Close has run

#### Scenario: In-flight build cannot re-pool after eviction

- **WHEN** an acquire-build for a sink is in flight and the sink is deleted before the build's store lands
- **THEN** the built backend is discarded, not pooled

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
