# Tasks

Slice 2 of INVENTORY-IDENTITY-01: release the ownership record on inventory deletion under both
deletion policies (operator decision, 2026-10-05).

## 1. Behavioural tests first (ROD-1..ROD-7, IEI-8)

- [x] 1.1 Engine tests on a local bare remote, both engines (`internal/sink/git/release_test.go`):
  Retain removes only the own record; Delete removes candidates, own recorded paths and the record and
  keeps a candidate another owner records; ownerless delete keeps recorded paths; absent record commits
  nothing; a record at the own path that names another owner is refused
- [x] 1.2 Sink tests through `RunCleanupExport` and the real Git backend
  (`internal/sink/release_on_delete_test.go`): L/N under Retain and Delete; shared identity; successor
  of another kind takes over; recreate starts a new record; unreachable backend is transient; missing
  identity is terminal
- [x] 1.3 Legacy directory prune keeps recorded paths, both engines (`release_test.go`)
- [x] 1.4 Controller tests: both finalizers pass their identity (owner asserted in the Retain and the
  two shared-identity tests, `cleanup_policy_test.go`); a failing release keeps the finalizer
  (`TestKollectInventoryReconciler_failedReleaseKeepsFinalizer`); envtest with the real reconciler and a
  real repository for Delete and Retain (`kollectinventory_cleanup_live_git_envtest_test.go`)
