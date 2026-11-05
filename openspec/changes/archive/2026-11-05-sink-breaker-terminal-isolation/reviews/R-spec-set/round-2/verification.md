# R round 2 (last) — verification and triage

Spec set `de1e7df12`. Reviewer A (spec): REQUEST_CHANGES. Reviewer B (spec): BLOCK. Every finding
checked against the code by the orchestrator on 2026-11-05.

| # | Finding | Sev | Reviewers | Verified | Disposition |
|---|---|---|---|---|---|
| 1 | Git auth, protected-branch and pre-receive rejections reach the breaker already Terminal (`git/export.go:128-149` returns `ClassifyExportError`); option A would stop sink-wide failures from opening the breaker | CRITICAL | A, B | yes; also: inventories re-export on every collected-resource event, so Terminal exports do repeat — the decision brief's premise "Terminal is never retried" was false | fix by decision: brief rewritten with the corrected facts, `decide.sh` re-run, both legs chose B (mark inventory-scoped results only); proposal, design, spec and tasks rewritten for B |
| 2 | SCB-2d at the breaker only does not guard the classification boundary | WARNING | A | yes | fix: SCB-2d through `RunExportEnvelope` with the Git classifier; SCB-1e through the real Git backend |
| 3 | Requirement omits the 60 s counting-interval reset | WARNING | A | yes (`circuit_breaker.go:54`) | fix: SCB-2 text, design *Behaviour* |
| 4 | Half-open assertions timing-sensitive; `State()` advances time-dependent state | WARNING | A, B | yes | fix: design timing protocol (1 s timeout, one 1.1 s sleep, assertions without sleep) |
| 5 | SCB-3 reds on `origin/main` are compile errors or nil-breaker panics | WARNING | B | yes (`newBreaker` absent) | fix: 1.1 starts with a pure `newBreaker` extraction and marker stubs; red base redefined |
| 6 | G2 red rule contradicts preventive isolation; "six times" left over | NOTE | A | yes | fix: tasks preamble and 2.1 allow "not reproduced; preventive"; test rules updated |
| 7 | Serial test with reset only in cleanup inherits a tripped breaker under `-count` | NOTE | B | yes | fix: 2.2 resets at start and in cleanup |
| 8 | SCB-1b needs a path-branching stub | NOTE | B | yes (`export_test.go:31-35`) | fix: design *Testing approach*, 1.1 |

Round 2 was the last spec-set round. Finding 1 was settled by the decision procedure (fact check, then
a rerun on the corrected brief, unanimous), not by a third review. Its premises are re-verified in
T0.1, which stops the loop if any does not hold, and the rewritten spec set is reviewed again as part
of the branch review at stage B.
