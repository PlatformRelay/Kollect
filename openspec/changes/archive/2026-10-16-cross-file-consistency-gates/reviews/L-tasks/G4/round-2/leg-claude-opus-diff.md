## Verdict: CLEAN

## Findings
- [NOTE] Nothing ties the curl retry budget to the job's `timeout-minutes`. That mismatch was the round-1 defect, and it is only held off by a comment today. — `hack/ci/go-patch-lag.sh:81-82`, `.github/workflows/go-patch-lag.yaml:23`
  Failure: someone later raises `--max-time` or `--retry-max-time`, or lowers `timeout-minutes`. A stalled go.dev then kills the job without the script's "cannot determine" message, and no test turns red.
  Fix: in the workflow section of `go_patch_lag_test.sh`, assert that `timeout-minutes*60` is greater than `--retry-max-time` + `--max-time`, both read from the script.
  Confidence: 70
- [NOTE] The Maintainability section of the evidence file is out of date: it says "15 cases, 14 mutants". The test now has 17 cases and 16 mutants, which matches the evidence file's own Sensors table. — `openspec/changes/cross-file-consistency-gates/evidence/G4.md:81`
  Failure: the record contradicts itself (lines 56–57 against 81). The code is not affected.
  Fix: change the line to "17 cases, 16 mutants".
  Confidence: 95
- [NOTE] The newer-minor note's regex accepts leading zeros, and the patch filter does not. — `hack/ci/go-patch-lag.sh:99`
  Failure: a listing entry `go1.27.02` becomes `go1.27.2` in the note. The cosmetic case is harmless: the note never affects the exit code.
  Fix: use the same `(0|[1-9][0-9]*)` pattern in the `capture`, or leave it as is.
  Confidence: 60

What I checked, and found correct:
- **Round-1 fix 1 (leading zeros):** the directive regex (`:67-68`) and the jq filter (`:91`) now accept only canonical numbers. Both comparisons use `10#` (`:103,111`). I traced `go1.27.09` in the listing: it is filtered out, the minor is absent, and the script exits 2. I traced `go 1.27.02` in go.mod: the regex rejects it and the script exits 2. Each mutant fails for the intended reason: the jq mutant gives behind (exit 1, want 2), and the `num` mutant gives `02` == `2` (exit 0, want 2).
- **Round-1 fix 2 (retry budget):** the worst case is 4 × 45 s plus 7 s of backoff, about 187 s. A `Retry-After` header can push it toward the `--retry-max-time 200` ceiling, which curl enforces, plus one final attempt. That stays under 300 s. curl errors are no longer swallowed (`-sS`).
- **CFC-5, all four scenarios:**
  - Behind: exit 1 with a message (`:107-113`).
  - Current: exit 0.
  - Lookup failure: exit 2 when the listing is unreachable, is HTML, is truncated, is an object, is empty, lacks the minor, or is older than go.mod (`:56,83,92-95,103`).
  - Newer minor: printed as a note only (`:115-117`).
  - `include=all` is used, so a go.mod two minors behind is still found. Only `stable == true` releases count.
- **Fail-closed paths:** an empty file makes jq exit 0 with no output, which lands in "no stable release" (exit 2). CRLF in go.mod is handled by `[[:space:]]`. The minor prefix `go1.2.` cannot match `go1.26.x`.
- **CFC-6 self-test:** there is a no-op control plus 16 mutants. Each mutant has a check that it actually landed and an assertion on the specific failure message. The mutant regexes still match the post-fix source (`10#` in the `behind` mutant, `:234-235`). Wiring the test into a required `pull_request` job is task 6.1 and out of scope here.
- **Workflow:** `contents: read` is set at workflow level only. Checkout is pinned to a SHA with `persist-credentials: false`. There are no secrets, no `if:`, and no `continue-on-error`. It is weekly cron plus `workflow_dispatch`, and the test pins all of this.
- **Real tree:** `go.mod:3` is `go 1.27.2`, which matches the evidence file's green live run.
- **Reporting:** a red scheduled run has no reporting path in this diff, which is correct, because change 7 owns that.

## Could not check
- I did not run anything: the test, the self-test, zizmor, ShellCheck, or the live fetch. The pass results I relied on are the ones recorded in `evidence/G4.md`; I did not reproduce them.
- I did not compare the fixture `listing-go1.26-go1.27.json` against the live go.dev output.
- I did not read the untracked `reviews/L-tasks/G4/round-2/` directory or `round-1/legs.log`.
- curl's enforcement of `Retry-After` against `--retry-max-time` comes from my memory of curl's source. I did not confirm it against the curl version on the runner.
- GitHub turns off scheduled workflows in a public repo after 60 days without activity, and the sensor would then just stop running. I did not check whether change 7's reporter covers that.
