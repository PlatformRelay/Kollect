**One blocking defect: advisory summaries can disappear from the CI failure issue because the scan step’s name contains `FAIL`.**

### Findings

1. **CRITICAL — The step name defeats the reporter’s log filter.**  
   Evidence: `.github/workflows/vulncheck-scheduled.yaml:38`; `hack/ci/ci-failure-issue-body.sh:132`.

   The step is named `govulncheck, one FAIL line per advisory`. GitHub CLI prefixes each log line with its job and step names ([CLI implementation](https://github.com/cli/cli/blob/trunk/pkg/cmd/run/view/view.go#L522)). Consequently, **every line from this step matches the reporter’s `grep -F FAIL`**, including downloads and ordinary scanner output.

   The builder takes only the first 50 matches. With 60 download lines followed by an advisory, my reproduction against the actual builder returned success, included 50 download lines, and omitted the GO id. Changing only the step name to `Run vulnerability scan` preserved the advisory.

   This violates SVS-2’s first-failing-day scenario. The current tests check raw wrapper output and workflow wiring separately, so neither catches the integration defect.

   **Fix:** remove `FAIL` from the step name. Add a regression test passing realistically prefixed logs through the body builder, with more than 50 ordinary lines before the advisory summary. Mutate the step name back to prove that test fails.

2. **WARNING — Capture failure is ignored and can misclassify an advisory.**  
   Evidence: `hack/ci/vulncheck-scheduled.sh:25–26`, `:52–55`.

   Only Task’s pipeline status is retained. A failing `tee` can leave an empty or incomplete capture:
   - Task exits 0: wrapper exits 0 despite capture failure.
   - Task exits 201 with an advisory: wrapper remains red but can print “no advisory … tool, network or build failure.”

   Both outcomes reproduced with an injected failing `tee`. This does **not** turn a nonzero Task status green, but it undermines the promised output and the policy’s classification guidance.

   **Fix:** capture both pipeline statuses immediately. Preserve nonzero Task status; explicitly report capture failure. Amend SVS-1.2 to permit a nonzero infrastructure status when Task succeeds but capture fails, then test both cases.

3. **WARNING — The policy promises detection earlier than the schedule guarantees.**  
   Evidence: `docs/security/sca-remediation-policy.md:49`, `:112`; `.github/workflows/vulncheck-scheduled.yaml:14`.

   A daily scan at 04:41 UTC cannot guarantee an issue “on the day the advisory lands” or “before it blocks the next merge.” An advisory published after that scan can first affect a PR, with scheduled detection the following day.

   **Fix:** use “on the next scheduled scan, without waiting for a code change.” Keep the zero-tolerance/no-SLA rule unchanged. Apply the same correction to the proposal, tasks and workflow commentary.

4. **NOTE — Parser and excerpt limits need an explicit boundary in the contract.**  
   Evidence: `hack/ci/vulncheck-scheduled.sh:39`; `hack/ci/ci-failure-issue-body.sh:37–39`; spec `:73`.

   CRLF works; wrapped titles intentionally contribute only their first line. ANSI-decorated headers and headers with trailing text produce the generic no-advisory summary. The pinned scanner’s default output supports the current parser, so these are format-drift risks rather than demonstrated current scanner defects.

   Independently, the existing reporter cannot carry every summary indefinitely: it caps matches at 50, lines at 4,000 bytes and the body below 60,000 bytes.

   **Fix:** document the supported header format and qualify SVS-2 by the existing excerpt limits. Add format-drift fixtures if broader tolerance is intended.

### Requirement assessment

| Requirement | Assessment | Evidence |
|---|---|---|
| SVS-1 | **Holds** for configured execution | One daily cron, inputless dispatch, timeout; nonzero Task status retained. |
| SVS-1.1 | **Holds** | Read-only contents permission; restricted job shape; no workflow/job softening or concurrency. |
| SVS-1.2 | **Partial** | Normal advisory, clean and non-advisory paths conform; capture failure is unhandled. |
| SVS-2 | **Partial; excerpt requirement contradicted** | Name/list and schedule-only wiring are correct; finding 1 loses advisory content. |
| SVS-3 | **Holds by inspection for enumerated scenarios** | Checks and named mutations cover the specified shapes; both modes are wired into `lint` and CWS-6. Runtime mutation coverage was not independently rerun. |

Additional behavior beyond the main requirements includes id-only summaries for missing titles, rejection of `shell`/`defaults`, reuse checks for action/version pins, and parser edge-case fixtures. The latter strengthen the guard; id-only summaries should be documented explicitly.

### Other checks

- **Failure handling:** immediate `PIPESTATUS[0]` capture is correct. `mktemp` fails closed; initialized variables and the cleanup trap expose no demonstrated success-on-Task-failure path.
- **Module-only findings:** the pinned scanner’s symbol-mode exit logic allows findings without reachable calls to remain green, consistent with SVS-1.
- **Guard integrity:** inspected query failures do not establish a green result; mutants require both failure and a matching diagnostic, with unmodified and positive controls. CWS-6 checks both modes in a required PR job; CFR checks exact list membership and unique workflow-name matching.
- **Security:** checkout disables persisted credentials; external actions are SHA-pinned. The shared Go cache is used without publishing credentials. No new zizmor suppression was added. CFR-6 excludes manual runs; `timed_out` is classified as failure.
- **Docs/privacy:** no private identifiers, home paths or authoring-agent references found in the two changed documentation pages.
- **Commits:** all eight titles follow the repository’s gitmoji/conventional form; no prohibited attribution trailers found. Tests and implementation land together. Per-commit green status is **not verified**. Whole-branch `git diff --check` reports trailing whitespace in historical review Markdown.

### Verification limits

Bash syntax and ShellCheck passed. Read-only, memory-backed probes exercised the wrapper and actual issue-body builder. Full guards/self-tests require temporary filesystem writes and were not rerun under this read-only sandbox; neither were full CI, zizmor, live scheduled reporting or every intermediate commit’s gates.

## Verdict

**BLOCK**
