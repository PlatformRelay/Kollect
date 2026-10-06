I'll avoid the `reviews/R/` directory (other reviewers' outputs). Reading the target's dependent code now.
Auth cache change (DR-8) checked at source. Now let me verify the zero-caller claims the proposal rests on, and read the git export file.
The change is a spec set (proposal + tasks) for a deletion-only refactor. Let me load the security-review skill as instructed, then check the two remaining breaker-related gaps.
## Verdict: CONCERNS

Deletion-only sweep, verified against source at this HEAD: every zero-caller claim holds (`RunExportItems`/`ExportItemsRequest` definition+tests+stale comment only; `MergeRequestAPI` zero refs; condition constants only at `api/v1alpha1/constants.go:10-11` with no text hits in config/charts/docs/specs/openapi/site; `RemoveCluster` is the sole shard-deleter and `RemoveTarget` never deletes shards, `store.go:194-217`; `Backend.Export` calls `ExportWithBranch` directly, `git/backend.go:60`; aliases have zero non-test refs). Security trace of the auth-cache change is clean: cache key is SHA-256 of the bearer token (`auth.go:112-113`), the hit path discards identity (`_ = user`, `auth.go:122`), so dropping `user` from the entry cannot change authz decisions and removes username/groups from process memory. No secrets, no new dependencies, live-path validation (`validateExportFiles`, malicious path/branch tests `validate_test.go:386-400`) untouched. One plan defect below.

## Findings

- [WARNING] Blanket premise "deleted tests exercise only the unreachable `RunExportItems` path" (proposal.md:87) is false for the circuit-breaker tests — `openspec/changes/dead-exported-surface/proposal.md:87` vs `internal/sink/circuit_breaker_test.go:20,77`
  Failure: task 1.2 deletes both breaker tests; they are the only automated proof that a repeatedly failing sink trips the breaker, refuses exports while open, and recovers after reset — that behaviour is reachable in production (`exportThroughBreaker`, `internal/sink/export.go:267`, under `RunExportEnvelope`) but exercised only through the dead wrapper (`circuit_breaker_test.go:20-75,77-130` are the only trip/open tests in the tree). Resource-isolation regression ships unproven; no task migrates them, and task 1.3 cannot catch a deleted test.
  Fix: extend task 1.2 to migrate the two breaker tests to drive `RunExportEnvelope` (the same test-local-helper pattern task 4.2 already uses) instead of deleting them.
  Confidence: 93
- [NOTE] Task 3.2's adaptation list misses a live caller of `MarshalTargetJSON` — `openspec/changes/dead-exported-surface/tasks.md:22` vs `internal/collect/store_test.go:252`
  Failure: `TestStoreSubscribeAndMarshal` also calls `MarshalTargetJSON`; following task 3.2 verbatim breaks the collect test compile. Task 3.4's `go vet`/suite gate catches it, but the task text is incomplete.
  Fix: add `store_test.go` to task 3.2's adapt list.
  Confidence: 90
- [NOTE] Combined test name hides a non-dead writer's coverage — `internal/collect/engine_setters_test.go:35`
  Failure: `TestEngineSetScrubKeysAndBindClusterTargetNamespaces` also covers `SetScrubKeys`; "delete tests that exist only to exercise the deleted writer" (task 6.2) read literally deletes `SetScrubKeys` coverage with it. Only the coverage floor (task 8.1) would notice.
  Fix: task 6.2: keep the `SetScrubKeys` half, strip only the `BindClusterTargetNamespaces` calls.
  Confidence: 85

## Could not check

- `data/kollect-xconsol-final/report.md` cited at proposal.md:5 — outside this repository (no `data/` at root); out of bounds per run rules.
- PR https://github.com/PlatformRelay/Kollect/pull/451 (proposal.md:68) — external.
- `openspec/changes/dead-exported-surface/reviews/R/*` — other reviewers' legs; deliberately not read.
- Execution of any probe/build/test (read-only run; all verification rows are `not-run`, as expected pre-implementation).
