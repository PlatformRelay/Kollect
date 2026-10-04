# ADR-0422: Inventory identity for export ownership

> Prune ownership must know which kind of inventory wrote a file, so cluster inventories can prune
> again without ever deleting a namespaced inventory's files.

**Theme:** 04 · Export & sinks · **Status:** Exploring

## Context

Git and GitLab tree exports (`perResource`, `split`) delete stale files through ownership records,
`.kollect-prune/<sha256(owner)>.json` ([ADR-0419](0419-git-export-serialization-layout.md), "Exact
file ownership for pruning"). An export deletes the paths its previous record lists and its current
export no longer projects; a file no record lists is never deleted.

The owner is the JSON triple `[cluster, inventoryNamespace, inventoryName]`
(`internal/sink/layout_export.go:203`). The sink parses namespace and name out of the object path
(`internal/sink/export.go:234`); the request carries no kind (`export.go:72-86`). A
`KollectClusterInventory` `X` uses object path `inventory/cluster/X.json`
(`internal/controller/kollectclusterinventory_controller.go:258`), the same as a `KollectInventory`
`X` in a namespace called `cluster` (`kollectinventory_controller.go:380`). Both get the same owner,
so either could prune the other's files.

PR #394 (7cb3348) made that safe by suppressing prune whenever the namespace component is `cluster`
(`layout_export.go:209-217`). Every cluster inventory has that component, so since #394 no
cluster inventory's tree export prunes: deleted resources stay in the repository, and an empty
snapshot is a no-op. v0.21.0 contains neither #394 nor ownership records (`CHANGELOG.md:10-30`
lists both as unreleased), so the next release is the first that could ship this regression.

Constraints from the code:

- The record schema is strict: `version: 1`, fields `version`, `owner`, `paths`, unknown fields
  rejected (`internal/sink/git/prune_owned.go:33-37`, `:108`, `:116`). An unreadable record is a
  terminal error for every owned export on that branch (`:146-149`).
- The owner is otherwise opaque: hashed for the record path (`:45-47`), compared as a string
  (`:269-270`). Two owners claiming one path is a terminal error (`:155-157`, `:269-270`).
- Default paths: resources at `{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}` (no
  inventory identity), the split index at `inventory/{namespace}/{name}{extension}`, the multipart
  set manifest at `inventory/{namespace}/{name}.manifest.json`
  (`api/v1alpha1/layout_spec_types.go:36-38`, `internal/sink/layout/manifest.go:24`).
- Records can exist only in repositories written by unreleased `main` builds since 584f97a. Builds
  before the #394 guard wrote `[cluster, "cluster", X]` records that both kinds shared; later builds
  write none for that triple, because a suppressed export never writes a record (`prune_owned.go:183-185`).

## Options

### A. Kind-qualified owner, paths unchanged

The reconcilers pass kind, namespace and name in the export request. The owner of a
`KollectClusterInventory`, and of a `KollectInventory` in namespace `cluster`, becomes
`["v2", kind, cluster, namespace, name]` (namespace empty for the cluster kind). Every other
`KollectInventory` keeps its triple byte for byte. A five-element owner can never equal a legacy
triple. The suppression stays only for requests without an identity. Record schema unchanged.
Shared projected paths (split index, set manifest, the same resource under the default template)
remain shared: whichever of the two exports second is rejected as "belongs to another inventory",
which is the rule for every other pair of inventories today.

### B. Kind-prefixed path layout

A cluster inventory renders its namespace component as `_cluster` (not a valid DNS-1123 label, so no
namespace can produce it): `inventory/_cluster/X.yaml`, owner `[cluster, "_cluster", X]`. Owners and
the record schema need no new encoding, and the two inventories' index and manifest paths no longer
collide. But it moves user-visible paths for every sink type that renders the object path (Git,
GitLab, S3, GCS, the Parquet `ns=` partition), the deletion cleanup path
(`cluster_inventory_finalizer.go:92`), status `lastExportPaths` evidence, and any `{namespace}` in a
custom template. Old documents at `inventory/cluster/X.*` are orphaned. The legacy `[c, "cluster", X]`
record would still need the ambiguity rule of option A, because the namespaced inventory keeps that
triple.

### C. Per-kind record directory

