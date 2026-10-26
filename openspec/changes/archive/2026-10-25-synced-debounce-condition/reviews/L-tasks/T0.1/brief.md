You are reviewing one task of the OpenSpec change `synced-debounce-condition`, independently of
whoever implemented it. Read-only: do not modify any file. Work from the repository root
(`feat/synced-debounce-condition`).

Inputs:

1. TASK — `openspec/changes/synced-debounce-condition/tasks.md`, item 0.1 (the probe; evidence
   only). It cites the load-bearing claims in `proposal.md` and `design.md` of the same directory.
   A failed claim, or a probe finding that contradicts proposal.md or design.md, is a
   SPECIFICATION finding that stops the loop (the task then reports BLOCKED with a decision
   request instead of editing the spec).
2. MATRIX — the verification matrix and every probe result in
   `openspec/changes/synced-debounce-condition/evidence/T0.1.md`.
3. DIFF — the staged diff: `git diff --cached` (base `690b50c1b`, nothing committed yet). It adds
   the evidence file and two rows in `openspec/changes/synced-debounce-condition/loop.md`.
   The probes ran against a scratch copy of `origin/main` (`0686f8276`) outside the repository;
   their code is inlined in the evidence. You may check any code claim against `git show
   origin/main:<path>` or the working tree (the branch has no code changes against
   `origin/main`) and the Go module cache (`go env GOMODCACHE`).
4. SENSORS — the *Sensors run* table in the evidence (exit codes and times as reported);
   `task lint:markdown` reported `Summary: 0 issues in 0 files` over 388 files including the new
   evidence file.
5. RULES — evidence under `openspec/changes/*/evidence/` is Markdown only, probe code inlined in
   fenced blocks with its command line; no private company strings, secrets, or model/tool names
   in committed files; every claim tagged verified (with date) or believed.

Judge, in this order:

a. INTENT — does the evidence cover every bullet of tasks.md 0.1 (export → sleep → reconcile
   states; total failure per-sink condition; read-API override with `<family>/<name>`;
   `resourceVersion` on an unchanged status update; `SetStatusCondition` with a file:line in the
   pinned `k8s.io/apimachinery`; sink generation on spec vs status update; `shouldSkip` on changed
   inputs; every reader of the `Synced` reason/message vs proposal *Impact*)? Is the BLOCKED
   verdict justified by finding 8, and is the decision request sound?
b. MATRIX — for each row: does the probe code shown actually exercise the claim, would it fail if
   the claim were false, is the evidence class honest? Check the file:line citations against
   `origin/main`. Check the reader table against
   `rg -n 'ReasonDebounced|PartiallySynced|ConditionSynced|conditionSynced|sink\(s\)' internal cmd api charts hack test`
   (on the working tree; it equals `origin/main` for those paths). Check finding 8 by reading
   `internal/controller/kollectinventory_debounce_test.go:186-231`,
   `internal/controller/kollectinventory_controller.go:463-519`, and the proposal *Impact* /
   tasks.md 3.1 sentences it says are contradicted.
c. RULES — any rule in input 5 the diff violates.
d. Failure modes — claims the evidence makes that the probe output does not show, misquoted
   line numbers, a reader missed in the table, a finding overstated.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else, under a heading `## Verdict`:

verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you could not verify>
summary: <one line>
