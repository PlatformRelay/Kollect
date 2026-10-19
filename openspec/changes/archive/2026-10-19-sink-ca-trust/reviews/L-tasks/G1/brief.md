# Review brief — sink-ca-trust G1 (tasks 0.1, 1.1, 1.2)

You are reviewing one task group, independently of whoever implemented it. Read-only: do not
edit, commit or push. Repository: the current directory (branch `feat/sink-ca-trust`).

Inputs:

1. TASK — `openspec/changes/sink-ca-trust/tasks.md`, section *0. Probe* (0.1) and *1. Shared pool
   and CA resolution* (1.1, 1.2), plus the test rules at the top of that file. Spec:
   `openspec/changes/sink-ca-trust/specs/sink-tls-trust/spec.md` requirements SCT-4 and SCT-5.
   Design: `openspec/changes/sink-ca-trust/design.md` sections *One pool rule* and *CA
   resolution, shared*.
2. MATRIX and SENSORS — `openspec/changes/sink-ca-trust/evidence/G1.md` (verification matrix,
   sensors run, iterations with the recorded red, mutation table, test changes). The 0.1 probe
   evidence is in the same file. The full `task`-tier suite is pending: it runs once after
   review, not missing.
3. DIFF — `git diff 0fb07e958..HEAD` (two commits: `9abf134a3` probe evidence, `HEAD` code and
   tests). Judge only this diff.
4. RULES — `AGENTS.md` (repo root) and `docs/development/spec-workflow.md`. Nothing private ships;
   no private key or PEM fixture with a key is committed; tests generate certificates at run time.

Judge, in order:

a. INTENT — does the diff do what 0.1, 1.1 and 1.2 ask, all of it, and nothing more? Check every
   R1 row and every R2 change named in tasks.md 1.1, and the `sinktls.FromSpec` comment fix.
b. MATRIX — for each row: is the sensor real, is the class honest, would it fail if the claim
   were false? Mutation-verify the new tests in a scratch copy OUTSIDE the repository (for
   example `cp -r . "$TMPDIR/g1-copy"` then edit there), judge by `go test` exit status, never
   edit the repository itself.
c. RULES — anything in AGENTS.md the diff violates (error handling, secrets, test conventions).
d. Failure modes — hallucinated APIs, tautological tests, weakened assertions, a dropped half of
   the task, prose claiming more than the code does. Security posture: can the new errors leak a
   Secret value or PEM; can a both-set spec or a key-less Secret still pass on any path (operator
   export, operator connection probe, CLI); does the probe evidence (0.1) contradict
   `proposal.md` or `design.md` anywhere the implementer did not flag?

For each P0/P1 you are about to file, argue the opposite in one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else, starting with a `## Verdict` heading:

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
```

P0 (security, invariant broken, vacuous gate presented as proof) → BLOCK. P1 (correctness gap,
untested new behaviour, dishonest evidence class, missing half of the task) → REQUEST_CHANGES.
P2 → note only.
