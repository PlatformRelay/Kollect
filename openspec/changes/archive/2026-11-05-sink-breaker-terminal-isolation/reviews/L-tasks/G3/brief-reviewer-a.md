# Review brief: group G3 (task 3.1, docs)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit, commit or create
files in the repository. Work from the repository root
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation`).

## Inputs

1. TASK: `openspec/changes/sink-breaker-terminal-isolation/tasks.md`, item 3.1 (read it). Spec:
   `openspec/changes/sink-breaker-terminal-isolation/specs/*/spec.md` (SCB-1, SCB-2, SCB-3).
2. MATRIX and evidence: `openspec/changes/sink-breaker-terminal-isolation/evidence/G3.md`.
3. DIFF: base `b8296d025`, staged (not yet committed): `git diff --cached b8296d025`. Product changes:
   `docs/operator-manual/troubleshooting.md` (two table rows) and `docs/adr/0602-error-taxonomy.md` (a dated
   note); plus the evidence file and the tasks.md tick.
4. SENSORS: recorded in the evidence (*Sensors*). `task docs:verify` could not run here (a host tool missing,
   recorded as not-run); a strict mkdocs build, the docs unit contracts and three doc guards ran instead.
5. RULES: `AGENTS.md`, `docs/development/spec-workflow.md`. These docs are public: written for operators, no
   internal backlog ids, no references to internal planning files, no private strings. The troubleshooting table
   keeps its five-column row format. The ADR gets a short dated note, not a rewrite.

## What matters

Check every claim the new text makes against the code, not against the evidence's summary:
`internal/sink/circuit_breaker.go` (thresholds, interval, timeout, `MaxRequests`, `IsExcluded`, the open-state
error text), `internal/sink/export.go` (the breaker key), `internal/errors/errors.go` (the inventory-scoped
marker) and `internal/sink/git/prune_owned.go` (which Git rejections are marked). gobreaker is
`github.com/sony/gobreaker/v2` (module cache). In particular: is "within its 60 s counting window" accurate for
gobreaker's closed-state `Interval`; does a counted failure on the half-open trial re-open it; does an
inventory-scoped result really leave the breaker half-open; is the claim that an ownership conflict "is reported
only in that inventory's status" true; and is every Git ownership rejection excluded, or only some (the docs
must not over-claim). Also: does 3.1's requirement to review `rg -n 'circuit breaker' docs` hold, i.e. is any
other docs claim left contradicted.

Then judge, in this order: (a) INTENT, all of 3.1 and nothing else; (b) MATRIX rows honest; (c) RULES; (d)
prose that claims more than the code does.

RETURN at most 15 lines, nothing else:

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

P0 → BLOCK; P1 (a false or over-claiming operator statement, a missed clause of 3.1) → REQUEST_CHANGES;
P2/P3 → note, APPROVE allowed. Write the result as `## Verdict` followed by those lines.
