## Verdict: CLEAN

## Findings
- [NOTE] The test helper `exportForTest` is a byte-for-byte copy of production `Backend.Export`'s commit-context derivation, so the ten migrated tests now exercise the copy, not the production path — `internal/sink/git/export_test.go:426-433` vs `internal/sink/git/backend.go:54-61`. Failure: a future change to `Backend.Export`'s derivation would not be caught by these ten tests. Mitigation present: `backend_test.go:139` calls `Backend.Export` directly, so the production derivation stays covered; the copy is intentional per tasks.md 4.2. Fix (only if you want the ratchet): have the helper construct a `Backend{cfg, auth}` and call its `Export`, removing the duplicate body. Confidence: 90.
- [NOTE] `exportMemory` retains `time.Now()` as commit time (`internal/sink/git/export_test.go:459`). Non-deterministic, but it was verbatim in the deleted production function and is confined to a test helper, so no behavioural change. Confidence: 95.

## What I checked (machine-verified)
- Diff `a1e54499..HEAD` is exactly 4 files, +72/−65; only the four declared in-scope files touched (`git diff --stat`).
- `Export` (export.go) and `ExportMemory` are absent from all non-test Go: `rg 'git\.Export\b|ExportMemory' --glob '*.go'` returns only the three `TestExportMemory*` function names (`export_test.go:23,36,48`).
- All 10 `Export(` call sites migrated to `exportForTest`: 6 in `export_test.go`, 1 in `export_integration_test.go:37`, 3 in `export_forgejo_integration_test.go:48,98,130`. The only remaining `Export(` definitions are the live `Backend.Export` (`backend.go:54`) and the `memBackend` registry mock (`owned_prune_fuzz_test.go:318`) — different symbols.
- `go build ./...` = 0, `go vet ./...` = 0, `go vet -tags integration ./internal/sink/git/` = 0 (tagged-file compile proof required by R1#3 is present and passes; the package has exactly four `//go:build integration` files, all compiled by that one command).
- Targeted affected tests green: `go test -count=1 -run 'TestExportMemory|TestExportFileRemote|TestExportGoGit_nonFastForwardCommitPolicy$'` = ok, 34.7s.
- Non-Go text sweep `rg 'ExportMemory|git\.Export\b' docs/ config/ charts/ test/ hack/` = exit 1 (no hits).
- Commit `17e508fe` body names both removed symbols and the migration shape; `refactor:` without `!`, appropriate for `internal/` symbols.
- R1#3 loop decision satisfied: 4.2 lists all three test files, 4.3 keeps the tag-on vet. R2#9 is a T3 (store) row, not touched by this diff.

## Could not check
- Full `go test ./internal/sink/git/...` (evidence claims 571s; I ran only the export-focused subset).
- `export_forgejo_integration_test.go` runtime execution (needs a live Forgejo server; compiled only).
- Coverage floor (90%) after moving `ExportMemory` out of the production file into `_test.go`.
- Whether any out-of-module consumer referenced the removed symbols (impossible: `internal/` is unimportable externally).
