# Tasks

Slice 3 of INVENTORY-IDENTITY-01: GitLab merge-request mode and duplicate claims (ADR-0422
follow-up). Design: option (a) plus a branch per inventory kind and an owner-naming duplicate error
(design.md D1–D3); the open-MR race stays open (D4).

## 1. Behavioural tests first

- [x] 1.1 Engine test through both engines with a warm mirror over a local bare `file://` remote
  (`internal/sink/git/prune_merge_target_test.go`, `TestBranchExport_warmMirrorRefusesClaimMergedOnTarget`):
  B's branch predates A's merged claim; B's next export must be refused naming A and must not move B's
  branch; B's own snapshot stays exportable (IEI-10)
- [x] 1.2 `TestLoadPruneRecords_duplicateNamesBothOwnersAndRecords` (memory and disk filesystems):
  the duplicate-ownership error names both owners and record files (IEI-12)
- [x] 1.3 GitLab backend tests over `file://` remotes (`internal/sink/gitlab/backend_branchmr_claims_test.go`):
  the two same-path kinds get one branch each, each carrying one record (IEI-11); a claim merged into
  the target refuses another inventory's first export, nothing pushed (IEI-10);
  `TestBranchNameForExport_kindSeparatesSamePathInventories` pins the names (IEI-11)
- [x] 1.4 `TestCheckMergeTargetClaims_readsTargetTreeFailClosed`: foreign claim, foreign set-manifest
  pre-claim, own claim, no records, symlinked record, record directory as a file, missing target
  branch (IEI-10)
- [x] 1.5 `TestRunExportEnvelope_commitContextCarriesKind`: the pipeline hands the kind to the backend
  (IEI-11)

## 2. Implementation

- [x] 2.1 `exportRemoteInWorkdir` / `exportViaCLIInWorkdir` extracted (no behaviour change) so tests
  can drive a warm mirror over `file://`
- [x] 2.2 `checkMergeTargetClaims` + `pruneMetadataFromTree` (`internal/sink/git/prune_target.go`),
  called by both engines when the push branch differs from the clone branch
- [x] 2.3 `CommitContext.Kind`, set by `RunExportEnvelope` from the request identity
- [x] 2.4 `BranchNameForExport(prefix, kind, ns, name)`: `prefix/_cluster/<name>` for cluster
  inventories, kind inferred from the path namespace when absent; used by `Export`, `ExportFiles` and
  `DeleteExport`
- [x] 2.5 `loadPruneRecords` duplicate error names both owners and record files (one-line change in
  `prune_owned.go`)
- [x] 2.6 Mutation controls with a no-op control (Verification)

## 3. Docs and review

- [x] 3.1 ADR-0407 (branch name, target check), ADR-0422 follow-up, upgrade note
  (docs/operator-manual/upgrading.md)
- [ ] 3.2 Independent adversarial review (reviewer, revision, verdict)
- [ ] 3.3 CI green on the PR head
- [ ] 3.4 Operator decision on the open-MR race (design.md D4): accept, or add option (c)
- [ ] 3.5 Archive the change as the last commit of the PR

## Verification

