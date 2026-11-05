## Verdict: CONCERNS

## Findings
- [WARNING] Five of the six converted raise sites have no test that checks the marker. Turning any of them back into `invalidPrune` leaves every test green. — `internal/sink/git/prune_target.go:66`, `internal/sink/git/prune_owned.go:174,268,275`
  Failure: only `foreignClaimError` (`prune_owned.go:317`) is exercised end to end, by SCB-1e (`circuit_breaker_test.go:522-556`). A grep for `IsInventoryScoped` finds no test in `internal/sink/git`. If someone changes the merge-target claim rejection (GitLab MR / branch export) back to `invalidPrune`, it starts counting again and opens the breaker for every inventory, and no test fails.
  Fix: add a table test in `internal/sink/git` that asserts `IsInventoryScoped && IsTerminal` for each `ownershipConflict` site. Calling `checkMergeTargetClaims` (as `prune_merge_target_test.go:318` already does) and `prepareOwnedPrune` is enough.
  Confidence: 90
- [WARNING] The design says SCB-1e covers "marker lost by a re-wrap … through the real Git backend". It only covers the CLI engine. — `internal/sink/inventory_identity_test.go:62`, `internal/sink/git/export.go:115-131` vs `:134-149,186-215`
  Failure: the fixture's `file://` endpoint makes `isFileRemote` route to `exportViaCLI`. The go-git engine (`exportRemoteInWorkdir` → `checkMergeTargetClaims` / `writeBillyExportFiles`), which serves production https/ssh sinks, is never driven through the breaker. Reading the code, the chain does survive: `ClassifyExportError` returns early on Terminal (`git/errors.go:29`), and `classifyExportFailure` wraps with `%w`. But that is not tested.
  Fix: narrow the claim in the design.md Risks section, or add a go-git-engine case alongside the unit table above.
  Confidence: 85
- [NOTE] SCB-1 says the marker "SHALL keep the error's Terminal class". `InventoryScoped` sets Terminal unconditionally (stronger than spec). — `internal/errors/errors.go:156-162`
  Failure: `InventoryScoped(Transient(x))` comes out Terminal, so a transient cause would stop being requeued. Every current caller passes a fresh unclassified `fmt.Errorf`, so nothing breaks today.
  Fix: document "always Terminal" in the spec and the doc comment, or preserve an existing class.
  Confidence: 60
- [NOTE] design.md:67-68 says `ErrTooManyRequests` "becomes Transient in classifyExportFailure". It actually goes through `git.ClassifyExportError` first, for every sink type, and comes out as `git transport: too many requests`. — `internal/sink/export.go:208`, `internal/sink/git/errors.go:45,72`
  Failure: during a half-open trial, a concurrent export on an S3 or Postgres sink logs a misleading "git transport" error. The troubleshooting row also does not mention this message. The behaviour is pre-existing.
  Fix: map `gobreaker.ErrTooManyRequests` in `exportThroughBreaker` the same way as `ErrOpenState`, or correct design.md.
  Confidence: 75
- [NOTE] Some own-path rejections still count toward the breaker. The prune-path checks at `prune_owned.go:53,57` are listed "undecided" in T0.1 (#1–2). The empty export at `git/export.go:95` is the same case (my reading; T0.1 does not list it). — `internal/sink/git/export.go:95,100`
  Failure: all of these fail inside `plan.run` without touching the backend, and still count. The spec allows this fallback, and I could not find a reachable input that triggers them.
  Fix: if a reachable input turns up, mark the error at the `validateExportFiles` call site, where the scope is known.
  Confidence: 40

**Compliance**
- **SCB-1: holds.**
  - `IsExcluded: kollecterrors.IsInventoryScoped` at `circuit_breaker.go:68`.
  - Tests for the streak case (`:359-378`) and the other-inventory case (`:331-356`).
  - Marker survival holds on the CLI engine (SCB-1e). The go-git engine is unverified, as in the second finding.
- **SCB-2: holds.**
  - `circuit_breaker_test.go:381-427` covers transient, unclassified, forbidden, NotFound and Git-auth Terminal.
  - The open state maps to Transient "circuit breaker open" at `circuit_breaker.go:39-41`.
  - Gaps: the 60 s interval is untested, and the Git-auth case uses a stub with a pre-classified error, not a real backend.
- **SCB-3: holds.**
  - `:449-518` covers half-open followed by success, by failure, and a concurrent export during the trial.
  - The concurrent case asserts only `err != nil`, not the error's class, which matches the spec.

**Behaviour no requirement mentions**
- Two limits now skip the breaker: the 4096-byte owner limit (unreachable for k8s names) and `maxPrunePaths`.
- `SinkErrorsTotal` still counts excluded failures.
- An inventory that keeps failing ownership checks can keep the breaker half-open indefinitely. It does not lock out other inventories, because each excluded trial frees the slot.

**Docs**: the troubleshooting rows (`troubleshooting.md:214,328`) and the ADR note match the code: key format, 60 s/30 s, counted classes, half-open semantics.

## Could not check
- Did not run any tests or `task check`. The ones behind gobreaker v2.4.0 behaviour (that `onExclusion` frees the half-open slot) are `circuit_breaker_test.go:449-518`, and I read only the diff and the design's citation of the library.
- Did not read the controller test diffs (`internal/controller/*_test.go`) in depth, or `evidence/G1-G3.md`, `loop.md`, `tasks.md`, `proposal.md`, or the committed review records. I checked the rewritten spec only against the code, not against those records.
- Did not check whether other backends (BigQuery, MongoDB) raise inventory-scoped Terminal errors that should also be marked.
- Did not check what `reviews/B-branch/` contains (untracked).
