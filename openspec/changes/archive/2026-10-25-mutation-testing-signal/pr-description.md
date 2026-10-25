# PR description — mutation-testing-signal

**Create at:** <https://github.com/konih/kollect/compare/main...feat/mutation-testing-signal?expand=1>

- **Source:** `feat/mutation-testing-signal`
- **Target:** `main`
- **Title:** `:white_check_mark: test(ci): nightly report-only mutation testing on three pure packages`
- **Opens as:** **ready** — every gate below ran locally on the branch; CI decides the merge.

Paste everything below the line.

---

## TL;DR

- **What:** a scheduled `Mutation testing` workflow (01:30 UTC) runs gremlins v0.6.0 on
  `internal/aggregate`, `internal/redact` and `internal/pathvalidate` and writes per-package
  killed / lived / not covered / timed out / not viable counts to the job summary.
- **Why:** coverage says which lines run, not which tests catch a change; this gives that signal
  (issue #446, OpenSpec change `mutation-testing-signal`).
- **Risk:** the required `lint` job gains a guard and its self-test, about +4–5 min (to ~18 min).
- **Reviewer action:** confirm report-only cannot be bypassed, and accept the three
  model-aligned decisions below, which still wait for operator validation.

Closes #446.

A low mutation score never fails the nightly. A broken tool, a missing or invalid report, a broken
fixture control, or a package that measured nothing always does, and `ci-failure-report.yaml`
reports a failed scheduled run.

## Most important things

- **Report-only is enforced, not promised.** `hack/ci/mutation.sh` passes `--config` pointing at
  the committed empty `hack/ci/gremlins.yaml` (`{}`), so gremlins searches no other config file;
  passes all 11 mutator flags of v0.6.0 explicitly and fails if `gremlins unleash --help` offers
  any other set; refuses to start when any `GREMLINS_*` variable is set; and passes no threshold.
  `hack/test/mutation_signal_test.sh` pins the argv, the config, the workflow wiring and the
  install chain, and rejects thresholds from flags, any `.gremlins` / `.gremlins.*` file and
  `GREMLINS_*THRESHOLD*` env at workflow, job or step level, each with a mutant in `--self-test`.
- **Three decisions are model-aligned and pending operator validation** (decisions.md,
  2026-10-19, change 9): **threshold sources A** (ban config-file and env thresholds too, not only
  CLI); **fixture placement A** (the MUT-3 surviving-mutant control runs in the nightly only;
  `lint` never installs or runs gremlins); **flat signal A** (keep three packages, turn on the
  default-off mutators, accept equivalent-mutant LIVED noise as a reported cost).
- **F1, warm test cache:** with a warm Go test cache every covered mutant came back TIMED OUT
  while the fixture still showed LIVED, so the control did not catch it. The runner exports
  `GOFLAGS=-count=1`.
- **B1, all-timed-out read as green:** 2 of 7 real runs reported every `internal/aggregate`
  mutant TIMED OUT and exited 0. Fixed twice over: the formatter fails a package with no KILLED
  and no LIVED mutant (case M9 on the recorded `warm-cache-pathvalidate.json`), and the runner
  passes `--timeout-coefficient=10` (0 of 6 runs timed out, against 4 of 6 at the default).
- **`MUTATION_FORCE_FAILURE`** (repository variable) only fails the run, to prove the failure
  report once after merge. Any value other than `true`, `false` or empty also fails.

## Risk

- **What can break:** the `lint` job gets slower (new guard self-test ~250–320 s, job ~18 min,
  near `tool-pins-selftest` at ~20 min); a gremlins help-text change fails the nightly on purpose.
- **Blast radius:** CI only. No product code, chart or API changes; the nightly is not a required
  check.
- **Rollback:** revert the PR; nothing is stateful. To silence the nightly alone, disable the
  workflow.
- **Residual risk accepted:** the guard's W3 scan is a text plus parsed-YAML scan, not a shell
  model (`eval` and run-time-built names are out of reach; the runner's `GREMLINS_` refusal covers
  the run-time side). LIVED classification on the real packages is agent-reviewed.

## Files to look at first

- `.github/workflows/mutation.yaml` — schedule, no score gate, artifact upload `if: always()`.
- `hack/ci/mutation.sh` — pinned config, 11 mutator flags, `GOFLAGS=-count=1`,
  `--timeout-coefficient=10`, `GREMLINS_` refusal, `MUTATION_FORCE_FAILURE`.
- `hack/ci/mutation-summary.sh` — report validation; fails a package with nothing killed or lived.
- `hack/test/mutation_signal_test.sh` — the guard (plain and `--self-test`), wired into `lint`.
- Test changes on other changes' guards: `.github/workflows/ci-failure-report.yaml` and
  `hack/test/ci_failure_report_test.sh` (the `workflow_run` list gains `Mutation testing`, mutant
  `MUT-1 list loses Mutation testing`); `hack/test/ci_workflow_security_test.sh` (`cfc_guards`
  gains the new guard in both modes, mutant `CFC-6 mutation signal guard unwired`). Neither old
  assertion was wrong; both lists grow by the new workflow and guard.

**Review this commit-by-commit, not as one flattened diff.** The fixtures land before the runner,
and each stage-B fix is its own commit with its own red case.

## Testing protocol

- [x] `bash hack/test/mutation_signal_test.sh` and `--self-test` green.
- [x] `ci_failure_report_test.sh`, `ci_workflow_security_test.sh` (both modes) and zizmor green.
- [x] `task lint:shell`, `task lint:markdown` (0 issues), `task spec:validate` green.
- [x] Seven real `task mutation` runs locally (B1 evidence in `evidence/B.md`).
- [ ] Every check on the PR head green, `dependency-review` included.
- [ ] After merge: the first scheduled run (01:30 UTC) is green, its summary shows the three
  package rows plus the fixture control, and the `mutation-reports` artifact holds four JSON
  reports. Failure signature: a red run with a `ci-failure-report` issue.
- [ ] Forced-failure night: set `MUTATION_FORCE_FAILURE=true`, confirm the red run and its
  failure-report issue, unset it, confirm the next night recovers.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the
deployment fine print — skip it if the top half already told you what you needed.*

- Spec set: `openspec/changes/mutation-testing-signal/` (`proposal.md`,
  `specs/mutation-testing/spec.md`, `tasks.md`). Run state and triage: `loop.md`.
- Spec-set review: two rounds; V1–V5 CRITICAL (config extensions, inert CLI/env thresholds,
  fail-open scans, unpinned commands, env narrowing) verified by a scratch gremlins probe and fixed;
  round 2 R2-1..R2-13 fixed (`reviews/R-spec-set/`).
- Branch review: round 1 found B1 (above); B2 and B4 fixed, B3 rejected, B6 and B7 deferred
  (`reviews/B-branch/round-1/verification.md`). Round 2 R2-1..3 fixed (help lines read in full,
  every parsed YAML string scanned).
- Measured cost: gremlins takes about 9 s for the fixture and the three packages; the 15-minute
  job timeout bounds a hung run, most of it Go setup and building gremlins.
- Out of scope: packages beyond the three, changing any existing test, any score gate.
