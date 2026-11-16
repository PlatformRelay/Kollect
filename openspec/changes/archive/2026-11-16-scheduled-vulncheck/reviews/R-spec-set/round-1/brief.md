# Cold review: OpenSpec change `scheduled-vulncheck` (lens: spec set)

You are an independent reviewer with read-only access to this repository (Go Kubernetes operator
"kollect"). Review the spec set in `openspec/changes/scheduled-vulncheck/`: `proposal.md`,
`specs/scheduled-vulnerability-scan/spec.md`, `tasks.md` (`loop.md` is run state). The change adds a
daily scheduled govulncheck workflow, wires it into `.github/workflows/ci-failure-report.yaml`, and
adds a guard test. Read the existing code it builds on: `.github/workflows/ci.yaml` (job `vulncheck`,
the `lint` job's guard steps), `.github/workflows/ci-failure-report.yaml`, `.github/workflows/mutation.yaml`,
`.github/workflows/go-patch-lag.yaml`, `hack/test/go_patch_lag_test.sh`, `hack/test/mutation_signal_test.sh`,
`hack/test/ci_failure_report_test.sh`, `hack/test/ci_workflow_security_test.sh`, `Taskfile.yml`
(`vulncheck`), `renovate.json`, `openspec/specs/ci-failure-reporting/spec.md`, `docs/security/sca-remediation-policy.md`.

Question: would executing these tasks as written deliver the spec, and is the spec the right fix for the
stated problem?

1. Factual premises in `proposal.md` *Why* that the repository can confirm or refute (trigger condition,
   reporter guard, go-patch-lag cron, Renovate config). Anything stated as fact that is wrong.
2. Is a daily report-only scan the right scope? Would something cheaper or stronger fit better (e.g.
   adding `schedule:` to ci.yaml, running in an existing nightly workflow, govulncheck in Renovate
   post-upgrade)? Is "Out of scope" justified?
3. Requirements SVS-1..3: testable, unambiguous, consistent with CFR-1..7 (especially CFR-6 schedule-only
   and CFR-1.8 conclusion-only), any gap (concurrency, timeout, cache poisoning via actions/cache on a
   scheduled run, zizmor findings, the database fetch failing closed).
4. Tasks: missing, misordered or unverifiable; each test case says what failure it exposes; red-first
   is real (not a compile/tool error); every command exists and runs locally; any registry/guard that
   must learn about a new workflow or guard step and is not named (CWS-6 required-job mapping, zizmor
   config, `tool-pins` guards counting setup-task sites, docs lists).
5. Anything private (internal hostnames, company names, agent/AI references) in the spec set.

Output: findings, each with severity CRITICAL / WARNING / NOTE, file:line evidence, and a concrete fix;
CRITICAL only when executing the tasks would fail to deliver the spec or ship wrong behaviour. End with
`## Verdict` and one of APPROVE / CONCERNS / REQUEST_CHANGES / BLOCK.
