# Tasks

Slice 1 of INVENTORY-IDENTITY-01. Option A for every inventory, decided by the operator on
2026-10-04 (ADR-0422).

## 1. Behavioural tests first (IEI-1..IEI-9)

- [x] 1.1 Sink tests through `RunExportEnvelope`, the Git backend (CLI engine) and a local bare remote
  (`internal/sink/inventory_identity_test.go`): cluster inventory prunes its own files down to empty
  (IEI-3); disjoint L/N trees with empty snapshots (IEI-2); shared path rejected with the owner named
  and the owner's bytes kept (IEI-2, IEI-7); foreign set manifest rejected on part 1 (IEI-6)
- [x] 1.2 Sink tests with a recording backend: owner per identity (IEI-1); identity/path mismatch and
  missing identity are terminal before the backend (IEI-4)
- [x] 1.3 Engine tests (`internal/sink/git/prune_identity_test.go`): golden owner bytes and record paths
  (IEI-1, IEI-9); rejection names the owner and record (IEI-7); claim paths checked against other
  owners (IEI-6); duplicate claims refused (IEI-5)
- [x] 1.4 Controller tests (`internal/controller/inventory_identity_export_test.go`): both reconcilers'
  tree exports reach the engine with their kind-qualified owner and prune requested (IEI-1)
- [x] 1.5 Extend `FuzzOwnedPrune`: L is a real `KollectClusterInventory` that prunes; N is
  `KollectInventory cluster/platform` (same object path); owner label in every resource file; opcode 5
  (identity on another object path); FORBIDDEN checks for duplicate claims and record/model divergence
  (invariant 7), a set committing parts while its manifest is foreign (8), an accepted identity/path
  mismatch (9), and rewritten bytes of another owner (1, 5)

## 2. Implementation

- [x] 2.1 `sink.InventoryIdentity` on `ExportEnvelopeRequest` and `ExportItemsRequest`; both reconcilers
  set it
- [x] 2.2 Identity/path consistency check in `RunExportEnvelope` before the backend is acquired
- [x] 2.3 `git.InventoryPruneOwner` (`["v2", kind, cluster, namespace, name]`); identity mandatory for
  every `FileExporter` export; #394 `SuppressPrune` guard deleted
- [x] 2.4 `checkSingleClaims` on the records a record-advancing export would commit; owner-naming
  rejection text
- [x] 2.5 `PruneClaimPaths` (set-manifest pre-claim on non-final parts), forwarded by the Git and GitLab
  backends
- [x] 2.6 Mutation controls with a no-op control (Verification)

## 3. Docs and review

- [x] 3.1 ADR-0422 rewritten as accepted, without migration content; ADR-0419 "Exact file ownership"
  updated; ADR-0421 `Retain` note on recreated inventories; ADR index status
- [x] 3.2 Release-note text (docs/operator-manual/upgrading.md, "Git export ownership by inventory identity") (by hand, git-cliff uses titles only): cluster inventories prune again;
  shared-path rejection naming the owner; records from earlier `main` builds are not migrated
- [x] 3.3 CI green on the PR head (40 pass at 19ba4ac81 before the final rebase; re-run on the archive commit)
- [x] 3.4 Independent adversarial review (REQUEST CHANGES F1/F2 fixed: non-final foreign-path test, GitLab claim-path forwarding test; mutants M1, M9 killed)
- [x] 3.5 Archive the change as the last commit of the PR

## 4. Follow-ups (later slices, out of scope here)

- [ ] 4.1 Release an inventory's ownership record when the inventory is deleted (files kept, claims
  dropped), so a successor of another kind or name can take over shared paths
- [ ] 4.2 GitLab `branchMR` duplicate-claim safety: the claim check reads the target branch, so two
  inventories can claim one path on separate feature branches

## Verification

