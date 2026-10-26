## Verdict: CONCERNS

## Findings
- [WARNING] Evidence file contradicts itself: header still says `Status: FRAMED` / "Next action: implement", and matrix rows 3–14 are `not-run`, while the Sensors table below reports every one as pass and the commit is landed — a reader cannot tell whether T3 was actually verified. — `openspec/changes/dead-exported-surface/evidence/T3.md:7-8,16-27`
  Failure: reviewer or auditor opens the matrix, sees the deletion rows `not-run`, and cannot confirm the deletions compiled; the authoritative result lives in a different section.
  Fix: set Status/Next-action to closed and flip the machine/agent matrix rows to `pass` (or delete the duplicated Result column) so the matrix agrees with the Sensors run.
  Confidence: 95
- [NOTE] Task 3.2 told the implementer to adapt the envelope tests "via `SnapshotTarget` or the envelope the subscriber actually delivers"; the diff instead calls the surviving `MarshalTargetExport(..., ExportMetadata{})` directly. — `internal/collect/store_test.go:193`, `internal/collect/engine_extract_failure_test.go:257,288`
  Failure: none — the deleted wrapper's body was verbatim this call, so semantics are identical and every assertion is preserved. Flagged only as a literal deviation from the task wording, acknowledged in T3.md:88.
  Fix: none required; or amend the task text to name `MarshalTargetExport` as the adaptation target.
  Confidence: 90
- [NOTE] The deleted `TestStoreNamespaceVersion_MonotonicAcrossRemoveClusterAndShardRecreation` was the only guard for version survival across shard deletion/recreation; nothing now tests that invariant. — `internal/collect/store.go:179-186`
  Failure: if a future mutation path deletes a shard again, the version map could re-issue a value with no test to catch it. Currently unreachable (no `delete(s.shards,…)` remains), and the accepted spec-set decision (loop.md R1-F7 / R2-F9) is not re-litigated.
  Fix: none for this diff; the surviving `bumpNamespaceVersion` comments already document the property a reintroduced path must preserve.
  Confidence: 70
- [NOTE] Repo-root, gitignored local build artifacts still name the deleted symbols at their old line numbers (`coverage-summary.txt:156 RemoveCluster`, `:159 MarshalTargetJSON`). — `coverage-summary.txt:156,159`
  Failure: a future repo-wide `grep` audit for dangling references hits these stale files; they are untracked (`.gitignore:51-53`) so they never ship and are outside the diff.
  Fix: none required; regenerate coverage on the final tree (task 8.1) will drop them.
  Confidence: 85

## Could not check
- Full-module `go build ./...` / `go vet ./...` (timed out at 180 s); I ran `go vet ./internal/collect/` (exit 0, compiles tests) and `go test -count=1 ./internal/collect/` (ok, 13.2 s) instead — the collect package is the only one this diff touches.
- `task lint`, `task coverage`, `task spec:validate`, `task test`, and any `//go:build`-tagged compile (task 8 / 3.4 gates).
- External consumers of `internal/collect` (package is internal, so unimportable outside the module — compile cannot prove the external half).
- `openspec/changes/dead-exported-surface/reviews/L-tasks/T3/` (an earlier review, out of bounds per instructions) and the external anchor report outside the repo.
- Verified independently: both symbols have zero hits across all tracked files and non-ignored untracked files; `RemoveCluster` was the sole `delete(s.shards,…)` path (pre-HEAD store.go:212), and `MarshalTargetExport` is the live production envelope path (`internal/pipeline/wire.go:289`, `stdout.go:97`).
