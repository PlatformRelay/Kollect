# R spec-set review, round 2: verification

Reviewed revision: 75a764bc4 (round-1 fixes R1-R15, R16 as a non-goal). Legs, both fresh:
astra spec (`codex exec --sandbox read-only`, gpt-6-astra; GO WITH FIXES, one CRITICAL) and
claude/opus spec (fanout.sh `--no-unify`, 147 s; CONCERNS). Round-1 closure: astra marks R1-R12
and R14-R15 CLOSED and R13 PARTIAL (A2 below); claude marks every round-1 finding closed except
R10 in substance (C2 below). This is the last round: every verified finding was mechanical, so
it was applied with no further review. Orchestrator verdict: GO. No CRITICAL remains.

| ID | Sev | Leg | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| A1 | WARNING (astra: CRITICAL) | astra | yes | 3.2 said every write goes through `--body-file`, but the recovery close is a write and `gh issue close --help` (gh 2.98.0) has no `--body-file` flag. A wording defect that the first red would expose, not a shipped hole | fix: 3.2 requires `--body-file` on `issue create` and `issue comment`, a bodyless `gh issue close <number>`, and a fake that rejects any other close flag |
| A2 | WARNING | astra | yes | `task-prompt.md:53` still forbids a missing-file red, while the tasks.md preamble allows 4.1's | fix: the dispatch line names 4.1 as the one exception |
| A3 | WARNING | astra | yes | No test puts `@` in an unfenced job or step name. 3.2 tested a log fetch that errors but not one that exits 0 with empty output, which CFR-6's scenario names | fix: 2.1 case for a job named `deploy @user`. The 3.2 log-fetch case covers a non-zero exit and an empty exit-0 |
| C1 | WARNING | claude | yes | CFR-1's prose ("no bot issue … open, SHALL open one") contradicted the table row "closed, equal or older: no write" and the CFR-3 stale-after-close scenario | fix: the prose now defers to the table and says when it opens an issue |
| C2 | WARNING | claude | yes | The R10 probe targeted the Renovate dashboard. This repository has no bot-authored issue: 29 issues, all `konih`, read-only `gh issue list --state all` on 2026-10-16. `renovate.yaml:33` authenticates with a PAT or `github.token` | fix: 1.1 probes a bot-authored issue in a public repository, derives the spellings from its `author` object, and adds the lookup argv to the post-merge row if `github-actions` cannot be probed |
| C3 | WARNING | claude | yes | `gh issue list` defaults to `--limit 30` (`gh issue list --help`). Once other workflows' bot issues exceed it, the lookup finds no issue and opens a second open issue | fix: CFR-4 requires an explicit `--limit` and a non-zero exit when the count reaches it. Added a 3.2 case |
| C4 | NOTE | claude | yes | "Started job" is undefined, and the cancelled-class split depends on it | fix: 1.1 derives the rule from the recorded cancelled-before-start shape, and 3.1's fixtures pin it. The spec wording is unchanged until the shape is recorded |
| C5 | NOTE | claude | yes | `gh label create` with no `--color` picks a random colour ("If a color isn't provided, a random one will be chosen"). `--force` then recolours the label on every run | fix: CFR-1 requires a fixed `--color` and `--description`, and 4.1 asserts both |
| C6 | NOTE | claude | yes | The cap case does not check that truncation closes the excerpt fence | fix: the 2.1 cap case asserts that the fence closes before the truncation marker |

No finding was rejected or deferred. The claude leg confirmed that the zizmor shape in
`tasks.md` passes `cws2_suppressions` (`ci_workflow_security_test.sh:236`) and that both
comment-removal mutants red. Both legs re-checked every cited file:line and found none stale.
The orchestrator re-read the citations it added:
`ci_workflow_security_test.sh:306` (inline grep), `ci_fetch_lib_hardening_test.sh:865-882`
(self-wiring) and `renovate.yaml:33`.

Not committed: `leg-claude-opus-spec.err` (empty) and `legs.log`.
