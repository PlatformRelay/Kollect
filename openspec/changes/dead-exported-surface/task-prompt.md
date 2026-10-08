You are closing ONE task in a spec-loop run. State lives in files only; you carry nothing in from any earlier session.

Repo: /Users/kheimel/.treehouse/kollect-79dca7/2/kollect (branch `fm/kollect-docs-review-followup`, must be clean when you start; check with `git status`).
Feature directory: openspec/changes/dead-exported-surface (proposal.md, tasks.md; no .specify/ — skip_specs pure refactor).
Records directory: openspec/changes/dead-exported-surface (review registers under reviews/L-tasks/<TID>/, evidence under evidence/<TID>.md).

Do this: invoke the `close-task` skill (Skill tool, name `close-task`) and follow it end to end for task T10 below. NOTE: this repo's tasks.md uses numbered sections, not Spec Kit task ids — work from the task text in this prompt and tasks.md section 10 instead. Helper scripts at ~/.claude/skills/close-task/scripts/.

Task T10 — Stage-B hygiene fixes (records + test names only). This task resolves the two fix-class findings from the whole-branch review (reviews/B/register.md, unified CLEAN 9/9 legs; findings 2 and 3). All other register entries are dispositioned in the loop — do NOT act on them.

- 10.1 In `internal/sink/git/export_test.go`: rename the test-local helper `exportMemory` and the `TestExportMemory*` test functions so NO identifier in the file contains the substring `ExportMemory` (that identifier was deleted from production by task T4; the surviving names make it greppable and mislead future probes — evidence/T8.md row 22 had to classify them specially). Pick a descriptive name for the helper (it builds an in-memory go-git repo, writes the payload at the object path, commits) and matching test names. Bodies and assertions unchanged; gofmt clean.
- 10.2 Refresh the stale line anchors the deletion invalidated, preferring SYMBOL anchors over line numbers so they cannot go stale again: (a) `openspec/changes/dead-exported-surface/evidence/T1.md` coverage-accounting rows cite export.go lines :389/:463/:517 which are now :70/:126/:180 — re-anchor by symbol/function name with current line numbers in parentheses; (b) the comment in `internal/sink/git/export_test.go` around :74 cites `export.go:125-127` — re-point it to the symbol(s) it means (the comment describes behaviour that now lives in the ExportFilesWithBranch pipeline; cite the function names).
- 10.3 Gates on the final tree: `go build ./...`, `go vet ./...`, `go test ./internal/sink/git/` (allow 1500s; the package ran 427s in T8's suite and 332s isolated — if a test panics with the 10-minute alarm in a subprocess hang you did not touch, classify ENVIRONMENT, isolate, retry once, record both runs), `task spec:validate` 14/14. Record command/sha/exit/seconds each.

Per-task review (this replaces the reviewer dispatch in close-task's REVIEW step). Record the pre-task sha before you change anything. Size the review to the diff (test-rename + records):
  ~/.claude/skills/fanout-review/scripts/fanout.sh --repo /Users/kheimel/.treehouse/kollect-79dca7/2/kollect --target "task T10: stage-B hygiene fixes (test-name degrep + stale-anchor refresh); verification matrix in openspec/changes/dead-exported-surface/evidence/T10.md; diff <pre-task sha>..HEAD; register findings 2+3 from reviews/B/register.md" --legs "DeepSeek-V4.1-Flash:diff" --out openspec/changes/dead-exported-surface/reviews/L-tasks/T10
(one free diff leg — the diff is a mechanical rename plus record wording; never add a Claude leg; where the prompt says `claude/opus:diff`, use `Qwen3.8-2.4T-A95B-NVFP4:diff` instead; __CLAUDE_LEGS_LEFT__ is 0.)
Read only register.md; verify every CRITICAL and every single-model WARNING against the files yourself before acting; map CLEAN → APPROVE, a verified finding → REQUEST_CHANGES, BLOCK → BLOCK. Record every gate run (command, sha, exit, result) in the evidence BEFORE writing the review brief — and FINALIZE the matrix statuses before dispatching the review. A second round happens only if round 1 found a verified BEHAVIOUR defect (evidence or wording findings: fix, no new round); it goes in a fresh round-2/ directory and is the last. If fanout.sh exits non-zero or register.md has no `## Unified verdict` line, read the leg files yourself, say so in the state file, and continue. If fanout.sh cannot run at all, say so in the state file and use a fresh general-purpose subagent with ~/.claude/skills/close-task/reference/review-brief.md instead. Say in the state file which legs ran and whether a Claude leg ran and why (it must not have).

Fitness functions: the loop state file (openspec/changes/dead-exported-surface/loop.md, section *Fitness functions*) lists the repository's architectural fitness functions. Never widen an allow-list, exclusion list or coverage baseline to get green: that is an architecture decision, report BLOCKED with the decision request.

Test budget: focused package tests only for this task (test-rename delta; the full suite and coverage are already green at the stage-B tip per loop.md). Do not create or edit files in the repo while the suite runs (untracked-file guards fail it).

Shell: keep each Bash command plain — no loops over hosts, no `$(…)` or variables feeding git/gh, no heredoc whose text mentions git, no long jq/python filters inline; write scripts and briefs with the Write tool and run them as files. `${PIPESTATUS[0]}` is a bash-ism, empty under zsh — record the tool's own ok line instead.

Lessons from earlier tasks: do not truncate probe output. Fanout diff legs need a committed diff range: the task commits its work, then amends only for evidence wording, keeping one logical commit. `internal/` deletions are plain `refactor:` commits (no `!`). Never run `task changelog` locally. Finalize the verification-matrix statuses BEFORE dispatching the review. Derive probe records mechanically via `git grep <sha>`, never transcribe live grep output. Evidence headers must not say FRAMED at review dispatch. A paired package-var shadow can be half-flagged (ctx reported, cancel not): probe both names when touching one (T9). Test fixtures calling `RegisterTarget` must `engine.Start(ctx)` first. `ResetBreakersForTest` is process-global (pre-existing flake, out of scope). kollect has no .specify/; the spec set lives in openspec/changes/dead-exported-surface/ (validation: task spec:validate).

If a trigger fires but no Claude review legs are left, run no Claude leg and close CLOSED-WITH-GAPS naming the review you skipped.

Task kinds: if the task text says "Owner session" or "Owner gate", or needs a host, account, push, pull request or decision you do not have, do NOT attempt that part.

Hard rules:

- Never read, print, decrypt or copy a secret value; reference entries and secret names only.
- Do not push, add or change any git remote, or open a pull request. Local commits on `fm/kollect-docs-review-followup` only, one logical commit including the state file, Conventional Commit style with a gitmoji prefix (repo convention, e.g. `:recycle: test(git): drop the deleted identifier from test names`).
- Do not edit CLAUDE.md, AGENTS.md, the constitution, .specify/ or .claude/; propose instead.
- Tag every claim in evidence as verified (you ran or read it, with date) or believed. British English, terse.
- Evidence: the close-task state file is openspec/changes/dead-exported-surface/evidence/T10.md. No key, token, kubeconfig, whole log or address that is not needed.
- Commit the review `register.md` and `leg-*.md` files with the task when records live in the repo; never the `*.err` transcripts.
- Tick the task in tasks.md (mark 10.1–10.3 `[x]`) exactly as close-task says.

Final report, at most 20 lines: task · verdict · matrix summary · sensors run · corrections · review verdict, legs and register path, Claude legs used · findings rejected or deferred · known red added or cleared · test changes · learnings · commit sha · next action or decision request. Nothing else.
