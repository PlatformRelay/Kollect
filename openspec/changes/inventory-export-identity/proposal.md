# Proposal

## Why

Git and GitLab tree exports (`perResource`, `split`) prune through per-owner ownership records
(`.kollect-prune/<sha256(owner)>.json`, `internal/sink/git/prune_owned.go:33-47`). The owner is the
JSON triple `[cluster, inventoryNamespace, inventoryName]` (`internal/sink/layout_export.go:203`),
and the inventory namespace and name are parsed back out of the object path
(`internal/sink/export.go:234`). The request carries no inventory kind.

A `KollectClusterInventory` named `X` exports with object path `inventory/cluster/X.json`
(`internal/controller/kollectclusterinventory_controller.go:258`), exactly what a `KollectInventory`
`X` in a namespace called `cluster` produces (`internal/controller/kollectinventory_controller.go:380`).
Both inventories get the same owner, so either could prune the other's files. PR #394 (7cb3348)
closed that hole by suppressing prune whenever the namespace component is `cluster`
(`layout_export.go:209-217`).

The suppression applies to every cluster inventory, because every cluster inventory uses that
namespace component. Since #394, Git and GitLab tree exports of every `KollectClusterInventory`
never prune: a deleted resource's file stays in the repository for good, and an empty snapshot is
a no-op (`layout_export.go:214-216`). v0.21.0 does not contain #394 or the ownership records
(`CHANGELOG.md:10-30` lists them as unreleased). The next release must not ship this regression.

## What Changes

- The export request carries an explicit inventory identity: kind, namespace (empty for the
  cluster kind) and name. Both controllers set it. The sink no longer infers the kind from the path.
- The prune owner of a `KollectClusterInventory`, and of a `KollectInventory` in namespace
  `cluster`, is a new kind-qualified owner that cannot equal any legacy triple. Every other
  inventory keeps its current owner byte for byte.
- The blanket `SuppressPrune` for namespace `cluster` is removed for requests that carry an
  identity. Cluster inventories prune their own stale files again, including empty snapshots.
- A legacy ownership record under the ambiguous triple `[cluster, "cluster", name]` is never used
  to delete anything. The first complete export of either inventory with that name retires it in
  the same commit and leaves every file it listed in place, unowned.
- A request without an identity keeps today's behaviour, including the suppression.

Recommended in [ADR-0422](../../../docs/adr/0422-inventory-export-identity.md) (Proposed, option A).
This change implements option A and is blocked on the operator's choice.

## Capabilities

### New Capabilities

- `git-tree-ownership`: which files a Git or GitLab tree export may delete, whose ownership record
  proves it, and how records written by earlier builds are treated.

### Modified Capabilities

None. Owned pruning has no living spec yet; its current contract is
[ADR-0419](../../../docs/adr/0419-git-export-serialization-layout.md) "Exact file ownership for
pruning".

## Impact

- Entry points: the `KollectInventory` and `KollectClusterInventory` reconcilers' snapshot export
  (`kollectinventory_controller.go:404-415`, `kollectclusterinventory_controller.go:320-331`), through
  `sink.RunExportEnvelope` (`internal/sink/export.go:148`) and `resolveSnapshotExport`
  (`layout_export.go:109`) to `ExportFiles` of the Git (`internal/sink/git/backend.go:66-75`) and
  GitLab (`internal/sink/gitlab/backend.go:87-111`) backends.
- Behaviour change for users: cluster inventories' tree exports delete their own stale files again.
  A cluster inventory and a namespaced inventory in namespace `cluster` that project the same file
  path no longer overwrite each other silently: the second export is rejected with
  "prune path ... belongs to another inventory" (`prune_owned.go:269-270`), as for any two other
  inventories today.
- Repository state: new records for the affected owners; the ambiguous legacy record, if present,
  is removed. No exported data file is moved or deleted by the migration itself.
- No CRD or path-layout change. Document-mode exports (`prune` off) are unaffected.

## Non-goals

- Independent storage for colliding projected paths. Two inventories that render the same path
  still cannot both own it; that needs a path-layout change (ADR-0422 option B).
- Releasing an ownership record when its inventory is deleted (see design.md, open question Q3).
- The document-path collision handled by the deletion cleanup's shared-identity check
  (`internal/sink/cleanup.go:80-86`, ADR-0421). Unchanged.
- Non-tree sinks (S3, GCS, databases, events) and `kollect-pipeline`, which exports through
  `backend.Export` with no prune owner (`internal/pipeline/wire.go:305`).
- Sinks sharing one `spec.cluster` value across clusters (ADR-0501 requires distinct values).

## Assumptions

- Ownership records exist only in repositories written by unreleased `main` builds. Source:
  `CHANGELOG.md:22-30` ("Prune only recorded inventory files", 584f97a, under `[Unreleased]`);
  v0.21.0 starts at `CHANGELOG.md:51`. Probe before implementing: `git tag --contains 584f97a`
  prints nothing.
- Builds from 584f97a up to the suppression guard wrote records under `[cluster, "cluster", X]`
  shared by both kinds; builds after the guard write none for that triple, because a suppressed
  export never reaches the record write (`prune_owned.go:183-185`). Read from source at f46afa2d1.
- A record's owner is opaque to the engine: it is hashed for the record path (`prune_owned.go:45-47`)
  and compared as a string (`:116`, `:269`). A new owner encoding therefore needs no record-schema
  change, and a record with a new owner still parses in the current reader (version 1, same fields,
  `DisallowUnknownFields` at `:108`).
- A Kubernetes namespace is a DNS-1123 label, so no namespace can render a component other than a
  lowercase label; the new owner's different arity makes collision with a legacy triple impossible
  regardless.
