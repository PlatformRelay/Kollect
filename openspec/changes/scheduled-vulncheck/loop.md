# Loop — scheduled-vulncheck (spec-loop-duo run state)

Repo: worktree `scheduled-vulncheck` · Branch: `ci/scheduled-vulncheck` · Base: `origin/main` @ `f31f94ef3` · Started: 2026-11-16 · Reviewers: two-model duo
Next: L — G1
Task prompt: local, `agent-context/loops/scheduled-vulncheck/task-prompt.md` (harness, not committed)
Budget: paid review legs 0/12 (B reserve 3; default) · active hours 0/8 (sessions: 2026-11-16) · estimate: R 1, G1 1 (CI diff → extra leg trigger), T0.1 0, B 2 → planned 4

Backlog VULNCHECK-SCHEDULED (P2). Spec set: proposal.md, specs/scheduled-vulnerability-scan/spec.md,
tasks.md (no design.md: no identity, persisted metadata, compatibility or migration change).

## Stages

- [x] 0 orient — new change; open PRs touching these files: Renovate #552 (github actions) may bump the
  action shas the new workflow pins; whichever lands second follows the other
- [x] P plan + tasks — written; spec set committed: (this commit)
- [x] R spec-set review — round 1 on `6e83424eb`: reviewer-a REQUEST_CHANGES (1 CRITICAL), reviewer-b CONCERNS; 13 fix, 1 partly rejected (`reviews/R-spec-set/round-1/verification.md`); CRITICAL had a mechanical fix both legs named → round 2 (last) on `067e0ace7`: reviewer-a REQUEST_CHANGES (1 CRITICAL), reviewer-b CONCERNS; the CRITICAL (Task's exit status, not govulncheck's) is a mechanical fix both legs named, verified against the 2026-11-12 CI log (`Process completed with exit code 201`); all 9 findings fixed (`reviews/R-spec-set/round-2/verification.md`). No third round (limit); T0.1 probes the Task status and title format, and stage B reviews the final set
- [ ] L task loop
- [ ] B branch review
- [ ] Hand-off

## Fitness functions

| Characteristic | Command | Kind | Baseline | Per task / at B |
|---|---|---|---|---|
| Workflow security | `bash hack/test/ci_workflow_security_test.sh` (+ `--self-test`) | triggered | green | per task |
| Reporter wiring | `bash hack/test/ci_failure_report_test.sh` (+ `--self-test`) | triggered | green | per task |
| Toolchain pins | `bash hack/test/consistency_toolchain_pins_test.sh` | triggered | green | per task |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + state edits |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base, minus known environment reds | at B |

## Tasks

