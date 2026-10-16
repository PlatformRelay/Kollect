# Tasks

Red first. Every test task comes before the implementation it drives and records its red in
evidence, failing on the target assertion (not on a parse error or a usage error). A case that
expects a write records its red against a stub that exits 0 and does nothing; a case that expects
no write records its red against a stub that writes unconditionally (it always runs
`gh issue create`). 4.1 is red on a missing workflow file and may report one diagnostic per
assertion naming what it found missing. `task check` runs every `hack/test/*_test.sh`
(`hack/check.sh:79`), so a red test cannot be committed alone: each test task closes in one commit
with the implementation task after it (2.1 with 2.2; 3.1 and 3.2 with 3.3; 4.1 with 4.2), and the
tree is green at every commit. The test's plain mode joins the required `lint` job (`ci.yaml:279`)
in the first of those commits; its `--self-test` mode joins in 5.1, when it exists. Until 5.1 the
test SHALL contain no non-comment `--self-test` token: mode detection is textual
(`ci_workflow_security_test.sh:459`, `hack/check.sh:93`), so an early token makes CWS-6 and
`task check` demand a mode that does not exist yet.

Fixtures live under `hack/test/testdata/ci-failure-report/`. Evidence under
`openspec/changes/ci-failure-reporting/evidence/` is Markdown only
(`hack/test/openspec_evidence_markdown_only_test.sh`). The fake `gh` records each call one
argument per line, so argument boundaries survive.

## 1. Probe

- [x] 1.1 Record fixtures and check the assumptions in proposal.md, each with its source. Every
  `gh` call here is read-only:
  - a `workflow_run` payload (GitHub docs example, trimmed to the fields CFR-1 to CFR-6 read,
    `workflow_id` and `path` included);
  - `gh run view 38871014363 --attempt 1 --json jobs` for scheduled nightly run 38871014363
    (11 job entries, all `success`, `scale-envtest-10k` `skipped`), edited into a two-failed copy
    and a timed-out copy;
  - `gh run list --workflow .github/workflows/e2e-nightly.yaml --event schedule --status completed
    --limit 1 --json databaseId,number,attempt,conclusion`, and, from any workflow with a re-run
    of an older run, whether `gh run list` still lists the newer run first;
  - a cancelled-before-start run (`gh run list --status cancelled` on any workflow), or the
    documented shape if none exists, marked as such; from it, the rule that tells a started job
    from an unstarted one (which job fields), which 3.1's fixtures then pin;
  - `gh run view --log-failed` on an in-progress run (expected: refused or empty);
  - the job conclusion and run conclusion of a failed `continue-on-error` job, from any repository
    run, to confirm the run stays `success`;
  - the bot-author spelling for `gh issue list`. This repository has no bot-authored issue
    (29 issues, all user-authored, read 2026-10-16), and Renovate here may author as a user or as
    `github-actions` (`.github/workflows/renovate.yaml:33`). So: find a bot-authored issue in any
    public repository, read its `author` object, and record the real output of
    `gh issue list --repo <it> --label <its label> --author <spelling> --state open --json
    number,author,labels` for the spellings that follow from it (`app/<bot>`, `<bot>[bot]`,
    `--app <bot>`), including `app/github-actions` when a `github-actions`-authored issue is
    found. The reporter uses the working spelling for `github-actions`, and the fake `gh` matches
    that exact argv. If no `github-actions`-authored issue can be probed, say so, and the lookup
    argv joins the post-merge row. Never `--search`.
  `gh label create --force` is not probed: its documented behaviour is cited in proposal.md, and
  its live proof is the post-merge row. If a payload field CFR-6 relies on is absent, stop: the
  change is BLOCKED. Evidence: `evidence/T1.1.md`. — closed 2026-10-16, evidence: evidence/T1.1.md

## 2. Body builder (CFR-5)