Rev: implementation commit "fix(sink/gitlab)!: refuse claims already merged on the branchMR target" on
`fix/gitlab-branchmr-claims` (PR #416, based on `origin/main` including #414). Local, Go 1.26.6
(`GOTOOLCHAIN=go1.26.6`, `GOFLAGS=-buildvcs=false`), linux/amd64, `umask 022`, git CLI from PATH.
Every red below was observed with the scaffolding in place (seams extracted, `CommitContext.Kind` and
the `kind` parameter present but unused, no target check, old error text), so each failed on its
assertion, not on compilation.

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEI-10 | `TestBranchExport_warmMirrorRefusesClaimMergedOnTarget/{cli,go-git}` | refused, terminal, names path, A, A's record and `"main"`; B's branch unchanged; no branch whose merge claims a path twice; B's own snapshot pushed afterwards | pass | red (both subtests): `FORBIDDEN: merging kollect/team-b/apps into main claims "prod/team-a/Deployment/api.yaml" twice: ["v2","KollectInventory","default","team-a","apps"] and ["v2","KollectInventory","default","team-b","apps"]`. Green: `go test -count=1 -v -run '...' ./internal/sink/git/`, `=== RUN` and `--- PASS` for both subtests |
| IEI-10 | `TestBackend_BranchMR_refusesPathMergedForAnotherInventory` (GitLab backend, `file://`) | terminal error naming A; C's branch not created; no branch merges into a duplicate | pass | regression guard, not a red: it passed before the change too, because a cold `file://` mirror checks out the target tip (design.md, Context table). Green with `-v` |
| IEI-10 | `TestCheckMergeTargetClaims_readsTargetTreeFailClosed` (7 cases) | foreign claim and foreign manifest pre-claim refused; malformed metadata refused; own claim, no records, missing branch accepted | pass | written after the implementation; its defect detection is shown by mutations M1 and M6 below |
| IEI-11 | `TestBackend_BranchMR_clusterAndNamespacedInventoryUseSeparateBranches` | two branches, each with exactly one inventory's record, no duplicate on merge | pass | red: `feature branches = [kollect/cluster/platform], want one per inventory`. Green with `-v` |
| IEI-11 | `TestBranchNameForExport_kindSeparatesSamePathInventories` | `kollect/_cluster/platform` vs `kollect/cluster/platform`; kind-less inference | pass | red: `FORBIDDEN: KollectClusterInventory platform and KollectInventory cluster/platform share branch "kollect/cluster/platform"`. Green with `-v` |
| IEI-11 | `TestRunExportEnvelope_commitContextCarriesKind` | the backend's commit context carries the identity's kind | pass | written after the implementation; detection shown by mutation M4 |
| IEI-12 | `TestLoadPruneRecords_duplicateNamesBothOwnersAndRecords/{memory,disk}` | error names path, both owners and both record files | pass | red: `duplicate ownership of "prod/team-a/Deployment/api.yaml"` does not name `KollectInventory team-a/apps (cluster "default")`. Green with `-v` |
| all | M0 no-op: comment added to `mr.go` | survives | pass | exit 0 |
| IEI-10 | M1: `checkMergeTargetClaims` returns nil first | tests fail | pass | exit 1: warm-mirror test (cli, go-git), target-tree test (foreign claim, symlink, record directory as file) |
| IEI-10 | M1b: go-git call site disabled (`if false`) | tests fail | pass | exit 1: warm-mirror test `/go-git` |
| IEI-10 | M1c: CLI call site disabled | tests fail | pass | exit 1: warm-mirror test `/cli` |
| IEI-10 | M5: keep set dropped from the checked paths | tests fail | pass | exit 1: warm-mirror test (both), target-tree foreign claim |
| IEI-10 | M6: `PruneClaimPaths` dropped from the checked paths | tests fail | pass | exit 1: target-tree `foreign claim on the set manifest` |
| IEI-11 | M2: branch name reverted to the path-parsed one (`if false`) | tests fail | pass | exit 1: separate-branches backend test, branch-name test |
| IEI-11 | M2b: no kind inference for kind-less callers | tests fail | pass | exit 1: branch-name test |
| IEI-11 | M4: `commitCtx.Kind` not set in `RunExportEnvelope` | tests fail | pass | exit 1: `TestRunExportEnvelope_commitContextCarriesKind` |
| IEI-12 | M3: old `duplicate ownership of %q` text | tests fail | pass | exit 1: duplicate test (memory, disk) |
| all | mutation command | judged by exit status | pass | scratch copy `cp -a` under `a temporary directory`, `go test -count=1 ./internal/sink/ ./internal/sink/git/ ./internal/sink/gitlab/ -run 'BranchExport\|LoadPruneRecords_duplicate\|CheckMergeTargetClaims\|BranchMR_\|BranchNameForExport\|commitContextCarriesKind' -v`, exit status per mutant; each file restored (sha256 before = after) and the scratch tree compared equal to the worktree (`diff -rq`); baseline exit 0 before and after |
| all | `go test -race -count=3 -skip '^TestResetBreakersForTest_clearsOpenBreaker$' ./internal/sink/...` | pass | pass | exit 0, N=3, 19 packages ok; the skipped test is the known SINK-BREAKER-RACE-01 flake |
| all | `go test -tags integration -count=1 ./internal/sink/git/ ./internal/sink/gitlab/` | pass | pass | exit 0; includes the loopback git-http-backend warm-mirror tests and `TestExportGitLabMergeRequestMode` against Docker Forgejo (ran, `--- PASS`) |
| all | `task lint` (golangci-lint + go-arch-lint); `go vet ./...`; `go vet -tags integration ./internal/sink/...` | pass | pass | lint exit 0, `0 issues.`, arch-lint OK; vet exit 0 both |
| all | `task spec:validate`; `task lint:markdown`; markdownlint-cli2 on the untracked change files | pass | pass | spec:validate exit 0 (4 items, `change/gitlab-branchmr-claims` strict-valid); lint:markdown exit 0, 190 tracked files, 0 issues. `lint:markdown` lists tracked files only, so the new, uncommitted change files were linted directly with the same config: 7 files, 0 issues |
| all | `go test ./internal/controller/...` | pass | not-run | the controllers are unchanged; `RunExportEnvelope` only gains one field assignment, covered by the sink tests above |
| IEI-10 | two merge requests open at once (design.md D4) | — | N/A | not addressed by design; operator decision 3.4 |
| — | independent adversarial review | APPROVE | not-run | pending |
| — | CI on the PR head | required checks green | not-run | pending |

| IEI-10 | `TestSameBranchExport_warmMirrorRefusesClaimPushedMeanwhile` (cli, go-git) | push branch == clone branch, warm mirror: a claim pushed meanwhile is refused, no duplicate on the branch | pass | red on cli before the fix: `FORBIDDEN: merging main into main claims ... twice` (pull --rebase merged both records); go-git already refused. Fix: the CLI target check runs in every branch mode (found by independent review) |
| IEI-11 | document-mode `Backend.Export` / `DeleteExport` kind routing | branch chosen from the commit context kind | not-run | `Export` rejects `file://` endpoints, so it is not reachable with the local-remote fixtures; mutants passing kind "" there survive. Impact limited to a namespaced inventory in namespace `cluster` that shares a name with a cluster inventory, documented as "use separate sinks" |
