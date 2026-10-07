## Verdict: CLEAN

Mechanical rename executed faithfully. Deleted production surface is exactly the four test-only aliases (`internal/sink/export.go:41-51`); the `Capabilities` type alias at `export.go:39` is retained and genuinely load-bearing (`registry.go:30`, `export.go:42,109,154`). Repo-wide grep for the four alias names and four test names returns zero hits in `*.go`; the only remaining textual hits are prior review logs under `openspec/.../reviews/`, which are not code. All 21 probe call sites migrated to the matching `cap.*` constructor, each a direct one-to-one substitution of the former one-line delegation, so semantics are identical. `gofmt -l` on the touched files: clean. Import placement is gofmt-valid. `internal/sink/cap` is inside the `sink` component (`.go-arch-lint.yml` `in: internal/sink/**`), and both `controller` and `pipeline` already `mayDependOn: sink` — no new dependency edge, no allow-list widened.

## Findings
- [NOTE] Evidence undercounts its own import additions: it claims "added the `internal/sink/cap` import to all 11 test files", but 13 files gained it — every one of the 13 migrated files had zero `sink/cap` import at `db07534b` and now has exactly one — `openspec/changes/dead-exported-surface/evidence/T5.md` (Iterations §2)
  Failure: an evidence reader auditing the probe against the matrix gets an inconsistent count (the matrix row #2 correctly says 13 files), undermining trust in the record.
  Fix: change "11" to "13" in that sentence.
  Confidence: 95 (verified: `git show db07534b:<f> | grep -c sink/cap` = 0 and HEAD = 1 for all 13 files).
- [NOTE] Deleting `TestRelationalStoreCapabilities` dropped the only exact-flag pin on `RelationalStore()`; the surviving `cap` tests pin the other three constructors by full struct equality but only exercise `RelationalStore()` behaviourally — `internal/sink/cap/capabilities_test.go:31-83` (`TestCapabilityConstructors` omits `RelationalStore`)
  Failure: if `RelationalStore()` later gained `Stream:true` or `ObjectStore:true`, `TestCapabilityConstructors` and the `postgres`/`bigquery` tests (`caps != cap.RelationalStore()`) would both still pass, silently changing routing semantics for the only `SupportsDelete` backend.
  Fix: add `if got := RelationalStore(); got != (Capabilities{SupportsDelete: true}) { ... }` to `TestCapabilityConstructors` (one line).
  Confidence: 80 (the gap is real; impact is low because `ExportPayload` behaviour and backend comparisons still guard `SupportsDelete`).

## Could not check
- Did not rerun `go build ./...`, `go vet ./...`, or the three package test suites (relied on the T5.md sensor record; the 34s/8min costs were not worth re-incurring in a read-only pass).
- Could not independently reproduce the `internal/sink/git` `TestTestConnection_HTTPSchemeUsesLsRemote` 10-minute timeout or confirm the ENVIRONMENT classification — but the probe shows zero alias/cap references in that package, and the diff touches nothing it imports, so the classification is consistent with the evidence.
- Did not open the sibling review logs under `openspec/.../reviews/` (prior reviews, out of bounds for this run) beyond the grep hits that proved no code references remain.
