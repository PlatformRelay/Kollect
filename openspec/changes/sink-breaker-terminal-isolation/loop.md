# Loop — sink-breaker-terminal-isolation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation` · Branch: `fix/sink-breaker-terminal-isolation` · Base: `origin/main` @ `96c41b0d0` · Started: 2026-11-05 · Reviewers: two-model duo
Next: L — dispatch T0.1 (probe)
Task prompt: local, `agent-context/loops/sink-breaker-terminal-isolation/task-prompt.md` (harness, not committed) · decisions by model alignment, logged in agent-context/decisions.md
Budget: paid review legs 2/12 (B reserve 3; default) · active hours 0/8 (sessions: 2026-11-05) · per task estimate 0 legs (no extra-leg trigger expected: no secrets, CI, infra or manifests), R 1, B 2 → planned 3

Backlog SINK-BREAKER-TENANT-01 (P2) and RACE-COUNT-BREAKER-ISOLATION (P3). Spec set: proposal.md,
design.md, specs/sink-circuit-breaker/spec.md, tasks.md. Decision 2026-11-05 `sink-breaker-terminal`
(first run A; revised to B after R2 showed A's premise false, both legs B; pending operator validation).

## Stages

- [x] 0 orient — new change; file:line references measured against `origin/main` @ `96c41b0d0` (2026-11-05). Open PRs touching these files: none (#523 mongo-driver v2 touches `internal/sink/mongodb/` only; #520 helm v4 CI only)
- [x] P plan + tasks — written; `task spec:validate` green (22 items); spec set committed: (this commit)
- [x] R spec-set review — round 1 on `a505029da`: reviewer-a REQUEST_CHANGES (1 CRITICAL), reviewer-b CONCERNS; all 8 findings verified and fixed (`reviews/R-spec-set/verification.md`); CRITICAL had a mechanical fix both legs named → round 2 (last) on `de1e7df12`: reviewer-a REQUEST_CHANGES, reviewer-b BLOCK; new CRITICAL (Git sink-wide Terminal reaches the breaker; A's premise false) settled by decision rerun → B (unanimous), spec set rewritten for B; 7 WARNING/NOTE fixed (`reviews/R-spec-set/round-2/verification.md`). No third round (limit); premises re-verified in T0.1, rewritten set reviewed again at B
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
| R1 | Parallel test resets every breaker key; `-count` reuses keys | CRITICAL | reviewer-a, reviewer-b | fix | design isolation rules, tasks rules, 1.2 | no |
| R1 | Mutable timeout seam races | WARNING | reviewer-a, reviewer-b | fix | design: constructor seam only | no |
| R1 | Cited Terminal errors return before the breaker | WARNING | reviewer-a, reviewer-b | fix | proposal, design three zones, 0.1 | no |
| R1 | Classification boundary, Forbidden, inferred Terminal unspecified | WARNING | reviewer-a, reviewer-b | fix | SCB-1/SCB-2 text + scenarios, cases 1d/2c/2d | no |
| R1 | SCB-3a proves no state; no concurrent trial | WARNING | reviewer-a | fix | SCB-3a/3b `State()`, SCB-3c | no |
| R1 | Flake mechanism overstated | WARNING | reviewer-a, reviewer-b | fix | proposal, 2.1 ten runs, 2.2 always | no |
| R1 | Docs "self-closes after timeout"; Terminal trial keeps half-open | NOTE | reviewer-a, reviewer-b | fix | 3.1 wording; SCB-3 accepted consequence | no |
| R2 | Git auth/protected-branch Terminal reaches the breaker; Terminal exports repeat on events; A drops load-shedding | CRITICAL | reviewer-a, reviewer-b | fix (decision) | decide.sh rerun → B; whole spec set rewritten | validation pending |
| R2 | SCB-2d does not guard the classification boundary | WARNING | reviewer-a | fix | SCB-2d via `RunExportEnvelope`; SCB-1e real Git | no |
| R2 | Counting-interval reset omitted | WARNING | reviewer-a | fix | SCB-2, design | no |
| R2 | Half-open timing flakes | WARNING | reviewer-a, reviewer-b | fix | design timing protocol | no |
| R2 | SCB-3 reds not on target assertion | WARNING | reviewer-b | fix | 1.1 refactor base | no |
| R2 | G2 red rule; reset only in cleanup; path-branching stub | NOTE | reviewer-a, reviewer-b | fix | tasks preamble, 2.1, 2.2, design | no |

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
