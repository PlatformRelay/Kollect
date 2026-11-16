The dedicated daily workflow is appropriate: it detects advisories without a dependency update and gives failures their own issue. Adding a schedule to `ci.yaml` would require broader changes to its filtering, concurrency and reporting behavior; Renovate post-upgrade checks would remain update-dependent.

The round-1 local-action contradiction is fixed. The reporter entry, CWS-6 registration, audit commands, post-merge observation and SLA clarification are now covered. Repository files confirm the CI trigger condition, reporter’s schedule-only guard, weekly go-patch-lag cron and absence of explicit Renovate vulnerability/indirect-update rules. Historical incident and alert claims still need T0.1’s external evidence.

1. **CRITICAL — The wrapper’s exit-status requirements contradict the real execution chain.**  
   **Evidence:** `openspec/changes/scheduled-vulncheck/specs/scheduled-vulnerability-scan/spec.md:47` requires preserving **Task’s** status, while line 53 requires **govulncheck’s** status. `tasks.md:82` models the advisory result as a fake Task exiting 3. But `Taskfile.yml:210` invokes govulncheck through `go run`; neither layer preserves the scanner’s status as specified. A read-only stdin probe using installed Task **3.54.0**, with a command exiting 3, returned **201**. [Task’s documented exit codes](https://taskfile.dev/docs/reference/cli) and [Go’s run documentation](https://go.dev/src/cmd/go/internal/run/run.go) confirm the distinction.  
   **Fix:** Consistently require preserving `task vulncheck`’s status. Change the diagnostic to “task vulncheck exited …”, record the actual command-chain status in T0.1, and use a realistic advisory fixture/status pair. Retain an arbitrary nonzero-status test to prove propagation. Otherwise, implementing task 1.2 literally fails SVS-1.2 despite passing the fake-Task tests.

2. **WARNING — SVS-1.1’s guard leaves ways to alter or suppress the scan.**  
   **Evidence:** `spec.md:29`–36 prohibits concurrency and softening, but `tasks.md:64` only rejects **top-level** extra keys; line 76 rejects `env:` only on the scan step. Job-level `concurrency:` and `env:` remain unchecked. Checkout at `spec.md:42` also permits additional inputs such as `ref:` or `repository:`. Those can cancel runs, change the vulnerability database through `GOVULNDB`, or scan a different revision.  
   **Fix:** Specify and test an allowlist of job keys, prohibit job-level environment/concurrency overrides, and restrict checkout inputs to `persist-credentials: false`. Add separate mutants for these cases.

3. **WARNING — The red-first instructions cannot hold for every listed case.**  
   **Evidence:** `tasks.md:56`–57 says every non-control case must be red against an absent workflow and a script stub exiting 0. The clean-output case at line 87 already passes that stub. Shape mutants cannot demonstrate their named failures against an absent workflow: they either cannot apply or encounter the missing-file check first.  
   **Fix:** Separate initial implementation reds from guard mutation proofs. Require V-1, failing-script cases and missing wiring to fail against the base; mark the clean stub case as initially passing. Run shape mutants against a valid fixture/control and require each named diagnostic.

4. **WARNING — The cron contract does not enforce once daily.**  
   **Evidence:** `spec.md:9` constrains only the last three fields. Thus `* * * * *` satisfies it, while the proposal promises one scan per day. `tasks.md:66`–67 tests weekly/monthly changes but not increased frequency.  
   **Fix:** Require exactly five fields with one integer minute, one integer hour and `* * *`; add hourly and every-minute mutants.

I found no private hostname or company reference in the three substantive spec files. The run-state file retains the already-triaged harness/model wording; I have not reopened that finding. This was a spec review, not an implementation gate run.

## Verdict

REQUEST_CHANGES
