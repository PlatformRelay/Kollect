# Proposal

## Why

Git and GitLab tree exports record which files each inventory owns in
`.kollect-prune/<sha256(owner)>.json` (ADR-0419, ADR-0422). Deleting an inventory leaves that record
in the repository under both deletion policies:

- `deletionPolicy: Retain` (the default) does not contact the backend at all (`retractionPrecheck`,
  `internal/sink/cleanup.go`), so the record survives.
- `deletionPolicy: Delete` removes the document, its parts and sidecars through `DeleteExport`
  (`internal/sink/git/delete.go`), which knows nothing about ownership records: the record survives,
  the files the record lists (the per-resource tree) stay, and a candidate path another inventory's
  record lists is deleted anyway. The adversarial review of #414 flagged this.

A surviving record keeps claiming its paths. Another inventory that projects one of them is rejected
with "belongs to another inventory" until someone deletes the record by hand, and every surviving
record counts against the hard cap of 1024 owners (`maxPruneOwners`,
`internal/sink/git/prune_owned.go`). ADR-0422 lists releasing the record on deletion as a follow-up.

## What Changes

- Deleting a `KollectInventory` or `KollectClusterInventory` releases its ownership record on every
  bound Git or GitLab sink: the record file is removed in a commit.
- `Retain`: the release commit removes only the record. Every exported file stays. Git and GitLab
  sinks are now contacted on a `Retain` deletion; this deliberately revises ADR-0421's "does not
  contact the backend" for them. S3, GCS and local sinks are unchanged.
- `Delete`: the retraction commit removes the inventory's candidate paths (document, parts, sidecars),
  every path its own record lists, and the record. It never deletes a path another inventory's record
  lists.
- The ownerless `DeleteExport` and the legacy directory-scoped prune (an ownerless tree export) never
  delete a path any record lists.
- The release is idempotent: an absent record is done, and nothing is committed.
- A failed release keeps the finalizer and retries, as a failed retraction does today.
- A recreated inventory with the same kind, namespace and name starts a new record (IEI-8 changes).

Decision: operator, 2026-10-05 (INVENTORY-IDENTITY-01 slice 2). ADR-0421 and ADR-0422 are updated.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `git-tree-ownership`: adds release on deletion (ROD-1..ROD-7) and changes IEI-8.

## Impact

- Entry points: the finalizers of both inventory reconcilers
  (`internal/controller/inventory_finalizer.go`, `cluster_inventory_finalizer.go`) through
  `cleanupSinkExports` (`internal/controller/sink_cleanup.go`) to `sink.RunCleanupExport`
  (`internal/sink/cleanup.go`), then `ReleaseExport` on the Git (`internal/sink/git/backend.go`) and
  GitLab (`internal/sink/gitlab/backend.go`) backends and the deletion engine
  (`internal/sink/git/delete.go`, `release.go`).
- Behaviour change for users: deleting an inventory bound to a Git or GitLab sink now commits to that
  repository under `Retain` (one commit removing one record file, only when a record exists), and a
  Git or GitLab sink with broken credentials now holds the inventory in `Terminating` until the
  credential is fixed or `kollect.dev/force-cleanup: "true"` is set.
- No CRD, record-format or path-layout change.

## Non-goals

- GitLab `branchMR` claim safety across feature branches (separate slice, another lane).
- Releasing the record when the sink CR is already gone (`CleanupSinkGone`) or the finalizer is
  forced: nothing can reach the backend; the record stays and the existing warnings say so.
- Dropping the structural `CleanupRetained` announcement for tree layouts now that `Delete` removes
  the recorded tree. Possible later, needs its own evidence.
- Migration or fallbacks for records from earlier builds (operator decision: not in production).
- S3, GCS, local, database and event sinks.

## Assumptions

- Every git-family export that writes a record carries the owner `git.InventoryPruneOwner(kind,
  cluster, namespace, name)` with `cluster` = the sink's `spec.cluster` or `default`
  (`inventoryPruneOwner`, `internal/sink/layout_export.go`). The cleanup derives the owner with the
  same function from the same inputs, so the owner it releases is the one the export wrote. Probe:
  `grep -n 'InventoryPruneOwner(' internal/sink/*.go` lists only `inventoryPruneOwner`, which both
  paths call.
- Both reconcilers already know their identity at deletion time (the object is still readable while
  its finalizer runs).
- A record's path is the SHA-256 of its owner and the strict reader rejects a record whose owner does
  not hash to its path (`readPruneRecord`), so removing the file at the deleting owner's path can
  never remove another owner's record.
