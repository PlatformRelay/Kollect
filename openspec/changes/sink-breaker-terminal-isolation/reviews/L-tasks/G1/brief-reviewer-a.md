# Review brief: group G1 (tasks 1.1 + 1.2)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not edit, commit or
create files in the repository. Work from the repository root
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation`). You may run
`go test` commands under a minute (they write only to the Go build cache); any scratch copy for a mutation
goes under `$TMPDIR`, never in the repository.

## Inputs

1. TASK: `openspec/changes/sink-breaker-terminal-isolation/tasks.md`, items 1.1 and 1.2 and the preamble
   (read them). Spec: `specs/sink-circuit-breaker/spec.md` (SCB-1, SCB-2, SCB-3). Design:
   `design.md` (*Marker*, *Behaviour*, *Testing approach*). The site classification it relies on:
   `evidence/T0.1.md` table (a) and table (b).
2. MATRIX and all evidence: `openspec/changes/sink-breaker-terminal-isolation/evidence/G1.md`.
3. DIFF: base `7f8c88804`, staged (not yet committed): `git diff --cached 7f8c88804`. Product code:
   `internal/errors/errors.go`, `internal/sink/circuit_breaker.go`, `internal/sink/git/prune_owned.go`,
   `internal/sink/git/prune_target.go`; tests: `internal/errors/errors_test.go`,
   `internal/sink/circuit_breaker_test.go`; plus evidence, tasks.md tick and loop.md.
4. SENSORS: recorded in the evidence (*Red on the refactor base*, *Mutations*, *Sensors run*). gobreaker
   source: `$(go env GOMODCACHE)/github.com/sony/gobreaker/v2@v2.4.0`. The unit fitness row runs once
   after review (pending, not missing).
5. RULES: `AGENTS.md`, `docs/development/spec-workflow.md`. The breaker registry is process-wide: a parallel
   test must not call `ResetBreakersForTest`, and keys must be unique per test invocation (`-count` safe).

## What matters

The per-sink circuit breaker must stop counting failures that are about one inventory (ownership
conflicts) while still counting every sink-wide failure. Getting it wrong either keeps the cross-tenant
outage (marker lost or a site missed) or stops load-shedding for a dead sink (over-exclusion).

Judge, in this order:

a. INTENT: does the diff do all of 1.1 and 1.2 and nothing else? Exactly the 5 sites T0.1 table (a) marks
   inventory-scoped are routed (`prune_owned.go:174,261,268,310`, `prune_target.go:66` on the base); no
   undecided or sink-wide site is. The marker meets design.md *Marker* (Terminal, `errors.Is` to the
   sentinel and to everything wrapped, text unchanged, nil for nil). `IsExcluded` is set in `newBreaker`;
   production timeout unchanged; no mutable package variable.
b. MATRIX: each row's sensor exists and would fail if the claim were false. Check that each recorded base
   red is on the target assertion. Mutation-verify any clause you doubt in a scratch copy.
c. RULES: test isolation (parallel tests vs `ResetBreakersForTest`, unique keys, half-open timing protocol
   in design.md); committed evidence is Markdown only.
d. AI-code failure modes: tautological tests, weakened assertions, a dropped half of the task, prose that
   claims more than the code does, flaky timing.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

RETURN at most 15 lines, nothing else, under a heading `## Verdict`:

```text
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

Severity: P0 (security, data loss, invariant broken, vacuous gate presented as proof) → BLOCK. P1
(correctness gap, untested new behaviour, dishonest evidence class, missing half of the task) →
REQUEST_CHANGES. P2/P3 → note only, APPROVE allowed.