Rev: uncommitted worktree (branch `design/inventory-identity`, rebased on `origin/main`), local,
Go 1.26.6 (`GOTOOLCHAIN=go1.26.6`), linux/amd64, `umask 022`, envtest assets 1.36.0. Every red below
was observed before the production edit with the scaffolding in place (identity fields wired, old owner
and guard still active), so each failed on its assertion, not on compilation.

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| IEI-1 | `TestRunExportEnvelope_ownerFromIdentity`, `TestInventoryPruneOwner_golden`, `TestResolveSnapshotExportOwnershipStableAcrossParts`; controller `TestKollectInventory_exportCarriesIdentity`, `TestKollectClusterInventory_exportCarriesIdentity` | engine receives `["v2",kind,cluster,ns,name]` with prune requested; owners of the two kinds differ; suffixes and generations do not change the owner | pass | red: sink test got `["default","cluster","platform"]` with `SuppressPrune:true`; controller tests got `["default","default","team-inventory"]` and `["default","cluster","platform"]` (prune false). Green: `go test -v -run ... ./internal/sink/ ./internal/sink/git/ ./internal/controller/`, all `=== RUN` and `--- PASS` |
| IEI-2 | `TestRunExportEnvelope_clusterKindAndClusterNamespaceIsolated`, `TestRunExportEnvelope_sharedPathRejectedNamesOwner`; `FuzzOwnedPrune` seeds with L and N | L's empty snapshot removes only L's files; shared path rejected, L's bytes unchanged | pass | red: "cluster inventory file default/clusterrole/admin.yaml still present after its empty snapshot"; "err = nil, want a terminal rejection". Green as above |
| IEI-3 | `TestRunExportEnvelope_clusterInventoryPrunesOwnFiles`; fuzz seeds 5 (shrink, empty, multipart) and 6 (A→B→A replay, interrupted L set) | exactly previous-minus-current deleted; empty snapshot clears; replay restores; interrupted set deletes nothing | pass | red: "repository files = [.../api.yaml .../web.yaml], want [.../api.yaml]". Green as above |
| IEI-4 | `TestRunExportEnvelope_identityPathMismatchRejected` (7 cases), `TestRunExportEnvelope_treeExportRequiresIdentity`; fuzz opcode 5 seed | terminal error, backend not called, no file or record changed | pass | red: "namespaced kind without ns: err = nil"; "identity-less tree export: err = nil". Green as above |
| IEI-5 | `TestCheckSingleClaims_rejectsDuplicate`; fuzz invariant 7 (records decoded after every step: no path twice, each record equals the model) | terminal error naming the path; committed records never claim a path twice | pass | red: "duplicate claim: err = nil" (stub). Green as above |
| IEI-6 | `TestRunExportEnvelope_multipartForeignManifestRejectedOnPart1`, `TestOwnedPrune_claimPathsCheckedAgainstOtherOwners`; fuzz seed 9 (invariant 8) | part 1 rejected naming the manifest; no file of the set committed | pass | red: "part 1 of a set whose manifest is foreign: err = nil"; "foreign claim path: err = nil". Green as above |
| IEI-7 | `TestOwnedPrune_rejectionNamesOwningInventory` (memory and disk fs), `TestRunExportEnvelope_sharedPathRejectedNamesOwner` | error contains `KollectInventory cluster/platform (cluster "default")` / `KollectClusterInventory platform (cluster "default")` and `.kollect-prune/<sha256(owner)>.json` | pass | red: rejection was `prune path "..." belongs to another inventory` with no owner. Green as above |
| IEI-8 | owner has no UID (`TestInventoryPruneOwner_golden`), so a recreated inventory sends the same owner and continues the record (`TestRunExportEnvelope_clusterInventoryPrunesOwnFiles` shrink step); other kind does not inherit (`..._clusterKindAndClusterNamespaceIsolated`) | same identity prunes predecessor's stale paths only; other kind deletes nothing of the old record | pass | by construction plus the tests named; no test deletes and recreates a CR, because the request carries no UID to differ |
| IEI-9 | `TestInventoryPruneOwner_golden` (record path = SHA-256 of owner); fuzz invariant 7 decodes every record (`version`, `owner`, `paths`); existing strict-reader tests unchanged | record format unchanged | pass | green as above |
| all | existing `TestOwnedPrune_rejectedExportLeavesOtherOwnersBytes` | unchanged, passes | pass | `go test -v -run TestOwnedPrune_rejectedExportLeavesOtherOwnersBytes ./internal/sink/git/` |
| IEI-3 | mutation (a): restore the #394 cluster guard in `resolveSnapshotExport` | tests fail | pass | exit 1; failing: 5 sink tests, `TestKollectClusterInventory_exportCarriesIdentity`, fuzz seeds 5–10 (invariant 7). File restored, sha256 `6c9a6a7b…` before = after |
| IEI-4 | mutation (b): drop `checkInventoryIdentity` call in `RunExportEnvelope` | tests fail | pass | exit 1; failing: `TestRunExportEnvelope_identityPathMismatchRejected`, fuzz seed 10 ("FORBIDDEN (invariant 9) ... was accepted"). Restored, sha256 `6850cdf9…` |
| IEI-5 | mutation (c): drop the `checkSingleClaims` call in `prepareOwnedPrune`; (c1) make its body return nil | tests fail | pass | (c) exit 1: shared-path, owner-naming, fingerprint, symlink/collision tests, `TestOwnedPrune_rejectedExportLeavesOtherOwnersBytes`, fuzz seeds 0, 5, 8 (invariants 1, 5). (c1) exit 1, additionally `TestCheckSingleClaims_rejectsDuplicate`. Restored, sha256 `5a6f269d…` |
| IEI-6 | mutation (d): `PruneClaimPaths = nil` (no part-1 manifest pre-claim) | tests fail | pass | exit 1; failing: `TestRunExportEnvelope_multipartForeignManifestRejectedOnPart1`, fuzz seed 9 (invariant 8). Restored, sha256 `6c9a6a7b…` |
| IEI-1 | mutation (e): owner encoded without the kind (`["v2",cluster,ns,name]`); (e2) owner from the path triple | tests fail | pass | (e) exit 1: golden, owner, controller and owner-naming tests. (e2) exit 1: 5 sink tests, both controller tests, all 12 fuzz seeds. Restored, sha256 `9eeeeb7f…` / `6c9a6a7b…` |
| IEI-7 | mutation (f): rejection text without owner and record | tests fail | pass | exit 1; owner-naming tests and `TestCheckSingleClaims_rejectsDuplicate`. Restored |
| all | no-op mutation control (comment added to `layout_export.go`) | survives | pass | exit 0, same command. Restored |
| all | mutation command | judged by exit status | pass | runner outside the repo: `go test -count=1 ./internal/sink/ ./internal/sink/git/ ./internal/controller/ -run 'Identity\|Owner\|OwnedPrune\|CheckSingleClaims\|RunExportEnvelope\|ResolveSnapshotExport\|exportCarriesIdentity\|FuzzOwnedPrune' -v`, judged by exit status; each file restored byte-identically (sha256 checked) |
| all | `go test -race ./internal/sink/... ./internal/controller/...` (N=1, exact command) | pass, or only the known race | pass (known race only) | exit 1: 2 data races, both `ResetBreakersForTest` in `TestResetBreakersForTest_clearsOpenBreaker` vs `exportThroughBreaker` (SINK-BREAKER-RACE-01); the other `--- FAIL` lines are "race detected during execution of test" collateral. All other packages ok |
| all | `go test -race -count=3 -skip '^TestResetBreakersForTest_clearsOpenBreaker$' ./internal/sink/`; `go test -race -count=1 ./internal/controller/...` repeated 3 times (Ginkgo refuses `-count>1`) | pass | pass | sink exit 0 (N=3); controller exit 0 three times. `-count=3` over `./internal/sink/...` with the known test included: 3 races, all SINK-BREAKER-RACE-01 |
| all | `go test -run='^$' -fuzz=FuzzOwnedPrune -fuzztime=60s ./internal/sink/git/` | no failure | pass | exit 0, 11,719 execs, 43 new interesting inputs (377 total incl. the local fuzz cache); no failing input |
| all | `task lint` (golangci-lint v2.11.4 + go-arch-lint); `go vet ./...`; `go vet -tags integration ./internal/sink/...` | pass | pass | lint: 0 issues, arch-lint OK; vet exit 0 both |
| all | `task test` with `GOFLAGS=-buildvcs=false` (the subagent sandbox blocks the git call Go uses for VCS stamping) | pass | pass | exit 0, 47 packages ok; envtest 1.36.2 downloaded by the Makefile; generated `config/` and `zz_generated` identical to the main checkout |
| all | `go test -tags integration ./internal/sink/ -run 'Multipart\|Prune\|LayoutExport\|RunExportEnvelope\|Resource'` | pass | pass | exit 0 (local Git remotes; the Docker-backed integration tier was not run) |
| all | `task spec:validate`; `task lint:markdown` (docs:verify skipped: needs a Python 3.12 venv) | pass | pass | spec:validate exit 0 (3 items passed, `change/inventory-export-identity` strict-valid); lint:markdown exit 0, 189 files, 0 issues |
| all | CI on the PR head | required checks green | not-run | needs a pushed PR |
| — | independent adversarial review | APPROVE | not-run | not the author |
