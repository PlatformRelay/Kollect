# Tasks

Red first. G1's test task records its red in `evidence/G1.md`: each guard runs against the base tree and
fails on the named check, not on a missing tool or a shell error. Base here means no workflow, the script
a stub that exits 0 without running anything, and no list entry. G1 closes in one commit with tests and
implementation, so no commit carries a red guard.

Each case reads: given / when / then · the failure it exposes.

Test rules:

- Guards follow the existing pattern of `hack/test/go_patch_lag_test.sh` and
  `hack/test/mutation_signal_test.sh` (its W1 step-shape checks are the model for V-2):
  - `set -euo pipefail`; `jq` and `yq` (mikefarah v4) are required;
  - an optional tree-root argument, and `--self-test`, which mutates copies in a temp dir outside the
    repository and judges each mutant by exit status and its message;
  - one unmutated control that must pass.
- Script cases run `hack/ci/vulncheck-scheduled.sh` with a fake `task` first on `PATH`. The fake prints a
  fixture and exits with a given status. The fixtures live under
  `hack/test/testdata/vulncheck-scheduled/`, and one is taken from the real 2026-11-12 govulncheck output
  (two advisories, exit 3), trimmed. Record the command that produced it in the evidence.
- Each group records:
  - `bash hack/test/vulncheck_schedule_test.sh` and `--self-test`;
  - `bash hack/test/ci_failure_report_test.sh` and `--self-test`;
  - `bash hack/test/ci_workflow_security_test.sh` and `--self-test`;
  - `bash hack/test/consistency_toolchain_pins_test.sh`;
  - the offline zizmor audit as `ci.yaml` runs it (the command is named in T0.1);
  - `shellcheck` on the new scripts;
  - `task lint:markdown` and `task spec:validate`.

## 0. Probe

- [ ] 0.1 Verify on `origin/main` the claims `proposal.md` makes and record each with file:line or command
  output in `evidence/T0.1.md`:
  - the `vulncheck` job's trigger condition and that it is the only place govulncheck runs in CI;
  - the `task vulncheck` command and its pinned version;
  - the reporter's list, its schedule-only guard and its `FAIL` excerpt rule
    (`hack/ci/ci-failure-issue-body.sh`);
  - the go-patch-lag cron and its last runs;
  - the 2026-11 incident: which open PRs had `vulncheck` red, and the run ids;
  - the Dependabot alert list (no brotli, compress or Go stdlib alert);
  - the Renovate config: no `vulnerabilityAlerts`, `osvVulnerabilityAlerts` or indirect gomod rule.

  Also:
  - govulncheck's exit codes (0 clean, 3 vulnerabilities found, other values for errors) and its report
    format, from its docs or source at the pinned version;
  - the wall time of a cold `vulncheck` job from a recent CI run, to set `timeout-minutes`;
  - the zizmor command `ci.yaml` and `hack/check.sh` run;
  - every place that has to know a new guard exists: CWS-6's `cfc_guards` list and header comment,
    `ci_failure_report_test.sh`'s expected list, any docs list.

  A claim that does not hold stops the loop with a decision request.

## 1. Daily vulnerability scan, reported (SVS-1, SVS-1.1, SVS-1.2, SVS-2, SVS-3)

