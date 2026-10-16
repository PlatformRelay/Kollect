## Verdict: CONCERNS

## Findings
- [WARNING] The job-vs-step timeout check (`job >= step`) lets through a setup where the job is killed before the script prints its message. That breaks the invariant the comment says it locks. — `hack/test/go_patch_lag_test.sh:200-201`
  Failure: the job and step timeouts are both 5 min and `--retry-max-time 290 --max-time 5` gives a 295 s budget. All three checks pass. Checkout and runner setup also count against the job's 300 s, so a stalled go.dev hits the job timeout while curl is still retrying, and the script's "cannot determine" message never prints. The real tree is safe today (245 s budget, ~55 s left for checkout), but the guard doesn't hold that margin, so a later edit could lose it.
  Fix: compare curl's budget plus a fixed overhead with the job timeout (`retry_max + max_time + 30 < job_timeout*60`), or require `job_timeout > step_timeout`. Add a mutant at the boundary.
  Confidence: 75
- [WARNING] The required `lint` job now depends on the network, has no timeout at all, and runs on docs-only PRs. — `.github/workflows/ci.yaml:569-579`
  Failure: the tool-pins plain step and its `--self-test` reach the module proxy, golangci-lint's custom build and the vuln DB. lint has no job-level `timeout-minutes` (`ci.yaml:277-280`), and step timeouts are blocked by GATE-CIWIRING-02. A proxy hiccup turns a required check red on a docs-only PR. A stalled fetch can hold it for up to 360 min. Expected lint wall time also roughly doubles to 17–21 min (G6.md, believed and not measured). G6.md does record this trade-off and proposes the fix, but nothing tracks it in this diff.
  Fix: get the operator's decision on G6.md's proposal before merge (a separate required `tool-pins` job, or a positive `timeout-minutes` allowed in the allowlist, with a mutant). Until then, put it in the INBOX and don't leave it as an evidence-only note.
  Confidence: 70
- [WARNING] Nothing reds if a consistency guard is unwired from CI completely, so CFC-6's "SHALL run in a required job" isn't held by any machine check. — `.github/workflows/ci.yaml:552-567`
  Failure: deleting both `consistency_metrics_test.sh` steps exits 0 under CWS-6 (G6.md row 3, mutant b). CWS-6 only checks the modes of guards CI already calls. `task check` still sweeps the guard, but CI no longer does.
  Fix: add a CWS-6 rule naming the five CFC guards, or covering every `*_test.sh` that has a `--self-test` mode, as required in a required job. At minimum, record it as an open gap and don't count CFC-6 as fully closed.
  Confidence: 85
- [NOTE] The "curl retry budget unbounded" mutant's probe checks for the marker comment, not for the removed flag. — `hack/test/go_patch_lag_test.sh:348-350`
  Failure: if the script's spacing changes, the first substitution misses and the marker still lands. The sentinel check then fails loudly, so there's no false pass. It's the same weak-probe pattern G6.md iteration 3 fixed for the step-timeout mutant.
  Fix: make the probe check that `--retry-max-time` is absent from the code lines (e.g. a negated grep in the mutation command).
  Confidence: 60
- [NOTE] The flag parse reads only the first `--max-time` / `--retry-max-time` in the script. — `hack/test/go_patch_lag_test.sh:204-205`
  Failure: a second curl call added later with a bigger budget isn't checked.
  Fix: parse every occurrence and fail on more than one curl call, or check the maximum.
  Confidence: 50
- [NOTE] The new gates sit before `Lint` with the default `success()`, so one red fast gate hides the `task lint` result for that run. This matches the existing `dist_*` steps, so it isn't a regression. — `ci.yaml:552-570`
  Confidence: 40

What I checked and found sound:
- All ten steps are plain commands with no `if:`, and lint's job keys are unchanged, so lint stays unconditional.
- Every guard has its tools in the job: yq, helm, PyYAML and go-cache/setup-go are all set up earlier in the job.
- The modcache fixture cleanup (`consistency_tool_pins_test.sh:92-106`) and the sequential steps make the G2 race safe in CI. The cache key is go.sum-based, so a post-job save happens only on a miss, and only after cleanup has run.
- The `check.sh:78-98` glob picks up all five guards in both modes, and none is excluded by its header.
- No guard was loosened: go_patch_lag only gained checks, and the existing `runs` assertion still pins the one plain step.

## Could not check
- I didn't run any tests, zizmor or CWS-6, so the G6.md sensor results are taken as recorded.
- The pipeline hasn't run on a GitHub runner (G6.md row 14), so runner tool availability (jq, make, zip, git) and real runtimes aren't verified.
- I didn't read `hack/test/dist_ci_wiring_test.sh`, so I don't know whether its job-key allowlist would also reject a job-level `timeout-minutes` on lint.
- I didn't read the internals of `ci_workflow_security_test.sh` (CWS-6), the full contents of the four consistency guards, or `hack/release/verify-eligibility.sh` `required_checks`.
- I didn't read the untracked `reviews/L-tasks/G6/` or the `/tmp/g6/*` mutant scripts the evidence cites.
