# Design

## Context

See proposal.md. Identity, persisted metadata (ownership records) and deletion change, so the workflow
requires this design, an ownership invariant, adversarial cases and mutation controls. The options and
the decision are in [ADR-0422](../../../docs/adr/0422-inventory-export-identity.md).

## Goals / Non-Goals

**Goals:** a cluster inventory and a namespaced inventory in namespace `cluster` can never delete or
rewrite each other's files; cluster inventories prune their own stale files again; every inventory's
owner follows one rule, enforced by terminal asserts.

**Non-Goals:** see proposal.md. No path-layout change, no record-schema change, no compatibility with
records from earlier builds.

## Decisions

### D1. Identity travels in the request, not in the path

`sink.InventoryIdentity{Kind, Namespace, Name}` (`internal/sink/identity.go`) is a field of
`ExportEnvelopeRequest` and `ExportItemsRequest` (`internal/sink/export.go`). The namespaced reconciler
sets `{KollectInventory, inv.Namespace, inv.Name}`, the cluster reconciler
`{KollectClusterInventory, "", inv.Name}`, next to the object path. The object path keeps driving path
rendering; only the owner uses the identity.

### D2. One owner encoding for every inventory

`git.InventoryPruneOwner` (`internal/sink/git/prune_owner.go`) encodes
`["v2", kind, cluster, namespace, name]` as JSON. `cluster` is the sink's `spec.cluster` or `default`;
`name` is the identity's name, which never carries a multipart suffix. `resolveSnapshotExport`
(`internal/sink/layout_export.go`) builds it for every export that goes through `FileExporter`
(perResource, split, and YAML document mode, which also records ownership when the sink enables git
prune). The #394 `SuppressPrune` guard is deleted.

Why every inventory and not only the two ambiguous ones: with no compatibility burden there is no
reason to keep two encodings, and one rule is easier to assert.

### D3. Mandatory identity and consistency asserts (terminal)

- `RunExportEnvelope` checks a present identity against the object path before acquiring the backend:
  path namespace and base name (multipart suffix of the envelope's part index/total removed) must equal
  the identity's; a `KollectClusterInventory` must have path namespace `cluster`; the kind must be known;
  a namespaced kind needs a namespace and the cluster kind must not have one.
- `resolveSnapshotExport` refuses a `FileExporter` export without an identity: "git layout export of
  inventory/<ns>/<name> requires an inventory identity (kind, namespace, name)".
- `prepareOwnedPrune` calls `checkSingleClaims` on the full record set it is about to commit and fails
  with "ownership records would claim %q twice: <owner> and <owner>". With the foreign-claim check in
  place this cannot fire through the engine today; it is the last line of defence against a future
  change to how records are assembled, and is tested directly.

### D4. The rejection names the owner

`checkForeignClaim` (`prune_owned.go`) fails with
`prune path "<p>" belongs to another inventory: <Kind> <namespace/name> (cluster "<c>"), ownership record .kollect-prune/<sha>.json`.
`describePruneOwner` decodes a v2 owner and prints any other owner as `unrecognised owner "<raw>"`.

### D5. Multipart manifest pre-claim

`ExportFilesOptions.PruneClaimPaths` / `Config.PruneClaimPaths` carry paths a later call of the same
export will write. For every non-final part of a prune-bearing set with a shared `PrunePlan`,
`setManifestClaim` puts the set-manifest path there, and `validateOwnedPrunePaths` checks it against
other owners' records like a written path. Claim paths are neither written nor recorded. Both backends
forward the field.

### D6. No UID: recreate continues the record

Export paths contain no UID, and neither does the owner. A recreated inventory with the same kind,
namespace and name continues its predecessor's record: its first complete export deletes the
predecessor's recorded paths it no longer projects, and nothing else. `deletionPolicy: Retain`
concerns only the deletion event (ADR-0421 note). A kind or name change never continues a record.

## Risks / Trade-offs

- [Silent overwrite becomes a loud rejection] → a cluster inventory and a namespaced inventory in
  `cluster` with the same name that project the same file (split index, set manifest, a shared resource
  under the default template) now get "belongs to another inventory" on whichever exports second. Same
  rule as for every other pair. Fix: rename one inventory or use a separate branch.
- [Old records block exports] → a repository holding triple-owner records from earlier `main` builds
  rejects exports of the paths they list until those records are deleted by hand. Accepted: not in
  production.
- [A deleted inventory's record blocks a successor of another kind or name] → pre-existing; release on
  deletion is a later slice.
- [GitLab branchMR] → the claim check reads the target branch; two inventories can claim one path on
  separate feature branches. Later slice.
- [Fingerprint cache] → every owner changes, so each inventory's first export after the upgrade is
  never coalesced. Harmless.