Keep the triple and store records under `.kollect-prune/<kind>/<sha>.json`. The loader must read both
directories to keep cross-owner conflict checks, which is option A with a different encoding. A
build that predates it sees a directory where it expects a record file, and `checkPruneFile` rejects
it (`prune_owned.go:82-84`): every owned export on that branch then fails terminally, including
unrelated inventories.

## Trade-off matrix

Scores 1 (worst) to 5 (best); weight × score.

| Criterion | Weight | A | B | C |
| --- | --- | --- | --- | --- |
| Safety against wrong deletion | 5 | 4 (20) | 5 (25) | 4 (20) |
| Migration risk for existing repos | 4 | 5 (20) | 2 (8) | 4 (16) |
| User-visible path stability | 3 | 5 (15) | 1 (3) | 5 (15) |
| Implementation size | 2 | 3 (6) | 2 (4) | 3 (6) |
| Reversibility | 3 | 3 (9) | 2 (6) | 1 (3) |
| **Total (max 85)** | | **70** | **46** | **60** |

- Safety: all three stop cross-kind deletion. B scores higher because the two inventories also stop
  sharing index and manifest paths. A and C reject the second writer of a shared path instead, which
  deletes nothing but stops that export.
- Migration: A touches only the two affected owners and leaves every other record as it is. B moves
  paths for every cluster inventory on every sink type.
- Reversibility: after A, a rollback to an earlier `main` build fails cluster-inventory exports on
  paths the new owner claims until those records are deleted by hand. A rollback to v0.21.0 never
  reads records. C breaks every owned export on the branch after a rollback.

## Migration and compatibility (option A)

- **No records** (v0.21.0 and earlier): the first export records only the paths it writes and deletes
  nothing. Files from earlier exports are unknown and kept. Unchanged from ADR-0419.
- **Records for namespaces other than `cluster`**: same owner, same record path. Pruning continues
  with no migration commit.
- **Ambiguous legacy record `[c, "cluster", X]`**: never used to delete. Its claims do not block
  `KollectClusterInventory X` or `KollectInventory cluster/X`, and still block every other owner. The
  first complete export of either (single part, or the final part of a set) removes that record in the
  same commit as its data and new record. Files it listed that this export does not write stay in the
  repository with no owner, so no later export deletes them. The release notes give their manual
  cleanup.
- **Interrupted multipart set**: changes no record, removes no legacy record, deletes nothing.
- **Delete and recreate** (new UID, same kind, namespace and name): continues the record, so its first
  complete export deletes the predecessor's recorded paths it no longer projects. A different kind or
  name never inherits a record.
- **Requests without an identity**: today's triple and suppression, unchanged.
- Unknown files and files of other owners are never deleted, before, during or after migration.

## Recommendation

**Recommended, pending the operator's choice: option A.** It restores pruning for cluster inventories
in the next release with no path change, no record-schema change and no migration step for the
populated case (namespaced inventories outside `cluster`). Its migration retires the only ambiguous
record and deletes nothing. The cost is a loud rejection where a cluster inventory and a namespaced
inventory in `cluster` with the same name share a projected file; today that is a silent overwrite.
If those two need to coexist on one branch with split or multipart layouts, choose B in a later
release as a separate, announced path change; A does not block it.

The change is specified in `openspec/changes/inventory-export-identity/`.

## Consequences

### Positive

- Cluster inventories delete their own stale files again; the regression does not ship.
- The kind is explicit in the request, so the sink stops inferring it from a path segment.

### Negative

- A same-name cluster inventory and namespaced inventory in `cluster` cannot share a projected path.
- Files listed only by a retired legacy record need manual cleanup.
- Two owner encodings exist side by side.

## Open questions

- Delete and recreate: continue the record (recommended), or treat a new UID as a new owner that
  retires its predecessor's record and keeps its files?
- Should deleting an inventory release its record (files kept), so a successor of another kind can take
  over shared paths?
- Should a tree export without an identity fail instead of falling back to the suppression?
- Does v0.21.0's directory-scoped prune touch `.kollect-prune/` after a rollback? Not verified.

## Related

- [ADR-0419](0419-git-export-serialization-layout.md) — layouts and owned pruning
- [ADR-0421](0421-snapshot-sink-deletion-policy.md) — deletion policy and shared export identity
- [ADR-0407](0407-git-object-store-layout.md) — repository and path layout
- [ADR-0501](0501-multi-cluster-fleet.md) — `spec.cluster` partitioning
