# ADR-0422: Inventory identity for export ownership

> Prune ownership must know which kind of inventory wrote a file, so cluster inventories can prune
> again without ever deleting a namespaced inventory's files.

**Theme:** 04 · Export & sinks · **Status:** Current (accepted 2026-10-04)

## Context

Git and GitLab tree exports (`perResource`, `split`) delete stale files through ownership records,
`.kollect-prune/<sha256(owner)>.json` ([ADR-0419](0419-git-export-serialization-layout.md), "Exact
file ownership for pruning"). An export deletes the paths its previous record lists and its current
export no longer projects; a file no record lists is never deleted.

Before this decision the owner was the JSON triple `[cluster, inventoryNamespace, inventoryName]`,
with namespace and name parsed out of the object path; the request carried no kind. A
`KollectClusterInventory` `X` uses object path `inventory/cluster/X.json`, the same as a
`KollectInventory` `X` in a namespace called `cluster`. Both got the same owner, so either could
prune the other's files.

PR #394 (7cb3348) made that safe by suppressing prune whenever the namespace component is `cluster`.
Every cluster inventory has that component, so since #394 no cluster inventory's tree export pruned:
deleted resources stayed in the repository, and an empty snapshot was a no-op. v0.21.0 contains
neither #394 nor ownership records, so the next release would have been the first to ship this
regression.

Constraints from the code:

- The record schema is strict: `version: 1`, fields `version`, `owner`, `paths`, unknown fields
  rejected (`internal/sink/git/prune_owned.go`). The owner is otherwise opaque to the engine: hashed
  for the record path and compared as a string. Two owners claiming one path is a terminal error.
- Default paths: resources at `{cluster}/{sourceNamespace}/{kind}/{sourceName}{extension}` (no
  inventory identity), the split index at `inventory/{namespace}/{name}{extension}`, the multipart
  set manifest at `inventory/{namespace}/{name}.manifest.json`
  (`api/v1alpha1/layout_spec_types.go`, `internal/sink/layout/manifest.go`).
- Kollect is not in production. Records written by earlier `main` builds need not be understood,
  migrated or rolled back to (operator decision, 2026-10-04).

## Options

### A. Kind-qualified owner for every inventory, paths unchanged

The reconcilers pass kind, namespace and name in the export request. Every inventory's owner becomes
`["v2", kind, cluster, namespace, name]` (namespace empty for the cluster kind). The suppression is
removed. Record schema and paths unchanged. Shared projected paths (split index, set manifest, the
same resource under the default template) remain shared: whichever of two inventories exports second
is rejected as "belongs to another inventory", which is the rule for every pair of inventories.

### B. Kind-prefixed path layout

A cluster inventory renders its namespace component as `_cluster` (not a valid DNS-1123 label):
`inventory/_cluster/X.yaml`. The two inventories' index and manifest paths stop colliding, but
user-visible paths move for every sink type that renders the object path (Git, GitLab, S3, GCS, the
Parquet `ns=` partition), the deletion cleanup path, status `lastExportPaths` evidence, and any
`{namespace}` in a custom template.

### C. Per-kind record directory

Keep the triple and store records under `.kollect-prune/<kind>/<sha>.json`. The loader must read
every directory to keep cross-owner conflict checks, which is option A with a different encoding and
a second place to look.

## Trade-off matrix

Scores 1 (worst) to 5 (best); weight × score.

| Criterion | Weight | A | B | C |
| --- | --- | --- | --- | --- |
| Safety against wrong deletion | 5 | 4 (20) | 5 (25) | 4 (20) |
| User-visible path stability | 3 | 5 (15) | 1 (3) | 5 (15) |
| Implementation size | 2 | 4 (8) | 2 (4) | 3 (6) |
| One owner rule for every inventory | 3 | 5 (15) | 3 (9) | 4 (12) |
| **Total (max 65)** | | **58** | **41** | **53** |

- Safety: all three stop cross-kind deletion. B scores higher because the two inventories also stop
  sharing index and manifest paths. A and C reject the second writer of a shared path instead, which
  deletes nothing but stops that export.

## Decision

**Option A, for every inventory** (operator decision, 2026-10-04).

- **Identity in the request.** `sink.ExportEnvelopeRequest.Inventory` carries kind
  (`KollectInventory` or `KollectClusterInventory`), namespace (empty for the cluster kind) and name.
  Both reconcilers set it next to the object path. The object path still drives path rendering; the
  identity decides the owner.
- **Owner.** `["v2", kind, cluster, namespace, name]`, built by `git.InventoryPruneOwner`. `cluster`
  is the sink's `spec.cluster` or `default`; `name` carries no multipart suffix. The owner carries no
  UID: a deleted and recreated inventory with the same kind, namespace and name continues its
  predecessor's record, and its first complete export prunes the predecessor's recorded paths it no
  longer projects. This holds for `deletionPolicy: Retain` too, which concerns only the deletion
  event ([ADR-0421](0421-snapshot-sink-deletion-policy.md)). A kind or name change never continues a
  record.
- **Identity is mandatory.** A git layout export (everything that goes through the tree writer and
  therefore carries an owner) without an identity is a terminal error. There is no fallback to a
  path-derived owner and no suppression.
- **Consistency asserts, all terminal.** The identity's namespace and name must equal those parsed
  from the object path (multipart suffix removed), and a `KollectClusterInventory` must come with path
  namespace `cluster`. The engine refuses, before writing, a commit whose ownership records would claim
  one path twice.
- **The rejection names the owner.** "prune path ... belongs to another inventory: \<kind\>
  \<namespace/name\> (cluster ...), ownership record .kollect-prune/\<sha\>.json".
- **Multipart manifest pre-claim.** Every non-final part of a prune-bearing set claim-checks the
  set-manifest path the final part will write, so a set whose manifest belongs to another inventory is
  rejected on part 1, before any part is committed.
- **No compatibility layer.** Records with any other owner encoding are treated as another
  inventory's records; there is no migration, dual encoding or rollback handling.

## Consequences

### Positive

- Cluster inventories delete their own stale files again, including on an empty snapshot.
- The kind is explicit in the request; the sink no longer infers it from a path segment.
- One owner rule for every inventory, enforced by asserts rather than by a special case.

### Negative

- A same-name cluster inventory and namespaced inventory in `cluster` cannot share a projected path
  (split index, set manifest, a resource under the default template); the second exporter is rejected.
- A repository that holds records from earlier `main` builds rejects exports whose paths those records
  list, until the old records are removed by hand. Acceptable while Kollect is not in production.
- An inventory deleted under `Retain` keeps its record, so a successor of another kind or name cannot
  take over its paths. Releasing records on deletion is a later slice.

## Follow-ups

- Release an inventory's ownership record when the inventory is deleted (files kept, claims dropped).
- GitLab `branchMR` mode: two inventories can claim one path on separate feature branches; the claim
  check sees only the target branch.

## Related

- [ADR-0419](0419-git-export-serialization-layout.md) — layouts and owned pruning
- [ADR-0421](0421-snapshot-sink-deletion-policy.md) — deletion policy and shared export identity
- [ADR-0407](0407-git-object-store-layout.md) — repository and path layout
- [ADR-0501](0501-multi-cluster-fleet.md) — `spec.cluster` partitioning
- `openspec/changes/inventory-export-identity/` — the change that implements it
