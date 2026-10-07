# Loop — 2026-10-06-product-decision-convergence

Repo: `/Users/kheimel/.treehouse/kollect-79dca7/1/kollect` · Branch: `fm/kollect-product-decisions-impl` · Base: `3ee21266` (= `origin/main`) · Started: 2026-10-06 · Reviewers: fanout (free only)
Next: L task loop — dispatch T05
Budget: claude review legs 0/0 (free-model-only overlay) · active hours 0/8 (session 2026-10-06–) · source: overlay default

## Stages
- [x] 0 orient — OpenSpec repo (no .specify/): change dir `openspec/changes/2026-10-06-product-decision-convergence/` is the feature dir; spec set = proposal.md + design.md + specs/*/spec.md + tasks.md. Spec set committed: `<sha>`
- [x] P plan + tasks — generated (OpenSpec change); spec set committed a93f6862
- [x] R spec-set review — round 1 on a93f6862: 4/6 legs ok (both DeepSeek legs exit=truncated at 900s), CRITICAL x1 (capability-id mismatch) fixed in place + 20 findings fixed; round 2 (bigQwen, 1 leg) on 8e2ee768: CONCERNS, 2 warnings closed (fixture migration, persisted-sink scenario, in-repo decisions table), no CRITICAL. Gate passed.
- [ ] L task loop — see *Tasks*
- [ ] B branch review — `reviews/B-branch/round-N/` · rounds: <pending>
- [ ] Hand-off — <date> · `pr-description.md` · PR: <pending; direct-PR contract allows non-draft>

## Fitness functions
| Characteristic | Command | Kind | Baseline (number, list, allow-list) | Per task / at B |
|---|---|---|---|---|
| Go vet/build | `go build ./...`, `go vet ./...` | triggered | clean | per task |
| Lint incl. depguard | `task lint` (golangci; dupl/lll excluded for all of internal/* — known debt, do not widen) | triggered | green | per task |
| Coverage floor | `task coverage` (COVERAGE_MIN 90, `hack/coverage.sh`) | holistic | 90% internal | at B |
| Codegen drift | `task verify` (`hack/verify.sh`: CRDs, RBAC, deepcopy, chart crds) | triggered | clean | per task (T07/T09) |
| Secrets | `gitleaks protect --staged` | triggered | clean | per task at commit |
| OpenSpec strict | `task spec:validate` (`openspec validate --all --strict`) | triggered | green | per spec-set change + T10 |
| Helm docs drift | `task helm-docs:verify` | triggered | green | per task touching chart values (none planned; `mode` parked) |
| Markdown/launch truth | `task lint:markdown`, `hack/test/docs_launch_truth_test.sh` | triggered | green | at B |
| Race (stateful rule) | `go test -race -count=2` on changed packages | triggered | green | per task (stateful/export changes) |
| Integration tier | `task test-integration` (Docker) | holistic | green in CI | not-run locally: no Docker in this env |

Holistic run at B: <pending — coverage over internal/ on tip vs base>

## Model routing (spec-loop-opencode overlay)
- Implementer: this session (GLM-5.3) + per-task `opencode run --agent build` processes via `scripts/run-task.sh`
- Per-task review legs: `DeepSeek-V4.1-Flash:diff,Qwen3.8-Flash-Next:diff` (different families from implementer)
- Strong legs at R and B: `Qwen3.8-2.4T-A95B-NVFP4:spec` (R), `:spec` + `:adversarial` (B)
- No Claude leg anywhere (`__NO_CLAUDE__` filled in task prompts)

## Triage
| Stage | Finding (one line) | Sev | Models | Disposition (fix / reject / defer) | Where it went / why | Needs user? |
|---|---|---|---|---|---|---|
| R | Capability ids promised but absent from deltas (ERA-3/TSP-2/BEP-3/GTE-4) | CRITICAL | GLM | fix | proposal now names the ids the deltas define | no |
| R | T04 regression fixtures reference Config.Engine that T09 deletes | WARNING | bigQwen r2 | fix | T04/T09 split fixture migration to T09 | no |
| R | Eviction can be undone by an in-flight build re-storing after delete | WARNING | QFN-adversarial+spec | fix | delete-tombstone in D4/BEP-1/T08 | no |
| R | §7 source untraceable from repo | WARNING | QFN r2 | fix | in-repo source-decisions table | no |
| R | engine:cli doc sites beyond the CRD ref left false | WARNING | bigQwen r1 | fix | doc-site list in D5/T09 | no |
| R | persisted engine:cli sink at upgrade unspecified | WARNING | bigQwen r2 | fix | GTE-1 scenario + upgrade note | no |
| R | requestedAt doc row over-broad; preview honesty unpinned | WARNING/NOTE | GLM, bigQwen, QFN | fix | ERA-1 doc clause + preview scenario | no |
| R | T02/T03 red tasks would not compile before implementation | WARNING | QFN-adversarial | fix | scaffolding seams named in T02/T03 | no |
| R | ns/name eviction dead in production; uid fallback for tombstones | NOTE | bigQwen | fix | UID-only + defensive fallback as BEP-1 clause | no |
| R | TSP escape hatch collides with filter-status hatch | WARNING | bigQwen r1 | fix | merged into one write (D3/T07/TSP-1 scenario) | no |
| R | misc: duplicate .gitignore line, tombstone type name, ssh_auth citation | NOTE | bigQwen/QFN | fix | corrected | no |

## Tasks
| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| T01 | CLOSED-WITH-GAPS (3 commits d67309bf, 066038ee, f8f643a5) | r1 2/2 legs (DeepSeek+QFN diff), r2 2/2 (GLM+QFN); all 7 findings verified+fixed; no Claude | gaps: full-suite + mutation deferred to T10; collect -count=2 flake pre-exists at base (owner) |
| T02 | CLOSED-WITH-GAPS (2 commits 31888ae5, eef75b58) | r1 2/2 legs (DeepSeek+QFN diff), REQUEST_CHANGES->fixed; no Claude | CRITICAL verified: scaffolding fields without regen make `task verify` red (exit 201) + CI exposure; T07 owns `make generate manifests` |
| T03 | CLOSED-WITH-GAPS (4 commits f0e82035, 0632218d, 3264913d, 0651939d) | r1 DeepSeek truncated + QFN -> REQUEST_CHANGES (fallback red added); r2 2/2 BLOCK -> all 3 verified+fixed; no Claude | deferred: pre-existing breakerRegistry parallel-test race (owner+harness proposal); gap: cheapest no-error-log sensor named |
| T04 | CLOSED-WITH-GAPS (commit 4161a55d) | test-task review 2 legs (GLM+QFN diff); #1 KEX block-first rejected (reason recorded), #2 skip-guard fixed; no Claude | gaps: full-suite/mutation to T10; gitleaks shim note |
| T04 | CLOSED-WITH-GAPS (1 commit 4161a55d, amended) | r1 2/2 legs (DeepSeek+QFN diff) CONCERNS -> 1 finding rejected with reason (KEX block-first reading), 1 verified scaffolding drift fixed, no round 2; no Claude | gaps: 4 reds green only at T09; routing branch has no outcome-level sensor (rides on T09's diff review); CRD-enum green depends on T09's regen incl. the schema-package golden |
| T03 | CLOSED-WITH-GAPS (3 commits f0e82035, 0632218d, 3264913d) | r1 1/2 legs (DeepSeek truncated 900s, QFN diff) CONCERNS->fixed; r2 2/2 (DeepSeek+QFN diff) BLOCK register->all 3 verified, 2 fixed as reds/guards, 1 deferred; no Claude | gaps: pre-existing breakerRegistry parallel-test race deferred (harness task + owner); log-sensor clause; watch-side Delete-only wiring is T08's |

## Known red
| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|
| internal/controller/requested_at_annotation_test.go (6 tests) | ERA-1 | T01 | |
| internal/collect/prune_collected_generation_test.go + dispatch test (4 tests incl. scrub-survival) | ERA-2 | T01 | |
| internal/controller/cluster_target_status_test.go (4 tests) | TSP-1 | T02 | |
| internal/sink/backend_pool_delete_hook_test.go (4 tests) | BEP-1 | T03 | |
| internal/validation engine_convergence_test.go + internal/sink/git kex test + test/schema engine_enum_test.go (GTE-1/GTE-2 reds) | GTE-1/GTE-2 | T04 | |
| internal/validation/engine_convergence_test.go TestValidateGitSpec_rejectsCLINamingGoGit | GTE-1 | T04 | |
| internal/sink/git/engine_convergence_test.go TestConfigFromSpec_rejectsCLIEngineNamingGoGit | GTE-1 | T04 | |
| test/schema/engine_enum_test.go TestKollectSnapshotSinkGitEngineEnumIsGoGitOnly | GTE-1 | T04 | |
| internal/sink/git/kex_convergence_test.go TestSSHKeyExchangeOffer_carriesModernAlgorithms | GTE-2 | T04 | |

## Test changes
| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons
- (T01) `go test -count>1` is rejected by the Ginkgo suites in internal/controller; the race gate there is -race -count=1 (matching hack/coverage.sh). Do not burn a correction on -count=2 for that package.
- (T01) TestExtractHotPathBudget fails under full-package -race load and TestRecordLabeledMetricSeries_CapsCardinalityDeterministically under -count=2 (the latter at base too); rerun alone before classifying either.
- (T01) Test-local contexts in internal/controller tests: name them `bg`, not `ctx` (govet shadow).
- (T01) Fake client needs .WithStatusSubresource(...) for Status().Update on unregistered objects (controller-runtime v0.24.1).
- (T01) PruneResource never reads Prune.ScrubKeys; the engine merges them into the *Scrubber (engine.go:266). A profile scrub rule reaches the stamp only through the scrubber argument.
- (T02) `task verify` regenerates in place before diffing: as a read-only probe with drift present it dirties the tree (revert or budget the regen commit). T07/T09/T10 run it for real.
- (T02) KUBEBUILDER_ASSETS must be an absolute path for internal/controller Ginkgo envtest suites; a relative path fails BeforeSuite (fork/exec bin/k8s/...).
- (T02) Run gates and record them in evidence BEFORE the review brief; round 1 then found nothing new (gate-runs-before-brief).
- (T03) internal/sink has a pre-existing parallel-test data race on breakerRegistry (ResetBreakersForTest replaces the global map, circuit_breaker.go:67): full-package -race is intermittently red; rerun before classifying; fix is a harness task.
- (T03) A SHALL clause without a scenario needs its own matrix row; reviewers reliably flag it.
- (T03) DeepSeek legs truncating at 900s recurs; when one leg fails, Qwen3.8-Flash-Next alone still produced a verdict both times.
- (T03) internal/sink has a pre-existing parallel-test race on `breakerRegistry` (circuit_breaker.go:67 vs :27): full-package `-race` is intermittently red with bystander failures (~2/5 with T03's tests, 0/4 at base); rerun before classifying; fix is a harness task (Proposed harness changes).
- (T03) Delta SHALL clauses without a `#### Scenario` still need a matrix row + red test: BEP-1's fallback and no-rebuild clauses shipped untested until review rounds forced them (r1 CONCERNS, r2 BLOCK register).
- (T03) A test task's missing red (verified) is a deliverable defect -> CORRECT + one fresh review round; not an "evidence/wording" fix.
- (T04) The gitleaks mise shim fails ("No version is set"); call `~/.local/share/mise/installs/gitleaks/8.30.1/gitleaks` directly for the per-commit fitness row.
- (T04) No outcome-level test distinguishes which branch served a `file://` remote (both paths succeed end-to-end); routing regressions pin outcomes, the branch condition rides on the implementing task's diff review.
- (T04) GTE-2's SHALL body and its scenario admit two orderings readings; task text pinned the scenario (relative-order) reading for T04 — the block-first reading was raised in review and rejected as over-constraining; T09 should not silently re-litigate it.

## Where a human should look first
<!-- filled at hand-off -->

## Proposed harness changes
| Lesson | Seen in | Proposed change |
|---|---|---|
| breakerRegistry parallel-test race in internal/sink | T03 | harness task: clear the map in place under a lock / drop t.Parallel() / per-test registry (owner decision) |
| collect -count=2 cardinality-metric flake + hot-path budget under load | T01, T03 | harness task: make the two tests load-tolerant or add them to the race-exempt list with a reason (owner decision) |
| Pre-existing parallel-test race on `breakerRegistry`: `ResetBreakersForTest` replaces the global map (circuit_breaker.go:67) while parallel export tests read it (circuit_breaker.go:27); full-package `-race` intermittently red (T03: 2/5, base 0/4) | T03 | Owner/harness task: clear the map in place under a lock, or drop `t.Parallel()` at circuit_breaker_test.go:78, or give breaker tests a private registry; otherwise CI and every later task's `-race` gate is a rerun lottery |

## Owner tasks (skipped by the loop)
| Task | Command sheet |
|---|---|
| Helm `mode` decision (D7) | Captain decides (a) delete value+enum / (b) fold tenantMode / (c) narrow enum to ["single"]; reported needs-decision in the status file |
