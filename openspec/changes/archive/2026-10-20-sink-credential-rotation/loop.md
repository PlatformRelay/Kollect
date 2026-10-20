# Loop — sink-credential-rotation (spec-loop-duo run state)

Repo: `/home/koni/Projects/PlatformRelay/worktrees/kollect/sink-credential-rotation` · Branch: `feat/sink-credential-rotation` · Base: `origin/main` @ `ae98ef106` · Started: 2026-10-19 · Reviewers: two independent review legs (duo)
Next: none — merged and archived
Task prompt: local, `agent-context/loops/sink-credential-rotation/task-prompt.md` (harness, not committed) · Budget: paid review legs uncapped (operator goal 2026-10-16) · decisions by model alignment, logged in agent-context/decisions.md

Backlog SECRET-ROTATION-01 (release inventory 2026-10-19 items 12-13). Release note: v0.22.0. Spec set: proposal.md, design.md, specs/sink-secret-rotation/spec.md, tasks.md.

## Stages

- [x] 0 orient — new change, created with `task spec:cli -- new change sink-credential-rotation`; file:line references measured against `origin/main` @ `ae98ef106` (2026-10-19)
- [x] P plan + tasks — written; spec set committed: `84f61bba3`
- [x] R spec-set review — round 1 on `84f61bba3`: reviewer-a BLOCK, reviewer-b CONCERNS; CRITICAL A1 (cached reads) accepted via model-aligned decision → C, CRITICAL A5 (SCR-4 scope) accepted and narrowed; all other findings fixed (`reviews/R-spec-set/verification.md`). Round 2 (last) on `a0ac1aefe`: reviewer-a BLOCK (WARNINGs only), reviewer-b CONCERNS, no CRITICAL; all verified findings fixed (`reviews/R-spec-set/round-2/verification.md`). No CRITICAL remains
- [x] L task loop — 0.1, 1.1–4.1 closed (see *Tasks*)
- [x] B branch review — round 1 on `df5170e57`: reviewer-a BLOCK (A1, A2 filed CRITICAL, verified and downgraded to WARNING, fixed 6.2, 6.3), reviewer-b CONCERNS (B1 pinned as residual 6.5, B2 rejected, B3 wording, B4 cluster twin 6.4); holistic H1 `go mod tidy` fixed 6.1. Round 2 (last) on the fixes: reviewer-a CONCERNS (C1 wording), reviewer-b CLEAN; no CRITICAL; C2 test fix 6.7, wording 6.8 (`reviews/B-branch/round-*/verification.md`). Final gates green on `00604732b`; tip `4f2f5e155`
- [x] Hand-off — 2026-10-20 · `pr-description.md` · PR: #480, every check green on `b7f2e6bfe` (CI 39346375954, Preflight 39346376361, Docs 39346376214, CodeQL 39346375920, E2E smoke 39346375919, E2E extended 39346376211; `test-integration` expected 68, executed 68, skipped 0; SonarCloud green); rebase-merged as `a56abe376`; main CI on `a56abe376` green (CI 39348859149, Preflight 39348859160, Docs 39348859496, CodeQL 39348859122, E2E smoke 39348859093); no GitHub issue tracks SECRET-ROTATION-01; archived in a follow-up PR

## Fitness functions

| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Unit tests, changed packages | `go test -v ./internal/sink/... ./internal/pipeline/...` | triggered | green | per task |
| Envtest, controller path | `go test -v -run TestControllers ./internal/controller/` (Ginkgo focus on the new specs) | triggered | green | G1, G2 |
| Integration-tagged build | `go vet -tags integration ./internal/sink/... ./internal/pipeline/...` | triggered | clean | per task |
| Integration rotation test | `go test -tags integration -v -run TestPostgresExport_passwordRotation <pkg>` (Docker) | triggered | runs, not skipped where Docker exists | G1 |
| Race | `go test -race -count=3` on changed packages (`-count=10 ./internal/sink/` in G3) | triggered | green | per task |
| Architecture | `task lint` (golangci-lint v2 and go-arch-lint) | triggered | no new findings | per task |
| Markdown | `task lint:markdown` (tracked files only; stage new files first) | triggered | 0 issues | per task + every orchestrator state edit |
| OpenSpec strict | `task spec:validate` | triggered | green | per spec-set edit |
| Full local gate | `task check` | holistic | green on base | at B |

Holistic run at B: not run yet.

## Triage

| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R1 | Cached Secret reads give only eventual next-export semantics | CRITICAL | reviewer-a | fix | decision C (uncached metadata GET, full GET on rebuild); SCR-1, design *Readers* | no (models agreed) |
| R1 | SCR-4 promised redaction of third-party constructor errors | CRITICAL | reviewer-a | fix | SCR-4 narrowed; non-goal | no |
| R1 | Cross-namespace Secret under restricted cache fails | WARNING | reviewer-a | fix | by decision C; envtest 1.1 | no |
| R1 | Key/values from different reads; slow builder replaces newer entry | WARNING | both | fix | design steps 3-4; 1.1, 3.1 tests | no |
| R1 | Stream-sink cleanup blocked by a deleted Secret | WARNING | reviewer-b | fix | retain before acquire; SCR-2; 2.1 | no |
| R1 | Tombstone wording, namespace param, reds per base, matrix gaps, Close assumption, legacy kind, citations, retire-on-any-error, partial S3 Secret, multiline rg | WARNING/NOTE | both | fix | verification.md rows A4, A6-A12, B2, B4, B6 | no |
| R1 | Metric for pool Secret reads | NOTE | decision counterpoint | defer | follow-up (proposal non-goals) | no |
| R2 | Store fence fails after own retire / absent-as-permission | WARNING | both | fix | generation counter, share-then-fence (design step 4) | no |
| R2 | Cleanup paths not wired to the uncached reader | WARNING | both | fix | `cleanupSinkExports`, `pruneRelationalExport`, reconciler default | no |
| R2 | Postgres assertion on the upsert path; reds green on G1; annotations in metadata; SCR-2/SCR-3 retire contradiction; terminal Git auth doc | WARNING | both | fix | round-2/verification.md A3-A5, B2, B3 | no |
| R2 | TTL wording, stream set, namespace attribution, empty refs, CLI message, CA compat row | NOTE | reviewer-b | fix | round-2/verification.md B4-B10 | no |
| L/T0.1 | `exportMinInterval: 0s` never re-exports an unchanged inventory and an inventory-level `0s` gives no timed retry after a transient total failure; contradicts the interval bound and SCR-2 recovery | SPECIFICATION | probe | fix | decision 2026-10-19 rotation-zero-interval, option B, models agreed: wording limited to intervals > 0, SCR-7 added, tasks 2.3/2.4 | no (models agreed) |
| B1 | A1: a missing or empty Secret seen by the full read (after a good metadata read) neither retires the pooled entry nor fences a build in flight | CRITICAL → WARNING | reviewer-a | fix | downgraded after verification: every later acquire's metadata read fails and retires, so no later export uses the stale backend; fixed red-first (6.2) | no |
| B1 | A2: an unchanged, exported `0s` binding re-exports once its coalescing state ages out (48 h TTL) | CRITICAL → WARNING | reviewer-a | fix | downgraded: one idempotent re-export per 48 h, no credential risk; a skip now counts as activity (6.3) | no |
| B1 | H1: test-only import of `k8s.io/utils/ptr` breaks `go mod tidy -diff` | WARNING | holistic | fix | package `boolPtr` (6.1) | no |
| B1 | B1: failed re-read after a stale first read retires the newer entry | WARNING → NOTE | reviewer-b | fix (pin) | accepted residual (one drain and rebuild, never stale use); pinned by a test (6.5), stated in SCR-2/design (6.6) | no |
| B1 | B2: SCR-7 watchdog on every degraded path of a `0s` inventory | NOTE | reviewer-b | reject | tasks 2.4 names the two functions; none of those paths exports; same as G2 rounds 1 and 2 | no |
| B1 | B3: fence starts at the pool miss, not at the acquire start | NOTE | reviewer-b | fix | SCR-3 wording (6.6) | no |
| B1 | B4: no cluster twin of the `0s` terminal no-requeue test | NOTE | reviewer-b | fix | `TestClusterReconcile_zeroIntervalTerminalNoRequeue` (6.4) | no |
| B2 | C1/C5: `0s` re-export after a restart or > 48 h on a degraded early return | WARNING | reviewer-a, reviewer-b | fix (wording) | not a behaviour defect; SCR-7, design and troubleshooting name both cases; scenario reads "on any later watchdog requeue" (6.8) | no |
| B2 | C2: the B1 residual test accepted both outcomes | NOTE | both | fix | test asserts the retire, drain and rebuild; mutants killed (6.7) | no |
| B2 | C3: SCR-2 said "observed a change" where the read saw only a mismatch | NOTE | reviewer-b | fix | wording "mismatch with the pooled entry" (6.8) | no |
| B2 | C4: full-read retire can hit an entry built from a recreated Secret | NOTE | reviewer-b | no action | covered by the design's "entry stored meanwhile is retired"; one rebuild, never stale use | no |

## Tasks