- [x] 2.1 Write the body-builder cases in `hack/test/ci_failure_report_test.sh` and record them
  red against a stub `hack/ci/ci-failure-issue-body.sh` that prints nothing:
  - given the nightly job list with two failed entries, when the body is built, then it lists
    exactly those two by name (exposes a builder that lists all jobs, or only the first);
  - given workflow `E2E nightly`, run URL and key, when the body is built, then its first line is
    the bot marker (CFR-4 shape, `workflow=.github/workflows/e2e-nightly.yaml`) and the body names
    the workflow and the run URL (exposes a body without the fields the issue exists to carry);
  - given a log with 80 `FAIL` lines, when the body is built, then it carries exactly 50 (exposes an
    unbounded excerpt);
  - given a failed log over the cap, when the body is built, then the whole body, marker included,
    is under 60000 characters, closes the excerpt fence before a visible truncation marker and
    still lists every failed job (exposes truncation that drops jobs, leaves a fence open, or
    yields a body GitHub rejects at 65536);
  - given a timed-out job with no `FAIL` line, when the body is built, then the job is listed with
    its failed step name (exposes a builder keyed on `FAIL` lines only);
  - given a log line `$(touch pwned)` and one with backticks, when the body is built in a scratch
    directory, then both appear verbatim and no `pwned` file exists (exposes `eval` or an unquoted
    expansion of log text);
  - given a log line with a run of five backticks, `@user` and a marker lookalike with a higher
    key, when the body is built, then the excerpt's fence is longer than five backticks and closes
    after the line, no `@user` appears outside a fence, and the first line is the real marker
    (exposes a fence the log can close, a bot that pings users, and a forgeable marker);
  - given a failed job named `deploy @user`, when the body is built, then the name is listed with
    its `@` neutralised and no `@user` mention appears outside a fence (exposes a mention through a
    job or step name);
  - given no failed log (fetch failed), when the body is built, then it lists the failed jobs and
    says "logs unavailable" (exposes a body that drops jobs when logs expire);
  - given a job list with no failed job, when the body is built, then it says so and lists none
    (exposes a builder that invents a failure). — closed 2026-10-16, evidence: evidence/G2.md
- [x] 2.2 Implement `hack/ci/ci-failure-issue-body.sh`; 2.1 green; plain mode wired into `lint`.
  One commit with 2.1. — closed 2026-10-16, evidence: evidence/G2.md

## 3. Decision logic (CFR-1 to CFR-4, CFR-6)

- [x] 3.1 Write the decision-table cases with a fake `gh` on `PATH` that records every call and
  serves a fake issue store, parameterised from the table in spec.md CFR-1, and record them red
  (preamble rule). The run acted on comes from the fake `gh run list`; each case asserts the exact
  recorded writes:
  - each failure-class conclusion (`failure`, `timed_out`, `startup_failure`, `cancelled` with a
    started job) with no bot issue: one `issue create` with label `ci-failure` and marker
    `event=failure` (CFR-1; exposes a timeout or startup failure read as green);
  - failure, open issue, newer key: one comment and no create (CFR-2; exposes a duplicate issue
    per night);
  - failure, open issue, equal key (identical replay), and older key: no write (CFR-2, CFR-3;
    exposes a duplicate comment on a reporter re-run);
  - failure, closed issue, newer key: one new issue, no reopen (CFR-3);
  - failure, closed issue keyed (N+1, 1), run (N, 2): no write (CFR-3; exposes a re-run of an old
    run reopening a recovered state);
  - success, no issue: no write (CFR-3; exposes noise on every green night);
  - success, open issue, newer key: recovery comment with marker `event=recovery`, then close
    (CFR-3; exposes an issue that never closes);
  - success, open issue, equal key with last marker `event=recovery`: close only; equal with
    `event=failure`, or older: no write (CFR-3; exposes a lost close and a stale close);
  - success, closed issue: no write;
  - each no-op conclusion (`cancelled` with no started job, `action_required`, `neutral`,
    `skipped`, `stale`), with and without an open issue: no write (CFR-1; exposes an issue for a
    replaced pending run);
  - run N attempt 1 recorded as failed, run N attempt 2 `success`: close (CFR-3; exposes ordering
    on `run_number` alone);
  - run `success` while one job failed under `continue-on-error`: no write (CFR-1, decision
    2026-10-16 A; exposes job-level reporting this change excludes);
  - sequence pass, fail, fail, pass, fail on one fake issue store: none, create, comment, close,
    create;
  - sequence for `Go patch lag`: fail, fail, fail, pass: create, comment, comment, close.
  — closed 2026-10-16, evidence: evidence/G3.md
