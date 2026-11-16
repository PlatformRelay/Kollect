# Review brief — T0.1 (probe, evidence only)

You are reviewing one task independently of whoever did it. Read-only. Repository root is the current
directory (branch `ci/scheduled-vulncheck`). `gh` is authenticated for `PlatformRelay/Kollect` and
you may run read-only `gh` and `git` commands.

## Inputs

1. TASK — `openspec/changes/scheduled-vulncheck/tasks.md`, task 0.1 (section "0. Probe"): verify on
   `origin/main` the claims `proposal.md` makes and record each with file:line or command output in
   `evidence/T0.1.md`; also probe govulncheck's exit codes and format, Task 3.54.0's exit status when
   govulncheck fails, cold job wall time, the zizmor command, and every place that must know a new
   guard exists. "A claim that does not hold stops the loop with a decision request."
2. MATRIX — the verification matrix in `openspec/changes/scheduled-vulncheck/evidence/T0.1.md`.
3. DIFF — `git diff 73ed25855..HEAD` (evidence file and `loop.md` only; no code).
4. SENSORS — the *Sensors run* table in the evidence file.
5. RULES — evidence is Markdown only; every claim tagged verified or *believed*; no secret, token or
   whole log in the evidence.

## Judge, in order

a. INTENT — does the evidence cover every clause of task 0.1 (split it on commas and "and"s), and
   nothing else?
b. MATRIX — spot-check the load-bearing claims against the source, not the evidence's prose:
   - the file:line citations (`git show origin/main:<path> | sed -n '<a>,<b>p'`);
   - the two "does not hold" findings (rows 6 and 7): check run 42151173159 (main, 2026-11-06) and the
     vulncheck conclusions of the open Renovate PRs (`gh run view <id> -R PlatformRelay/Kollect --json jobs`),
     and PR #492's author (`gh pr view 492 -R PlatformRelay/Kollect`). Is the BLOCKED verdict justified
     by task 0.1's rule, or is any finding wrong or overstated?
   - the Task 201 claim against a CI log (`gh run view 43151425860 -R PlatformRelay/Kollect --log --job 130961301039 | grep -n 'exit'`).
   Dispute rows whose evidence would not change if the claim were false.
c. RULES — any rule above broken.
d. Failure modes — invented line numbers or run ids, claims stated as verified that were not, a
   dropped clause of the task.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
APPROVE with no findings is valid when substantiated.

## Return — at most 15 lines, nothing else

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <what>
summary: <one line>
```

Severity: P0 (dishonest evidence presented as proof) → BLOCK; P1 (wrong claim, missing clause) →
REQUEST_CHANGES; P2 → note.
