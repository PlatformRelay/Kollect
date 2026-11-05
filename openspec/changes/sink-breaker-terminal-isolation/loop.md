# Loop — sink-breaker-terminal-isolation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation` · Branch: `fix/sink-breaker-terminal-isolation` · Base: `origin/main` @ `96c41b0d0` · Started: 2026-11-05 · Reviewers: two-model duo
Next: R spec-set review
Task prompt: local, `agent-context/loops/sink-breaker-terminal-isolation/task-prompt.md` (harness, not committed) · decisions by model alignment, logged in agent-context/decisions.md
Budget: paid review legs 0/12 (B reserve 3; default) · active hours 0/8 (sessions: 2026-11-05) · per task estimate 0 legs (no extra-leg trigger expected: no secrets, CI, infra or manifests), R 1, B 2 → planned 3

Backlog SINK-BREAKER-TENANT-01 (P2) and RACE-COUNT-BREAKER-ISOLATION (P3). Spec set: proposal.md,
design.md, specs/sink-circuit-breaker/spec.md, tasks.md. Decision 2026-11-05 `sink-breaker-terminal`
(both legs A, pending operator validation).

## Stages

- [x] 0 orient — new change; file:line references measured against `origin/main` @ `96c41b0d0` (2026-11-05). Open PRs touching these files: none (#523 mongo-driver v2 touches `internal/sink/mongodb/` only; #520 helm v4 CI only)
- [x] P plan + tasks — written; `task spec:validate` green (22 items); spec set committed: (this commit)
- [ ] R spec-set review — `reviews/R-spec-set/`
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/`
- [ ] Hand-off — `pr-description.md`

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/sink/ ./internal/controller/ -skip TestControllers` | triggered | green | per task |
| Race | `go test -race -count=3 ./internal/sink/ ./internal/controller/` | triggered | RACE-COUNT-BREAKER-ISOLATION flake on base (4/6 runs, 2026-10) | per task; G2 makes it green |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint) | triggered | no new findings | per task |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base, minus known environment reds | at B |

Holistic run at B: not yet run.

## Triage

| Stage | Finding (one line) | Sev | Reviewers | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

## Where a human should look first

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