- T0.1 · BLOCKED (SPECIFICATION) → CLOSED after the orchestrator corrected proposal *Why* to the verified facts (run 42151173159, #492; no design or task change) · proposal *Why*: `main` red 2026-11-06..13 (not 11-12/13), 7 of 10 Renovate PRs red (not 9), #492 was the maintainer's · every other claim holds · `evidence/T0.1.md` · review `reviews/L-tasks/T0.1/` reviewer A REQUEST_CHANGES (evidence: real `task vulncheck` probe missing), fixed, no round 2
- G1 · CLOSED-WITH-GAPS (SVS-2 in production is 2.1's; docs:verify's mkdocs build not run locally) · workflow, script, guard (64 → 68 self-test cases), reporter entry, lint steps, CWS-6/CFR updates, docs · `evidence/G1.md` · review `reviews/L-tasks/G1/` reviewer A APPROVE (P2 evidence), reviewer B (paid leg, CI diff trigger) CLEAN with notes; title-rule edge cases added (V-3e), no round 2

## Known red

## Test changes

## Lessons

- T0.1: read job logs with `gh run view <run> --log --job <job>`; `gh api .../jobs/<id>/logs` refuses output with escape sequences.
- T0.1: the `task` first on `PATH` here is 3.53.1; prove Task 3.54.0 with `~/.local/share/mise/installs/task/3.54.0/task`.
- T0.1: CI run 43406013666 holds all three advisories (stdlib included) in one real `task vulncheck` output; govulncheck wraps titles at 80 columns.
- G1: `consistency_toolchain_pins_test.sh` models Renovate over tracked files: `git add` a new workflow first, or its setup-task pin reads as unextracted.
- G1: local `docs:verify` stops at `mkdocs build` (`redirects` plugin missing, main checkout's venv too); record the steps before it and leave the build to CI.
- G1: generate whitespace-sensitive fixtures inside the test at run time; a committed one is at the mercy of whitespace hooks.

## Triage

| Stage | Finding (one line) | Sev | Reviewers | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R1 | 40-hex `uses:` rule contradicts the required local go-cache action | CRITICAL | reviewer-a, reviewer-b | fix | spec SVS-1.2 exact step shape; tasks V-2 (mutation W1 model) | no |
| R1 | No test that the cron is daily, or that a trigger is removed | WARNING | reviewer-a, reviewer-b | fix | SVS-1 cron fields; V-2 mutants | no |
| R1 | CWS-6 `cfc_guards` not named; guard could be unwired silently | WARNING | reviewer-b (reviewer-a NOTE) | fix | SVS-3 + V-6 | no |
| R1 | Issue excerpt is FAIL-lines only; govulncheck prints none, so the issue cannot name the advisory | WARNING | reviewer-b | fix | new SVS-1.2 wrapper script printing `FAIL: <GO id>` lines; V-3 | no |
| R1 | Post-merge: red run assumed an advisory; dispatch never exercises the reporter | WARNING | reviewer-a | fix | tasks 2.1 classify by FAIL line; observe first scheduled run | no |
| R1 | SLA clock could start on an infrastructure failure | WARNING | reviewer-a | fix | tasks 1.3 wording; proposal | no |
| R1 | "Every open PR red at once" overstates; checks re-run only on new runs | WARNING | reviewer-a | fix | proposal *Why*; T0.1 records the run ids | no |
| R1 | Renovate indirect-updates exclusion justified wrongly | WARNING | reviewer-a | fix | proposal *Out of scope* reworded (detection vs. fix supply) | no |
| R1 | `ci_workflow_security_test.sh` is not the zizmor audit; run the audit, shellcheck, toolchain pins | WARNING | reviewer-a | fix | tasks test rules; T0.1 names the zizmor command | no |
| R1 | timeout only checked for presence | WARNING | reviewer-a | fix | SVS-1 positive integer; V-2 | no |
| R1 | `secrets.` grep trips on comments, misses `github.token` | NOTE | reviewer-b | fix | SVS-3; V-2 expression-only, comment control | no |
| R1 | Concurrency unspecified; cancel-in-progress would fake a failure | NOTE | reviewer-b | fix | SVS-1 no `concurrency:`; V-2 top-level keys | no |
| R1 | Transient DB outage opens an issue | NOTE | reviewer-b | fix | proposal *Impact* | no |
| R1 | loop.md absolute home path / harness wording | NOTE | reviewer-a | fix (path) / reject (wording) | path made relative; "two-model duo" is the loop convention in every archived change | no |
| R2 | Script status is Task's (201), not govulncheck's; fake exit 3 hides it | CRITICAL | reviewer-a, reviewer-b | fix | SVS-1.2 "task's status"; V-3 fixtures exit 201 + pass-through case; T0.1 probes | no |
| R2 | Job-level `concurrency:`/`env:` and extra checkout inputs unchecked | WARNING | reviewer-a | fix | SVS-1.1 job-key allow-list, no `env:`; V-2 mutants | no |
| R2 | Red-first claimed for cases that cannot be red on the base | WARNING | reviewer-a | fix | tasks: *base red* marks; mutants proven by self-test after 1.2 | no |
| R2 | Cron rule admits hourly / every-minute | WARNING | reviewer-a, reviewer-b | fix | SVS-1 integer minute+hour; V-2 mutants | no |
| R2 | SLA-clock sentence contradicts the policy's zero-tolerance "no SLA" rule | WARNING | reviewer-b | fix | tasks 1.3 targets the zero-tolerance and enforcement rows; proposal | no |
| R2 | Title whitespace unspecified | NOTE | reviewer-b | fix | SVS-1.2 trimmed | no |
| R2 | `timed_out` conclusion not named | NOTE | reviewer-b | fix | SVS-1 scenario | no |
| R2 | Fixture lacks the stdlib advisory | NOTE | reviewer-b | fix | test rules: stdlib entry added to the fixture | no |
