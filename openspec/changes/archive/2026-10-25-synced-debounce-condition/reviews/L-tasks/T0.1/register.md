# Review register — T0.1 (probe)

- Target: tasks.md 0.1; staged diff over `690b50c1b` (evidence file, `loop.md` rows).
- Legs: reviewer-a, lens diff (`leg-reviewer-a-diff.md`, brief `brief.md`). One round. No
  reviewer-b leg (no BLOCK; evidence-only diff touching no secrets, CI, infra or manifests).

## Unified verdict

REQUEST_CHANGES (evidence wording only) → both findings verified and fixed; no second round.

| # | Finding | Sev | Verified | Disposition |
|---|---|---|---|---|
| 1 | Reader table omits `internal/controller/kollecttarget_scope_forbidden_test.go:68`, `:99` | P2 | yes (`rg` output lists both) | fixed: row added (target `Synced`, unaffected) |
| 2 | Decision request says G1/G2 do not touch the preview path; tasks 1.2 and 2.2 change it | P2 | yes (tasks.md 1.2 names `:498-506`, 2.2 names the preview path) | fixed: *Unblocks* line rewritten |

Reviewer-a on the substance: the probes support every tasks.md 0.1 claim; finding 8 justifies
BLOCKED; option A is sound.
