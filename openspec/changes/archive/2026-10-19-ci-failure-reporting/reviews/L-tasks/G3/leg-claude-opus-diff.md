## Verdict: CONCERNS

I found no correctness or security defect in the reporter. The concern is that the payload-field validation it relies on has no test.

## Findings
- [WARNING] No test covers the payload-field guards. The workflow-path check, the workflow-name cleanup and the `GITHUB_REPOSITORY` check have no case — `hack/ci/ci-failure-report.sh:55`, `:67`, `:68-69`
  Failure: delete line 67, or drop the `tr -d '[:cntrl:]' | cut -c1-200` from 68-69, and every case still passes. Every `payload` call in `hack/test/ci_failure_report_test.sh` passes a well-formed path and the names `E2E nightly` or `Go patch lag`. The brief lists path, name and head repository as injection surfaces, but only the head-repository guard has a case (`evil/Kollect`). The G3 matrix has no row for these guards, and none of the 28 mutants targets them.
  Fix: add cases for a payload path that fails the regex (for example `.github/workflows/../x.yaml`, or a path with a space or a leading `-`), expecting exit 1 and no gh call. Add one for a name with a newline and `$(touch pwned)`, expecting a single-line title in the stored issue and no `pwned` file. Add one for a malformed `GITHUB_REPOSITORY`.
  Confidence: 85
- [NOTE] The 200-character title cut counts bytes, not characters, because of `LC_ALL=C` (line 34). It can split a multi-byte UTF-8 workflow name and leave invalid UTF-8 in the `--title` argument — `hack/ci/ci-failure-report.sh:69`
  Failure: a name over 200 bytes with non-ASCII characters near byte 200 gets a broken last character in the title. This is cosmetic, and both reported names are short ASCII today.
  Fix: cut by character in jq (`.[0:200]` before `-r`), or leave it and record the limit.
  Confidence: 60
- [NOTE] The lookup limit counts every `github-actions[bot]` issue in the repository, in any state, not only `ci-failure` ones. Once that total reaches 500, every report exits 1 at line 117–118, permanently, and nothing recovers it. Each report also downloads up to 500 full bodies — `hack/ci/ci-failure-report.sh:39`, `:114-118`
  Failure: this is not a bug in the change — spec CFR-4 and T1.1 F1 call for a hard stop at the limit. But the ceiling is shared with any other workflow whose token opens issues (tasks.md flags Renovate as possibly authoring as `github-actions`), and nothing in the repo records it.
  Fix: name the ceiling and what to do when it is reached (raise `LOOKUP_LIMIT`, or move to REST) in G3.md "Harness gaps" or `decisions.md`.
  Confidence: 70

Checked, and it holds:
- **Decision table:** every row of the spec CFR-1 table maps to code at `:93-111` and `:164-177`, and to at least one case. That covers 4 failure-class conclusions, open newer, equal/older (162.1, 163.1, 162.2), closed newer, the (163.1 vs 162.2) closed row, closed equal, success none/open/closed, equal+recovery close, the 12 no-op cases, the 162.1→162.2 re-run, continue-on-error, and both sequences.
- **Ordering:** the key comparison at `:153-162` is lexicographic on (run_number, run_attempt), and the arithmetic only runs after `need_int` checks at `:85-87` and `:147-148`.
- **Issue lookup:** an issue counts only with bot login + `is_bot` + label + a first-line marker for this workflow (`:125-128`). A comment counts only with login `github-actions` and a first-line marker, and the last such comment wins (`:143-146`).
- **Untrusted text:** the code never evaluates text from the run or the payload. Every gh value is a quoted argument, and both bodies go through `--body-file` (`:198`, `:201`, `:207`).
- **Failure handling:**
  - No write follows a failed call: `die` on every gh call except the log fetch.
  - A failed or empty log fetch still writes, then exits 1 (`:186-193`).
  - The close retry is idempotent (`:174-175`).
  - Nothing prints `GH_TOKEN`.
- **Fake gh:**
  - Its argv checks match what the reporter sends.
  - It matches the real gh shapes recorded in T1.1 (P7 bot-author spelling, F2 comment author without `is_bot`, `run-list-latest.json` from the exact full-path `--workflow` argv) and the G3 probes (`OPEN`/`CLOSED`, more than 100 comments).
  - It rejects `--search`, `--label` on the lookup, `--body`, `--repo`, `--reopen`, a run read whose id or attempt is not the listed run's, and an extra argument on close.
- **Test assertions:** `expect` checks the exact write list, the exit code, and that the fake rejected nothing. The non-numeric-id and gh-error cases also assert which call came last, so they cannot pass vacuously; the G3 mutation notes show this is what killed M15 and M19.
- **Scope:** the diff touches only the three files the task names, and G2's body cases are unchanged.

## Could not check
- I did not run the test, the 28 mutants, `task lint:shell` or the CWS tests. I relied on the exits recorded in G3.md. The mutant scripts under `/tmp/g3/` are outside the repository, so I did not read them.
- I could not check against the live GitHub API:
  - Whether a real `workflow_run.path` can ever carry a suffix such as `@refs/heads/main`. If it does, line 67 makes every report exit 1. That failure would be visible, not silent.
  - Whether `gh run view --attempt N --json jobs` returns that attempt's jobs.
  - Whether a non-bot account can hold the login `github-actions`. The comment check at `:143-144` depends on it, and spec CFR-4 accepts that dependency.
- The uncommitted `reviews/L-tasks/G3/` directory, which other reviewers' records may be in. I did not read it on purpose.