| Task | Verdict | Review (legs, rounds, paid leg?) | Gaps / decision request |
|---|---|---|---|
| T0.1 | CLOSED-WITH-GAPS (first BLOCKED, SPECIFICATION, resolved by decision B) | reviewer-a diff, 1 round, REQUEST_CHANGES on evidence class and wording, fixed; no paid leg (evidence only, no BLOCK) | rows 4, 5 by reading (gap); the `0s` finding became SCR-7 and tasks 2.3, 2.4 (spec-set commit after `74e8376c1`) |
| G1 | CLOSED-WITH-GAPS | reviewer-a and reviewer-b (paid leg: credential handling is security posture), diff lens, 2 rounds: r1 APPROVE / CONCERNS (cluster export wiring untested, fixed), FINAL red on CA-before-credential precedence fixed, r2 REQUEST_CHANGES / CONCERNS (CA-read masking, nameless caSecretRef, fixed after the last round) | rows 15, 16 agent-review (sensors in G2); G1-G3 must land together (G1 alone: stream cleanup reads the Secret, rotated backend closed under a concurrent export) |
| G3 | CLOSED-WITH-GAPS | reviewer-a and reviewer-b (paid leg: credential handling is security posture), diff lens, 2 rounds: r1 BLOCK (stale-read guard kept an older entry after an observed change; aged-out generation read as 0; both fixed) / CLEAN, r2 BLOCK (a failed re-read dropped the observed change, fixed after the last round) / CLEAN | the round-2 fix (`observeAndLookup` failed re-read path) is unreviewed: stage B should read it; accepted residual: a store during the forced re-read is retired (one extra build) |
| G4 | CLOSED | reviewer-a, diff lens, 1 round: REQUEST_CHANGES on wording (event-page `spec.secretRef` list; "every cleanup" overstated), fixed, no second round; no paid leg (docs only, no BLOCK) | none; upgrade section titled per tasks.md ("Sink Secret rotation"), not the dispatch's "Sink credential rotation" |
| G2 | CLOSED-WITH-GAPS | reviewer-a and reviewer-b (paid leg: credential handling is security posture), diff lens, 2 rounds: r1 REQUEST_CHANGES (a CA Secret seen gone only by the CA-rules read did not retire, fixed) / CONCERNS (watchdog on every degraded path rejected per tasks.md 2.4; empty-CA compatibility pinned by a test), r2 APPROVE / CLEAN | cluster-side terminal-no-requeue control missing (gap, stage B); `TestSecretMeta_notFoundIsTransient` and the redacted-status envtest were green on G1 (outcome already held); G1's stream-cleanup gap closed; the rotated-backend-under-export gap stays for G3, and G2 widens it (retire before rebuild) |
| B | stage B: 2 rounds; fixes 6.1-6.8 | reviewer-a (adversarial) and reviewer-b (spec, paid leg), whole branch: r1 BLOCK (two CRITICAL, verified, downgraded to WARNING: full-read not-found did not retire or fence; 0s state aged out and re-exported) / CONCERNS (failed re-read residual, watchdog on every degraded path rejected again, fence wording, cluster twin); holistic: go.mod tidy red (fixed), four environment reds traced; r2 on the fixes CONCERNS (0s re-export after 48 h degraded: wording) / CLEAN | none open: every verified finding fixed or rejected with a reason (`reviews/B-branch/round-*/verification.md`); final gates green on `00604732b` |

## Known red

Tests a test task left red for a later task; excluded from "green" at close, nothing else is.

| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes

Tests from a test task changed by a later task (close-task `TEST` class); each one changes what green means.

| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|
| `internal/controller/secret_reader_wiring_test.go`, `secret_rotation_envtest_test.go` (G1), `secret_retire_envtest_test.go` (G2): manager config | G1, G2 | B (6.1, `e2b3193e1`) | not wrong in what it asserts: `ptr.To(true)` became `boolPtr(true)` so `go mod tidy -diff` stays clean; assertions unchanged |
| `internal/sink/backend_pool_reread_residual_test.go:TestAcquireBackend_failedReReadAfterStaleReadDrainsNewer` | B (6.5) | B (6.7, `b7c415293`) | it accepted both outcomes, so dropping the failed re-read retire left it green (round 2 C2) |

## Lessons

<!-- master list: one line per reusable lesson with the task ids that hit it; two or more ids → a proposed harness change at hand-off -->

