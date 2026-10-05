# Design

## Context

See proposal.md. This changes persisted metadata checks (ownership records) and the identity of a
merge-request branch, so the workflow requires a design. Slice 3 of INVENTORY-IDENTITY-01, the
follow-up "GitLab `branchMR` mode" in [ADR-0422](../../../docs/adr/0422-inventory-export-identity.md).

Where the existing check looks: `prepareOwnedPrune` reads the records of the checked-out working tree.

| Mirror | Checked-out tree in merge-request mode | Sees claims merged into the target since the last push? |
| --- | --- | --- |
| cold (`file://` remotes, first export) | new feature branch at the fetched target tip | yes |
| warm, feature branch exists locally (`https://` GitLab, production) | the local feature branch, unchanged | **no** |

So the review's "validates against the target" holds only for a cold mirror. In the warm production
path the check reads a stale tree, and that is where a duplicate claim gets pushed.

## Goals / Non-Goals

**Goals:** a merge-request export never pushes a feature branch whose merge into the current target
would claim a path twice; no merge request carries another inventory's records; a polluted branch
says which record to delete.

**Non-Goals:** see proposal.md.

## Decisions

### D1. Option (a): check the target tip as well as the feature branch, refuse before writing

Options evaluated:

| Option | What it closes | Cost |
| --- | --- | --- |
| (a) check claims against the fetched target tip and the checked-out feature branch, refuse before writing | every case where the conflicting claim is on the target when this inventory exports | read one tree that is already fetched; no API, no new fetch |
| (b) rebase the feature branch onto the target when it diverged, or refuse | (b)'s refuse half is (a). The rebase half replays the inventory's commits onto a tree with a conflicting record and still needs (a)'s check | a rebase step in both engines, conflict handling, force-push of the feature branch |
| (c) also check the claims of other open merge requests | the race of two merge requests open at once | list open MRs (GitLab and Gitea APIs differ), then fetch every source branch or call the files API per MR and record: 1 + N × (1 + records) requests per export. Branches of closed MRs need filtering, or they block forever |

(a) is the smallest change that closes the sequential case, which is the one the warm production path
hits. (c) is not cheap: the API client today only lists MRs by source branch and creates them, and a
check over other branches needs either a git fetch of every feature branch in both engines or several
API calls per merge request on every export. Not done (D4).

Implementation: `checkMergeTargetClaims` (`internal/sink/git/prune_target.go`) runs when the push
branch differs from the clone branch and the export has a prune owner. It reads the target tip
(`refs/heads/<target>` in a go-git mirror, `refs/remotes/origin/<target>` in a CLI mirror, opened with
`git.PlainOpen`), copies the `.kollect-prune` entries of its tree into a memory filesystem
(`pruneMetadataFromTree`, bounding entries and bytes before reading a blob and keeping symlinks and
non-files so the strict loader rejects them), loads them with the existing `loadPruneRecords`, and
refuses any path in the keep set plus `PruneClaimPaths` that another owner's record lists. The paths
are the same set `validateOwnedPrunePaths` checks against the feature branch. One reader serves both
engines, and the check is generic in the git package: any export to a branch other than its clone
branch is headed for a merge into that clone branch.

The error reuses the "belongs to another inventory" wording of IEI-7 and adds the target branch:
`prune path "<p>" belongs to another inventory on merge target branch "<target>": <Kind ns/name>
(cluster "<c>"), ownership record .kollect-prune/<sha>.json`. It is terminal: retrying cannot help
until an operator changes the inventories or the repository.

Why the result is sound for the sequential case: merging a feature branch leaves the target's records
plus the one record the feature branch rewrites (its own; D2 makes that true). That record lists
exactly the paths checked. If none of them is another owner's on the target, and the target itself
has no duplicate (otherwise `loadPruneRecords` already fails), the merge result claims each path once.

### D2. One branch per inventory: `prefix/_cluster/<name>` for cluster inventories

`BranchNameForExport(prefix, kind, namespace, name)`: a `KollectInventory` keeps
`prefix/<namespace>/<name>`; a `KollectClusterInventory` gets `prefix/_cluster/<name>`. A namespace is
a DNS-1123 label and cannot contain `_`, so the two kinds never collide and there is no ref
directory/file clash between them. Namespaced inventories' branches do not move; only cluster
inventories' branches do, which the operator accepted (no compatibility burden).

Before, the second of the two same-path inventories pushed to the first's branch, was rejected as
non-fast-forward and rebased onto it, so one merge request carried both records. The red run showed
one remote branch, `kollect/cluster/platform`, for both inventories.

The kind reaches the backend in the commit context: `RunExportEnvelope` sets `CommitContext.Kind`
from the request's inventory identity, which both reconcilers always send (IEI-1). `Export`,
`ExportFiles` and `DeleteExport` all name the branch with the same function.

The inventory deletion cleanup does not attach a commit context, so `DeleteExport` has no kind. For a
caller without a kind the branch follows the path: namespace `cluster` selects the cluster
inventory's branch (every cluster inventory has that namespace component), anything else the
namespaced one. The one wrong guess is a `KollectInventory` in a namespace called `cluster` deleted
in merge-request mode: its deletion goes to `prefix/_cluster/<name>` instead of its export branch.
The fix belongs to the cleanup path (attach `CommitContext{Kind}` from the deleting inventory), which
the parallel deletion slice owns.

### D3. The duplicate-ownership error names both owners and records

`loadPruneRecords` now fails with `duplicate ownership of "<p>": <owner A> (ownership record <A's
file>) and <owner B> (ownership record <B's file>); delete one of the two records to repair the
branch`. Before it said only `duplicate ownership of "<p>"`. Record files are named by the hash of
the owner, so without the owners an operator had to decode every record to find the pair. The record
paths are exact: `readPruneRecord` already asserts that each file's name is the hash of its owner.

### D4. Residual: two merge requests open at once

(a) does not see another inventory's unmerged feature branch. If A and B both push a claim on P before
either merges, both merge requests exist. What bounds it:

- If the two inventories wrote different bytes to P, the second merge request has a content conflict
  on P and GitLab refuses to merge it without a manual resolution.
- After the first merges, the other inventory's next export that is not coalesced by the export
  fingerprint (a changed snapshot, or the first export after a controller restart) fails with D1's
  error, naming the owner. An unchanged snapshot is skipped before the check runs.
- If both merge anyway, every export to the target fails with D3's error, which names both records;
  deleting one record repairs the branch, and the next export of that inventory is then refused by D1
  if it still claims the path.

Closing the race needs (c) or a different record layout (for example one claim marker file per path,
so two claims become a git add/add conflict; this changes the record format IEI-9 fixes and multiplies
the file count). Recorded as an open operator decision.

## Risks / Trade-offs

- [Stale feature branch is stricter than the target] → the feature-branch check still runs, so a
  foreign record that is on the stale local feature branch but was later removed from the target (by
  hand, or by record release on deletion once that lands) still refuses the export. Conservative: it
  refuses, it never writes. Removing the stale branch (or the warm mirror) clears it.
- [Fingerprint coalescing hides a new conflict] → see D4. Not changed here: bypassing the fingerprint
  in merge-request mode would add a fetch per reconcile per inventory.
- [CLI mirror read with go-git] → the CLI engine's workdir is opened with `git.PlainOpen` to read one
  tree. If go-git could not read the repository, the export fails with an error rather than skipping
  the check.
- [Old cluster-inventory branches] → merge requests on `prefix/cluster/<name>` opened by a cluster
  inventory before this change are no longer updated. Close them by hand.
