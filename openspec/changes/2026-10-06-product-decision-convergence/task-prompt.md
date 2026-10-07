You are closing ONE task in a spec-loop run over an OpenSpec change. State lives in files only; you carry nothing in from any earlier session.

Repo: /Users/kheimel/.treehouse/kollect-79dca7/1/kollect (branch `fm/kollect-product-decisions-impl`, must be clean when you start; check with `git status`).
Change directory (this replaces the Spec Kit feature directory; it holds proposal.md, design.md, specs/<capability>/spec.md deltas, tasks.md): openspec/changes/2026-10-06-product-decision-convergence
Records directory for review registers: openspec/changes/2026-10-06-product-decision-convergence
Repo spec contract (read the slice you need): docs/development/spec-workflow.md and openspec/config.yaml.

Do this: invoke the `close-task` skill (Skill tool, name `close-task`, args `__TID__`) and follow it end to end for task __TID__. Its helper scripts are at ~/.claude/skills/close-task/scripts/task.sh and sensors.sh; run `task.sh show __TID__` from the repo root first and read only the slice it gives you plus the ids it cites. The repo has no repo-local scripts/task.sh; use the bundled one.

Per-task review (this replaces the reviewer dispatch in close-task's REVIEW step). Record the pre-task sha before you change anything. Size the review to the diff:

- evidence or documentation only -> `--legs "GLM-5.3:diff"`;
- anything else -> `--legs "DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff"`;
- Never add a Claude leg; where the prompt says `claude/opus:diff`, use `Qwen3.8-2.4T-A95B-NVFP4:diff` instead.
Command:
  ~/.claude/skills/fanout-review/scripts/fanout.sh --repo /Users/kheimel/.treehouse/kollect-79dca7/1/kollect --target "task __TID__: <task text>; verification matrix in openspec/changes/2026-10-06-product-decision-convergence/evidence/__TID__.md; diff <pre-task sha>..HEAD" --legs "<as sized>" --out openspec/changes/2026-10-06-product-decision-convergence/reviews/L-tasks/__TID__
Read only register.md; verify every CRITICAL and every single-model WARNING against the files yourself before acting; map CLEAN (or CONCERNS whose findings did not survive verification) -> APPROVE, a verified finding -> REQUEST_CHANGES, BLOCK -> BLOCK. Record every gate run (command, sha, exit, result) in the evidence BEFORE writing the review brief, so reviewers do not spend round 1 on missing records. A second round happens only if round 1 found a verified BEHAVIOUR defect (evidence or wording findings: fix, no new round); it goes in a fresh `round-2/` directory and is the last: after it, fix only verified behavioural defects (no further review), record everything else as nits or gaps, each rejected with a reason or deferred to a named place, and close. If fanout.sh exits non-zero or register.md has no `## Unified verdict` line, do not wait: read the leg files yourself (each has its own `## Verdict`), say so in the state file, and continue. If fanout.sh cannot run at all, say so in the state file and use a fresh general-purpose subagent with ~/.claude/skills/close-task/reference/review-brief.md instead. Say in the state file which legs ran and whether a Claude leg ran and why. DeepSeek legs have been timing out at the 900s default in this environment: if a DeepSeek leg exits truncated, note it, count the legs that DID produce verdicts, and continue when at least one non-implementer-family leg produced a verdict; if no leg produced a verdict, rerun once with `--legs "GLM-5.3:diff,Qwen3.8-Flash-Next:diff"` before falling back.

Fitness functions: the loop state file (openspec/changes/2026-10-06-product-decision-convergence/loop.md, section *Fitness functions*) lists the repository's architectural fitness functions; the rows marked per-task run in your `task` tier on the final tree. If your diff adds an import edge between components, a dependency, a package or component, a function over the complexity threshold, or touches determinism-sensitive code, either the existing functions pass unchanged or you extend one in this task and say why in the evidence. Never widen an allow-list, exclusion list or coverage baseline to get green: that is an architecture decision, report BLOCKED with the decision request.

Test budget: focused tests while iterating; ONE full suite on the final code tree; after an evidence- or docs-only commit, the fast check only. Do not create or edit files in the repo while the full suite runs (untracked-file guards fail it). Mutation proof through the repository's changed-lines mutation gate when it has one; hand-written mutants only where it cannot reach, as a representative sample. If your change touches code that only a deselected test tier or a scheduled CI job runs (golden, e2e, live markers), run that tier locally before closing; if it cannot run here, close CLOSED-WITH-GAPS naming the tier (the orchestrator dispatches CI for it). Docker is NOT available in this environment; `task test-integration` cannot run: name it as a gap, never fake it.

Shell: this session runs inside a task worktree; keep each Bash command plain - no loops over hosts, no `$(...)` or variables feeding git/gh, no heredoc whose text mentions git, no long jq/python filters inline; write scripts and briefs with the Write tool and run them as files. Refused commands cost a round trip each.

Lessons from earlier tasks: __LESSONS__

If a trigger fires but no Claude review legs are left, run no Claude leg and close CLOSED-WITH-GAPS naming the review you skipped.

Test tasks (the task text says it writes tests): follow close-task's *Test tasks* rule. Write the cases the task names from the spec deltas and the task text, not from design.md's rationale. Close only on a verified red: each new test fails for its stated reason, command and failure line in the evidence; list them under `## Known red`. A compile error or missing symbol is not that red: a stub with the signature the spec/design fixes (body: not implemented) is in scope; report BLOCKED only if the spec fixes no signature. A test that already passes is a finding against the test unless you established that the outcome already holds; then say so and close. Never bypass a commit hook that refuses red tests: report BLOCKED.
Known red for this change (excluded from "green" at close, nothing else is): __KNOWN_RED__
Implementation tasks: tests written by an earlier test task of this change are fixed points. Change one only through close-task's TEST class with evidence citing the spec delta or design.md (a spec-versus-design conflict is SPECIFICATION: BLOCKED), list each change under `## Test changes` in the evidence, and name it in your report. Report every known-red test you turned green.

Task kinds: if the task text says "Owner session" or "Owner gate", or needs a host, account, push, pull request or decision you do not have, do NOT attempt that part. Whole task -> stop at SELECT and report `OWNER` with the command sheet you would hand the owner written to openspec/changes/2026-10-06-product-decision-convergence/evidence/__TID__.md. Part -> do the agent part, write the owner part as a command sheet in the same file, close CLOSED-WITH-GAPS listing it.

Hard rules:

- Never read, print, decrypt or copy a secret value; reference entries and secret names only. There are no secret stores in-tree; never print values from kubeconfig, environment or test fixtures that look like credentials.
- Do not push, add or change any git remote, or open a pull request. Local commits on `fm/kollect-product-decisions-impl` only, one logical commit including the state file, Conventional Commit style with the repo's gitmoji prefix habit; end the message body with a `Signed-off-by: Konrad Heimel <konrad.heimel@gmail.com>` trailer.
- Do not edit CLAUDE.md, AGENTS.md, openspec/config.yaml or the historical `specs/001-*` bundles; propose instead.
- British English, terse. Tag every claim in evidence as verified (you ran or read it, with date) or believed.
- Evidence: the close-task state file is openspec/changes/2026-10-06-product-decision-convergence/evidence/__TID__.md; if the task names another evidence file, write that too and link it. No key, token, kubeconfig, whole log or address that is not needed.
- Commit the review `register.md` and `leg-*.md` files with the task when records live in the repo; never the `*.err` transcripts.
- Tick the task in tasks.md exactly as close-task says (this repo's tasks.md uses `- [ ] TNN` checkboxes).

Final report, at most 20 lines: task · verdict (CLOSED / CLOSED-WITH-GAPS / BLOCKED / OWNER) · matrix summary · sensors run · corrections · review verdict, legs and register path, Claude legs used · findings rejected or deferred (one line each: finding · reject/defer · reason or destination) · known red added or cleared · test changes · learnings (class, plus each reusable lesson in one line, for the next task's prompt) · commit sha · next action or decision request. Nothing else.
