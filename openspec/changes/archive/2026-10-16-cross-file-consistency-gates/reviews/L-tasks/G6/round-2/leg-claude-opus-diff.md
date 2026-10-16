## Verdict: CONCERNS

## Findings
- [WARNING] The new CFC-6 rule takes the modes it requires from the guard's own code, not from CFC-6. Removing `--self-test` from a guard therefore removes the requirement to run it. — `hack/test/ci_workflow_security_test.sh:602-606`
  Failure: someone deletes the `--self-test` branch from `consistency_metrics_test.sh` and deletes its CI step. `guard_modes` (`:454-459`) then returns only `bare`, so CWS-6 passes. The comment at `:596` says these guards "SHALL run … in every mode (plain and --self-test)", which is the CFC-6 requirement this task wires. The meta-test's own self-pin already avoids this trap by hard-coding both modes (`:560`).
  Fix: use `for mode in bare --self-test; do` for the CFC guards instead of `guard_modes`. Add a CWS-7 mutant that strips one guard's `--self-test` handling.
  Confidence: 70
- [NOTE] The new comment says "the job never ends first", but `job > step` only leaves 60 s for setup. The safety margin actually depends on curl's budget, and nothing checks that margin. — `hack/test/go_patch_lag_test.sh:191-204`
  Failure: with step 5, job 6, `--retry-max-time 250 --max-time 45` (295 s), every assertion passes. If checkout and runner setup take more than 65 s, the job is killed before the script prints its "cannot determine" message. Today the margin is 360 − 245 = 115 s, so this is not live.
  Fix: also assert `retry_max + max_time + 60 < job_timeout*60`, with a boundary mutant. Or soften the comment to "the step's timeout fires before the job's".
  Confidence: 55
- [NOTE] Round 1's WARNING still stands. Required `lint` now runs network-bound tool builds with no step or job timeout (the 360 min default) and is expected to take about 17–21 min. The only record is a decision request in evidence. — `.github/workflows/ci.yaml:569-579`, `evidence/G6.md:181-189`
  Failure: a stalled module proxy holds a required context on docs-only PRs for up to 6 h. Branch protection only sees this after merge.
  Fix: have the operator answer Option A or B before merge. The repo rule puts decision requests in the repo INBOX, not only in evidence.
  Confidence: 60

Checked and found sound:
- **ci.yaml wiring:** ten steps, each one guard in one mode. No `if:`, and only `name`/`run` keys, which fits GATE-CIWIRING-02's allowlist. The tool-pins `--self-test` runs last. The comment explaining why the modcache fixture cleanup is safe matches the sequential steps.
- **CFC-6 rule against the CWS-6 data:** `required_rows` is filled from required jobs on `pull_request` workflows only. Without nullglob, a missing `consistency_*` glob stays literal and fails the `-f` test, so a missing guard set reds. `fail` exits (`:51-54`). In the CWS-7 "unwired" mutant, the metrics steps are deleted, the generic loop cannot see the guard, and the CFC branch is what fires. The sentinel matches the message at `:604`.
- **go_patch_lag new assertion:**
  - Comment lines are stripped before parsing.
  - `[[:space:]]--max-time` cannot match inside `--retry-max-time`.
  - Numbers are bounded and read base-10 before any arithmetic.
  - The budget is 245 < 300 s and the job (6) is above the step (5).
- **go_patch_lag mutants:**
  - Each new probe is anchored correctly: the job probe uses 4 spaces and the step uses 8, so they cannot cross-match.
  - The "unbounded" mutant's marker is written by the substitution itself.
  - The anchor for the second-curl mutant (`# --- the listing`) exists in `hack/ci/go-patch-lag.sh`.
  - The "step timeout 3" mutant reaches the budget assertion, not the job > step one.
- **Round-1 fixes:**
  - The job timeout went 5 → 6, and `>=` became `>` with an equal-timeouts mutant.
  - The parse now requires exactly one curl call with one of each flag, plus a second-curl mutant.
  - The probe fix is in.
  - Nothing was loosened: no allowlist or baseline was widened, and no existing mutant was changed.

## Could not check
- I ran no tests or mutants (read-only leg). The G6.md sensor rows, including the CWS-7 self-test passing with the new mutant and the 5/5 gate mutants in `/tmp/g6/fix-gate-mutants.sh`, are taken as recorded. `/tmp/g6/*` is out of bounds.
- I couldn't check whether the decision request was filed in kollect's `agent-context/INBOX.md`, because that path is not in this worktree.
- I didn't read `dist_ci_wiring_test.sh`, `hack/check.sh`, or the four `consistency_*_test.sh` bodies. The `task check` glob coverage (row 6) and per-guard tool needs come from evidence.
- There has been no GitHub runner run (row 14), so real lint wall time and runner tool availability (perl, jq, zip, make) are unverified.
