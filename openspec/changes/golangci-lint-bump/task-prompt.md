You are closing ONE task in a spec-loop run. State lives in files only; you carry nothing in from any earlier session.

Repo: /Users/kheimel/.treehouse/kollect-79dca7/6/kollect (branch `fm/kollect-dev-continue`, must be clean when you start; check with `git status`).
Feature directory: openspec/changes/golangci-lint-bump (an OpenSpec repo — NO .specify/: the spec set is proposal.md + specs/lint-toolchain/spec.md + tasks.md there; the loop state is loop.md in the same directory; repo standards live in CONTRIBUTING.md and docs/development/coding-standards.md; there is no constitution file).
Records directory for review registers: openspec/changes/golangci-lint-bump

Do this: invoke the `close-task` skill (Skill tool, name `close-task`, args `1.2`) and follow it end to end for task 1.2. Its helper scripts are at ~/.claude/skills/close-task/scripts/task.sh and sensors.sh; run `task.sh show 1.2 openspec/changes/golangci-lint-bump/tasks.md` from the repo root first (NOTE: pass that tasks.md path explicitly — task.sh cannot find it without it in this repo) and read only the slice it gives you plus the ids it cites.

Task-specific notes for 1.2 (version-only commit):

- The evidence/state file is openspec/changes/golangci-lint-bump/evidence/1.2.md (link it from loop.md's task row; tag every claim verified or believed).
- Read `task.sh show 1.2 openspec/changes/golangci-lint-bump/tasks.md` first, plus `task.sh show 1.1 ...` for what the probe already established (evidence/probe.md holds the resolved logtools version the plugin build used — the pin must equal it; read evidence/probe.md's version section before editing).
- The commit edits EXACTLY three version strings: `GOLANGCI_LINT_VERSION` (Makefile:184) v2.11.4 → v2.13.1; `version:` (hack/tooling/.custom-gcl.yml:6) v2.11.4 → v2.13.1; the logcheck plugin's `version: latest` (hack/tooling/.custom-gcl.yml:11) → the resolved pin from the probe (currently floats). Nothing else changes in this commit — version-only (LTB-1 equality + LTB-4).
- Gates to record in the evidence (LTB-3 sense: runnable, not clean): `make golangci-lint` (the plain-file hazard from the Lessons means it will re-run install + custom build, ~40–90 s — let it), `bin/golangci-lint version` (must report v2.13.1), `task lint` (EXPECTED non-clean: the 57 findings are known red until task 1.3 — record exit code and the findings-class summary, not a fix), `task format:check` (expected clean), and the pin-equality grep of both files. Known red for this task: those 57 findings (goconst 44, gosec G710 ×1, staticcheck SA1019 ×12) — excluded from "green" at close, nothing else is.
- Commit message: version-only conventional commit, e.g. `:arrow_up: build(lint): bump golangci-lint to v2.13.1 and pin the logcheck plugin` (check CONTRIBUTING.md's type table; build type owns version bumps).
- Per-task review (this replaces the reviewer dispatch in close-task's REVIEW step). Record the pre-task sha before you change anything. Size the review to the diff — config/manifest diff → two free diff legs on DIFFERENT model families from the implementer (GLM-5.3 is the implementer):
  ~/.claude/skills/fanout-review/scripts/fanout.sh --repo /Users/kheimel/.treehouse/kollect-79dca7/6/kollect --target "task 1.2: version-only pin bump; evidence at openspec/changes/golangci-lint-bump/evidence/1.2.md; diff <pre-task sha>..HEAD" --legs "DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff" --out openspec/changes/golangci-lint-bump/reviews/L-tasks/1.2
Never add a Claude leg; where the prompt says `claude/opus:diff`, use `Qwen3.8-2.4T-A95B-NVFP4:diff` instead. 0 Claude review legs left for this task.
Read only register.md; verify every CRITICAL and every single-model WARNING against the files yourself before acting; map CLEAN (or CONCERNS whose findings did not survive verification) → APPROVE, a verified finding → REQUEST_CHANGES, BLOCK → BLOCK. Record every gate run (command, sha, exit, result) in the evidence BEFORE writing the review brief, so reviewers do not spend round 1 on missing records. A second round happens only if round 1 found a verified BEHAVIOUR defect (evidence or wording findings: fix, no new round); it goes in a fresh `round-2/` directory and is the last: after it, fix only verified behavioural defects (no further review), record everything else as nits or gaps, each rejected with a reason or deferred to a named place, and close. If fanout.sh exits non-zero or register.md has no `## Unified verdict` line, do not wait: read the leg files yourself (each has its own `## Verdict`), say so in the state file, and continue. If fanout.sh cannot run at all, say so in the state file and use a fresh general-purpose subagent with ~/.claude/skills/close-task/reference/review-brief.md instead. Say in the state file which legs ran and whether a Claude leg ran and why.

Fitness functions: the loop state file (openspec/changes/golangci-lint-bump/loop.md, section *Fitness functions*) lists the repository's architectural fitness functions; the rows marked per-task run in your `task` tier on the final tree. If your diff adds an import edge between components, a dependency, a package or component, a function over the complexity threshold, or touches determinism-sensitive code, either the existing functions pass unchanged or you extend one in this task and say why in the evidence. Never widen an allow-list, exclusion list or coverage baseline to get green: that is an architecture decision, report BLOCKED with the decision request.

Test budget: focused checks while iterating; after an evidence- or docs-only commit, the fast check only (this task is evidence-only). Do not create or edit files in the repo while the full suite runs (untracked-file guards fail it). Mutation proof is not expected for an evidence-only task.

Shell: if the session is worktree-isolated, keep each Bash command plain — no loops over hosts, no `$(…)` or variables feeding git/gh/codex, no heredoc whose text mentions git, no long jq/python filters inline; write scripts and briefs with the Write tool and run them as files. Refused commands cost a round trip each.

Lessons from earlier tasks: none yet.

If a trigger fires but no Claude review legs are left, run no Claude leg and close CLOSED-WITH-GAPS naming the review you skipped.

Task kinds: if the task text says "Owner session" or "Owner gate", or needs a host, account, push, pull request or decision you do not have, do NOT attempt that part. Whole task → stop at SELECT and report `OWNER` with the command sheet you would hand the owner written to openspec/changes/golangci-lint-bump/evidence/1.2.md. Part → do the agent part, write the owner part as a command sheet in the same file, close CLOSED-WITH-GAPS listing it.

Hard rules:

- Never read, print, decrypt or copy a secret value; reference entries and secret names only. This repo has no known secret store in-tree; never print environment variables, tokens, or ~/.gitconfig contents; git commands run with the worktree's own config.
- Do not push, add or change any git remote, or open a pull request. Local commits on `fm/kollect-dev-continue` only, one logical commit including the state file, Conventional Commit style with an optional gitmoji shortcode prefix per CONTRIBUTING.md (no attribution trailer).
- Do not edit CLAUDE.md, AGENTS.md, the constitution, .specify/ or .claude/; propose instead.
- British English, terse. Tag every claim in evidence as verified (you ran or read it, with date) or believed.
- Evidence: the close-task state file is openspec/changes/golangci-lint-bump/evidence/1.2.md; if the task names another evidence file, write that too and link it. No key, token, kubeconfig, whole log or address that is not needed.
- Commit the review `register.md` and `leg-*.md` files with the task when records live in the repo; never the `*.err` transcripts.
- Tick the task in tasks.md exactly as close-task says (tasks.md path: openspec/changes/golangci-lint-bump/tasks.md).

Final report, at most 20 lines: task · verdict (CLOSED / CLOSED-WITH-GAPS / BLOCKED / OWNER) · matrix summary · sensors run · corrections · review verdict, legs and register path, Claude legs used · findings rejected or deferred (one line each: finding · reject/defer · reason or destination) · known red added or cleared · test changes · learnings (class, plus each reusable lesson in one line, for the next task's prompt) · commit sha · next action or decision request. Nothing else.
