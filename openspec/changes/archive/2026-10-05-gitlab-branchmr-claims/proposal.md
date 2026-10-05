# Proposal

## Why

The GitLab sink's `merge_request` mode (`spec.gitlab.mergeRequest.mode: merge_request`) clones the
target branch, commits to a per-inventory feature branch and opens a merge request
(`internal/sink/gitlab/backend.go`). The ownership check (`prepareOwnedPrune`,
`internal/sink/git/prune_owned.go`) reads the records of the checked-out tree. In a warm mirror, the
production path for an `https://` GitLab remote, that tree is the inventory's own feature branch as
it was last pushed (`checkoutMirrorBranch` keeps an existing branch as is): a claim another inventory
merged into the target since then is not on it. The export pushes a second claim on the same path,
the merge request merges cleanly (the record files differ), and from then on `loadPruneRecords`
fails with `duplicate ownership of "<path>"` for every inventory exporting to that target branch. The
error named neither owner nor record file, so an operator could not tell which record to delete.

The feature branch was also derived from the object path, `prefix/<namespace>/<name>`
(`BranchNameForExport`, `internal/sink/gitlab/mr.go`). Since #414 a `KollectClusterInventory` `X` and
a `KollectInventory` `X` in namespace `cluster` are distinct owners, but both export as
`inventory/cluster/X` and so pushed to the same branch, `prefix/cluster/X`. The second exporter's push
was rejected as non-fast-forward and rebased onto the first's commits, so one merge request carried
both inventories' records, including two claims on a file both project.

## What Changes

- An export to a branch other than the one it cloned (a merge-request feature branch) is also checked
  against the ownership records of the freshly fetched target tip, in both git engines, before
  anything is written. A path another inventory claims there is refused with a terminal error naming
  the path, the target branch, the owning inventory and its record file. Nothing is pushed.
- The feature branch is per inventory kind: `prefix/<namespace>/<name>` for a `KollectInventory`
  (unchanged) and `prefix/_cluster/<name>` for a `KollectClusterInventory`. `_` cannot occur in a
  namespace, so no two inventories share a branch. The export pipeline passes the kind to the backend
  in the commit context.
- The `duplicate ownership` error names both owners and both record files, and says to delete one.
- **BREAKING (accepted, no migration):** a cluster inventory's merge requests move from
  `prefix/cluster/<name>` to `prefix/_cluster/<name>`. An open merge request on the old branch is not
  updated any more; close it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `git-tree-ownership`: adds the merge-request target check, the per-kind branch and the
  duplicate-ownership error text (IEI-10..IEI-12).

## Impact

- Entry points: both inventory reconcilers' snapshot export through `sink.RunExportEnvelope`
  (`internal/sink/export.go`, which now copies the identity's kind into the commit context) to the
  GitLab backend's `ExportFiles`/`Export` (`internal/sink/gitlab/backend.go`), and from there the git
  engines `exportRemote` (go-git, `internal/sink/git/export.go`) and `exportViaCLI` (CLI,
  `internal/sink/git/export_file.go`). The target check lives in `internal/sink/git/prune_target.go`.
- Only exports with a prune owner (git layout exports) are checked; document-mode exports without an
  owner make no claims and are unchanged.
- Direct-push mode and the plain git sink without a branch spec are unchanged: their pushed branch is
  the cloned branch, which the existing check already reads.

## Non-goals

- Claims on other open merge requests. Two merge requests opened before either merges can still
  claim the same path; see design.md D4 for why and what bounds it.
- Rewriting or closing another inventory's merge request, or retracting this inventory's own already
  pushed branch.
- Deletion (`DeleteExport`) and record release on deletion, which another slice owns. Deletion keeps
  using the same branch name function, with the kind inferred when the caller supplies none.
- Merge-request titles, which still read `<namespace>/<name>` for both kinds.
- Migration of branches or records.

## Assumptions

- The production GitLab path is the go-git engine with a persistent warm mirror, and a warm mirror
  checks out an existing local feature branch unchanged. Source: `exportRemote` →
  `prepareMirrorWorkdir` (`mirrorDirFor` for non-`file://` remotes) → `openOrWarmMirror` (fetches only
  the clone branch) → `checkoutMirrorBranch` (`internal/sink/git/mirror.go`). The CLI engine does the
  same (`prepareCLIWorkdir` → `gitCheckoutPushBranch`).
- After the fetch, the target tip is `refs/heads/<target>` in a go-git mirror and
  `refs/remotes/origin/<target>` in a CLI mirror. Source: the fetch refspec in `openOrWarmMirror`;
  `gitCheckoutPushBranch` already bases new branches on `origin/<cloneBranch>`.
- go-git can read the object database of a repository the git CLI cloned (loose and packed objects,
  shallow). Probe: `TestBranchExport_warmMirrorRefusesClaimMergedOnTarget/cli` opens the CLI mirror
  with `git.PlainOpen` and reads the target tree.
- A Kubernetes namespace is a DNS-1123 label, so it never contains `_`.
- Merging a feature branch into the target leaves the target's records plus the feature branch's
  version of the one record it changes: a feature branch only writes its own inventory's record.
