# Stage B round 2: verification

Change `mutation-testing-signal`, branch at `603030e03` (round-1 fixes S1-S5 and their review
records), reviewed 2026-10-19. Round 2 ran because round 1 had a CRITICAL (B1). Scope: are the
round-1 findings closed, plus anything new. This is the last round: verified behaviour defects
are fixed below with mutants and no further review.

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (adversarial lens, fresh cold brief `brief-astra.md`) | `leg-astra-adversarial.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`) | `leg-claude-opus-spec.md` | 0 | CONCERNS |

## Round-1 closure

| Round-1 id | astra | opus | Orchestrator |
| --- | --- | --- | --- |
| B1 all-TIMED-OUT package green; default coefficient | closed (`mutation-summary.sh:80`) | closed (`:80`, `mutation.sh:71,98`) | closed |
| B2 new mutator silently off after a bump | closed for v0.6.0's help format | closed except a shorthand flag: R2-3 | closed by S3 plus S7 |
| B4 W3 split `task mutation` | the S4 examples close; a plain scalar does not: R2-1 | the same: R2-1 | closed by S4 plus S6 |
| B5 unused warm-cache fixture | closed (M9) | closed (M9) | closed |

## Findings

| Id | Sev (after verification) | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| R2-1 | WARNING (astra CRITICAL, opus WARNING) | astra, opus | yes: a `ci.yaml` step `run: task` with `mutation` on the next indented line, and the same double-quoted, parse to `task mutation` (`yq`: `["task mutation",""]`, `["task mutation","double"]`) and passed `MUTATION_SIGNAL_ONLY=W3` (scratch worktree, exit 0) | W3's S4 scan read block scalars only; YAML folds plain and quoted scalars too | fix, S6 (`9037e6e3d`): W3 scans every parsed string; two mutants. Graded WARNING, not CRITICAL: the same class as B4 (a deliberately unusual spelling past a text-level backstop, in a reviewed PR), but fixed all the same |
| R2-2 | WARNING | astra | yes: mutation.yaml's run step as `run: \|` / `task mutation` passes W1 and was red in W3 (two `FAIL: W3` lines, scratch worktree) | a false red on a form W1 accepts | fix, S6: the bare `task mutation` is the run step in mutation.yaml only (W1 pins it as the only `run:`); a C2 control |
| R2-3 | WARNING (opus NOTE) | opus | yes: the S3 regex starts `^[[:space:]]+(--`, so a `-X, --flag  enable "..." mutants` line is skipped; red as R9 variant extra-short (exit 0 before the fix) | after a bump, a new mutator flag with a shorthand stays off unnoticed | fix, S7 (`9339b2cd3`): shorthand accepted, and any mutator line the parse did not read fails the run; two R9 variants, two mutants |
| R2-4 | NOTE | opus | yes by reading: `explode(.)` expands aliases before the parsed scan | an anchored string is scanned once per alias | no action: duplicate hits, never a missed one |
| R2-5 | NOTE | astra | yes by reading | `--timeout-coefficient=10` scales the measured coverage time; it is not an absolute deadline | no action: `timeout-minutes: 15` (`mutation.yaml:26`) bounds the job |

Astra's other NOTE lines (report-only kept, no score threshold, install chain, reporter entry,
the force-failure knob, scan errors of the new queries) are confirmations, no action. Round-1
dispositions B3 (reject), B6 and B7 (defer) stand.

## Result

No CRITICAL remains open. Astra's round-2 CRITICAL (R2-1, graded WARNING here) and R2-2, R2-3
are fixed with mutants in S6 and S7, unreviewed (last round). Final gates on `9339b2cd3`: see
`evidence/B.md`, *Final gates*.
