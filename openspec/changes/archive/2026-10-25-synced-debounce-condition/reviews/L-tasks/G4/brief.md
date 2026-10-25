You are reviewing one task group of the OpenSpec change `synced-debounce-condition`,
independently of whoever implemented it. Read-only: do not modify any file. Work from the
repository root (branch `feat/synced-debounce-condition`).

Inputs:

1. TASK — `openspec/changes/synced-debounce-condition/tasks.md`, item 4.1 (documentation), group
   G4. It cites SDC-7 in `openspec/changes/synced-debounce-condition/specs/inventory-sync-status/spec.md`;
   the behaviour SDC-1..6 is already implemented on the pre-task commit `eed058ee5` (G1-G3).
   Design: `design.md` *Compatibility* in the same directory.
2. MATRIX — `openspec/changes/synced-debounce-condition/evidence/G4.md` (texts checked against
   the code, named edits quoted before/after, verification matrix, gate runs).
3. DIFF — the staged diff `git diff --cached eed058ee5` (nothing committed yet): seven files under
   `docs/` plus the evidence and the tasks.md tick.
4. SENSORS — the evidence's matrix and *Gate runs* (exit codes as reported). The per-line check
   is quick; rerun it: `rg -n -i 'debounc' docs | rg 'Synced=False|PartiallySynced'` (allowed:
   the upgrade note, the ADR-0413 amendment, `docs/REQUIREMENTS.md:89`).
5. RULES — docs only; `CHANGELOG.md` untouched; no Go change; no private company strings,
   secrets, or model/tool names in committed files.

Judge, in this order:

a. INTENT — does the diff make every edit 4.1 names (each file and line range) and nothing else?
   Does the upgrade note, under *Smaller behaviour changes (after v0.21.0)*, cover each item 4.1
   lists (aggregate `True` on a debounce, per-sink `True/Debounced`, `PartiallySynced` only for
   partial failure, the new messages, the per-sink record kept, a sink edit or replacement
   re-exports once, the read API per-sink status) and say plainly what alerts and scripts must
   change? Is the ADR-0413 §6 amendment present?
b. ACCURACY — every condition status, reason and message text the docs now state must match the
   code on the tree: `internal/controller/per_sink_export.go` (`setSinkExportDebounced`,
   `aggregateInventorySync`, `sinksUpToDateMessage`), `internal/controller/kollectinventory_controller.go`
   and `kollectclusterinventory_controller.go` (`Ready`, `ExportSucceeded`, failure reasons),
   `internal/inventory/status.go` (read API mapping). Old messages quoted in the upgrade note must
   match the base `0686f8276` (`git show 0686f8276:<path>`). Flag any doc claim the code does not
   support, and any doc line elsewhere in `docs/` that still says a debounce sets `Synced=False`
   or `PartiallySynced` (also split across lines or table rows of one statement).
c. MATRIX — are the evidence's quotes the real diff, are the rows honest (e.g. the
   `task docs:verify` row is red for an environmental reason; is that substantiated)?
d. AI-prose failure modes — claims stronger than the code, invented messages or reasons, broken
   links or anchors, contradictions between pages.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else, under a heading `## Verdict`:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you had no access, time, or evidence to verify>
summary: <one line>
