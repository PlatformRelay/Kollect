## Verdict: CONCERNS

## Findings
- [WARNING] Case 12 passes even if the 4000-byte line cut is removed, so the cut it is named after is never checked — `hack/test/ci_failure_report_test.sh:327-332`
  Failure: delete the `if [[ "${#l}" -gt "${LINE_CAP}" ]]` block at `hack/ci/ci-failure-issue-body.sh:149-151`, or set `LINE_CAP=100000`. The 6000-byte line then goes through whole. It is still valid UTF-8 and still fits the budget, so case 12 stays green. Case 4 also stays green, because a few uncut 5060-byte lines still truncate under 60000 bytes. M17 only removes the `sed`, so no mutant in the table covers this. The case title says "cut at 4000 bytes, mid-character", but no assertion checks that a cut happened.
  Fix: in case 12 (or case 4), assert that the body contains `[line cut]` and that the cut line is at most 4000+11 bytes. Add a mutant that removes the cut block.
  Confidence: 90
- [NOTE] Job and workflow names outside the fence still render as Markdown links, images and issue references; only `@`, `&`, `<` and control characters are escaped — `hack/ci/ci-failure-issue-body.sh:81-82`
  Failure: a matrix-derived job name `x [y](https://evil) ![](https://t/p.png) org/repo#1` renders as a link and an image beacon (proxied by camo), and leaves a cross-reference event on `org/repo#1`. Job names come from workflow YAML on the default branch, so the input is mostly trusted. CFR-5 only requires that `@` be neutralised, so this is outside the spec.
  Fix: optional. Put each name in a code span whose backtick run is longer than any run in the name, or backslash-escape ``[]()!#`*_|``.
  Confidence: 55
- [NOTE] A job that fails under `continue-on-error` (`race`) is listed as failed when another job fails the run — `hack/ci/ci-failure-issue-body.sh:89-91`
  Failure: T1.1 (line 177) records that such a job carries `conclusion: failure`. The body would then list it next to the job that actually failed the run. That is arguably useful, but it is an undocumented choice, and spec.md:21 says such a job "is not reported".
  Fix: write the decision down in the G2 Interface section. If it is not wanted, take `race` out of the list.
  Confidence: 60
- [NOTE] When the log has more than 50 FAIL lines, the excerpt stops at 50 without saying so; the note appears only for byte truncation — `hack/ci/ci-failure-issue-body.sh:132,176`
  Failure: a run with 300 FAIL lines shows 50 and does not say there were more. This is consistent with "at most 50" in the spec.
  Fix: optional "first 50 FAIL lines shown" note when `grep -c` is over 50.
  Confidence: 70

Checked and holding:
- **Fence length:** the run length is measured after the cut, across all 50 lines, at least 3 (`:155-160`). A line containing `FAIL` cannot be a closing line. An embedded bare CR, which CommonMark treats as a line break, cannot close the fence either, because the backtick run after it is shorter than the fence.
- **`@` outside fences:** escaped via `@&#8203;` in every name field. Log text only appears inside the fence.
- **Marker:** it is built only from regex-validated arguments (`:70-76`) and is always line 1. Marker lookalikes in names come out as `&lt;!--`.
- **50-line bound:** `grep -m 50` gives the first 50 lines in log order, and case 3 compares their numbers with `seq 1 50`.
- **60000 cap:** `budget` works out to at most CAP-1 bytes in both branches. The fence is closed before the note (`:175-178`). The precheck (`:121`) guarantees that the first line, which is at most 4012 bytes, always fits. Lengths are byte counts under `LC_ALL=C`.

Round-1 fixes verified in code:
- An info string containing a backtick no longer opens a fence (`outside`/`fences` regex `[^`]*$`), and case 0 is the control.
- An unreadable log takes the `grep_rc > 1` path to "logs unavailable" (`:131-136`). The case 9 variant is a directory, which also gives exit 2 when CI runs as root.
- Case 8b covers `<`, `&` and newline escaping in job names.
- Case 13 checks that an oversized job list is refused, with stderr checked.
- The truncation note now says "bytes".

Vacuity:
- `FAILURES` drives the exit code, and a missing builder or fixture exits 1.
- Case 4 requires both a fence close and the truncation note after it, outside the fence.
- Case 7 requires a fence longer than 5 backticks, a close after the line, and exactly one marker outside fences.
- Apart from the cut in the first finding, I found no case that passes when its target behaviour is missing.

## Could not check
- I did not run the test, ShellCheck or the mutant runner. The 19/19 kill count and the sensor exits come only from `evidence/G2.md`, and `/tmp/g2cfr/mutants.sh` is outside the repository.
- I did not read the round-1 legs under `reviews/L-tasks/G2/`; I relied on the G2.md summary of them.
- I did not check GitHub's actual rendering: entity decoding of `&#8203;` before mention parsing, or whether a bare CR splits a line. Both are reasoned from CommonMark and GitHub's mention filter.
- I did not check how task 3.3 will call the builder, or whether the CFR-4 lookup really reads only line 1. That code does not exist yet.