- [x] 3.2 Write the lookup, trigger and robustness cases with the same fake `gh`, red by the
  preamble rule:
  - only a user-authored open issue with the label, the reporter's title and a marker: a new bot
    issue and no write to the user's (CFR-4; exposes a title or label-only lookup);
  - a bot issue with a user comment carrying a marker with a higher key, then a newer failure: one
    comment (CFR-4; exposes a key read from untrusted text);
  - a bot issue whose excerpt holds a marker lookalike with a higher key, then a newer failure:
    one comment, because only the first-line marker counts (CFR-4; exposes a key forged by a log);
  - `E2E nightly` and `Go patch lag` issues both open, nightly `success`: only the nightly's issue
    gets the recovery and closes (CFR-4; exposes a lookup without the workflow filter);
  - the lookup argv equals the spelling recorded in 1.1, uses `--state all` and an explicit
    `--limit`, and never `--search` (CFR-4; exposes a lookup that returns `[]` on the real API);
  - the fake returns exactly `--limit` bot issues, all from another workflow: the script exits
    non-zero with no write (CFR-4; exposes a truncated lookup that opens a second issue);
  - the payload names run N but the fake lists N+1 as the latest completed scheduled run: the
    decision uses N+1 (CFR-1; exposes acting on a late or replaced payload);
  - event `workflow_dispatch`, or `push`, or a head repository other than this one: no `gh` call
    at all (CFR-6; exposes reports from manual or fork runs);
  - the fake `gh` rejects any `run view` without `--attempt` equal to the listed run's attempt; a
    failing run with attempt 2 still reports (exposes jobs and logs read from the wrong attempt);
  - every `issue create` and `issue comment` passes its body through `--body-file`, never
    `--body`; the close is a bodyless `gh issue close <number>` and the fake rejects any other
    close flag; a non-numeric run id or issue number from the fake makes the script exit non-zero
    with no write (CFR-6);
  - `gh` exiting non-zero at each call in turn (run list, jobs, issue list, issue comments, create,
    comment, close): the script exits non-zero, stderr carries the `gh` error, and no write follows
    the failing call (CFR-6; exposes a swallowed error that leaves the run green);
  - recovery comment written, close fails, then the reporter runs again: one close, no second
    comment (CFR-3; exposes a retry that skips the close);
  - log fetch exits non-zero, and separately exits 0 with empty output: each time the issue is
    still opened with the job names and "logs unavailable", and the script exits non-zero (CFR-6).
  — closed 2026-10-16, evidence: evidence/G3.md
- [x] 3.3 Implement `hack/ci/ci-failure-report.sh`; 3.1 and 3.2 green. Quoted arguments,
  `--body-file`, numeric ids checked. One commit with 3.1 and 3.2. — closed 2026-10-16, evidence:
  evidence/G3.md

## 4. Workflow (CFR-1, CFR-6, CFR-7)

- [ ] 4.1 Write the wiring and guard assertions on `.github/workflows/ci-failure-report.yaml` and
  record them red on today's tree (the file is absent, so each assertion must name what it found
  missing, not stop at the first):
  - every name under `on.workflow_run.workflows` equals the `name:` of exactly one file under
    `.github/workflows/`, and the list is exactly `E2E nightly` (`e2e-nightly.yaml:2`) and
    `Go patch lag` (`go-patch-lag.yaml:6`) (exposes a rename that stops `workflow_run` firing);
  - `types: [completed]` only;
  - the job's `if:` requires `github.event.workflow_run.event == 'schedule'` and
    `github.event.workflow_run.head_repository.full_name == github.repository` (CFR-6);
  - concurrency group exactly `ci-failure-report-${{ github.event.workflow_run.workflow_id }}`,
    `cancel-in-progress: false` (CFR-1; exposes a group that collides with another workflow's,
    whose groups are prefixed too: `e2e-nightly.yaml:18`, `go-patch-lag.yaml:17`);
  - workflow `permissions: contents: read`; job permissions exactly `contents: read`,
    `actions: read`, `issues: write` (exposes a job map that silently drops `contents`);
  - every `actions/checkout` sets `persist-credentials: false` and no `ref:` or `repository:`
    (exposes a checkout of the triggering run);
  - a step running `gh label create ci-failure --force` with a fixed `--color` and
    `--description` precedes the step that runs `hack/ci/ci-failure-report.sh` (exposes a first
    report failing on a missing label, and a label recoloured at random on every run);
  - no step's `run:` interpolates `github.event.workflow_run.*` directly; values reach scripts
    through `env:` (exposes template injection through a branch or title);
  - a comment on the workflow list names the `race` gap and SINK-BREAKER-RACE-01.
