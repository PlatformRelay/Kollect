# Loop — sink-ca-trust (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-ca-trust` · Branch: `feat/sink-ca-trust` · Base: `origin/main` @ `7f97d59ec` · Started: 2026-10-19 · Reviewers: astra + claude (duo)
Next: R spec-set review on the spec-set commit
Budget: claude review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SINK-CA-UNUSED-01 (decisions 2026-10-04 "→ A, honour it" and 2026-10-19 "CA semantics", outcome A). Release note: v0.22.0. Spec set: proposal.md, design.md, specs/sink-tls-trust/spec.md, tasks.md.

## Stages

- [x] 0 orient — new change, created with `task spec:cli -- new change sink-ca-trust`; file:line references measured against `origin/main` @ `7f97d59ec` (2026-10-19)
- [x] P plan + tasks — written; spec set committed: `70c847a8e`
- [ ] R spec-set review — `reviews/R-spec-set/` on <sha> · verdict: <…> · CRITICAL: <…>
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: <n> · verdict: <…>
- [ ] Hand-off — <date> · `pr-description.md` · PR: <none>

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/sink/... ./internal/pipeline/...` | triggered | green | per task |
| Integration-tagged build | `go vet -tags integration ./internal/sink/... ./internal/pipeline/...` | triggered | clean | per task |
| Integration TLS tests | `go test -tags integration -v -run '<names>' <pkg>` (Docker for Postgres/MongoDB) | triggered | new tests run, none skipped where Docker exists | per task |
| Race | `go test -race -count=3` on changed packages | triggered | green | per task |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint; `internal/sink/**` is one component, `.go-arch-lint.yml:63-64`) | triggered | no new findings | per task |
| Codegen drift | `task verify` | triggered | green | G6 and at B |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|

## Tasks

| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `internal/pipeline/sink_credentials_test.go:TestCLICredentials_matchOperatorResolver` | before this change | 1.1 (planned) | compares error presence only (`:384`); case `"inline bundle wins"` (`:356`) pins a precedence SCT-4 forbids |

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it -->

## Where a human should look first

<!-- filled at hand-off -->

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
