# PR description — cross-file-consistency-gates

**Create at:** <https://github.com/PlatformRelay/Kollect/compare/main...feat/cross-file-consistency-gates>

- **Source:** `feat/cross-file-consistency-gates`
- **Target:** `main`
- **Title:** `:sparkles: feat(ci): cross-file consistency gates and Go 1.27.2 toolchain`
- **Opens as:** ready (operator-authorised; merges by rebase once every check on the head is green)

Paste everything below the line.

---

## TL;DR

- **What:** five `hack/test/consistency_*` / patch-lag guards, each with a `--self-test`, wired into `lint` and `task check`; `go.mod` bumped to Go 1.27.2 with the analysis tools that break under it; a new required job `tool-pins-selftest`; a weekly `go-patch-lag` workflow.
- **Why:** facts that live in two files (Go version, tool pins, chart RBAC vs `role.yaml`, metric names) drifted silently; the shipped binary was built with Go 1.27.2 while tests and govulncheck ran on 1.26.9.
- **Risk:** the new guards are fail-closed and run in required jobs, so a guard bug blocks every PR until fixed.
- **Reviewer action:** check the CI wiring and the RBAC exceptions file (a trust boundary); approving means accepting Go 1.27.2 as the module's Go.

Closes #441.

`go.mod` said `go 1.26.9` while both Dockerfiles built with `golang:1.27.2`, so no test or vulnerability scan ran under the toolchain that produced the shipped image. This PR makes the module's Go equal to the image's, then adds guards so this drift, and three similar ones, fail CI.

## Most important things

