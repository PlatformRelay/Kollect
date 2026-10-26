# Review register — G2 (tasks.md 2.1, 2.2)

- Target: tasks.md 2.1 and 2.2 (SDC-5, SDC-3 transitions); staged diff over `fddd8e761`
  (production, tests, evidence).
- Legs: reviewer-a, lens diff (`leg-reviewer-a-diff.md`, brief `brief.md`). One round. No
  reviewer-b leg (no BLOCK; the diff touches no secrets, CI, infra or manifests).

## Unified verdict

REQUEST_CHANGES, one P2 (test assertion) → verified and fixed; no second round (test-only fix,
no behaviour defect).

| # | Finding | Sev | Verified | Disposition |
|---|---|---|---|---|
| 1 | `TestCarryOverSinkExport` compares the previous entry's `lastExportTime` with `t1`, the value it points at, so a shallow time copy would still pass (matrix row 1 over-claims time isolation) | P2 | yes (`previous[1].LastExportTime` is `&t1`) | fixed: compare with an independent `exportedAt`; mutant M2t (shallow time copy) now fails at `per_sink_export_test.go:490` |

Reviewer-a on the substance: every 2.1 case and 2.2 clause is covered; controls are honestly
labelled; no stale-condition, cleanup-evidence or seeding-race regression found.
