## Verdict: CONCERNS

## Findings
- [WARNING] The deleted monotonicity guard was reproducible without the dead method, so the recorded rejection reason ("scenario unreachable without the dead method", loop.md:40) does not hold for the test. — `internal/collect/store_test.go:103` (pre-deletion), `internal/collect/store.go:42-48`
  Failure: the test lived in package `collect`, so `delete(s.shards, "ns-a")` + `Upsert` recreates the shard-recreation trap in five lines with `RemoveCluster` gone. Today `store.go:42-48` and `:179-182` still assert the version counter is never re-issued across shard deletion/recreation, but a future refactor moving `versions` onto `storeShard` now passes the whole suite: a reused version makes a stale fingerprint cache entry *hit* on wrong content.
  Fix: restore the test with `delete(s.shards, "ns-a")` in place of `s.RemoveCluster("ns-a")` (same package, no production surface needed), or delete the "survive shard deletion/recreation / never re-issued" clauses from the doc comments so the invariant is no longer claimed unguarded.
  Confidence: 85 (that the scenario is reachable from an in-package test: 100; the reject rationale is factually wrong as written)
- [NOTE] Coverage floor (90, `Taskfile.yml:13`) not re-measured in the evidence sensor table. — `openspec/changes/dead-exported-surface/evidence/T3.md:70-81`
  Failure: deleted code (`RemoveCluster`, the wrapper) was covered almost entirely by the deleted tests, so drift should be ≈0, but the floor is asserted, not measured; CI catches any surprise.
  Fix: one `hack/coverage.sh` run appended to the sensor table.
  Confidence: 90 that it was not run; low that it matters
- [NOTE] Reworded comment is now mid-flow: "counter bumped on every mutation (Upsert/Remove/\nRemoveTarget)" wraps awkwardly after the `RemoveCluster` entry was removed from the list. — `internal/collect/store.go:174-175`
  Fix: reflow the sentence. Cosmetic only.
  Confidence: 100

Verified clean (checked, not assumed): scope is exactly the four in-scope files (`git diff --stat`); zero remaining Go/non-doc references outside `openspec/` records (repo-wide ripgrep); the three `bumpNamespaceVersion` call sites (store.go:160, 200, 254) are all under a shard `mu`, so the "(shard mu)" lock-claim rewrite at :182 is true; `MarshalTargetExport(…, ExportMetadata{})` is byte-identical to the deleted wrapper's body, so all three re-pointed call sites preserve their assertions (`store_test.go:193`, `engine_extract_failure_test.go:257,288`); `store_cluster_test.go` held only `TestStoreRemoveCluster`, so whole-file deletion is correct; commit message matches content, including the prompt-correction note; re-ran `go build ./...`, `go vet ./...`, `go test -count=1 ./internal/collect/...` on HEAD — all green (11.1s).

## Could not check
- `loop.md` R1 F7 full text (only the round-2 re-rejection line and grep context read; the round-1 wording of the rationale could differ).
- CI-side coverage/arch-lint runs (only local build/vet/test executed; `.go-arch-lint.yml`, `.github/workflows/ci.yaml` contents not read).
