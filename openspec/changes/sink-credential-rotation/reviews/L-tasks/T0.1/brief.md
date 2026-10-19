# Review brief — task 0.1 (probe), sink-credential-rotation

You are reviewing one task independently of whoever did it. Read-only: do not edit files. Repo root
is the current directory (branch `feat/sink-credential-rotation`, base commit `d0f150d83`).

Inputs:

1. TASK — `openspec/changes/sink-credential-rotation/tasks.md`, item 0.1 (section "## 0. Probe"):
   verify the load-bearing assumptions in `proposal.md` (section *Assumptions* and the items 0.1
   lists) and record each as verified (command, versions, date) in `evidence/T0.1.md`; a failed
   one stops the loop as a specification finding. Spec set: `proposal.md`, `design.md`,
   `specs/sink-secret-rotation/spec.md` in the same directory.
2. MATRIX — the verification matrix and findings in
   `openspec/changes/sink-credential-rotation/evidence/T0.1.md`.
3. DIFF — the staged, uncommitted change: `git diff --cached d0f150d83` (only that evidence file).
   The task is evidence only: no Go code changes. The probe tests were throwaway files, run and
   deleted; their sources and command lines are inlined in the evidence.
4. SENSORS — recorded in the evidence's *Sensors run* table (probe runs, `task lint:markdown`,
   `task scrub`, `gitleaks protect --staged`). Trust their exit codes as reported.
5. RULES — evidence is Markdown only (no scripts committed under `openspec/changes/*/evidence/`);
   no secret values; every claim tagged verified (with date) or believed; file:line citations
   must match the code at `d0f150d83`.

Judge, in order:

a. INTENT — does the evidence cover every clause of task 0.1 (eight bullets), each with a result?
b. MATRIX — for each row: does the cited code (file:line) actually say what the evidence claims?
   Check the citations against the tree. Would each probe, as inlined, fail if its claim were
   false? Is any evidence class over-stated?
c. The evidence reports one SPECIFICATION finding (finding 3: `exportMinInterval: 0s` bindings
   never re-export an unchanged inventory, and an inventory-level `0s` gives no timed requeue
   after a total transient failure). Check it against `internal/controller/per_sink_export.go`,
   `internal/controller/kollectinventory_controller.go`, `internal/validation/export_interval.go`
   and the proposal text it says it contradicts. Is it real, and does it really contradict the
   proposal/design/tasks, or is it already covered by their wording?
d. Failure modes — claims stronger than the probe shows, wrong line numbers, a dropped bullet,
   anything that should have been a finding and is not (in particular: any probe result that
   contradicts `design.md` but is not reported as such).

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

Return at most 15 lines, in this shape, under a heading `## Verdict`:

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

Severity: P0 (security, secret leaked, vacuous proof presented as proof) → BLOCK. P1 (wrong
claim, wrong citation that changes a conclusion, missing bullet, dishonest class) →
REQUEST_CHANGES. P2 → note only. A correctly reported SPECIFICATION finding is not itself a
reason to BLOCK this review.
