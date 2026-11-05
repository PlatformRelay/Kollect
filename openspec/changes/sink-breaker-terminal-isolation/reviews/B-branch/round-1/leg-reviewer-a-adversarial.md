No CRITICAL defect found. SCB-1, SCB-2 and SCB-3 hold in the final implementation. The remaining concerns are regression coverage and inaccurate validation claims.

1. **WARNING — The merge-target ownership exclusion lacks a regression assertion.**  
   `internal/sink/git/prune_target.go:66` is a separate marked raise site, but the new boundary test at `internal/sink/circuit_breaker_test.go:522` exercises the ordinary ownership conflict. Existing merge-target tests assert Terminal class and message, without asserting the marker (`internal/sink/git/prune_merge_target_test.go:139`, `internal/sink/gitlab/backend_branchmr_claims_test.go:165`). Reverting just this site to `invalidPrune` would preserve those assertions while restoring cross-inventory breaker trips for this path. G1’s recorded site mutation covers only `foreignClaimError` (`evidence/G1.md:67`).

   **Fix:** Assert `IsInventoryScoped` in the warm-mirror merge-target test for both engines and in the GitLab boundary test. Add marker assertions for the other three newly marked validation sites, and verify each site-specific reversion fails.

2. **WARNING — The final task contract still prescribes a known-failing validation command.**  
   `openspec/changes/sink-breaker-terminal-isolation/tasks.md:21` requires `-count=3` across the controller package without excluding its Ginkgo suite. `evidence/G1.md:81` explicitly records that this command fails because Ginkgo rejects that repetition count. The implementation evidence uses a sensible substitute, but the final contract remains inconsistent.

   **Fix:** Specify separate commands: repeated race tests for errors/sinks; repeated controller tests with `-skip TestControllers`; and a separate single-run Ginkgo gate.

3. **NOTE — T0.1 overstates which cleanup paths bypass the breaker.**  
   `evidence/T0.1.md:132` says deletion/cleanup does not go through it. Git ownership release does bypass it, but relational cleanup calls `RunExportEnvelope` at `internal/sink/cleanup.go:379`, thereby using the shared breaker.

   **Fix:** Narrow the statement to Git/GitLab ownership release and snapshot deletion; explicitly document the relational exception.

4. **NOTE — “Green at every commit” is stronger than the evidence supports.**  
   `tasks.md:8` claims every commit is green. G2 records a failing debounce test before its follow-up fix (`evidence/G2.md:97`), and earlier commits retain the breaker-isolation flake being repaired. This does not demonstrate a compilation failure, but it contradicts an unconditional green-tree claim.

   **Fix:** Say that tests and implementation land together without deliberate red-test commits, while identifying inherited flakes and the commit that resolves each.

**Compliance and attack results**

| Requirement | Assessment | Evidence |
|---|---|---|
| SCB-1 | **Holds** | Marker preserves text and wrapping; `IsExcluded` excludes marked results without resetting failures (`internal/errors/errors.go:157`, `internal/sink/circuit_breaker.go:69`). |
| SCB-2 | **Holds** | Unmarked failures retain gobreaker’s failure semantics. Git classification preserves already-Terminal marked errors and leaves auth/protected-branch errors unmarked. |
| SCB-3 | **Holds** | Exclusions release the half-open slot without changing state; success closes and counted failure reopens. Concurrent-trial tests passed. |

- All five marked sites match T0.1’s classification. Shared metadata corruption, duplicate records and aggregate capacity limits remain unmarked.
- I found no marker loss through the Git CLI, go-git, GitLab, multipart or layout return paths. Relevant wrappers return errors directly or use `%w`; no reachable outer reclassification overrides the marked Terminal class.
- GitLab ownership checks use the same marked helpers. Object-store exports have no corresponding Git ownership rejection. Git cleanup’s unmarked ownership errors cannot trip this breaker.
- Repeated marked trials **can indefinitely retain half-open operation and deny competing exports while a trial runs**. The final design explicitly accepts this; SCB-3 provides slot release, not fairness or bounded recovery.
- Behavior outside the three requirements includes the three marked validation failures, controller-test isolation and debounce-test serialization. These are described in the design/tasks.
- G2 preserves existing assertions, including the sink-qualified status name. New keys are invocation-unique. The new tests visibly ran and reached their intended boundaries.
- The two public documentation changes accurately describe counting and recovery; I found no private identifiers or agent-context references there.
- All ten commit titles follow the gitmoji/conventional pattern. Commit messages contain no prohibited attribution. The `fix(sink)` title is suitable for a changelog.

**Validation and limits**

Passed independently:

- New marker/breaker tests with verbose execution.
- `go test -race -count=3 ./internal/errors/ ./internal/sink/...`
- `go test -race -count=3 -skip TestControllers ./internal/controller/`

I could not independently build every historical commit or rerun mutations under read-only access. `go build ./...` was blocked creating its temporary build directory. Historical compilation therefore remains unverified; static inspection found no cross-commit symbol dependency.

## Verdict

CONCERNS
