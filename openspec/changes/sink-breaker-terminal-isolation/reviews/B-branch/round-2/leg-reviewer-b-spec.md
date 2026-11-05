## Verdict: CLEAN

## Findings
- [NOTE] The troubleshooting row calls the breaker's counting period a "60 s counting window", but gobreaker's `Interval` is a fixed cycle — `docs/operator-manual/troubleshooting.md:214`, `internal/sink/circuit_breaker.go:61`
  Failure: gobreaker clears the closed-state counts when each 60 s generation ends. Five counted failures can straddle that boundary, for example three at t=58 s and two at t=61 s. The breaker then stays closed, while an operator reading "window" expects a rolling 60 s window and expects it to open. The breaker itself behaves as SCB-2 says ("within its closed-state counting interval"); only the operator wording is loose.
  Fix: write "within one 60 s counting interval (counts reset every 60 s while closed)".
  Confidence: 55

Checked, and holds:
- **Round-1 #1 (per-site pinning):** `internal/sink/git/prune_scoped_marker_test.go:979-1081` has one row for each routed site: `prune_owned.go:174,268,275,317`, the two `checkSingleClaims` calls at `:340` and `:342`, and `prune_target.go:66`. Each row calls the function that raises the error. The control row (`:135`, `invalidPrune`) stays unmarked. I traced the inputs of each row through `prepareOwnedPrune`, `validateOwnedPrunePaths`, `pruneKeepSet` (`git/export.go:250`) and `checkSingleClaims`; each one reaches the line it names. For example, the sort order in the two `checkSingleClaims` rows selects `:340` and `:342` separately. Mutants M1–M8 are recorded in `evidence/B1.md:71-81`.
- **Round-1 #2 (go-git engine):** `prune_scoped_marker_test.go:1089` drives `exportRemoteInWorkdir` and then `ClassifyExportError`. What the test skips does not affect the marker: `exportRemote` only picks auth and the workdir (`git/export.go:152-181`), `withRepoExportLock` returns `fn()` unchanged (`export_lock.go:28`), and `ClassifyExportError` returns a Terminal error unchanged (`git/errors.go:29`). The design *Risks* text was narrowed to match.
- **Round-1 #6:** the SCB-1 text now reads "make the error Terminal". That matches `errors.go:354-360`, the doc comment, and `TestInventoryScoped_alwaysTerminal`.
- **Round-1 #7/#8/#9:** these were deferred or rejected. I found no new evidence on them.
- **SCB-1:** holds.
  - `IsExcluded: kollecterrors.IsInventoryScoped` is at `circuit_breaker.go:68`. The breaker sees the raw `plan.run` error before the reclassification and wrap at `export.go:200-212`.
  - The wraps keep `%w`, so `errors.Is` still finds the marker.
  - The text is kept: `errors.go:371`.
  - Scenarios: `circuit_breaker_test.go:643,660,688,851`.
- **SCB-2:** holds. Every non-excluded error counts (`circuit_breaker.go:63-65`), and an open breaker returns a Transient "circuit breaker open" error (`:39-40`). Tests: `circuit_breaker_test.go:710,739`.
- **SCB-3:** holds through gobreaker's half-open exclusion together with `MaxRequests: 1` (`:59`). Tests: `circuit_breaker_test.go:778,798,821`.
- **Test isolation:** `halfOpenBreaker` puts a breaker in the shared registry under a fresh key, so a `ResetBreakersForTest` that ran during these tests would replace it and break them. I looked for one that could. Every caller in package `sink` runs serially: `circuit_breaker_test.go:164,184`, `release_on_delete_test.go`, `inventory_identity_test.go`, and none of them calls `t.Parallel`. Go runs serial tests before the paused parallel ones, so no reset overlaps a half-open test. The `git`-package fuzz tests reset a separate process's registry.

Beyond what the spec asks: the branch makes controller-test sink names unique (`kollectinventory_export_test.go:193-201`), runs the debounce metric test serially, adds an ADR 0602 note and updates the troubleshooting rows. Marking the "owner > 4096 bytes", "keep set > maxPrunePaths" and "written path missing from keep set" sites inventory-scoped matches the T0.1 classification, since each is about the exporter's own configuration.

## Could not check
- I ran no sensors. The pass results above come from `evidence/B1.md`.
- No holistic `task check` is recorded at head `d243fd5f4`; the last one ran on `4e50a9a2f`. That means `verify`, `scrub`, `gitleaks` and `spec:validate` have not been re-run on the B1.1 tree. B1.1 recorded lint, markdown, unit and race runs.
- I did not re-run mutants M0–M8.
- I did not check CI at the head, and did not confirm that `skipWithoutGit` does not skip in CI.
- I did not check the gobreaker v2 source for its `IsExcluded` half-open semantics; I relied on design.md and the SCB-3 tests.
- I did not read the full evidence files (G1–G3, T0.1), the earlier review rounds, or the untracked `reviews/B-branch/round-2/` contents.
