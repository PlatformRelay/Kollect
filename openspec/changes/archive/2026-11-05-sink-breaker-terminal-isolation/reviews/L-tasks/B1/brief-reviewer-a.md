# Review brief — task B1.1 (diff-only, independent)

You are reviewing one task, independently of whoever implemented it. Read-only: do not modify any
file in the repository. You may copy the tree to a temp directory outside it for mutation checks.

Repository: current directory (git worktree, branch `fix/sink-breaker-terminal-isolation`).
Base: `7a26657c0`. The change is the working tree against the base (new files are intent-to-add):
run `git diff 7a26657c0` to see it. Judge only that diff.

## 1. TASK

`openspec/changes/sink-breaker-terminal-isolation/tasks.md`, line B1.1 (read it there). In short:
a table test in `internal/sink/git` asserting `kollecterrors.IsInventoryScoped(err) &&
kollecterrors.IsTerminal(err)` for every site routed through the inventory-scoped helper
(`prune_owned.go`, `prune_target.go`, listed from the code at HEAD), driven through the functions
that raise them; one boundary case through the go-git engine (`exportRemoteInWorkdir`), or, if that
cannot run without a network remote, a unit case on the go-git return path through
`ClassifyExportError`; mutation: revert each site to `invalidPrune` one at a time, each killed by the
table; the `InventoryScoped` doc comment says it always yields Terminal whatever the input's class;
`evidence/T0.1.md`'s cleanup statement is narrowed (Git/GitLab ownership release and snapshot
deletion bypass the breaker; relational cleanup goes through `RunExportEnvelope`,
`internal/sink/cleanup.go:379`). Spec: `openspec/changes/sink-breaker-terminal-isolation/specs/sink-circuit-breaker/spec.md` (SCB-1).
Constraint: no production behaviour change beyond the doc comment.

This is a test-strengthening task on code that already behaves correctly: there is no red on the
base for the table; its proof is the mutation run.

## 2. MATRIX

`openspec/changes/sink-breaker-terminal-isolation/evidence/B1.md`, sections *Routed sites*,
*go-git engine case*, *Verification matrix*, *Mutations*.

## 3. DIFF

`git diff 7a26657c0`.

## 4. SENSORS

`evidence/B1.md` *Sensors run* (build/vet/gofmt, unit tests, `task lint`, two race commands,
mutants M0-M8 all as recorded). The full unit fitness run is `pending: runs once in FINAL`, not missing.

## 5. RULES

Repo rules: tests must not use a shared circuit-breaker key or call `ResetBreakersForTest` from a
parallel test; committed evidence is Markdown only; no production behaviour change in this task.

## Judge, in order

a. INTENT — does the diff do all of B1.1 and nothing more? Check the site list against
   `rg -n 'ownershipConflict|foreignClaimError' internal/sink/git/` at the base: is any routed site
   missing from the table?
b. MATRIX — is each sensor real, would it fail if the claim were false, is the class honest? Rerun
   anything under a minute that a disputed row depends on (e.g. one mutant, in a scratch copy).
c. RULES — any rule above the diff violates.
d. AI-code failure modes — tautological tests, weakened assertions, a dropped half of the task,
   prose (doc comment, evidence) claiming more than the code does.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
APPROVE with no findings is valid when substantiated.

RETURN ≤15 lines, nothing else:

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

Severity: P0 (invariant broken, vacuous gate presented as proof) → BLOCK; P1 (correctness gap,
untested claim, dishonest evidence class, missing half of the task) → REQUEST_CHANGES; P2 → note,
APPROVE allowed.
