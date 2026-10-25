# Review register — G3 (tasks.md 3.1, 3.2)

- Target: tasks.md 3.1 and 3.2 (SDC-4); staged diff over `e90da31ac` (production, tests,
  evidence).
- Legs: reviewer-a, lens diff (`leg-reviewer-a-diff.md`, brief `brief.md`). One round. No
  reviewer-b leg (no BLOCK; the diff touches no secrets, CI, infra or manifests).

## Unified verdict

APPROVE, no findings, no matrix disputes. Could not check: envtest, the recorded reds and the
mutations (accepted as reported); the full suite and race run were pending at review time and
ran afterwards on the reviewed tree (`evidence/G3.md` *Sensors run*).

Reviewer-a on the substance: every 3.1 case, the both-kind scenarios and the controls are
covered; all three resolvers and the six failure branches are wired; the identity is stable
across the preview and the full path; no lock inversion; failure reasons and messages, snapshot
handling and requeue behaviour unchanged; existing assertions unchanged. It reran the focused
unit tests (pass).