- T0.1: the metadata GET of a missing Secret returns an API NotFound that `ClassOf` classes terminal; `secretMeta` must rewrite it to the plain "not found" text (as `ResolveSecret` does) or a deleted Secret is never retried.
- T0.1: the fake client keeps the UID a test sets and starts `resourceVersion` at 999, +1 per write; model delete-and-recreate by setting a new UID.
- T0.1: a probe that only prints is agent-review, not machine; assert the reported outcome in the probe.
- T0.1: check "bounded by the export interval" claims against `exportMinInterval: 0s` (material-change only, `per_sink_export.go:85-87`).
- G1: a list-driven rewrite of an ordered reader keeps the read set but can change error precedence; the pinned CA-before-credential order (`internal/pipeline` `TestCAParity_caRulesPrecedeCredentialLookup`) only showed up in the full suite.
- G1: Ginkgo refuses `go test -count>1` and `-ginkgo.repeat` is CLI-only; repeat `go test -race -count=1 -run '^TestControllers$'` N times.
- G1: run the full suite with `umask 022`; `internal/sink/git` mirror tests fail on a group-writable temp dir.
- G1: the worktree has no `bin/`; envtest needs `KUBEBUILDER_ASSETS=<main checkout>/bin/k8s/1.36.0-linux-amd64` or `BeforeSuite` fails on etcd.
- G1: a never-started manager with a static REST mapper (`MapperProvider`) lets a plain unit test call `SetupWithManager` without an API server.
- G2: a "red on base" test can already hold when the earlier group built the shared helper; record it green-on-base and make it load-bearing through a mutant.
- G2: an error path that reads a second Secret (CA rules after a failed credential read) is a second observation point; retire rules must look at the final error too.
- G2: changing a shared status helper (`setInventoryDegraded`) widens behaviour to every caller; list the callers in the evidence before review.
- G2: G3 must cover a stale-read interleaving (an acquire whose metadata read predates a rotation retires the newer entry at lookup).
- G3: a guard that skips an action because "the pool changed since my read" must keep the read's observation (re-read; keep it when the re-read fails); list each attempt's read outcomes (ok, gone, other error) before review.
- G3: a fence on "value unchanged" needs values never reissued, also after aging out; an absent key must not read as a value a caller could have recorded.
- G3: a concurrency test with one barrier after the read lets late goroutines hit; hold all in the factory and assert the build count, or surplus-close mutants survive.
- G3: check a line-deleting mutant's line is live before citing it (G3's first M14 deleted dead code).
- G4: check "every export and cleanup" doc claims against each cleanup branch (stream retain, Retain, shared identity skip the Secret read).
- G4: `task docs:verify` in a worktree needs `MISE_KUBECTL_VERSION=1.36.1` and a temp mkdocs venv from `docs/requirements-docs.txt` on `PATH`.
- B: `task check` in a worktree needs `MISE_KUBECTL_VERSION=1.36.1`, `operator-sdk` in the worktree `bin/` (`bash hack/install-operator-sdk.sh ./bin`) and no stale ignored `.cache/` (mkdocs social cache trips `gitleaks-detect --no-git`); trace each red before blaming the branch.
- B: a test-only import of a module `go.mod` lists as indirect (`k8s.io/utils/ptr`) fails `go mod tidy -diff` (Preflight) while every per-task gate stays green; run `go mod tidy -diff` per task when a test adds an import.
- B: MinIO integration tests fail with `XMinioStorageFull` when the host disk is nearly full; check `df -h /` before reading it as a regression.
- B: every read that can observe a missing Secret (metadata, full read, CA-rules read) must apply the same retire and fence; per-task reviews checked the metadata path and missed the full read (same class as the G2 CA-rules lesson).
- B: a spec promise "at any later time" must be checked against in-memory state lifetimes (TTL prunes, restarts), not only against timers.
- B: a test that branches on the outcome it should pin passes both ways; assert the one outcome the design states.

## Where a human should look first

- `internal/sink/backend_pool.go`: holders, `retireLocked`, the store fence and `observeAndLookup` (the failed re-read path and the full-read retire of 6.2).
- `internal/sink/secret_refs.go`: the one list of Secret references the key and the build share; a new `*SecretReference` field must be added there (`TestReferencedSecretRefs_coversEverySecretReferenceField` guards it).
- `internal/controller/per_sink_export.go` and the two `setDegraded` helpers: the `0s` watchdog (SCR-7) and the skip-as-activity prune (6.3).
- Accepted residuals: a failed re-read after a stale first read costs one rebuild (B1); a `0s` binding re-exports an unchanged payload once after a restart or more than 48 h on a degraded early return (C1).

## Proposed harness changes

| Lesson | Seen in | Proposed change (AGENTS.md line / sensor / close-task.yml) |
|---|---|---|

## Owner tasks (skipped by the loop)

| Task | Command sheet |
|---|---|
