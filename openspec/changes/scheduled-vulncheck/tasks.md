# Tasks

Red first. G1's test task records its red in `evidence/G1.md`: the guard runs against the base tree (no
workflow yet) and fails on the named check, not on a missing tool or a shell error. G1 closes in one
commit with tests and implementation, so no commit carries a red guard.

Each case reads: given / when / then · the failure it exposes.

Test rules:

- Guards follow the existing pattern of `hack/test/go_patch_lag_test.sh` and
  `hack/test/mutation_signal_test.sh`:
  - `set -euo pipefail`; `jq` and `yq` (mikefarah v4) are required;
  - an optional tree-root argument, and `--self-test`, which mutates copies in a temp dir outside the
    repository and judges each mutant by exit status and its message;
  - one unmutated control that must pass.
- Each group records:
  - `bash hack/test/vulncheck_schedule_test.sh` and `--self-test`;
  - `bash hack/test/ci_failure_report_test.sh` and `--self-test`;
  - `bash hack/test/ci_workflow_security_test.sh` and `--self-test` (CWS: pinned actions, zizmor and the
    required-job wiring of guard steps);
  - `task lint:markdown` and `task spec:validate`.

## 0. Probe

- [ ] 0.1 Verify on `origin/main` the claims `proposal.md` makes and record each with file:line or command
  output in `evidence/T0.1.md`:
  - the `vulncheck` job's trigger condition and that it is the only place govulncheck runs in CI;
  - the `task vulncheck` command and its pinned version;
  - the reporter's list and its schedule-only guard;
  - the go-patch-lag cron and its last runs;
  - the Dependabot alert list (no brotli, compress or Go stdlib alert);
  - the Renovate config, which has no `vulnerabilityAlerts`, `osvVulnerabilityAlerts` or indirect gomod
    rule.

  Also:
  - the wall time of a cold `vulncheck` job from a recent CI run, to set `timeout-minutes`;
  - every place that has to know a new guard exists: CWS required-job wiring, `ci_failure_report_test.sh`'s
    expected list and its "must be exactly" check, and any registry of guards or workflows, such as
    `hack/test/ci_workflow_security_test.sh`, zizmor config or a docs list.

  A claim that does not hold stops the loop with a decision request.

## 1. Daily vulnerability scan, reported (SVS-1, SVS-2, SVS-3)

- [ ] 1.1 Tests first (red on base):
  - V-1: given the tree, when `vulncheck_schedule_test.sh` runs, then it fails with
    "`.github/workflows/vulncheck-scheduled.yaml` is missing" · exposes a guard that passes when the
    workflow is absent.
  - V-2, the shape checks, one self-test mutant each, each mutant failing with a message that names its
    check:
    - `name:` other than `Vulnerability scan`;
    - a trigger beyond `schedule` and `workflow_dispatch`, a second cron, or `workflow_dispatch` inputs;
    - permissions other than exactly `contents: read`, or job-level `permissions:`;
    - a second job; `continue-on-error` or `if:` on the workflow's job or any step;
    - no job `timeout-minutes`;
    - no step whose `run` is exactly `task vulncheck`, or a second `run` step;
    - an unpinned `uses:` (a ref that is not a 40-hex sha);
    - `secrets.` anywhere in the file;
    - checkout without `persist-credentials: false`.

    Exposes a guard that cannot fail on that property.
  - V-3: a self-test control: the unmutated copy passes · exposes a self-test that fails everything.
  - V-4 (`ci_failure_report_test.sh`): the expected list includes `Vulnerability scan`, and a new self-test
    case deletes `- Vulnerability scan` from the list and expects "on.workflow_run.workflows must be
    exactly" · exposes the list check not covering the new entry. Red on base because the list lacks the
    entry.
  - V-5 (CWS): the plain and `--self-test` steps of `vulncheck_schedule_test.sh` run in the `lint` job,
    a required job. If CWS keeps a registry of such guards, it fails when either step is missing ·
    exposes a guard nobody runs.
- [ ] 1.2 Implement:
  - the workflow:
    - steps: checkout (pinned, `persist-credentials: false`), `./.github/actions/go-cache`,
      `go-task/setup-task` at the sha and version the other workflows use, and `task vulncheck`;
    - a daily cron at a minute no other workflow uses;
    - `timeout-minutes` from T0.1's measurement, with a `# why:` comment;
    - a header comment naming SVS-1 to SVS-3 and the guard.
  - the reporter list entry;
  - the two `lint`-job steps;
  - any registry entries T0.1 found.

  Every guard above passes in both modes.
- [ ] 1.3 Docs:
  - `docs/security/sca-remediation-policy.md`: VM-05.03 and the govulncheck tooling row ("Code-affecting
    push / PR, and daily on `main`", linking the new workflow); one sentence in the SLA clock paragraph
    that a scheduled scan's `ci-failure` issue starts the clock;
  - `docs/development/testing.md`: a row in *Scheduled and manual tiers* for `vulncheck-scheduled.yaml`.

  `task lint:markdown` and `task docs:verify` pass. Compare `docs:verify` with a clean main: the
  untracked `ui/` gotcha.

## 2. After merge (integrator)

- [ ] 2.1 Dispatch `Vulnerability scan` once on `main` (`gh workflow run vulncheck-scheduled.yaml`) and
  record the run's conclusion in `evidence/post-merge.md`. A red run is a real advisory: report it, do not
  silence it. Then archive the change through a post-merge PR.
