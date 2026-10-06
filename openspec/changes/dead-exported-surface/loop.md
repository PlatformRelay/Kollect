# loop.md — dead-exported-surface (spec-loop-opencode run state)

- **Repo:** /Users/kheimel/.treehouse/kollect-79dca7/2/kollect (treehouse pool worktree; primary checkout untouched)
- **Branch:** `fm/kollect-docs-review-followup`, base `3ee21266` (default-branch tip at start); spec-set commit `af6c58f5`.
- **Feature:** openspec change `dead-exported-surface` (Kollect has no `.specify/`; the spec set lives in `openspec/changes/dead-exported-surface/`; `skip_specs: true` — no observable behaviour changes).
- **Reviewers:** fanout (free only — the firstmate brief forbids any `claude/*` leg).
- **Implementer model:** `GLM-5.3`; **review pairing:** `DeepSeek-V4.1-Flash:diff` + `Qwen3.8-Flash-Next:diff`; strong gate legs `Qwen3.8-2.4T-A95B-NVFP4:spec` / `:adversarial` (spec-loop-opencode's substitute for the Claude strong leg).
- **Sources:** `data/kollect-xconsol-final/report.md` §4 Sweep 2 (read at /Users/kheimel/Projects/Tools/firstmate/data/kollect-xconsol-final/report.md); every item re-verified at 3ee21266 before the spec set was written.
- **Budget:** `--budget hours=8`, claude=0 (never a Claude leg).

## Stages

| Stage | State |
| --- | --- |
| P plan+tasks | done — spec set committed `af6c58f5` (proposal.md, tasks.md, .openspec.yaml skip_specs) |
| R spec-set review | done — round 1 BLOCK (3 CRITICALs, all code-verified, fixed, commit `92360d72`); round 2 CONCERNS 6/6 legs, no CRITICAL: 8 findings fixed in the spec set (commit below), 1 WARNING rejected with reason (below). Two rounds used; loop continues per the no-CRITICAL gate |
| L task loop | pending |
| B branch review | pending |
| hand-off | pending |

## R round-2 triage

| # | Finding | Disposition |
| --- | --- | --- |
| 1 | HIGH: five deleted `TestRunExportItems_*` tests assert live-path behaviours, only the breaker pair was migrated | fix — 1.3 now records per deleted test which live test/suite covers the behaviour (coverage accounting in evidence) |
| 2 | HIGH: "6 production references" to `RunExportEnvelope` unreproducible | fix — proposal corrected: 3 production call sites (cleanup.go, both inventory controllers); the old count included comment mentions and the runner's own delegation |
| 3 | HIGH: external anchor report not in the repo | fix — the report's Sweep 2 excerpt is quoted in the proposal; the change is self-contained |
| 4 | HIGH: no API-compat note for the constants deletion | fix — commit bodies name the removed exported symbols (changelog is commit-derived); proposal records the pre-1.0 assumption |
| 5 | WARNING: task 6.2 understates RegisterTarget fixture needs | fix — fixture plan named (profile + synthetic object, shared helper, narrowest same-package equivalent where sufficient) |
| 6 | WARNING: `export.go:267` line anchor shifts after 1.2 | fix — symbol anchor |
| 7 | WARNING: `go build ./...` does not compile tests | fix — proposal now cites `go vet ./...` (which compiles test files) as the missed-caller backstop |
| 8 | WARNING: deleting `RunExportItems` orphans `sinkNamespaceForExport` → `unused` linter red | fix — task 1.2 deletes it with the runner (sole caller verified: the dead runner) |
| 9 | WARNING: version-monotonicity invariant unguarded after test deletion | reject — same reason as R1 F7 (scenario unreachable without the dead method; rationale comments stay in code) |

## R round-1 triage (commit 92360d72)

| # | Finding | Disposition |
| --- | --- | --- |
| 1 | CRITICAL: breaker tests drive the dead `RunExportItems`; wholesale deletion loses live `exportThroughBreaker` coverage | fix — task 1.3 migrates both breaker tests to `RunExportEnvelope` keeping trip/reset assertions |
| 2 | CRITICAL: `TestStoreSubscribeAndMarshal` calls `MarshalTargetJSON` (task 3.2 missed it) | fix — added to the adapt list |
| 3 | CRITICAL: integration-tagged `export_integration_test.go` calls `git.Export`; default build skips tagged files | fix — 4.2 lists all three test files; 4.3 keeps tag-on vet as the compile proof |
| 4 | WARNING: alias ref counts understated; migration list must come from the probe | fix — counts dropped, probe-derived lists mandated |
| 5 | WARNING: `api/v1alpha1` constants importable externally, unverified | reject — pre-1.0 module; constants never wired to a status write (no observable surface); zero in-repo refs incl. docs/CRDs; recorded as an accepted assumption |
| 6 | WARNING: "probe is the per-task red" misuses the red rule | fix — reworded to evidence framing in both files |
| 7 | WARNING (single-model): deleting the version-monotonicity test removes the only guard for shard-recreation monotonicity | reject — the guarded scenario is unreachable without the dead method (production `RemoveTarget` never deletes shards); the `bumpNamespaceVersion` rationale comments stay in code for a future re-introduction. Decision: do not keep a white-box test that must fabricate the unreachable state by map surgery |

Logged decision: round-1 CRITICALs were mechanical spec-set fixes (both legs agree, code-verified), so the set was fixed and re-reviewed instead of stopping, per the gate's mechanical-fix rule.

Limits the legs could not check (recorded): the consolidating report lives outside the repo (`data/kollect-xconsol-final/` in the firstmate home — the orchestrator read it directly, legs could not); external module consumers unverifiable in-repo; no leg executes gates (implementation tasks do).

## Fitness functions (inventory at orient)

| Characteristic | Command | Type | Baseline |
| --- | --- | --- | --- |
| Go layering | `.go-arch-lint.yml` via `task arch-lint` | triggered | allow-list |
| Go coverage floor | `task coverage` (internal/, `COVERAGE_MIN=90`) | holistic, per change | ≥90% |
| Static Go analysis | golangci-lint (`make lint`) + Sonar + govulncheck (`task vulncheck`) | holistic | green on main |
| OpenSpec validity | `task spec:validate` (openspec validate --all --strict + config check) | triggered | green |
| Shell hygiene | `task lint:shell` (shellcheck over hack/**) | triggered | no warnings |
| Go unit suite | `task test` (envtest-backed) | holistic | green |

This branch deletes dead Go surface; the Go rows are the load-bearing ones. No allow-list
widening is permitted: if a gate goes red, classify before touching anything.

## Lessons

- kollect has no `.specify/`; the spec set lives in `openspec/changes/<change>/` with
  `skip_specs: true` in `.openspec.yaml` for pure-refactor changes (validation is `task spec:validate`).

## Known red

none

## Test changes

none
