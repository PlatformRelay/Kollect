# Tasks

Red first. G1's test task records its red in `evidence/G1.md`: each guard runs against the base tree and
fails on the named check, not on a missing tool or a shell error. Base here means no workflow, the script
a stub that exits 0 without running anything, and no list entry. Only the cases 1.1 marks *base red* are
expected red on that base. The shape mutants (V-2) and the script mutants (V-3) are proven after 1.2, by
the self-test against a valid copy, each failing with its named message. G1 closes in one commit with tests and
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
  `hack/test/testdata/vulncheck-scheduled/`. The advisory fixture combines the real 2026-11-12 CI output of
  `task vulncheck` (two module advisories, task exit 201), trimmed, with the stdlib GO-2026-6859 entry
  from a local run on go1.27.3, so the parser also meets `Standard library` text. Record the source run or
  command of each part in the evidence.
- Each group records:
  - `bash hack/test/vulncheck_schedule_test.sh` and `--self-test`;
  - `bash hack/test/ci_failure_report_test.sh` and `--self-test`;
  - `bash hack/test/ci_workflow_security_test.sh` and `--self-test`;
  - `bash hack/test/consistency_toolchain_pins_test.sh`;
  - the offline zizmor audit as `ci.yaml` runs it (the command is named in T0.1);
  - `shellcheck` on the new scripts;
  - `task lint:markdown` and `task spec:validate`.

## 0. Probe

- [x] 0.1 Verify on `origin/main` the claims `proposal.md` makes and record each with file:line or command
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
  - govulncheck's exit codes and report format at the pinned version (indentation of the title line), and
    the exit code `task vulncheck` (Task 3.54.0) returns when govulncheck fails: probe it locally;
  - the wall time of a cold `vulncheck` job from a recent CI run, to set `timeout-minutes`;
  - the zizmor command `ci.yaml` and `hack/check.sh` run;
  - every place that has to know a new guard exists: CWS-6's `cfc_guards` list and header comment,
    `ci_failure_report_test.sh`'s expected list, any docs list.

  A claim that does not hold stops the loop with a decision request. — closed 2026-11-16, evidence:
  evidence/T0.1.md (two narrative claims corrected in proposal.md by the orchestrator)

## 1. Daily vulnerability scan, reported (SVS-1, SVS-1.1, SVS-1.2, SVS-1.3, SVS-2, SVS-3)

- [x] 1.1 Tests first. Add the script as a stub that exits 0 without running anything. The cases marked
  *base red* fail on that base; the rest are proven by the self-test after 1.2:
  - V-1 (*base red*): given the tree, when `vulncheck_schedule_test.sh` runs, then it fails with
    "`.github/workflows/vulncheck-scheduled.yaml` is missing" · exposes a guard that passes when the
    workflow is absent.
  - V-2, the shape checks, one self-test mutant each, each mutant failing with a message that names its
    check:
    - `name:` other than `Vulnerability scan`;
    - a top-level key beyond `name`, `on`, `permissions` and `jobs` (`concurrency:`, `env:` or
      `defaults:`), or a job key beyond `runs-on`, `timeout-minutes`, `steps` and `name` (job-level
      `concurrency:`, `env:`, `if:`, `permissions:`);
    - a trigger removed or added, a second cron, `workflow_dispatch` inputs, or a cron made weekly
      (`0 5 * * 1`), monthly, hourly (`17 * * * *`) or every five minutes (`*/5 * * * *`);
    - permissions other than exactly `contents: read`, or job-level `permissions:`;
    - a second job; `continue-on-error`, `if:`, `shell:` or `defaults:` anywhere in the workflow;
    - `timeout-minutes` missing, `0` or not an integer;
    - a step count other than 4;
    - checkout not at a SHA, without `persist-credentials: false`, or given another input (`ref:`,
      `repository:`);
    - go-cache given inputs, or replaced by another local action;
    - setup-task not at a SHA, given inputs other than `version:`, or at a version no other workflow uses;
    - an action SHA that no other workflow uses;
    - the scan step's `run` other than exactly `bash hack/ci/vulncheck-scheduled.sh`, or given `env:`;
    - a `${{ secrets.X }}` or `${{ github.token }}` expression in a step. A comment that mentions
      secrets is a *control* and passes.

    Each exposes a guard that cannot fail on that property.
  - V-3, the script verdicts, with a fake `task`:
    - (a) (*base red*) the advisory fixture, fake exit 201: output contains the full report, then one
      `FAIL:` line each for GO-2026-6859, GO-2026-6888 and GO-2026-6865 with the title trimmed (e.g.
      `FAIL: GO-2026-6888 Excessive memory allocation when decompressing crafted streams in`), and the
      script exits 201;
    - (b) (*base red*) a fixture with no `Vulnerability #` entry, exit 201: exactly one
      `FAIL: task vulncheck exited 201; no advisory in its output (tool, network or build failure)`, exit
      201; and with exit 7, the same line naming 7 and exit 7 (status passed through, not hard-coded);
    - (c) (*control* on the stub) a clean fixture, exit 0: no `FAIL:` line, exit 0;
    - (d) (*base red*) the fake `task` records its arguments; the case fails unless they are exactly
      `vulncheck`.

    Each self-test mutant of a copy of the script must fail its case: the status not propagated (always
    `exit 0`, or always `exit 1`), the FAIL lines dropped, or the output swallowed. Exposes a script that
    hides the advisory or turns a failure green.
  - V-4 (*control*): the self-test's unmutated copies pass · exposes a self-test that fails everything.
  - V-5 (*base red*, `ci_failure_report_test.sh`): the expected list includes `Vulnerability scan`, and a new self-test
    case deletes `- Vulnerability scan` from the list and expects "on.workflow_run.workflows must be
    exactly" · exposes the list check not covering the new entry.
  - V-6 (*base red* once the guard is listed, CWS-6): add `vulncheck_schedule_test.sh` to `cfc_guards` in
    `hack/test/ci_workflow_security_test.sh` and to its header comment. Add a self-test mutant, modelled
    on the existing "guard unwired" ones, that removes either `lint` step and expects CWS-6 to fail ·
    exposes a guard nobody runs. — closed 2026-11-16, evidence: evidence/G1.md
