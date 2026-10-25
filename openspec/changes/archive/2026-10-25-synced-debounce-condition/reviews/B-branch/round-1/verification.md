# Stage B round 1 — verification of the findings

Branch `feat/synced-debounce-condition` rebased on `origin/main` @ `cf448c4a6`; reviewed head
`df2c9e501`. Legs: reviewer-a (adversarial, brief `brief-reviewer-a.md`), reviewer-b (spec
conformance against SDC-1..7). Verdicts: reviewer-a REQUEST_CHANGES, reviewer-b CONCERNS.
Every finding below was checked against the code by the orchestrator on 2026-10-25 (verified).

## Holistic gates on `df2c9e501` (before the review)

| Gate | Command | Exit | Result |
|---|---|---|---|
| Full local gate | `task check` | 1 | 4 gates red, none from this branch: `vulncheck` (GO-2026-6809 in the Go standard library, red on `main` too; toolchain bump in its own pull request); `gitleaks-detect` (a hit in the git-ignored mkdocs cache `.cache/plugin/social/manifest.json`; the same scan with `.cache` moved aside: exit 0, no leaks); `dist_olm_bundle_test.sh` and `dist_operatorhub_pr_test.sh` (no `operator-sdk` on the host; after `make operator-sdk` both exit 0). Every other gate ok, incl. `verify`, `lint`, `test`, `scrub`, `spec:validate`, `build`, `helm` |
| Integration build | `go vet -tags=integration ./...` | 0 | clean |
| Per-commit gate | each of the 10 commits in a temp worktree: `go build ./...`, `go test -count=1 ./internal/controller/ ./internal/inventory/` | 0 | all build; `aa39da6c5` red once while `task check` ran in parallel, then green in three reruns alone (load, not the commit) |

## Findings

| # | Leg | Sev | Finding | Verified? | Disposition |
|---|---|---|---|---|---|
| A1 | reviewer-a | WARNING | An export succeeds, `record` stores the new baseline, the status write conflicts; the next pass debounces against that baseline and carries the old persisted T1/C1 forward. With a `0s` interval the entry stays stale as long as the payload does not change; both kinds | yes: `record` at `kollectinventory_controller.go:446` and `kollectclusterinventory_controller.go:364` runs before `Status().Update` in `updateStatus`, which returns on a conflict without touching the tracker; red test below | fix (B1): `forgetUnpersistedExports` on a failed status write; SDC-5 sentence and scenario |
| A2 | reviewer-a | WARNING | The `BREAKING CHANGE:` footer of the G1 commit omits the read API change and the per-sink message change | yes: the footer names only the alerts and the aggregate and cluster messages | fix: footer reworded to name both (commit message only) |
| B1 | reviewer-b | WARNING | The spec does not carry the proposal's non-goal: a pass that stops before exporting leaves per-sink entries as they were | partly: SDC-3 already says "in an export pass", but the carve-out is only in `proposal.md` *Non-goals* | fix: carve-out sentence in SDC-3 |
| B2 | reviewer-b | NOTE | `docs/REQUIREMENTS.md:89` FR-EXP-9 pairs "debounced pass" with `PartiallySynced`, a hit the SDC-7 scenario does not allow | yes | fix: "in one pass"; the scenario search now finds only the two allowed notes |
| B3 | reviewer-b | NOTE | SDC-7 names "v0.22.0 behaviour changes"; the entry is under *Smaller behaviour changes (after v0.21.0)* | yes | fix: SDC-7 names that section |
| B4 | reviewer-b | NOTE | The aggregate message's denominator is exported+debounced, not the bound-sink count | yes, but every binding ends exported, debounced or failed on every branch today | reject: no reachable defect; a future branch that drops a binding is a review matter |
| B5 | reviewer-b | NOTE | The read API rebuilds the entry key instead of sharing `sinkExportKey` | yes | defer: backlog SINK-KEY-SHARED-HELPER (pull request description) |
| B6 | reviewer-b | NOTE | A matched entry with no `Synced` condition keeps the inventory-level status | yes, but no write path produces such an entry | reject: unreachable |

## B1 fix — verification matrix

| Claim | Sensor | Class | Result |
|---|---|---|---|
| A failed status write after an export makes the next pass export again, both kinds | `go test -count=1 -v -run StatusWriteFailureAfterExport ./internal/controller/` | machine | red before the fix on the target line: `per_sink_export_persist_test.go:117: second pass = 0 exported / 1 debounced, want 1/0` and `:189` the same (cluster); controls green. Green after |
| A successful status write still debounces (control) | same tests, case `status write succeeds` | machine | green before and after |
| Each line of the fix is load-bearing | hand mutants in a copy outside the repository, judged by the test's exit and its own failure line | machine | M1 drop namespaced `forgetUnpersistedExports`: killed (`:117`); M2 drop cluster call: killed (`:189`); M3 drop namespaced `markExported`: killed; M4 drop cluster `markExported`: killed; M5 `forget` no-op in the helper: killed (both); M0 comment-only control: survived |
| No other test changed | `git diff` on `*_test.go`: one new file only | machine | yes |
| Package tests | `go test -count=1 ./internal/controller/ ./internal/inventory/` | machine | ok, ok |
| Race | `go test -race -count=3 -skip TestControllers ./internal/controller/ ./internal/inventory/`; envtest once with `-race` | machine | unit: only the four reconciler tests of the known shared-breaker flake (RACE-COUNT-BREAKER-ISOLATION) fail, 0 `DATA RACE`; envtest ok |
| Lint | `task lint` | machine | 0 |
| Spec | `task spec:validate` | machine | 19 passed |
