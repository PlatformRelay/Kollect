## Verdict: CONCERNS

## Findings
- [NOTE] B1 is closed: `hack/ci/mutation-summary.sh:80` now rejects a report with no KILLED or LIVED mutant. It runs before any row is built (`:99`), so no counts table is printed. `--timeout-coefficient=10` is passed on every call (`hack/ci/mutation.sh:71,98`) and pinned by R1 plus two mutants (`hack/test/mutation_signal_test.sh` R1 judges). Evidence/B.md records the red-then-green runs.
  Confidence: 90
- [NOTE] B5 is closed: M9 reads `reports/warm-cache-pathvalidate.json` (`hack/test/mutation_signal_test.sh` M9 block, about line 281).
  Confidence: 95
- [NOTE] B2 is closed for the flag shape gremlins uses today, but the help parser cannot see a mutator flag that has a short form — `hack/ci/mutation.sh:84`
  Failure: the regex starts at `^[[:space:]]+(--…)`, so a bumped gremlins whose help line reads `  -x, --invert-foo   enable "INVERT_FOO" mutants` matches nothing. The offered set then still equals FLAGS and the run passes with the new mutator off. That is the silent-off case MUT-1 forbids. No v0.6.0 mutator flag has a short form (`unleash-help.txt:36-46`), so this only bites after a bump.
  Fix: allow an optional short form: `^[[:space:]]+(-[A-Za-z], )?(--[a-z0-9-]+)…`, capture group 2. Alternatively, fail when any help line contains `enable "` but the regex does not match it. Add an R9 "extra with shorthand" variant.
  Confidence: 45
- [WARNING] The B4 fix only covers block scalars, but a multi-line plain or quoted YAML scalar still bypasses W3 — `hack/test/mutation_signal_test.sh:855-867`
  Failure: the comment at `:857-858` says plain scalars "are already one source line each", which is false in YAML. A step in `ci.yaml` written as `- run: task` with `mutation` indented on the next line parses as the plain scalar `task mutation`. A double-quoted `"task` / `mutation"` parses the same way. The line scan sees only `run: task` and `mutation`, and the block query keeps only `style == "literal" or "folded"`, so W3 passes. The step runs `task mutation` outside `mutation.yaml`, against MUT-1 "only the `mutation.yaml` run step runs `task mutation`".
  Fix: in the yq query, select every `!!str` whose value contains `\n`, whatever its style. The exempted run step is single-line, so it is unaffected. Add a W3 mutant for the multi-line plain form.
  Confidence: 75
- [NOTE] The fixes add no code path that no requirement covers. Two side effects are worth knowing:
  - The help query adds one more gremlins process call. R5, R6 and R8 now check its ordering (`mutation_signal_test.sh` R5/R6/R8 additions plus three mutants).
  - Because `explode(.)` follows aliases, the block scan reads anchored blocks once per alias. That costs some duplicate hits, never a missed one.
  Confidence: 70

## Compliance (code is the evidence)
| Req clause | Status | Decided at |
| --- | --- | --- |
| MUT-1 three packages, one call each, fresh dir, all flags, absolute `--config` | holds | `mutation.sh:64-66,73,95-101,119-121` |
| MUT-1 `--timeout-coefficient=10` | holds | `mutation.sh:71,98` |
| MUT-1 help check before the first unleash; exact equality | holds for flags without a short form, partial otherwise (finding 3) | `mutation.sh:82-92` |
| MUT-1 `reports=` before the first gremlins call (help included); directory kept | holds | `mutation.sh:73-77`; the R8 help.ghout assertion |
| MUT-1 formatter args, validation before print, five counts | holds | `mutation-summary.sh:33-55,57-102,104-119` |
| MUT-1 scenario: invoked outside the pin (W3) | partial (finding 4) | `mutation_signal_test.sh:838-899` |
| MUT-2 *Tool broken*, including the new "no killed or lived" clause | holds | `mutation-summary.sh:76-81`, `mutation.sh:99-100` |
| MUT-2 every parse fails closed; failures collected | holds for the new yq/jq calls | `mutation_signal_test.sh:861-868` |
| MUT-2 refusal of `GREMLINS_*` before any gremlins call | holds; the help query comes after it | `mutation.sh:40-47` vs `:82` |
| MUT-3 control LIVED exactly once, before the packages | holds; unchanged | `mutation.sh:103-117` |
| MUT-4 forced failure fails only, before the help query | holds | `mutation.sh:49-53`; R5 `help` assertion |
| B3, B6, B7 (rejected or deferred in round 1) | unchanged, as recorded | — |

## Could not check
- I ran no tests or self-tests. The green and red results above come from `evidence/B.md` as recorded.
- I did not check the yq `style` selector against the yq version CI installs, or whether `explode(.)` keeps `style` on expanded aliases.
- I did not see the real gremlins binary's help output for any version other than the recorded `unleash-help.txt`.
- I did not read the untracked `reviews/B-branch/round-2/` or `round-1/legs.log`, other reviewers' legs, or anything outside this worktree.
