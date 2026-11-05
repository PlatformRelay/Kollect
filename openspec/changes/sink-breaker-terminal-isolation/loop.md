# Loop — sink-breaker-terminal-isolation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-breaker-terminal-isolation` · Branch: `fix/sink-breaker-terminal-isolation` · Base: `origin/main` @ `96c41b0d0` · Started: 2026-11-05 · Reviewers: two-model duo
Next: L — dispatch B1.1 (branch review fixes)
Task prompt: local, `agent-context/loops/sink-breaker-terminal-isolation/task-prompt.md` (harness, not committed) · decisions by model alignment, logged in agent-context/decisions.md
Budget: paid review legs 3/12 (B reserve 3; default) · active hours 0/8 (sessions: 2026-11-05) · per task estimate 0 legs (no extra-leg trigger expected: no secrets, CI, infra or manifests), R 1, B 2 → planned 3

Backlog SINK-BREAKER-TENANT-01 (P2) and RACE-COUNT-BREAKER-ISOLATION (P3). Spec set: proposal.md,
design.md, specs/sink-circuit-breaker/spec.md, tasks.md. Decision 2026-11-05 `sink-breaker-terminal`
(first run A; revised to B after R2 showed A's premise false, both legs B; pending operator validation).

## Stages

- [x] 0 orient — new change; file:line references measured against `origin/main` @ `96c41b0d0` (2026-11-05). Open PRs touching these files: none (#523 mongo-driver v2 touches `internal/sink/mongodb/` only; #520 helm v4 CI only)
- [x] P plan + tasks — written; `task spec:validate` green (22 items); spec set committed: (this commit)
- [x] R spec-set review — round 1 on `a505029da`: reviewer-a REQUEST_CHANGES (1 CRITICAL), reviewer-b CONCERNS; all 8 findings verified and fixed (`reviews/R-spec-set/verification.md`); CRITICAL had a mechanical fix both legs named → round 2 (last) on `de1e7df12`: reviewer-a REQUEST_CHANGES, reviewer-b BLOCK; new CRITICAL (Git sink-wide Terminal reaches the breaker; A's premise false) settled by decision rerun → B (unanimous), spec set rewritten for B; 7 WARNING/NOTE fixed (`reviews/R-spec-set/round-2/verification.md`). No third round (limit); premises re-verified in T0.1, rewritten set reviewed again at B
- [x] L task loop — see *Tasks* (T0.1, G1, G2, G3 closed; B1.1 from stage B)
- [ ] B branch review — round 1 on `4e50a9a2f` (rebased on main after #520/#523): reviewer-a CONCERNS, reviewer-b CONCERNS, no CRITICAL; SCB-1..3 hold; 6 fix (B1.1 + spec wording), 1 defer, 2 reject (`reviews/B-branch/round-1/verification.md`). Next: B1.1, then round 2 + per-commit gate
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

Holistic run at B: `task check` on `4e50a9a2f` exit 1; four gates red, none from this branch (git-ignored mkdocs cache hit, `kubectl` shim, two `operator-sdk` guards; the last three red on clean `origin/main` too). Details `reviews/B-branch/round-1/verification.md`.

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
| L G2 | 2.1 race command also flakes on `TestClusterInventory_DebouncedExportIncrementsMetric` (parallel test reading a global counter delta), 1 in 10 | WARNING | reviewer-a (P1), task agent | fix (option A, by fact check: one line with in-repo precedent `kollectinventory_finalizer_test.go:974`; B weakens the criterion and leaves a known flake) | tasks.md 2.2 extended | no |
| B1 | Only one routed raise site pinned by a test | WARNING | reviewer-a, reviewer-b | fix | B1.1 | no |
| B1 | go-git engine return path untested | WARNING | reviewer-b | fix | B1.1, design *Risks* | no |
| B1 | Race command in test rules fails on Ginkgo | WARNING | reviewer-a | fix | tasks.md test rules | no |
| B1 | Green-at-every-commit claim; T0.1 cleanup wording; marker always Terminal | NOTE | reviewer-a, reviewer-b | fix | tasks.md, B1.1, SCB-1 text | no |
| B1 | `ErrTooManyRequests` logged as "git transport" for every sink | NOTE | reviewer-b | defer | backlog SINK-BREAKER-TOOMANY-MSG | no |
| B1 | Own-path rejections at undecided sites still count; marked trial keeps half-open | NOTE | reviewer-a, reviewer-b | reject | spec fallback; accepted consequence (proposal *Impact*) | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| T0.1 | CLOSED | reviewer-a APPROVE (1 P2 fixed), 1 round, no paid leg (`reviews/L-tasks/T0.1/`) | none; 2 non-blocking spec wording corrections (25 not 26 `invalidPrune` call sites; proposal *Impact* misses `git/prune_target.go`), see `evidence/T0.1.md` *Spec corrections*. Race on base: 2/10 red |
| G1 (1.1 + 1.2) | CLOSED | reviewer-a APPROVE (1 P2 evidence wording fixed), 1 round, no paid leg (`reviews/L-tasks/G1/`) | none; race row split: Ginkgo `TestControllers` refuses `-count=3`, run with `-skip TestControllers` |
| G2 (2.1 + 2.2) | CLOSED | round 1 reviewer-a REQUEST_CHANGES (P1 9/10 green missed 2.2's clause → decision A; P2 evidence wording fixed); round 2 reviewer-a APPROVE; no paid leg (`reviews/L-tasks/G2/`, `round-2/`) | none. Base 3/10 red (shared `default/postgres-demo`); after per-invocation sink names and the debounce-metric test made non-parallel: 10/10 green |
| G3 (3.1) | CLOSED | reviewer-a REQUEST_CHANGES (2 P1 operator-wording over-claims, verified and fixed, no new round); no paid leg (`reviews/L-tasks/G3/`) | none. `task docs:verify` not-run (host `kubectl` shim); strict mkdocs build, docs unit contracts and doc guards green instead |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons

- T0.1: G1 marks exactly 5 sites (`prune_owned.go:174,261,268,310`, `prune_target.go:66`); 7 sites are undecided (shared helpers `prune_owned.go:53,57,77,80,83` called for own paths and shared metadata; capacity sites `:215,:230`) and stay unmarked.
- T0.1: `identityRemote.export` (`internal/sink/inventory_identity_test.go:85`) resets every breaker before each export and uses the fixed key `default/identity-git`; SCB-1e must call `RunExportEnvelope` directly with a per-invocation sink name, non-parallel.
- T0.1: gobreaker `Counts()` does not advance state; call `State()` before reading counts after a time step.
- G1: the controller package's Ginkgo suite refuses `-count>1` (package FAIL, no test failure); every `-count=3` command naming `./internal/controller/` needs `-skip TestControllers`.
- G1: a stub snapshot backend receives `objectstore.ObjectPath(...)`, not `req.ObjectPath`; branch a stub on the inventory name. Breaker test helpers `uniqueBreakerKey`/`uniqueBreakerName`/`stubExportRequest`/`halfOpenBreaker` live in `internal/sink/circuit_breaker_test.go` for G2 to reuse.
- G2: the ten-run race loop is probabilistic and cannot kill a mutant; a deterministic sensor for breaker sharing is `-count=3 -run` over only the tests whose backend fails (no success between iterations to reset the streak): 5 counted failures in iteration 1 open a shared breaker.
- G2: `TestClusterInventory_DebouncedExportIncrementsMetric` is a separate parallel metric-delta flake in the same race command (~1/10); a red there is not the breaker.
- G3: `task docs:verify` can fail on a missing mise `kubectl` version before its mkdocs step; build strict from a scratch venv of `docs/requirements-docs.txt` and run `test/docs` unit contracts plus the guards naming the edited files.

## Where a human should look first

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