- [x] 1.5 `FuzzOwnedPrune` opcode 6 (delete inventory X under Retain, Delete, or Delete with a shared
  identity) through `RunCleanupExport`; model tracks record existence; invariants 10 (record released),
  11 (Retain deletes or rewrites nothing), 12 (another owner's record and files untouched); seeds 12, 13

## 2. Implementation

- [x] 2.1 `sink.CleanupExportRequest.Inventory`; both finalizers set it (`cleanupTarget.inventory`)
- [x] 2.2 `git.ReleaseOptions`, `Config.ReleaseOnly`, `planRelease`, ownership-aware candidate removal,
  exact removal of own recorded paths and the record, release commit subject (`release.go`, `delete.go`)
- [x] 2.3 `ReleaseExport` on the Git and GitLab backends; `sink.OwnershipReleaser`
- [x] 2.4 `RunCleanupExport` routing (design D3); the release call carries a commit context built from
  the identity (TODO for `CommitContext.Kind` from the parallel GitLab lane)
- [x] 2.5 Legacy directory prune skips recorded paths (`prune_tree.go`)
- [x] 2.6 Mutation controls with a no-op control (Verification)

## 3. Docs and review

- [x] 3.1 ADR-0421 (`Retain` contacts Git/GitLab sinks to release the record), ADR-0422 (release on
  deletion; recreate starts a new record; follow-up removed), CRD field doc and generated CRDs/chart
  CRDs/schema golden, `docs/crds/kollectsnapshotsink.md`, upgrade note
- [x] 3.2 Independent review: REQUEST CHANGES (fuzz model, dead-code guard, spec.cluster pin, plus F4–F7) → APPROVE at `bc6d62c80`; mutants X2, X3, X5, X9, F6 and R10 killed
- [x] 3.3 Archive the change as the last commit of the PR

## Verification

Rev: uncommitted working tree of branch `feat/release-ownership-on-delete` (base origin/main,
which includes #414); the coordinator commits. Local, Go 1.26.6 (`GOTOOLCHAIN=go1.26.6`, `GOFLAGS=-buildvcs=false`),
linux/amd64, `umask 022`, envtest assets 1.36.0. Reds were observed with the API scaffolding in place
(types, `ReleaseExport` delegating to the old deletion, old routing), so each failed on its assertion.
Controller-level reds are shown by mutation M1b, which restores the old routing.

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| ROD-1 | `TestReleaseExport_retainRemovesOnlyOwnRecord`, `..._deleteRetractsOwnTreeKeepsForeign` (cli, go-git); `TestRunCleanupExport_retainReleasesRecordKeepsFiles` (both kinds), `..._deleteRetractsOwnTreeAndRecord`, `..._sharedIdentityReleasesRecordOnly`, `..._gitReleaseNeedsIdentity`; envtest "releases the ownership record ... (Delete/Retain)"; fuzz invariant 10 | record absent after deletion under both policies and shared identity; no identity is terminal before the backend | pass | red: engine "Retain release changed [inventory/team-a/apps.json ...]"; sink "Retain deletion of KollectClusterInventory platform changed []"; "identity-less git cleanup: err = <nil>". Green: `go test -v -run ...` all `=== RUN`/`--- PASS` |
| ROD-2 | `TestReleaseExport_retainRemovesOnlyOwnRecord`, `TestRunCleanupExport_retainReleasesRecordKeepsFiles`, envtest Retain; fuzz invariant 11 | only the record changes, one commit, subject "release inventory ownership record" | pass | red: Retain deleted the three candidate files (engine). Green as above |
| ROD-3 | `TestRunCleanupExport_retainReleasesRecordKeepsFiles` (L and N), `..._deleteRetractsOwnTreeAndRecord`, `TestReleaseExport_foreignRecordAtOwnPathRefused`; fuzz invariant 12, seed 12 | other owner's record and files byte-identical; a record at the own path naming another owner is refused terminally | pass | red: "release of a record naming another owner: err = <nil>". Green as above |
| ROD-4 | `TestReleaseExport_deleteRetractsOwnTreeKeepsForeign`, `TestDeleteExport_ownerlessKeepsRecordedPaths`, `TestExportFiles_legacyDirectoryPruneKeepsRecordedPaths` (cli, go-git); fuzz seed 13 | own tree, candidates and record removed in one commit; foreign recorded candidate kept; ownerless delete and legacy prune keep recorded paths | pass | red: "Delete release changed [... apps.manifest.json ...]" (B's file deleted, A's tree kept); "ownerless delete changed [...]"; "legacy directory prune deleted default/team-b/deployment/api.yaml". Green as above |
| ROD-5 | `TestReleaseExport_absentRecordIsNoop` (absent record under both modes, retry after success); fuzz seed 13 (`del(oA)` twice) | no error, no commit | pass | regression guard: passed before the change too (old deletion of nothing was already a no-op); M6 shows it catches a non-idempotent release |
| ROD-6 | `TestRunCleanupExport_gitReleaseFailureIsRetried` (git/gitlab build fails, missing remote), `TestKollectInventoryReconciler_failedReleaseKeepsFinalizer` | transient error, finalizer kept, not reported as retained by policy | pass | red: "outcome 3, err = <nil>; want a transient error"; "Retain release against a missing remote succeeded". Controller red via M1b |
| ROD-7 | `TestRunCleanupExport_successorTakesOverReleasedPaths`; fuzz seed 12 | successor export succeeds, its record lists the path | pass | red: "prune path ... belongs to another inventory: KollectClusterInventory platform". Green as above |
| IEI-8 | `TestRunCleanupExport_recreateStartsNewRecord`; fuzz seed 13 (replay after delete) | predecessor's files kept, new record lists only the new snapshot | pass | red: "repository files = [.../api.yaml], want [.../api.yaml .../web.yaml]". Green as above |
| ROD-1 | mutation M1: skip the record removal (Retain plan empty, Delete plan without the record) | tests fail | pass | exit 1: engine Retain/Delete, 5 sink release tests, envtest, fuzz |
| ROD-1, ROD-6 | mutation M1b: Retain/shared returns before the backend for git (pre-change routing) | tests fail | pass | exit 1: sink release tests, `failedReleaseKeepsFinalizer`, Retain and both shared-identity controller tests, envtest Retain, fuzz seeds 12, 13 |
| ROD-3 | mutation M2: Delete removes every owner's record | tests fail | pass | exit 1: `..._deleteRetractsOwnTreeKeepsForeign` (cli, go-git), `..._deleteRetractsOwnTreeAndRecord`, fuzz seed 13 |
| ROD-3 | mutation M2b: owner derived without the kind (path triple as `KollectInventory`) | tests fail | pass | exit 1: L Retain release, shared identity, successor, recreate, cluster shared-identity controller test, fuzz seed 12 |
| ROD-2 | mutation M3: Retain deletes files (`ReleaseOnly` ignored) | tests fail | pass | exit 1: engine Retain (cli, go-git), sink Retain (both kinds), shared identity, recreate, envtest Retain, fuzz seeds 12, 13 |
| ROD-4 | mutation M4: Delete ignores other owners' records | tests fail | pass | exit 1: `..._deleteRetractsOwnTreeKeepsForeign`, `TestDeleteExport_ownerlessKeepsRecordedPaths` |
| ROD-4 | mutation M5: legacy directory prune ignores records | tests fail | pass | exit 1: `TestExportFiles_legacyDirectoryPruneKeepsRecordedPaths` (cli, go-git) |
| ROD-5 | mutation M6: an absent record is an error | tests fail | pass | exit 1: `TestReleaseExport_absentRecordIsNoop`, envtest, fuzz seed 13 |
| ROD-6 | mutation M7: a Retain release error is swallowed | tests fail | pass | exit 1: `TestRunCleanupExport_gitReleaseFailureIsRetried/git/remote_unreachable` |
| all | no-op mutation M0 (comment in `release.go`) | survives | pass | exit 0 |
| all | mutation command | judged by exit status | pass | a scratch copy: `go test -count=1 -v ./internal/sink/ ./internal/sink/git/ ./internal/controller/ -run 'Release\|DeleteExport_ownerless\|legacyDirectoryPrune\|RunCleanupExport\|FuzzOwnedPrune\|Reconciler_\|TestControllers'`; every file restored and compared with `cmp` to a pristine copy |
| all | `go test -race -count=1 -skip '^TestResetBreakersForTest_clearsOpenBreaker$' ./internal/sink/... ./internal/controller/...` | pass | pass | exit 0, 20 packages ok |
| all | `go test -race -count=3 -skip '^TestResetBreakersForTest_clearsOpenBreaker$' ./internal/sink/...`; `go test -race -count=1 ./internal/controller/...` twice more (Ginkgo refuses `-count>1`) | pass | pass | sink exit 0 (N=3); controller exit 0 (N=3 runs in total) |
| all | `go test -run='^$' -fuzz=FuzzOwnedPrune -fuzztime=60s ./internal/sink/git/` (first revision) | no failure | fail | exit 0 locally, 27,598 execs; the reviewer's 60 s run found the model defect F1 below, so this row is superseded |
| all | `task lint` (golangci-lint v2.11.4 + go-arch-lint); `go vet ./...`; `go vet -tags integration ./...` | pass | pass | 0 issues, arch-lint OK; vet exit 0 both |
| all | `go test -count=1 ./...` | pass | pass | exit 0, 47 packages ok (incl. `test/schema` with the regenerated golden) |
| all | `task spec:validate`; `task lint:markdown`, plus markdownlint on the four untracked change files (the task lints tracked files only) | pass | pass | spec: 4 passed, `change/release-ownership-on-delete` strict-valid; markdown: 190 files, 0 issues; change files 0 issues |
| all | CRDs regenerated (controller-gen v0.20.1), `hack/helm-sync-crds.sh`, `UPDATE_GOLDEN=1 go test ./test/schema/` | generated files match the new field doc | pass | `task verify` not run (needs the install step's network) |
| ROD-1..5 | F1: `FuzzOwnedPrune` crashers `"2800A1081C081"` and `"2012001001C100"` kept as seeds 14 and 15 | pass after the model fix | pass | red: seed 14 "deletion of B ... must delete exactly [... 5 paths], deleted [3]"; seed 15 "FORBIDDEN (invariant 3) ... must delete exactly [default/team-a/service/api.yaml ...]". Cause: the model, not the engine. A record lists what the export projected, so it can list absent files; a retried final part could also resume a set begun before the owner's deletion. Fix: predictions count only present files, a complete export's record replaces the owner's claims, and a deletion drops the owner's pending multipart accumulators (in production a `PrunePlan` lives for one reconcile). Green: `go test -run FuzzOwnedPrune`, all 16 seeds |
| all | `go test -run='^$' -fuzz=FuzzOwnedPrune -fuzztime=60s ./internal/sink/git/`, twice | no failure | pass | run 1: exit 0, 20,748 execs; run 2: exit 0, 16,558 execs; no failing input |
| ROD-6 | F4: `TestRunCleanupExport_gitReleaseFailureIsRetried/git/remote_unreachable` now asserts the error is not terminal | transient | pass | mutation X9 (`classifyCleanupFailure` always terminal): exit 1, killed by this test and `TestClassifyCleanupFailure_PreservesErrorClass` |
| ROD-1, ROD-3 | F3: `TestRunCleanupExport_releaseUsesSinkCluster` (Retain, Delete): one inventory exported through a `spec.cluster: prod` sink and a default-cluster sink of one repository | the prod deletion changes only the prod record (and prod files under Delete) | pass | mutation X3 (`resolved.Cluster = ""` in `cleanupPruneOwner`): exit 1, both subtests fail |
| ROD-2 | F5: `TestReleaseExport_damagedForeignRecord` (cli, go-git) | Retain succeeds and removes only the own record; Delete is terminal and changes nothing | pass | mutation X5 (Retain loads every record strictly): exit 1, both engines fail |
| ROD-4 | F2: dead `removeWorktreeCandidates` deleted; `TestReleaseInWorktree_UntrackedDirtIsTolerated` and `..._SkipsMissingAndNonMatching` call the go-git production path `releaseInWorktree` | untracked dirt removed from disk, not reported, not staged | pass | mutation X2 (the go-git stage reports untracked dirt as removed): exit 1, killed by `TestReleaseInWorktree_UntrackedDirtIsTolerated` |
| ROD-2 | F6: `TestBackend_ReleaseExport_RetainNoRecordSkipsMergeRequest` (GitLab branchMR, the deleted inventory's feature branch exists, no record) | no merge request is opened or reopened | pass | mutation (the `KeepFiles` skip disabled): exit 1, killed by this test |
| IEI-8 | F7: the delta renames IEI-8 to "Delete and recreate starts a new record" (`## RENAMED Requirements`) and modifies it under the new name | heading and body agree | pass | `task spec:validate` 4 passed; an archive dry run in a scratch copy applied "~ 1 modified, → 1 renamed" and the living spec carries the new heading |
| all | review-round mutations, a scratch copy: `go test -count=1 -v ./internal/sink/ ./internal/sink/git/ ./internal/sink/gitlab/ ./internal/controller/`, judged by exit status, each file restored and compared with `cmp`; no-op control (comment in `release.go`) | no-op survives, the rest fail | pass | no-op exit 0; X2, X3, X5, X9 and the F6 mutant exit 1 |
| all | review round: `task lint`, `go vet ./...`, `go vet -tags integration ./...`, `go test -count=1 ./...`, race gate as above (sink `-count=3`, controller 3 runs), `task spec:validate`, `task lint:markdown` | pass | pass | lint 0 issues, arch-lint OK; vet exit 0 both; 47 packages ok; race exit 0 throughout; spec 4 passed; markdown 0 issues |
| — | CI on the PR head | required checks green | pass | 40 pass at `bc6d62c80`: https://github.com/PlatformRelay/Kollect/actions/runs/37287503962 (codecov/patch advisory red: error branches in release.go/delete.go; project 88.11→87.97%, repo floor 90% met) |
| — | independent review | APPROVE | pass | APPROVE at `bc6d62c80` after one REQUEST CHANGES round |
