# Review brief: group G2 (tasks 2.1 + 2.2)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not edit, commit or
create files in the repository. Work from the repository root
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation`). You may run
`go test` commands under a minute (they write only to the Go build cache); any scratch copy for a mutation
goes under `$TMPDIR`, never in the repository. The Ginkgo suite in `internal/controller` refuses `-count>1`:
add `-skip TestControllers` to any `-count=N` command there.

## Inputs

1. TASK: `openspec/changes/sink-breaker-terminal-isolation/tasks.md`, items 2.1 and 2.2 and the preamble
   (read them). Design: `design.md` *Testing approach*. Base measurement: `evidence/T0.1.md`
   *RACE-COUNT-BREAKER-ISOLATION on base*.
2. MATRIX and all evidence: `openspec/changes/sink-breaker-terminal-isolation/evidence/G2.md`.
3. DIFF: base `0e7897dd1`, staged (not yet committed): `git diff --cached 0e7897dd1`. Code: test files only,
   `internal/controller/kollectinventory_export_test.go`, `internal/controller/kollectinventory_finalizer_test.go`,
   `internal/controller/cluster_inventory_finalizer_test.go`; plus evidence, tasks.md tick and loop.md.
4. SENSORS: recorded in the evidence (*2.1*, *Deterministic sensor*, *Mutations*, *2.2*, *Sensors*). The unit
   fitness row runs once after review (pending, not missing).
5. RULES: `AGENTS.md`, `docs/development/spec-workflow.md`. The breaker registry
   (`internal/sink/circuit_breaker.go`) is process-wide, keyed by sink namespace/name
   (`internal/sink/export.go:200`). No parallel test may call `ResetBreakersForTest`; never a retry or sleep.

## What matters

Controller tests must stop sharing one breaker key, without changing what they assert. Check in particular:
every test that used the fixed sink `postgres-demo` now uses a per-invocation name for the sink object AND
the inventory's ref (a mismatch would turn the test into a "sink not found" test that may still pass); the
one assertion naming the sink is derived, not dropped; nothing else in the test bodies changed; the
deterministic sensor and its mutants are real. Judge whether the debounce-metric red in 2.2's ten runs is
honestly classified (not the breaker) and whether deferring it is acceptable against 2.2's "ten green runs"
clause, or whether it makes the group's verdict wrong.

## Judge, in this order

a. INTENT: does the diff do what 2.1 and 2.2 ask, all of it, and nothing they did not ask for?
b. MATRIX: per row, is the sensor real, does it exercise the claim, is the class honest, would it fail if the
   claim were false? Mutation-verify where cheap.
c. RULES: any rule the diff touches or violates.
d. AI-code failure modes: tautological tests, weakened assertions, a dropped half of the task, prose claiming
   more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

RETURN at most 15 lines, nothing else, under a `## Verdict` heading:

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

P0 → BLOCK, P1 → REQUEST_CHANGES, P2 → note only, APPROVE allowed.
