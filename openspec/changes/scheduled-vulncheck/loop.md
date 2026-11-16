# Loop — scheduled-vulncheck (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/scheduled-vulncheck` · Branch: `ci/scheduled-vulncheck` · Base: `origin/main` @ `f31f94ef3` · Started: 2026-11-16 · Reviewers: two-model duo
Next: R spec-set review
Task prompt: local, `agent-context/loops/scheduled-vulncheck/task-prompt.md` (harness, not committed)
Budget: paid review legs 0/12 (B reserve 3; default) · active hours 0/8 (sessions: 2026-11-16) · estimate: R 1, G1 1 (CI diff → extra leg trigger), T0.1 0, B 2 → planned 4

Backlog VULNCHECK-SCHEDULED (P2). Spec set: proposal.md, specs/scheduled-vulnerability-scan/spec.md,
tasks.md (no design.md: no identity, persisted metadata, compatibility or migration change).

## Stages

- [x] 0 orient — new change; open PRs touching these files: Renovate #552 (github actions) may bump the
  action shas the new workflow pins; whichever lands second follows the other
- [x] P plan + tasks — written; spec set committed: (this commit)
- [ ] R spec-set review
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

## Known red

## Test changes

## Lessons

## Triage

| Stage | Finding (one line) | Sev | Reviewers | Disposition | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
