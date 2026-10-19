# Stage B fix review: verification

Fixes S1-S4 (`fa16e8409..49310e2ad`), reviewed 2026-10-19 by one astra leg (cold brief
`brief-astra.md`, diff lens). No Claude leg: the fixes change the runner, the formatter and the
guard, not CI wiring (no workflow, `ci.yaml` or reporter edit).

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (`codex exec --sandbox read-only -`) | `leg-astra-diff.md` | 0 | APPROVE |

Astra's read: S1-S4 close B1, B2, B4 and B5; a partial measurement (some killed or lived, the
rest timed out) rightly stays green, report-only; coefficient 10 has empirical support and the
job's `timeout-minutes: 15` bounds the run; the help parse matches v0.6.0's generator and rejects
an empty or duplicated list. It could not run the guard (sandbox denied temp files).

## Findings

| Id | Sev | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- |
| F1 | P2 | yes: a runner with a help query inserted before `refused=()`, and one before `REPORTS="$(mktemp -d)"`, both passed `MUTATION_SIGNAL_ONLY='R5\|R6\|R8'` of the guard at `49310e2ad` (exit 0) | the fake gremlins did not count `unleash --help`, so R5/R6 and R8 could not see a help query moved before the refusal, the forced failure or the hand-off | fix, S5 (`2200cb61d`): the fake records each help query and `$GITHUB_OUTPUT` at that moment; R5, R6, R8 assert on it; three mutants |

One round: the only finding was a P2 fixed with mutants; round 2 of the branch review covers S5.
