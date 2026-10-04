# Design

## Context

See proposal.md. Identity, persisted metadata (ownership records), deletion and migration all
change, so the workflow requires this design, an ownership invariant, adversarial cases and a
mutation control. The options and their weighting are in
[ADR-0422](../../../docs/adr/0422-inventory-export-identity.md); this file details option A, the
recommended one, so it can be reviewed before the operator decides.

## Goals / Non-Goals

**Goals:** a cluster inventory and a namespaced inventory in namespace `cluster` can never delete
each other's files; cluster inventories prune their own stale files again; repositories holding
records from earlier builds upgrade without any file being deleted on unproven ownership; owners of
every other inventory stay byte-identical.

**Non-Goals:** see proposal.md. In particular, no path-layout change and no record-schema change.

## Decisions

### D1. Identity travels in the request, not in the path

`ExportEnvelopeRequest` (`internal/sink/export.go:72-86`) gains an inventory identity: kind
(`KollectInventory` or `KollectClusterInventory`), namespace (empty for the cluster kind) and name.
The field name and type are proposed, not existing API; a small struct such as
`Inventory sink.InventoryIdentity` is the expected shape. The namespaced reconciler sets it from
`inv.Namespace`/`inv.Name` next to `ObjectPath` (`kollectinventory_controller.go:404-415`), the
cluster reconciler from `inv.Name` (`kollectclusterinventory_controller.go:320-331`).
`RunExportEnvelope` passes it to `resolveSnapshotExport` (`layout_export.go:109`). The object path
keeps driving path rendering exactly as today (`export.go:234-236`); only the owner uses the
identity.

### D2. Owner encoding: legacy triple where it is unambiguous, kind-qualified owner where it is not

`layout_export.go:199-207` builds the owner. With an identity present:

| Inventory | Owner (JSON) | Change |
| --- | --- | --- |
| `KollectInventory` in namespace `N != "cluster"` | `[cluster, N, name]` | none |
| `KollectInventory` in namespace `cluster` | `["v2", "KollectInventory", cluster, "cluster", name]` | new |
| `KollectClusterInventory` | `["v2", "KollectClusterInventory", cluster, "", name]` | new |

`cluster` is the sink's `spec.cluster` or `default`, and `name` is the multipart base name
(`baseInventoryName`, `layout_export.go:91-96`), as today. A five-element array can never equal a
three-element one, so no new owner collides with a legacy owner, whatever `spec.cluster` contains.

Why not move every inventory to the new form: owners outside namespace `cluster` are already
unambiguous once cluster inventories leave the triple. Changing them would force a record migration
for every repository and make a rollback to an earlier `main` build fail every tree export with
"belongs to another inventory". Keeping them costs one conditional.

Why `KollectInventory` in namespace `cluster` moves too: its legacy triple is exactly the ambiguous
one. Giving it a new owner turns the rule into "a `[c, "cluster", name]` record is always legacy and
always ambiguous", which needs no record-age heuristics.

### D3. UID is not part of the owner (pending Q1)

Export paths contain no UID, so a recreated inventory with the same kind, namespace and name writes
the same paths as its predecessor. If the owner contained the UID, the recreated inventory would be
rejected on its predecessor's claims (`prune_owned.go:269-270`) until someone released the old record.
Without the UID, the recreated inventory continues the record: its first complete export deletes the
predecessor's recorded paths it no longer projects, and nothing else. A kind or name change never
continues a record.

### D4. Migration: retire the ambiguous legacy record, never adopt it

The git engine gains one input, proposed as `Config.PruneReleaseOwners []string` (forwarded through
`ExportFilesOptions` like `PruneOwner`, `git/backend.go:73-75`, `gitlab/backend.go:109-111`). For a
request with the new owner of D2, `resolveSnapshotExport` sets it to the legacy triple
`[cluster, "cluster", name]` of the same name. The engine then:

1. On every part, does not count paths claimed by a release owner as "another inventory" in
   `validateOwnedPrunePaths` (`prune_owned.go:252-277`), so a non-final part is not rejected on them.
2. On the export that advances the record (`cfg.Prune`, i.e. single-part or final part), removes the
   release owner's record file in the same commit as the data and the new record.
3. Never adds a release owner's paths to the deletion set. The deletion set stays "this owner's
   previous record minus the current keep set" (`prune_owned.go:186-194`).

Files the legacy record listed and the current export does not write remain in the repository with
no owner. They are never deleted by any later export (an unrecorded file is never deleted,
ADR-0419), and need manual cleanup, which the release notes state.

Repositories without any record (written by v0.21.0 or earlier) need no step: the first export adopts
only its current paths and deletes nothing (`prune_owned.go:187`, empty previous record).

### D5. Requests without an identity keep today's fail-safe

When the identity is absent (tests, future callers), the owner stays the triple parsed from the
object path and the `SuppressPrune` guard at `layout_export.go:209-217` stays as it is. Only the two
reconcilers produce tree exports today, and a test pins that both set the identity.

## Risks / Trade-offs

- [Silent overwrite becomes a loud rejection] → a cluster inventory and a namespaced inventory in
  `cluster` with the same name that project the same file (the split index
  `inventory/{namespace}/{name}{extension}`, the multipart set manifest, or a shared resource under the
  default `{cluster}/{sourceNamespace}/{kind}/{sourceName}` template) now get "belongs to another
  inventory" on whichever exports second. Today the second silently overwrites the first. Intended:
  the same rule already applies to every other pair of inventories. The release notes name the fix
  (rename one inventory or use a separate branch).
- [Unowned leftovers after migration] → retained by design; only `main` builds since 584f97a can have
  them. Release notes give the manual cleanup.
- [Rollback to an earlier `main` build] → that build exports a cluster inventory under the legacy
  triple and finds its paths claimed by the new owner: terminal error until the new records are
  deleted by hand. A rollback to v0.21.0 is unaffected: it never reads records (open question Q4).
- [Recreated inventory prunes retained files] → with `deletionPolicy: Retain` (ADR-0421) the files of a
  deleted inventory stay; recreating the same identity later prunes the ones it no longer projects.
  This is the D3 trade-off and Q1.
- [A deleted inventory's record blocks a different owner forever] → pre-existing for every owner pair;
  now also reachable for the cluster/`cluster` pair. Q3.
- [Fingerprint cache] → the owner-scoped key (`git/fingerprint.go:63`) changes for the two affected
  kinds, so their first export after upgrade is never coalesced. Harmless.

## Open questions

- **Q1.** Delete and recreate with the same kind, namespace and name: continue the record (D3,
  recommended) or treat a new UID as a new owner whose first export releases the predecessor's record
  and retains its files?
- **Q2.** Option A (this design) or option B (`inventory/_cluster/<name>` paths) or C, per ADR-0422.
- **Q3.** Should deleting an inventory release its ownership record (files kept, claims dropped), so a
  successor of another kind or name can take over shared paths? Separate change if yes.
- **Q4.** Does v0.21.0's directory-scoped prune ever touch `.kollect-prune/` when a user rolls back?
  Not verified; it is reachable only by a rollback across this release.
- **Q5.** Should the identity become mandatory for tree exports (terminal error when absent) instead of
  the D5 fallback?
