# Design

## Context

See proposal.md. Persisted metadata (ownership records) and deletion change, so the workflow requires
this design, an ownership invariant, adversarial cases and mutation controls. The owner encoding and
the record format are ADR-0422's and do not change.

## Goals / Non-Goals

**Goals:** a deleted inventory's ownership record does not survive its deletion on any reachable Git
or GitLab sink; `Retain` never deletes an exported file; no deletion removes a record or a file that
belongs to another inventory; the release is idempotent and keeps the finalizer's retry semantics.

**Non-Goals:** see proposal.md.

## Decisions

### D1. The owner comes from the identity, through the export's own function

`sink.CleanupExportRequest` gains `Inventory sink.InventoryIdentity`. Both finalizers set it next to
the object path (`cleanupTarget.inventory`): `{KollectInventory, namespace, name}` and
`{KollectClusterInventory, "", name}`. `RunCleanupExport` checks it against the object path
(`checkObjectPath`, the export's assert) and derives the owner with `inventoryPruneOwner`, the
function the export uses, on the layout resolved from the sink spec. For a Git or GitLab sink a
missing or inconsistent identity is a terminal error before the backend is built.

### D2. One release operation in the deletion engine, two modes

`git.ReleaseOptions{Owner, KeepFiles}` reaches `DeleteExportWithBranch` as `Config.PruneOwner` and
`Config.ReleaseOnly`. The Git and GitLab backends expose `ReleaseExport(ctx, paths, opts)`; the sink
calls it through the `sink.OwnershipReleaser` interface for every Git-family sink. A Git-family
backend without it is a terminal error, not a silent skip.

`planRelease` (`internal/sink/git/release.go`) decides everything on the checked-out branch before
anything is removed:

| Mode | Reads | Removes |
| --- | --- | --- |
| `KeepFiles` (Retain, shared identity) | only the file at `.kollect-prune/<sha256(owner)>.json`, strict decode, owner must equal the deleting owner | that file |
| owner, files (Delete) | every record (`loadPruneRecords`: strict, bounded, no duplicate claims) | candidate matches not listed by another owner's record, every path the own record lists, the own record |
| no owner (`DeleteExport`) | every record | candidate matches no record lists |

Removal happens in one commit with the existing scoped staging (`git add -A -- <paths>`, scoped
commit) on the CLI engine and index removal on go-git, under the repository export lock. A plan with
nothing to remove commits nothing; the stranded-commit delivery logic is unchanged. The `KeepFiles`
commit subject is `chore(<cluster>/<namespace>/<name>): release inventory ownership record`.

`KeepFiles` reads no other record on purpose: a damaged record of another inventory must not hold a
`Retain` deletion. `Delete` reads all of them because it must know what it may not delete; a damaged
record makes it terminal, as it makes every export to that repository terminal today.

### D3. Routing in `RunCleanupExport`

| Sink | Policy / state | Backend contact | Release call | Outcome |
| --- | --- | --- | --- | --- |
| Git, GitLab | Retain | yes | `KeepFiles` | `CleanupRetainedByPolicy` |
| Git, GitLab | Delete, shared identity | yes | `KeepFiles` | `CleanupRetainedSharedIdentity` |
| Git, GitLab | Delete | yes | files | as before (evidence rules unchanged) |
| S3, GCS, local | any | unchanged | none | unchanged |

Under a shared identity the two inventories' owners differ (ADR-0422), so releasing the deleting
inventory's record is safe; its document candidates may be the other's, so files are kept.

### D4. Failure semantics are the retraction's

A release error goes through `classifyCleanupFailure`: transient unless the backend classified it
terminal. The finalizer keeps its retry (transient: requeue with error; terminal: 5-minute re-check,
`kollect_cleanup_terminal_total`, Warning event, `force-cleanup` escape). A failure to build the
backend is classified as before (`ClassifyAPI`). Nothing new can drop the finalizer.

### D5. Legacy directory prune respects records

An ownerless tree export with prune (`PruneOwner == ""`, no production caller since ADR-0422) loads
the records and never removes a path any of them lists, on both engines.

### D6. Recreate starts a new record

The record is gone after a reachable deletion, so a recreated inventory with the same identity starts
a new record and prunes nothing of its predecessor's. IEI-8 is rewritten accordingly. A deletion that
could not reach the backend (sink gone, forced finalizer) leaves the record, and a recreated
inventory continues it, as before.

## Risks / Trade-offs

- [Retain now needs a working Git credential] → a Git or GitLab sink with a revoked credential holds
  the inventory in `Terminating`; the operator fixes the credential or sets
  `kollect.dev/force-cleanup`. Accepted by the operator decision; ADR-0421 says so.
- [Commit noise] → one commit per deleted inventory per Git sink, only when a record exists.
- [Sink spec drift] → the owner uses the sink's current `spec.cluster`. If it changed since the last
  export, the old record is not found and stays, like objects at an old `pathTemplate`.
- [GitLab branchMR] → the release lands on the feature branch and its merge request; the target branch
  loses the record when the MR merges. Same mechanism as the `Delete` retraction.
- [Damaged foreign record under Delete] → terminal until repaired, as for exports.