- [ ] 4.2 Add `.github/workflows/ci-failure-report.yaml` and the `dangerous-triggers` rule in
  `.github/zizmor.yml`, replacing `rules: {}` (`.github/zizmor.yml:14`) with exactly this shape,
  each comment directly above its line:

  ```yaml
  rules:
    # dangerous-triggers: default-branch code only, no checkout of the triggering run, no log
    # text evaluated, schedule and same-repository guard.
    dangerous-triggers:
      ignore:
        # dangerous-triggers: exception limited to the guarded CI failure reporter.
        - ci-failure-report.yaml
  ```

  No comment anywhere under `.github/` may contain the literal `zizmor: ignore` (CWS-2's inline
  grep, `ci_workflow_security_test.sh:306`, matches it in comments too). Update the header comment
  that says there are no suppressions (`.github/zizmor.yml:10-13`). Then: 4.1 green; zizmor at the
  gate's command (`zizmor --offline --no-progress --min-severity=high --config .github/zizmor.yml
  .github/`) has no findings, which proves the filename selector; `hack/test/ci_workflow_security_test.sh`
  green in both modes, CWS-2 (`:236`) included. One commit with 4.1.

## 5. Proof and wiring (CFR-7)

- [ ] 5.1 Add `--self-test` to `hack/test/ci_failure_report_test.sh`, mutating copies, each judged; also assert the 4000-byte line cut happens (`[line cut]` present) with a cut-removed mutant (deferred from G2, its mutant M20 survived)
  by exit status and the message of the assertion it targets, plus a no-op control:
  - recovery branch removed: the pass-after-fail case reds;
  - label-and-author lookup reduced to title or label: the spoofed-issue case reds;
  - workflow filter dropped from the lookup: the both-open case reds;
  - ordering check removed: the stale-event and closed-issue stale cases red;
  - event or repository guard removed (script and workflow `if:` separately): the dispatch and
    fork cases red;
  - conclusion widened to job-level failures: the `continue-on-error` case reds;
  - `name: E2E nightly` changed in a copy of `e2e-nightly.yaml`: the wiring check reds naming
    `E2E nightly`;
  - label-create step removed, and moved after the report step: the wiring check reds;
  - `ref: ${{ github.event.workflow_run.head_sha }}` added to the checkout: the wiring check reds;
  - the comment above `dangerous-triggers:` removed, and separately the comment above
    `- ci-failure-report.yaml` removed, in copies of `.github/zizmor.yml`: CWS-2 reds on each
    (proves CFR-7's suppression scenario against the existing rule, not a new one);
  - both `lint` steps running the test deleted from a copy of `ci.yaml`: the self-wiring
    assertion reds.
  Add that self-wiring assertion to plain mode: `ci.yaml` invokes the test bare and with
  `--self-test` on uncommented lines (precedent `ci_fetch_lib_hardening_test.sh:865-882`), since
  CFC-6's fixed list (`ci_workflow_security_test.sh:602-603`) does not name this test. Wire
  `--self-test` into `lint` next to the plain step. Grep `hack/test/` for step-index or step-count
  assertions on `ci.yaml`'s `lint` job and update any that the new step moves.
  `hack/test/ci_workflow_security_test.sh` both modes and `task lint:shell` green.

## 6. Land

- [ ] 6.1 Independent review; PR with green CI on the head SHA; rebase merge with the `post-merge`
  rows open; archive in a follow-up evidence PR once they pass.

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFR-1 | 3.1 table rows per conclusion class, `continue-on-error` case; 3.2 late-payload case; 4.1 label step first, concurrency group | one issue; never two; no-op conclusions and job-level failure write nothing | not-run | |
| CFR-2 | 3.1 fail+open newer and equal; Go patch lag sequence | comment only; replay writes nothing | not-run | |
| CFR-3 | 3.1 pass-fail-pass sequence, re-run attempt, older and equal keys, closed-issue stale row; 3.2 close retry | recovery closes; stale writes nothing; failed close retried | not-run | |
| CFR-4 | 3.2 spoofed issue, spoofed comment marker, both-open, lookup argv | not touched; only own issue closes | not-run | |
| CFR-5 | 2.1 body fixtures incl. hostile text, Markdown injection, `FAIL` bound, logs unavailable | listed, capped, inert | not-run | |
| CFR-6 | 3.2 dispatch, fork, non-schedule, `gh` error at every call, log fetch failure, `--attempt`, `--body-file`; 4.1 `if:`, permissions and checkout | no action; error visible | not-run | |
| CFR-7 | 5.1 mutants judged by exit status plus no-op; rename mutant; both zizmor comment mutants; unwiring mutant; zizmor gate on the workflow | each mutant red; gate green | not-run | |
| all | `gh label list` after the first scheduled run following the merge (proves `--force` and `issues: write` live) | `ci-failure` exists | post-merge | follow-up evidence PR |
| all | first real scheduled night(s) and Monday(s) after merge; the bot-author lookup argv if 1.1 could not probe it | issue opens on a failure, closes on recovery; a green run makes no write; the lookup finds the bot issue | post-merge | follow-up evidence PR |
| all | independent review; CI on the PR head | APPROVE, green; reviewed and archive revisions recorded | not-run | |
