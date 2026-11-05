# Review brief: group G2 round 2 (task 2.2, decision A)

You are reviewing one change, independently of whoever implemented it. Read-only: do not edit, commit or create
files in the repository. Work from the repository root
(`/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation`). You may run `go test`
commands under a minute; the Ginkgo suite in `internal/controller` refuses `-count>1`, so add
`-skip TestControllers` to any `-count=N` command there.

## Inputs

1. TASK: `openspec/changes/sink-breaker-terminal-isolation/tasks.md` item 2.2 as extended in `8c4f6f875`
   (make `TestClusterInventory_DebouncedExportIncrementsMetric` non-parallel with a comment; ten green runs of
   the 2.1 command). Decision: `loop.md` *Triage* row "L G2". Round 1 review: `reviews/L-tasks/G2/leg-reviewer-a-diff.md`.
2. MATRIX and evidence: `openspec/changes/sink-breaker-terminal-isolation/evidence/G2.md` (rows 7 and 11,
   *2.2 after decision A*, *Harness gaps*, *Verdict*).
3. DIFF (this round only): staged, `git diff --cached 8c4f6f875`. Code: one test file,
   `internal/controller/kollectclusterinventory_debounce_checksum_test.go`.
4. SENSORS: recorded in the evidence. Round 1 already reviewed the sink-name isolation (`8ada865fd`); do not
   re-review it.
5. RULES: `AGENTS.md`, `docs/development/spec-workflow.md`. Precedent for the change:
   `internal/controller/kollectinventory_finalizer_test.go:974` (`terminalCleanupIncrementsMetric`).

## What matters

Does removing `t.Parallel()` actually stop concurrent increments of the counter label the test reads (Go runs
non-parallel top-level tests before releasing parallel ones)? Does the assertion stay unchanged? Does any linter
rule (e.g. paralleltest/tparallel) or other test now conflict? Is the evidence honest that the ten-run result is
probabilistic and that the verdict CLOSED is supported?

## Judge, in this order

a. INTENT, b. MATRIX (rows 7, 11), c. RULES, d. AI-code failure modes. For each P0/P1 argue the opposite for one
line; keep only what survives.

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
