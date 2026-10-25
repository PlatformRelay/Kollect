# Review register — G4 (tasks.md 4.1)

- Target: tasks.md 4.1 (SDC-7); staged diff over `eed058ee5` (seven doc files, evidence,
  tasks.md tick).
- Legs: reviewer-a, lens diff (`leg-reviewer-a-diff.md`, brief `brief.md`). One round. No
  reviewer-b leg (no BLOCK; the diff touches no secrets, CI, infra or manifests).

## Unified verdict

REQUEST_CHANGES with two P2 wording findings; both verified against the code and fixed, wording
only, so no second round. After the fixes: APPROVE.

| Finding | Verified | Disposition |
| --- | --- | --- |
| `docs/crds/kollectclusterinventory.md:133`: `ExportSucceeded=True` described as "no sink failed", but `True/PartiallySynced` is also set on a partial failure (`kollectclusterinventory_controller.go:772-779`) | yes | fixed: the row names both reasons and the `Exported` message |
| `docs/operator-manual/metrics.md:89`: a debounced sink "keeps the aggregate `Synced=True`" without "when no sink failed" | yes | fixed |

Matrix dispute (quotes accurate, claims not checked by the matrix) answered by the two fixes.
Reviewer-a reproduced the environmental `task docs:verify` stop (kubectl shim) and reran the
per-line check (three allowed lines). Could not check: lint, guards and the site build (accepted
as reported, rerun after the fixes).
