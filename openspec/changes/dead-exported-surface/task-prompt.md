You are closing ONE task in a spec-loop run. State lives in files only; you carry nothing in from any earlier session.

Repo: /Users/kheimel/.treehouse/kollect-79dca7/2/kollect (branch `fm/kollect-docs-review-followup`, must be clean when you start; check with `git status`).
Feature directory: openspec/changes/dead-exported-surface (proposal.md, tasks.md; there is no .specify/ tooling — the change declares skip_specs, a pure refactor).
Records directory: openspec/changes/dead-exported-surface (review registers under reviews/L-tasks/<TID>/, evidence under evidence/<TID>.md).

Do this: invoke the `close-task` skill (Skill tool, name `close-task`) and follow it end to end for task T2 below. NOTE: this repo's tasks.md uses numbered sections, not Spec Kit task ids — `task.sh show T2` will not resolve; work from the task text in this prompt and tasks.md section 2 instead. Its helper scripts (sensors.sh) are at ~/.claude/skills/close-task/scripts/.

Task T2 — Zero-reference deletions (DR-2, DR-3):
- 2.1 Probe then delete `MergeRequestAPI` from `internal/sink/gitlab/client.go` (zero refs anywhere, tests included — record the probe output)
- 2.2 Probe: `grep -rn "ConditionConnected\|ConditionCredentialsVerified" --include='*.go' .` → only `api/v1alpha1/constants.go:10-11`; delete both constants. The deletion commit's body names the removed exported constants (the changelog is commit-derived, no manual CHANGELOG.md edits)
- 2.3 Compile clean; `go test ./internal/sink/gitlab/... ./api/...` green; grep confirms no docs/CRD text names the constants

Context you need (verified by the orchestrator at this HEAD): `MergeRequestAPI` has zero references outside its own definition (not even tests). The two condition constants have zero references anywhere in code, docs, charts or CRDs. The surrounding file content in both packages stays. Do NOT touch anything else. Commit body example: `:recycle: refactor(api): delete unused exported symbols (MergeRequestAPI, ConditionConnected, ConditionCredentialsVerified)` — name every removed exported symbol so the commit-derived changelog records the API change.

Per-task review (this replaces the reviewer dispatch in close-task's REVIEW step). Record the pre-task sha before you change anything. Size the review to the diff (code):
  ~/.claude/skills/fanout-review/scripts/fanout.sh --repo /Users/kheimel/.treehouse/kollect-79dca7/2/kollect --target "task T2: zero-reference deletions (gitlab interface, api constants); verification matrix in openspec/changes/dead-exported-surface/evidence/T2.md; diff <pre-task sha>..HEAD" --legs "DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff" --out openspec/changes/dead-exported-surface/reviews/L-tasks/T2
Never add a Claude leg; where the prompt says `claude/opus:diff`, use `Qwen3.8-2.4T-A95B-NVFP4:diff` instead. __CLAUDE_LEGS_LEFT__ is 0.
Read only register.md; verify every CRITICAL and every single-model WARNING against the files yourself before acting; map CLEAN (or CONCERNS whose findings did not survive verification) → APPROVE, a verified finding → REQUEST_CHANGES, BLOCK → BLOCK. Record every gate run (command, sha, exit, result) in the evidence BEFORE writing the review brief, so reviewers do not spend round 1 on missing records. A second round happens only if round 1 found a verified BEHAVIOUR defect (evidence or wording findings: fix, no new round); it goes in a fresh round-2/ directory and is the last: after it, fix only verified behavioural defects (no further review), record everything else as nits or gaps, each rejected with a reason or deferred to a named place, and close. If fanout.sh exits non-zero or register.md has no `## Unified verdict` line, do not wait: read the leg files yourself (each has its own `## Verdict`), say so in the state file, and continue. If fanout.sh cannot run at all, say so in the state file and use a fresh general-purpose subagent with ~/.claude/skills/close-task/reference/review-brief.md instead. Say in the state file which legs ran and whether a Claude leg ran and why (it must not have).

Fitness functions: the loop state file (openspec/changes/dead-exported-surface/loop.md, section *Fitness functions*) lists the repository's architectural fitness functions; the rows marked per-task run in your `task` tier on the final tree. If your diff adds an import edge between components, a dependency, a package or component, a function over the complexity threshold, or touches determinism-sensitive code, either the existing functions pass unchanged or you extend one in this task and say why in the evidence. Never widen an allow-list, exclusion list or coverage baseline to get green: that is an architecture decision, report BLOCKED with the decision request.

Test budget: focused tests while iterating; ONE full suite on the final code tree; after an evidence- or docs-only commit, the fast check only. Do not create or edit files in the repo while the full suite runs (untracked-file guards fail it). This task's tier: `go build ./...`, `go vet ./...`, `go test ./internal/sink/gitlab/... ./api/...` are the task-tier sensors; run them on the final tree and record command/sha/exit/seconds.

Shell: keep each Bash command plain — no loops over hosts, no `$(…)` or variables feeding git/gh, no heredoc whose text mentions git, no long jq/python filters inline; write scripts and briefs with the Write tool and run them as files. Refused commands cost a round trip each.

Lessons from earlier tasks: do not truncate probe output — a truncated grep in the orchestrator's planning missed live breaker tests (cost: one spec-set round). The combined sink+controller suite needs a 900s+ timeout window. Fanout diff legs need a committed diff range: the task commits its work, then amends only for evidence wording, keeping one logical commit. `ResetBreakersForTest` is process-global (pre-existing flake, out of scope). kollect has no .specify/; the spec set lives in openspec/changes/dead-exported-surface/ (validation: task spec:validate).

If a trigger fires but no Claude review legs are left, run no Claude leg and close CLOSED-WITH-GAPS naming the review you skipped.

Task kinds: if the task text says "Owner session" or "Owner gate", or needs a host, account, push, pull request or decision you do not have, do NOT attempt that part.

Hard rules:
- Never read, print, decrypt or copy a secret value; reference entries and secret names only.
- Do not push, add or change any git remote, or open a pull request. Local commits on `fm/kollect-docs-review-followup` only, one logical commit including the state file, Conventional Commit style with a gitmoji prefix (repo convention, e.g. `:recycle: refactor(api): ...`).
- Do not edit CLAUDE.md, AGENTS.md, the constitution, .specify/ or .claude/; propose instead.
- Tag every claim in evidence as verified (you ran or read it, with date) or believed. British English, terse.
- Evidence: the close-task state file is openspec/changes/dead-exported-surface/evidence/T2.md. No key, token, kubeconfig, whole log or address that is not needed.
- Commit the review `register.md` and `leg-*.md` files with the task when records live in the repo; never the `*.err` transcripts.
- Tick the task in tasks.md (mark 2.1–2.3 `[x]`) exactly as close-task says.

Final report, at most 20 lines: task · verdict (CLOSED / CLOSED-WITH-GAPS / BLOCKED / OWNER) · matrix summary · sensors run · corrections · review verdict, legs and register path, Claude legs used · findings rejected or deferred (one line each: finding · reject/defer · reason or destination) · known red added or cleared · test changes · learnings (class, plus each reusable lesson in one line, for the next task's prompt) · commit sha · next action or decision request. Nothing else.
