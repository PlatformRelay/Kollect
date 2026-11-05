# Review brief: task 0.1 (probe, evidence only)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit, commit or
create files in the repository. Work from the repository root
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation`).

## Inputs

1. TASK: `openspec/changes/sink-breaker-terminal-isolation/tasks.md`, item 0.1 (read it). Spec set it checks:
   `proposal.md`, `design.md`, `specs/sink-circuit-breaker/spec.md` in the same directory.
2. MATRIX and all evidence: `openspec/changes/sink-breaker-terminal-isolation/evidence/T0.1.md`.
3. DIFF: base `f16ec6baa`, staged (not yet committed): `git diff --cached f16ec6baa`. The diff is evidence,
   the tasks.md tick and loop.md only; no code. The code the evidence describes is the tree at `f16ec6baa`.
4. SENSORS: recorded in the evidence (*Sensors run*, race table). gobreaker source is in
   `$(go env GOMODCACHE)/github.com/sony/gobreaker/v2@v2.4.0`.
5. RULES: `AGENTS.md`, `docs/development/spec-workflow.md`. Committed evidence is Markdown only.

## What matters

This probe is the premise gate for the implementation that follows: table (a) decides which error sites get
marked inventory-scoped (excluded from the sink circuit breaker) and table (b) decides test isolation. A wrong
row means either a sink-wide fault stops shedding load (marked wrongly) or the tenant bug stays (missed).

Judge, in this order:

a. INTENT: does the evidence cover every clause of task 0.1 (breaker settings and key; gobreaker `IsExcluded`
   semantics incl. half-open slot and interval reset; three zones; Git auth/protected-branch Terminal at the
   breaker; re-export on next event-driven reconcile; table (a) with every `invalidPrune` call site and every
   other Terminal inside an export plan across Git, GitLab, object-store and database backends; table (b);
   the SCB-1e fixture; ten race runs with exit status and failing names)?
b. ACCURACY: spot-check file:line citations against the code at `f16ec6baa`. Above all check table (a):
   is any site missing (grep `invalidPrune`, `kollecterrors.Terminal(`, other classifiers under
   `internal/sink`)? Is any classification wrong under design.md's definition (inventory-scoped = about the
   inventory's own paths or its ownership claim; shared repository or prune-metadata state = sink-wide; unsure
   = undecided)? Is any site wrongly placed inside or outside the export plan (`internal/sink/export.go:200`
   is the only breaker call)? Check table (b) callers and their parallelism.
c. SPEC: does any finding contradict a load-bearing claim in proposal.md or design.md that the evidence fails
   to flag? Are the two "non-blocking" spec corrections really non-blocking?
d. Honesty: does the evidence claim more than it ran or read (verified vs believed tags, evidence classes)?

You may rerun anything under a minute (e.g. a focused `go test -run`); do not rerun the ten-run race loop.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

RETURN at most 15 lines, nothing else, starting with a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <item>
summary: <one line>
```

Severity: P0 (a wrong classification that would mark a sink-wide fault, a missed load-bearing contradiction,
evidence claiming a run that did not happen) → BLOCK. P1 (a missing site, a wrong file:line that changes a
row, an unflagged spec contradiction) → REQUEST_CHANGES. P2 → note only.