- **Go 1.27.2 forces three tool bumps.** Tools embedding `golang.org/x/tools` cannot analyse a newer Go: golangci-lint v2.13.1 → v2.14.0, go-arch-lint v1.15.0 → v1.19.0, govulncheck v1.1.4 → v1.9.0. The go-arch-lint bump was the same failure, version-only, and was not part of the operator's original ruling.
- **New required job `tool-pins-selftest`** (45 min timeout, listed in `hack/release/verify-eligibility.sh`). It is separate from `lint` because the tool-pins self-test builds every pinned tool and takes about 5 min locally. It has to be added to the `protect-main` ruleset **after** this merges, because before the merge no PR on main can report it.
- **CFC-1 fails on any `$` in a `FROM` image.** No variable image is parsed around, `golang:` included. A comment line inside a `FROM \` continuation is skipped the way BuildKit skips it.
- **CFC-3 RBAC exceptions are a closed list.** `hack/test/testdata/consistency-rbac-exceptions.txt` holds 37 tenant-mode exceptions, each mode-scoped with a closed reason class. Workload reads are exempt per team-operator.md §3 (operator ruling A, 2026-10-15). There is no drift in default mode.
- **Real drift fixed:** five registered metrics were missing from `docs/operator-manual/metrics.md` (docs-only commit).

## Risk

- **What can break:** a false red in a guard blocks every PR, because `lint` and `tool-pins-selftest` are required. `tool-pins-selftest` downloads modules and the vuln DB, so a network stall shows up as a timeout. `go-patch-lag` turns red on the first Monday after a Go patch release, which is intended.
- **Blast radius:** CI gating and the build toolchain. Product code changes only in `internal/metrics/metrics.go` (registration made testable) and its tests; the binary now builds and is tested on 1.27.2 everywhere.
- **Rollback:** revert the PR. Reverting before the ruleset step is clean. After it, also remove `tool-pins-selftest` from protect-main (ruleset 18802410), or every PR waits on a check that never runs.
- **Residual risk accepted:** BACKLOG items CFC1-REGISTRY-PREFIX (`localhost:5000/golang:x` is not recognised as golang), a heredoc `FROM ${B}` false red (fails closed), and collectors whose names are built by concatenation or Sprintf being invisible to the metrics literal scan (there are none today).

## Files to look at first

- `.github/workflows/ci.yaml`: 10 new lint steps plus the new job `tool-pins-selftest`.
- `hack/test/ci_workflow_security_test.sh`: new CFC-6 rule that fails on an unwired guard. This changes what green means for CI wiring.
- `hack/test/testdata/consistency-rbac-exceptions.txt`: 37 tenant exceptions (trust boundary: RBAC).
- `.github/workflows/go-patch-lag.yaml` + `hack/ci/go-patch-lag.sh`: new scheduled sensor with network access, `contents: read`, fails closed.
- `internal/metrics/metrics_test.go`: `TestRegister` now uses `registerOnce`, because a second `Register` panics.
- `hack/test/task_check_test.sh`: one mutant re-anchored.
- `hack/test/consistency_go_toolchain_test.sh`: the `golang:${GO_VERSION}` mutant sentinel changed.
- `go.mod`, `Dockerfile*`, `.devcontainer/devcontainer.json`, `Makefile`, `Taskfile.yml`, `hack/tooling/.custom-gcl.yml`, `renovate.json`: the Go 1.27.2 bump, tool pins, and a Renovate group so the `go` directive and the golang images move in one PR.

**Review this commit-by-commit, not as one flattened diff.** Each guard landed test-first with its review fixes as separate commits, and the 99-file diff is mostly OpenSpec evidence under `openspec/changes/`.

## Testing protocol

- [x] Locally: `task lint`, `task test` (under 1.27.2), `task vulncheck`, `task spec:validate`, `task lint:markdown`, each guard plain and with `--self-test`.
- [x] Scrub grep and `gitleaks detect` over `origin/main..HEAD` clean.
- [ ] Every check on the PR head SHA is green, including `lint` (all ten new steps appear in its log) and `tool-pins-selftest` (10-20 min expected).
- [ ] Rebase-merge: `gh pr merge <n> --rebase --delete-branch`.
- [ ] Right after merge: add `tool-pins-selftest` (integration_id 15368) to ruleset 18802410, then read it back and diff against the backup. Only that context may differ.
- [ ] `gh workflow run go-patch-lag.yaml --ref main`; it must finish `success` while go.mod is on the current 1.27 patch. A red run with "lookup failed" means go.dev was unreachable, not lag.
- [ ] Main's post-merge CI, filtered on the merge headSha, is green.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the deployment fine print. Skip it if the top half already told you what you needed.*

**The six requirements.** CFC-1 Go equality (go.mod, Dockerfiles, devcontainer, Renovate group). CFC-2 tool pins: each `go run`/`go install` pin builds under exactly go.mod's Go (`GOTOOLCHAIN=go<version>`), and each analysis tool runs once under it. CFC-3 chart RBAC covers `config/rbac/role.yaml` in default and tenant mode. CFC-4 registered metric names match the docs and `prometheusrule.yaml`, and a Go test compares the live registry with `registeredMetricNames`. CFC-5 is the weekly patch-lag sensor. CFC-6 requires every guard to have a negative self-test and to be wired in CI.

**Review history.** Spec set reviewed before #437. Tasks G1-G8 each had independent review legs (two rounds where applicable). The branch review ran two rounds; round 2 findings C1 and C2 were fixed in G8 with mutants, and the rest were deferred to BACKLOG. Full records are in `openspec/changes/cross-file-consistency-gates/loop.md`, `reviews/` and `evidence/G1.md`-`G8.md`.

**Rejected alternatives.** Running the tool-pins self-test inside `lint` would add about 5 min to the job every PR waits on. A `toolchain` line in `go.mod` was rejected because the equality guard is simpler when there is one version. A key-by-key `values.yaml` vs schema gate would be vacuous (`additionalProperties: true`).

**Out of scope here.** Archiving the change waits for a follow-up evidence PR once the first *scheduled* `go-patch-lag` run exists (task 6.2). Reporting go-patch-lag red runs belongs to change 7.
