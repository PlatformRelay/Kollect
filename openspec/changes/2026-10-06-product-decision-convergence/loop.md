# Loop — 2026-10-06-product-decision-convergence

Repo: `/Users/kheimel/.treehouse/kollect-79dca7/1/kollect` · Branch: `fm/kollect-product-decisions-impl` · Base: `3ee21266` (= `origin/main`) · Started: 2026-10-06 · Reviewers: fanout (free only)
Next: R spec-set review — fanout.sh on the spec set, free legs only
Budget: claude review legs 0/0 (free-model-only overlay) · active hours 0/8 (session 2026-10-06–) · source: overlay default

## Stages
- [x] 0 orient — OpenSpec repo (no .specify/): change dir `openspec/changes/2026-10-06-product-decision-convergence/` is the feature dir; spec set = proposal.md + design.md + specs/*/spec.md + tasks.md. Spec set committed: `<sha>`
- [ ] P plan + tasks — generated (OpenSpec change); spec set committed
- [ ] R spec-set review — `reviews/R-spec-set/` on <sha> · verdict: <pending>
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

## Tasks
| Task | Verdict | Review (legs, rounds, Claude?) | Gaps / decision request |
|---|---|---|---|
| T01..T10 | <pending> | <pending> | <pending> |

## Known red
| Test (file:name) | Story | Written in | Cleared in |
|---|---|---|---|

## Test changes
| Test (file:name) | Written in | Changed in | Evidence it was wrong |
|---|---|---|---|

## Lessons
<!-- master list; __LESSONS__ renders from it -->

## Where a human should look first
<!-- filled at hand-off -->

## Proposed harness changes
| Lesson | Seen in | Proposed change |
|---|---|---|

## Owner tasks (skipped by the loop)
| Task | Command sheet |
|---|---|
| Helm `mode` decision (D7) | Captain decides (a) delete value+enum / (b) fold tenantMode / (c) narrow enum to ["single"]; reported needs-decision in the status file |
