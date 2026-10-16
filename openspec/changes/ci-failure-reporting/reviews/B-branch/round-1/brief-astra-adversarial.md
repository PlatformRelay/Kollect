# Stage B round 1: astra adversarial brief

Sent verbatim to `codex exec --sandbox read-only -`.

```text
You are an independent, adversarial reviewer of a whole git branch, with no context from whoever wrote it. Read-only: you may read files and run read-only commands (git, grep, sed). Writing, including to /tmp, is not available: where a check needs a write, reason from the code instead. You have no network.

Repository: current directory (kollect, branch feat/ci-failure-reporting).
Range: `git diff 81637dcde6c02bd8c30afbc3756e79219d5bd627...HEAD` (base = origin/main merge-base). Product files in scope:
- .github/workflows/ci-failure-report.yaml (new privileged `workflow_run` reporter workflow, issues: write)
- .github/zizmor.yml (dangerous-triggers suppression for that file)
- .github/workflows/ci.yaml (lint job wiring of the test)
- hack/ci/ci-failure-report.sh (decision logic, gh calls)
- hack/ci/ci-failure-issue-body.sh (issue/comment body builder over untrusted log text)
- hack/test/ci_failure_report_test.sh, hack/test/testdata/ci-failure-report/ (fake gh, recorded fixtures), hack/test/ci_workflow_security_test.sh (CWS rules)
Spec: openspec/changes/ci-failure-reporting/specs/ci-failure-reporting/spec.md (requirements CFR-1..CFR-7 and the decision table in CFR-1). proposal.md and tasks.md in the same directory give context. Evidence files under evidence/ are the implementer's claims: treat them as claims to test, not truth.

Sensor results already run by others (trust the exit codes, do not rerun multi-minute tiers): ci_failure_report_test.sh plain and --self-test green at HEAD; ci_workflow_security_test.sh plain and --self-test green; zizmor offline high: no findings.

Attack this as a privileged reporter. Answer each question explicitly, with path:line:
1. Can a fork, a pull request, a workflow_dispatch, or a foreign/unlisted workflow cause any write (issue create/comment/close/label)? Check the `if:` guards, `on.workflow_run.workflows`, head_repository comparison, and what the script re-reads itself vs trusts from the event payload.
2. Can untrusted text (job names, step names, log lines, branch names, issue titles/bodies/comments by other users) reach a shell evaluation, a GitHub `${{ }}` expression, a gh argv flag position, or the bot marker line? Check quoting, `--body-file`, numeric-id validation, `@` neutralisation, backtick fence length, and first-line-only marker parsing including author checks.
3. Can the reporter open duplicate issues, comment twice, close the wrong issue (another workflow's, a user's spoof), reopen, or loop (e.g. the reporter's own run triggering itself, or comment->event chains)? Check the ordering key comparison (run_number, run_attempt) lexicographic, numeric not string, the lookup completeness (--limit and the "count reaches limit -> fail" rule), client-side label check, and the concurrency group.
4. What happens on gh/API errors, partial writes (comment ok, close fails), empty outputs, malformed JSON, rate limits (HTTP 403/429)? Does every error exit non-zero with no further write, as CFR-6 requires, and is the log-fetch failure path ("logs unavailable", exit non-zero after write) right? Watch for `set -e` traps: errors inside `$(...)`, `local x=$(...)`, pipelines, `if`/`||` contexts that swallow failures.
5. Does each row of the CFR-1 decision table match the code exactly? Build the row-by-row mapping. Include no-op class conclusions, `cancelled` started-job definition (non-empty steps), empty scheduled-run list -> no write exit 0, equal-key-with-recovery-marker -> close only.
6. Is the CI wiring and its self-test sound? Would the test's wiring assertions fail if the lint steps, the label-create ordering, the permissions, the guards or a workflow-name mismatch regressed? Is any assertion vacuous (grep that matches unconditionally, a mutant whose anchor never applies, "0 tests" passing)? Spot-check a few mutants by reading the self-test code.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives. An APPROVE with no findings is valid when substantiated. Do not invent findings.

RETURN at most 40 lines, nothing else:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
answers:
  1. <one or two lines>  ... through 6.
decision_table: <rows that do not match the code, or "all rows match">
findings:
  - [P0|P1|P2] <one line, with path:line>
could_not_check:
  - <...>
summary: <one line>
Severity: P0 (a write reachable by an attacker, injection, invariant broken, vacuous gate presented as proof) -> BLOCK; P1 (correctness gap, untested behaviour, decision-table mismatch) -> REQUEST_CHANGES; P2 note only.
```