- [ ] 1.1 Tests first. Add the script as a stub that exits 0 without running anything. Every case below is
  red on that base unless marked *control*:
  - V-1: given the tree, when `vulncheck_schedule_test.sh` runs, then it fails with
    "`.github/workflows/vulncheck-scheduled.yaml` is missing" · exposes a guard that passes when the
    workflow is absent.
  - V-2, the shape checks, one self-test mutant each, each mutant failing with a message that names its
    check:
    - `name:` other than `Vulnerability scan`;
    - a top-level key beyond `name`, `on`, `permissions` and `jobs` (`concurrency:`, `env:` or
      `defaults:`);
    - a trigger removed or added, a second cron, `workflow_dispatch` inputs, or a cron made weekly
      (`0 5 * * 1`) or monthly;
    - permissions other than exactly `contents: read`, or job-level `permissions:`;
    - a second job; `continue-on-error`, `if:`, `shell:` or `defaults:` anywhere in the workflow;
    - `timeout-minutes` missing, `0` or not an integer;
    - a step count other than 4;
    - checkout not at a SHA or without `persist-credentials: false`;
    - go-cache given inputs, or replaced by another local action;
    - setup-task not at a SHA, given inputs other than `version:`, or at a version no other workflow uses;
    - an action SHA that no other workflow uses;
    - the scan step's `run` other than exactly `bash hack/ci/vulncheck-scheduled.sh`, or given `env:`;
    - a `${{ secrets.X }}` or `${{ github.token }}` expression in a step. A comment that mentions
      secrets is a *control* and passes.

    Each exposes a guard that cannot fail on that property.
  - V-3, the script verdicts, with a fake `task`:
    - (a) the real fixture, exit 3: output contains the full report, then exactly
      `FAIL: GO-2026-6888 Excessive memory allocation when decompressing crafted streams in` and the
      GO-2026-6865 line, and the script exits 3;
    - (b) a fixture with no `Vulnerability #` entry, exit 1: exactly one
      `FAIL: govulncheck exited 1; no advisory in its output (tool, network or build failure)`, exit 1;
    - (c) a clean fixture, exit 0: no `FAIL:` line, exit 0;
    - (d) a fake `task` that runs without a `vulncheck` argument fails the case.

    Each self-test mutant of a copy of the script must fail its case: the status not propagated (always
    `exit 0`, or always `exit 1`), the FAIL lines dropped, or the output swallowed. Exposes a script that
    hides the advisory or turns a failure green.
  - V-4 (*control*): the self-test's unmutated copies pass · exposes a self-test that fails everything.
  - V-5 (`ci_failure_report_test.sh`): the expected list includes `Vulnerability scan`, and a new self-test
    case deletes `- Vulnerability scan` from the list and expects "on.workflow_run.workflows must be
    exactly" · exposes the list check not covering the new entry.
  - V-6 (CWS-6): add `vulncheck_schedule_test.sh` to `cfc_guards` in
    `hack/test/ci_workflow_security_test.sh` and to its header comment. Add a self-test mutant, modelled
    on the existing "guard unwired" ones, that removes either `lint` step and expects CWS-6 to fail ·
    exposes a guard nobody runs.
- [ ] 1.2 Implement:
  - the script: `set -uo pipefail`; capture `task vulncheck`'s combined output to a temp file while
    printing it; keep its status; print the SVS-1.2 lines; exit with the status;
  - the workflow:
    - the four steps;
    - a daily cron at a minute and hour no other workflow uses;
    - `timeout-minutes` from T0.1's measurement, with a `# why:` comment;
    - a header comment naming SVS-1 to SVS-3 and the guard;
  - the reporter list entry;
  - the two `lint`-job steps.

  Every check in the test rules passes.
- [ ] 1.3 Docs:
  - `docs/security/sca-remediation-policy.md`:
    - VM-05.03 and the govulncheck tooling row: "Code-affecting push / PR, and daily on `main`", linking
      the new workflow;
    - one sentence in the SLA clock paragraph: a vulnerability first named by the scheduled scan's
      `ci-failure` issue starts its clock there, unless it was reported earlier. An issue whose only
      FAIL line is the "no advisory" line is a tool failure and starts no clock;
  - `docs/development/testing.md`: a row in *Scheduled and manual tiers* for `vulncheck-scheduled.yaml`.

  `task lint:markdown` and `task docs:verify` pass. Compare `docs:verify` with a clean main: the
  untracked `ui/` gotcha.

## 2. After merge (integrator)

- [ ] 2.1 Dispatch `Vulnerability scan` once on `main` (`gh workflow run vulncheck-scheduled.yaml`) and
  record its conclusion in `evidence/post-merge.md` as a smoke run (a dispatched run is never reported,
  CFR-6). Classify a red run by its FAIL lines (advisory or tool failure) before reporting it; never
  silence it. Then record the first scheduled run's conclusion and the CI failure report run it triggered.
  This is the first proof of SVS-2 wiring in production. Then archive the change through a post-merge PR.
