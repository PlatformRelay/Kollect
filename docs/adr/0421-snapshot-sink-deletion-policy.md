# ADR-0421: Snapshot sink deletion policy

> Deleting an inventory releases its cleanup finalizer; whether the objects it exported to a
> snapshot sink are retracted is the sink owner's explicit choice, and the default keeps them.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

An inventory exports snapshot documents to Git, GitLab, S3, and GCS. When the inventory is deleted,
the operator can either leave those objects where they are or retract them: a deletion commit on
the git branch (or on the merge-request feature branch), or object deletion in the bucket.

Retraction is destructive, can touch shared repositories and buckets, and is not always provable:
per-resource layout trees interleave with other inventories' files, `{generation}` path templates
leave one object per past generation, a changed `pathTemplate` or format leaves the old objects at
addresses the cleanup no longer renders, and a `merge_request` deletion only lands when a human
merges it. Retracting by default would make deleting an inventory a write to someone else's
repository or bucket that the sink owner never opted into.

The finalizer itself must never wedge on these questions: a sink deleted before its inventory
(namespace cascade) and a terminal backend failure must both let deletion finish.

## Decision

`KollectSnapshotSink.spec.deletionPolicy` takes `Retain` or `Delete`. The CRD defaults it to
`Retain`, admission rejects any other value, and the controller reads anything other than an exact
`Delete` as `Retain`. The field exists only on snapshot sinks: database sinks keep their
empty-export prune of the inventory's rows, and event sinks cannot retract what they emitted.

- **`Retain` (default):** the exported objects stay, the finalizer is released, and a `Normal`
  event with reason `CleanupRetainedByPolicy` names the sink. For S3, GCS and local sinks deletion
  does not contact the backend, so a broken credential cannot block it. **Revised 2026-10-05:**
  a Git or GitLab sink is contacted once, to release the inventory's ownership record
  ([ADR-0422](0422-inventory-export-identity.md)): one commit that removes only
  `.kollect-prune/<sha256(owner)>.json`, and no commit when there is no record. A failed release
  keeps the finalizer and retries like a failed retraction, so a broken Git credential now holds a
  `Retain` deletion until it is fixed or `kollect.dev/force-cleanup` is set. An inventory recreated
  later with the same kind, namespace and name starts a new record and prunes none of its
  predecessor's files.
- **`Delete`:** inventory deletion retracts the objects the inventory exported: the document, its
  `.part-NNNN-of-NNNN` siblings, layout sidecars, and for parquet the inventory's hive partition
  including the per-part partitions a multipart export writes. On Git and GitLab the same commit
  removes every file the inventory's ownership record lists and the record, and never removes a
  file another inventory's record lists (ADR-0422). When a full retraction cannot be
  proven, the deletion still proceeds and a `CleanupRetained` warning names the retained identity:
  layout trees, past generations, an unmerged deletion merge request, a recorded export path the
  cleanup no longer addresses, or an export from before export paths were recorded.

Both policies share the finalizer guarantees: a vanished sink announces `CleanupSinkGone` and
releases the finalizer, a terminal failure keeps the finalizer, re-checks every five minutes and
counts each attempt in `kollect_cleanup_terminal_total`, and `kollect.dev/force-cleanup: "true"`
drops the finalizer without backend contact.

### Shared export identity

A `KollectClusterInventory` named `X` exports as `inventory/cluster/X`, which is also the identity of
a `KollectInventory` named `X` in a namespace called `cluster`. With `Delete`, retracting one would
remove the other's export. Before a `Delete` retraction the cleanup checks whether the other object
exists. When it does, the retraction is skipped and a `CleanupSharedIdentity` warning is recorded; on Git
and GitLab the deleting inventory's ownership record is still released, because the two owners are
kind-qualified and never share a record.
When the other scope is one the operator never reconciles — cluster-scoped kinds in tenant mode, or
namespace `cluster` outside `--watch-namespaces` — or the lookup is `NotFound` or `Forbidden`, the
operator cannot have exported the other object, and the retraction runs. Any other lookup error is
transient: cleanup retries and keeps the finalizer. The export-side collision itself is a known limit of the path layout ([ADR-0407](0407-git-object-store-layout.md)).

## Consequences

- Upgrading keeps the existing outcome for snapshot sinks: before this decision, inventory deletion
  left their exported objects untouched (the empty-item cleanup export was skipped by every snapshot
  backend), and `Retain` does the same; only Git and GitLab sinks are contacted, to release the
  ownership record. Owners who want
  retraction set `deletionPolicy: Delete` per sink.
- Retraction has to be announced whenever it cannot be proven, so `Delete` users can see
  `CleanupRetained` warnings on deletions that did remove the document.
- The decision is per sink, not per inventory: every inventory bound to a sink gets the same
  behaviour, which matches who owns the destination.

## Related

- [ADR-0414](0414-sink-family-crds.md) — sink family CRDs
- [ADR-0407](0407-git-object-store-layout.md) — Git / object-store export layout
- [ADR-0419](0419-git-export-serialization-layout.md) — Git serialization and layout trees
- [ADR-0602](0602-error-taxonomy.md) — terminal vs transient errors
