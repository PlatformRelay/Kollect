## Verdict: CONCERNS

## Findings
- [CRITICAL] Task 1.1's stated probe result is false and task 1.2 misses a whole file: `internal/sink/circuit_breaker_test.go` references `RunExportItems`/`ExportItemsRequest` (lines 20–128), and the probe scope `internal/` would show it. — `openspec/changes/dead-exported-surface/tasks.md:9`
  Failure: deleting the symbols leaves `TestResetBreakersForTest_clearsOpenBreaker` (not matched by the `TestRunExportItems_*` prefix) referencing them → `go test ./internal/sink/...` fails to compile, contradicting task 1.3's "green".
  Fix: add `circuit_breaker_test.go` to 1.2; migrate both tests to `RunExportEnvelope` and correct 1.1's expected output.
  Confidence: 95

- [CRITICAL] Proposal assumption "deleting the dead runner's tests loses no reachable coverage" is contradicted: the circuit breaker is production-live via `RunExportEnvelope` → `exportThroughBreaker` (`internal/sink/export.go:267`), and `circuit_breaker_test.go` holds the only direct breaker-trip assertions (`TestRunExportItems_circuitBreakerTripsAfterRepeatedFailures`). — `openspec/changes/dead-exported-surface/proposal.md:85`
  Failure: executing task 1.2 deletes the only tests of a live feature; the 90% floor may hold while the behaviour goes unverified.
  Fix: migrate the breaker tests to the live `RunExportEnvelope` path instead of deleting; restate the assumption.
  Confidence: 90

- [WARNING] Task 4.2's migration list omits `internal/sink/git/export_integration_test.go`, which calls package-level `Export` under `//go:build integration` (line 37). — `openspec/changes/dead-exported-surface/tasks.md:29`
  Failure: after deleting `git.Export`, task 4.3's `go vet -tags integration ./internal/sink/git/` fails to compile the tagged file.
  Fix: add `export_integration_test.go` to 4.2's call-site list.
  Confidence: 95

- [WARNING] Task 3.2 misses `MarshalTargetJSON`'s call in `TestStoreSubscribeAndMarshal` (`internal/collect/store_test.go:252`), which task 3.2 does not name. — `openspec/changes/dead-exported-surface/tasks.md:22`
  Failure: deleting `MarshalTargetJSON` breaks compile of that test; task 3.4's "collect suite green" is unreachable as written.
  Fix: extend 3.2 to adapt `TestStoreSubscribeAndMarshal` to `MarshalTargetExport` (it also tests Subscribe/Count/Namespace marshal, so it must be adapted, not deleted).
  Confidence: 92

- [WARNING] Proposal (line 41) and tasks header (line 5) call the zero-caller probe the per-task "red"; the openspec `tasks` rule says "a compile error … is not a red; say what assertion failed". — `openspec/changes/dead-exported-surface/tasks.md:5`
  Failure: the verification narrative violates the repo's own spec-workflow contract; reviewers cannot distinguish a genuine red from a probe.
  Fix: mark the red as N/A with reason (pure deletion, no observable behaviour) and keep the probe as evidence, not as a red.
  Confidence: 80

- [NOTE] Requirement traceability: the proposal defines no requirement IDs, yet the tasks invent `DR-1`…`DR-10` (incl. `DR-4b`), which the Verification table maps to. — `openspec/changes/dead-exported-surface/proposal.md:16`
  Failure: `DR-9`/`DR-10`/`DR-4b` have no anchor in the spec, so "map each requirement ID" cannot be checked.
  Fix: number the proposal's What-Changes bullets to match, or drop unmapped ids.
  Confidence: 85

- [NOTE] Task 5.1 says the four aliases have "~19 sites"; there are 33 test references, in 8 non-sink packages (`internal/controller`, `internal/pipeline`) that would need to import `internal/sink/cap`. — `openspec/changes/dead-exported-surface/tasks.md:34`
  Failure: the estimate understates the migration and hides the cross-package import change; `sink.Capabilities = cap.Capabilities` makes it type-safe but non-trivial.
  Fix: correct the count and name the packages to migrate.
  Confidence: 88

## Could not check
- Did not run `go build`/`go test`/`task coverage` (read-only session); findings are static, verified against HEAD `af6c58f5`.
- Did not read `reviews/R/*` (other reviewers' legs — out of bounds by instruction).
- Did not execute the integration-tagged suites; only confirmed the build tag and call sites.
- Did not verify `MergeRequestAPI`, `RemoveCluster`, `BindClusterTargetNamespaces` truly have zero production reach beyond grep (grep showed definition + tests only, but reflection/tagged files unruled out).