- [x] 1.2 Implement:
  - the script: `set -uo pipefail`; capture `task vulncheck`'s combined output to a temp file while
    printing it; keep `task`'s status (`PIPESTATUS`, not the tee's); print the SVS-1.3 lines; exit with that
    status;
  - the workflow:
    - the four steps;
    - a daily cron at a minute and hour no other workflow uses;
    - `timeout-minutes` from T0.1's measurement, with a `# why:` comment;
    - a header comment naming SVS-1 to SVS-3 and the guard;
  - the reporter list entry;
  - the two `lint`-job steps.

  Every check in the test rules passes. — closed 2026-11-16, evidence: evidence/G1.md
- [x] 1.3 Docs:
  - `docs/security/sca-remediation-policy.md`:
    - VM-05.03 and the govulncheck tooling row: "Code-affecting push / PR, and daily on `main`", linking
      the new workflow;
    - the *Zero-tolerance gates* row for reachable vulnerabilities and the *Reachable Go CVEs* row of
      *Enforcement model*: these findings have no SLA clock. The scheduled scan's `ci-failure` issue makes
      the gate visible on the day the advisory lands, before it blocks the next merge. An issue whose only
      FAIL line is the "no advisory" line is a tool failure, not a finding;
  - `docs/development/testing.md`: a row in *Scheduled and manual tiers* for `vulncheck-scheduled.yaml`.

  `task lint:markdown` and `task docs:verify` pass. Compare `docs:verify` with a clean main: the
  untracked `ui/` gotcha. — closed 2026-11-16, evidence: evidence/G1.md

## 2. After merge (integrator)

- [x] 2.1 Dispatch `Vulnerability scan` once on `main` (`gh workflow run vulncheck-scheduled.yaml`) and
  record its conclusion in `evidence/post-merge.md` as a smoke run (a dispatched run is never reported,
  CFR-6). Classify a red run by its FAIL lines (advisory or tool failure) before reporting it; never
  silence it. Then record the first scheduled run's conclusion and the CI failure report run it triggered.
  This is the first proof of SVS-2 wiring in production. Then archive the change through a post-merge PR.
  — closed 2026-11-16 with a gap: smoke run and CFR-6 recorded in evidence/post-merge.md; the first
  scheduled run is observed by the next session (handoff)

## B. Branch review fixes

- [x] B1.1 Fix branch review round 1 (`reviews/B-branch/round-1/`), tests first, in one commit:
  - (CRITICAL) the scan step's name contains `FAIL`. `gh run view --log-failed` prefixes each line with
    `<job>\t<step>\t`, so every line of the step matches the excerpt filter, and 50 download lines push
    the advisory out of the issue. Rename the step without `FAIL`. V-2 gains a check, with a mutant, that
    no job or step `name:` contains `FAIL`. V-3 gains an integration case: the script's output for
    `advisories.txt`, preceded by more than 50 ordinary lines and prefixed as `gh` prints it
    (job name, a tab, the step name, a tab, a timestamp, then the line), is fed through `hack/ci/ci-failure-issue-body.sh` (its real
    arguments) and all three GO ids appear in the excerpt. A mutant restoring a `FAIL` step name must turn
    that case red. Read the step name from the workflow file, so the case breaks when the name changes;
  - capture failure: keep `tee`'s status too (SVS-1.3). Add V-3 cases with a failing `tee` on
    `PATH`, for task exit 0 (the script exits 1) and for task exit 201 (it exits 201). Both print the
    capture-failure line;
  - wrapped titles: join continuation lines (SVS-1.3). The GO-2026-6888 line then ends with
    `github.com/andybalholm/brotli` when the fixture wraps it there; update V-3a and V-3e expectations
    from the spec;
  - the token check also matches `github[` inside `${{ }}`, with a mutant;
  - timing wording: the policy rows, `docs/development/testing.md`, the workflow header and the proposal
    say "on the next scheduled scan, within a day, without waiting for a code change", never "on the day it
    lands" or "before it blocks the next merge";
  - `evidence/G1.md`: replace the home-relative Task path with "Task 3.54.0";
  - requirement ids: the script's rules are now SVS-1.3 (SVS-1.2 is the step shape); update every comment,
    message and evidence row that cites them.

  Gates: as in *Test rules*. — closed 2026-11-16, evidence: evidence/B1.1.md
