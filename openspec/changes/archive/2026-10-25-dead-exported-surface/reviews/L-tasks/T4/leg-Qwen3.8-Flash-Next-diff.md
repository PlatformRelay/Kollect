## Verdict: CLEAN

## Findings
- [NOTE] Test function names still reference the deleted symbol — `internal/sink/git/export_test.go:23,36,48` (`TestExportMemory*`) keep capital-E names after the identifier went away; evidence line 83 records this as intentional grep hygiene, so cosmetic only.
  Fix: none required, or rename to `TestExportMemoryHelper` for consistency with the row-10 grep claim.
  Confidence: 90 (verified present; judged intentional per T4.md)
- [NOTE] The commit-context derivation now exists in two copies — `internal/sink/git/backend.go:55-58` and `export_test.go:426-431` (`exportForTest`). If the production derivation diverges later, the ten migrated tests silently exercise stale wrapper semantics. Not a regression: the deleted `Export` was already a second copy (diff hunk removed verbatim, byte-identical to the new helper), and `Backend.Export`'s own path is covered at `backend_test.go:139`.
  Fix: none in scope; a future task could re-point these tests at `Backend.Export`.
  Confidence: 85
- [NOTE] The two `//go:build integration` files now call `exportForTest` defined in the *untagged* `export_test.go` — works today (helper visible under both builds), but a future re-tagging of `export_test.go` breaks the tagged build. Already guarded by the matrix row-8 sensor (`go vet -tags integration`), which I re-ran.
  Fix: none; keep row 8 in the check matrix.
  Confidence: 80

## Could not check
- Full package suite `go test -count=1 ./internal/sink/git/...` (571 s per evidence) — I ran only the `-run TestExport` subset (164.8 s, ok); other tests in the package not re-run.
- The integration-tagged tests themselves (need a live Forgejo/network); only their compilation was verified.
- `golangci-lint` / `make lint` not run; CI pipeline not inspected.
- R2#9 quote (version-monotonicity test deletion) references files outside this diff — not verifiable here; this diff touches only the four in-scope files, matching evidence row 11.

Checked independently: helper body byte-identical to the deleted wrapper and to `Backend.Export`'s derivation; all four `//go:build integration` files confirmed tagged, `export_test.go` untagged; repo-wide greps — only `git.ExportFiles*` (live, different symbol) and test-function names survive, zero callers of either deleted symbol, no docs/config hits; `go build ./...`, `go vet ./...`, `go vet -tags integration ./internal/sink/git/ ./internal/pipeline/ ./internal/sink/` all exit 0; no allow-list, exclusion, baseline or coverage-config widening in the diff (4 files, all Go); diff stat (+72/−65) matches T4.md line 87.
