# Proposal

## Why

Git and GitLab tree exports (`perResource`, `split`) prune through per-owner ownership records
(`.kollect-prune/<sha256(owner)>.json`, `internal/sink/git/prune_owned.go`). The owner was the JSON
triple `[cluster, inventoryNamespace, inventoryName]`, with namespace and name parsed back out of the
object path (`internal/sink/export.go`). The request carried no inventory kind.

A `KollectClusterInventory` named `X` exports with object path `inventory/cluster/X.json`
(`internal/controller/kollectclusterinventory_controller.go`), exactly what a `KollectInventory` `X` in
a namespace called `cluster` produces (`internal/controller/kollectinventory_controller.go`). Both got
the same owner, so either could prune the other's files. PR #394 (7cb3348) closed that hole by
suppressing prune whenever the namespace component is `cluster` (`internal/sink/layout_export.go`).

The suppression applied to every cluster inventory, because every cluster inventory uses that
namespace component: Git and GitLab tree exports of every `KollectClusterInventory` never pruned, and
an empty snapshot was a no-op. v0.21.0 contains neither #394 nor the ownership records (`CHANGELOG.md`
lists both as unreleased). The next release must not ship this regression.

## What Changes

- The export request carries an explicit inventory identity: kind, namespace (empty for the cluster
  kind) and name. Both controllers set it. The sink no longer infers the owner from the path.
- Every inventory's prune owner is the kind-qualified `["v2", kind, cluster, namespace, name]`.
- The `SuppressPrune` guard for namespace `cluster` is deleted. Cluster inventories prune their own
  stale files again, including empty snapshots.
- A git layout export without an identity, or with an identity that disagrees with its object path,
  is a terminal error.
- The ownership engine refuses a commit whose records would claim one path twice, and its "belongs to
  another inventory" error names the owning inventory and its record file.
- Every non-final part of a multipart set claim-checks its set manifest, so a set whose manifest
  belongs to another inventory commits nothing.
- **BREAKING (accepted):** records written by earlier `main` builds use the triple owner. They are not
  migrated; their paths count as another inventory's. Kollect is not in production.

Decided in [ADR-0422](../../../docs/adr/0422-inventory-export-identity.md) (option A, operator
decision 2026-10-04).

## Capabilities

### New Capabilities

- `git-tree-ownership`: which files a Git or GitLab tree export may delete and whose ownership record
  proves it.

### Modified Capabilities

None. Owned pruning has no living spec yet; its contract is
[ADR-0419](../../../docs/adr/0419-git-export-serialization-layout.md) "Exact file ownership for
pruning".

## Impact

- Entry points: the `KollectInventory` and `KollectClusterInventory` reconcilers' snapshot export,
  through `sink.RunExportEnvelope` (`internal/sink/export.go`) and `resolveSnapshotExport`
  (`internal/sink/layout_export.go`) to `ExportFiles` of the Git (`internal/sink/git/backend.go`) and
  GitLab (`internal/sink/gitlab/backend.go`) backends, and the ownership engine
  (`internal/sink/git/prune_owned.go`).
- Behaviour change for users: cluster inventories' tree exports delete their own stale files again. A
  cluster inventory and a namespaced inventory in namespace `cluster` that project the same file path no
  longer overwrite each other silently: the second export is rejected, as for any two inventories.
- Repository state: every inventory writes its record under the new owner on its next complete export.
  An old triple-owner record keeps claiming its paths until removed by hand.
- No CRD or path-layout change. Record schema unchanged (`version: 1`).

## Non-goals

- Independent storage for colliding projected paths (ADR-0422 option B).
- Releasing an ownership record when its inventory is deleted. Later slice.
- GitLab `branchMR` duplicate-claim safety across feature branches. Later slice.
- Migration, dual encoding or rollback handling for records from earlier builds (operator decision).
- The document-path collision handled by the deletion cleanup's shared-identity check
  (`internal/sink/cleanup.go`, ADR-0421). Unchanged.
- Non-tree sinks (S3, GCS, databases, events) and `kollect-pipeline`, which exports through
  `backend.Export` with no prune owner (`internal/pipeline/wire.go`).

## Assumptions

- Only the two reconcilers produce git layout exports through `RunExportEnvelope` in production.
  Probe: `grep -rn 'RunExportEnvelope(\|RunExportItems(' --include='*.go' cmd internal | grep -v _test.go`
  lists the two reconcilers, `RunExportItems` (no production caller) and the relational cleanup
  (`cleanup.go`, `SupportsDelete` backends only, never a git layout).
- A record's owner is opaque to the engine except for error messages: it is hashed for the record path
  and compared as a string, so a new owner encoding needs no record-schema change.
- A Kubernetes namespace is a DNS-1123 label and an object name cannot be empty, so the identity
  checks never reject a real inventory.
