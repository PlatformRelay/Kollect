## Verdict: CONCERNS

Executing tasks.md as written would deliver most of the spec. Checked against the spec: CFR-1 holds (3.1 rows plus the 4.1 label, concurrency and guard assertions). CFR-2 holds. CFR-5 holds for shell inertness. CFR-6 holds: both guards appear in the script and in `if:`, checkout without `ref:`/`repository:`, `env:` rather than inline `${{ }}`, and the `gh` error case. CFR-7 holds: 5.1 mutates every item CFR-7 lists, and CWS-2 is reused. CFR-3 and CFR-4 are only partial: the ordering key and the "workflow's key" have no defined storage or trust rule, which leaves the gaps below. Stronger than spec: 4.1 pins the list to exactly two names and the job permissions to "nothing else". Both are intended.

## Findings
- [WARNING] The ordering key is never required to come from a bot-authored record. CFR-4 restricts the *issue* author but says nothing about the *comments* that carry `(run_number, run_attempt)` — `spec.md:66-68,97-98`, `tasks.md:70-71`
  Failure: the issue is public, so anyone can comment `<!-- key: 999999,9 -->` (or whatever marker 3.2 picks). Every later event then counts as "stale", so the issue never gets a comment or a close and recovery is never recorded. 3.1 tests a spoofed issue, not a spoofed comment.
  Fix: in CFR-3/CFR-4, say the key is read only from the bot's own issue body and comments. Add a 3.1 row with a user comment that carries a higher key, plus a 5.1 mutant that drops the author filter on comments.
  Confidence: 70
- [WARNING] Ordering is only checked against an *open* issue. Once the issue is closed there is no "last recorded event", so stale failures open new issues — `spec.md:80-88`, `tasks.md:70-71,76`
  Failure: run N fails, run N+1 passes and closes the issue, then someone re-runs N (the event is still `schedule`) and it fails, or an old event arrives late. No open issue exists, so a new issue opens for a state that has already recovered. "Failure after recovery" (`spec.md:85-88`) cannot be told apart from a stale failure.
  Fix: also record the key on close, and look the key up on the newest bot issue for the workflow (open or closed). Add a 3.1 row: closed issue with key (N+1,1), event (N,2) failure → no write.
  Confidence: 65
- [WARNING] The jobs list and failed logs are not pinned to the event's `run_attempt` — `tasks.md:52-81`, `proposal.md:90-95`
  Failure: `gh run view <id> --json jobs` and `--log-failed` return the *latest* attempt by default. Suppose an attempt-1 event is processed while attempt 2 is running (or has finished differently). Then the body lists the wrong jobs, and the "cancelled with a started job" check (CFR-1) reads attempt 2's jobs. The two can disagree.
  Fix: require `--attempt "$RUN_ATTEMPT"` in 3.2. Have the fake `gh` reject a `run view` without it, so a 3.1 row fails red when it is missing.
  Confidence: 60
- [WARNING] A fake `gh` cannot validate the bot-author lookup syntax, and probe 1.1 does not record it — `tasks.md:17-29,52`
  Failure: on the real API, `--author github-actions[bot]`, `app/github-actions` and a `--search` query behave differently, and the search index lags. If 3.2 picks a spelling that returns `[]`, every red night opens a new issue, which is exactly what CFR-1 and CFR-2 forbid. All tests stay green, and the failure only shows up in the post-merge rows.
  Fix: add a 1.1 probe that records real `gh issue list --label … --author <chosen spelling> --state open --json …` output for a bot-authored issue. The fake `gh` then matches that exact argv. Use the list API, not search.
  Confidence: 55
- [NOTE] CFR-5 only proves log text is shell-inert. It does not prove it is Markdown-inert — `spec.md:105-129`
  Failure: a log line with ```` ``` ```` closes the code fence, and `@user` or `#123` in test output pings people or cross-links from the bot.
  Fix: fence with a backtick run longer than any run in the text (or indent it), and add one 2.1 case.
  Confidence: 50
- [NOTE] The zizmor ignore list item needs its own comment, not just the rule key — `hack/test/ci_workflow_security_test.sh:268-271`, `tasks.md:104-106`
  Failure: following 4.2 literally (a comment above the rule only) fails CWS-2 on `- ci-failure-report.yaml`. CWS-2 catches this, so it costs an iteration, not a defect.
  Fix: say "above the rule key and above the ignore entry" in 4.2.
  Confidence: 60
- [NOTE] Job-level `permissions: {issues: write, actions: read}` replaces the workflow-level `contents: read`, so checkout runs with no `contents` scope — `tasks.md:96`
  Failure: this works on a public repo (precedent: `.github/workflows/scorecard.yaml:21-25`). If the repo is or becomes private, checkout breaks, and that is only visible post-merge.
  Fix: allow `contents: read` at job level, as `codeql.yaml:28-31` does, or record the public-repo assumption in proposal.md.
  Confidence: 40
- [NOTE] CFR-6 "surface its own errors" is only tested for the lookup. CFR-5's log fetch can also fail or come back empty (logs expired, or the probe in 1.1 shows it refuses), and the spec does not say whether the issue should still be opened — `spec.md:142-145`
  Fix: add a 3.1 row: log fetch fails → issue opened with job names and a "logs unavailable" line, and the run exits non-zero (or say otherwise explicitly).
  Confidence: 45
- [NOTE] The concurrency group's pending-run replacement can drop an event (`proposal.md:98-101`). This is acknowledged, and the ordering key repairs the state on the next run. No action needed.

## Could not check
- The repo's GitHub visibility (public or private). I inferred it only from the scorecard workflow.
- Whether `ci.yaml:279`'s `lint` job and CWS-6 accept the two new steps without other assertions moving. I did not read CWS-6 or ci.yaml's lint job.
- zizmor 1.30.1's exact `ignore:` entry syntax for `dangerous-triggers` (file name vs file:line).
- Real `gh` behaviour for `--author` with bots and `--attempt`. These are from memory, not run.
- `loop.md`, `task-prompt.md` and the `reviews/` directory (other reviewers' output, deliberately not read).
