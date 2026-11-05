# Cold review: spec set `sink-breaker-terminal-isolation` (lens: spec)

You are an independent reviewer. You have read-only access to this repository (Go Kubernetes
operator "kollect"). Review the OpenSpec change at
`openspec/changes/sink-breaker-terminal-isolation/`: `proposal.md`, `design.md`,
`specs/sink-circuit-breaker/spec.md`, `tasks.md` (commit `de1e7df12`; round 2 — round 1 findings and their dispositions are in `reviews/R-spec-set/verification.md`, check that each fix holds and look for new defects the fixes introduced). Ignore `loop.md` except as context.

Question: **would executing these tasks as written deliver the spec, safely?**

Check, against the actual code (read it; do not trust the spec's file:line claims):

1. Every factual claim in proposal.md/design.md: breaker settings and key
   (`internal/sink/circuit_breaker.go`, `internal/sink/export.go`), which errors reach the breaker
   already Terminal vs classified after it, gobreaker v2.4.0 `IsExcluded` semantics in closed and
   half-open state (module source under the Go module cache, `github.com/sony/gobreaker/v2@v2.4.0`),
   and the RACE-COUNT-BREAKER-ISOLATION root cause (`internal/controller/*_test.go`).
2. Requirements SCB-1..3: are they testable, unambiguous, and complete? Missing scenarios (e.g.
   concurrency in half-open, `ErrTooManyRequests`, Forbidden-class errors, a Terminal error that is
   in fact sink-wide)?
3. Tasks: missing, misordered or unverifiable tasks; a test case that cannot be red on base for its
   stated reason; a case whose stated failure it would not actually expose; mutations a case would
   not kill; a test seam for the 30 s timeout that could change production behaviour or race.
4. Anything the change would break elsewhere (other tests or docs relying on Terminal errors
   tripping the breaker; metrics; docs claims).

Output: findings as a list, each with severity CRITICAL / WARNING / NOTE, file:line evidence, and a
concrete fix. CRITICAL only for something that makes the tasks fail to deliver the spec or ship a
defect. End with `## Verdict` and one of APPROVE / CONCERNS / REQUEST_CHANGES / BLOCK.
