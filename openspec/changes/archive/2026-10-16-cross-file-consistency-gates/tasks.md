# Tasks

## 1. Go equality and bump (CFC-1, CFC-6)

- [x] 1.1 Probe: `go build ./... && go vet ./...` and `task test` under the target Go; probe whether Renovate bumps the `go` directive and which group rule wins (the golang-image group must come after the existing "go minor and patch dependencies" and "docker images" rules); probe CodeQL's Go extractor on the target Go (`codeql.yaml`) — closed 2026-10-15, evidence: evidence/G1.md
- [x] 1.2 Write `hack/test/consistency_go_toolchain_test.sh` (plain and `--self-test`) with fixtures (equal, image newer, image older, devcontainer differs, disagree, toolchain line, literal, none found, group rule missing); watch the newer/older cases fail on the assertion — closed 2026-10-15, evidence: evidence/G1.md
- [x] 1.3 Bump `go.mod` (and the devcontainer image) to the version the Dockerfiles ship (version-only commit, no `toolchain` line); add the Renovate group; guard passes; `task test`, `task lint`, `task vulncheck` recorded — closed 2026-10-15, evidence: evidence/G1.md

## 2. Tool pins (CFC-2, CFC-6)

- [x] 2.1 Write `consistency_tool_pins_test.sh` with a fixture pin whose Go directive is newer and an unresolvable pin; watch them fail; implement through `go mod download -json`; add the analysis-tool smoke run (golangci-lint, govulncheck, go-arch-lint under go.mod's Go) with a fixture that pins an analysis tool too old for go.mod's Go (e.g. golangci-lint v2.13.1 vs go 1.27.2) and watch it fail — closed 2026-10-15, evidence: evidence/G2.md

## 3. RBAC (CFC-3, CFC-6)

- [x] 3.1 Probe `helm template` for default and `tenantMode=true`; record the real current difference — closed 2026-10-15, evidence: evidence/G3.md
- [x] 3.2 Write `consistency_rbac_test.sh` with a fixture missing a triple; add the exceptions file with reasons; implement; fix real drift in its own commit — closed 2026-10-15, evidence: evidence/G3.md

## 4. Sensor (CFC-5)

- [x] 4.1 Probe `https://go.dev/dl/?mode=json`; write the formatter test (behind, current, lookup failure, newer minor); implement `hack/ci/go-patch-lag.sh` and `.github/workflows/go-patch-lag.yaml` (weekly, `contents: read`); the workflow satisfies the zizmor gate — closed 2026-10-15, evidence: evidence/G4.md

## 5. Metrics (CFC-4, CFC-6)

- [x] 5.1 Probe histogram suffix use and the literal scan (excluding `metrics_catalog.go`) — closed 2026-10-15, evidence: evidence/G5.md
- [x] 5.2 Write `consistency_metrics_test.sh` with renamed-metric, undocumented, catalog-only, stale-mirror, suffix and empty-scan cases; watch them fail; implement; fix real drift in its own commit — closed 2026-10-15, evidence: evidence/G5.md

## 6. Wiring and land

- [x] 6.1 Run all guards in `lint` (plain and `--self-test` as separate steps) and, by the glob, in `task check` — closed 2026-10-15, evidence: evidence/G6.md
- [x] 6.2 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes — closed 2026-10-16: merged in PR #463 (merge 4e8c7b40d); post-merge rows filled below; archived in this follow-up PR

## 7. Stage-B fixes (round 1)

- [x] 7.1 Fix markdownlint (MD032, MD038) in this change's loop.md, task-prompt.md and evidence/*.md so `task lint:markdown` is green (H1) — closed 2026-10-16, evidence: evidence/G7.md
- [x] 7.2 CFC-1: fail on a `FROM` whose image is a variable (or resolve it from its `ARG` default); fixture + mutant (A1) — closed 2026-10-16, evidence: evidence/G7.md
- [x] 7.3 CFC-4: add a Go test that gathers the metrics registry and compares the registered names with `registeredMetricNames`, so dropping a collector from `MustRegister` goes red; mutant (A2) — closed 2026-10-16, evidence: evidence/G7.md
- [x] 7.4 consistency_go_toolchain_test.sh: call `new_copy` in the parent shell so the EXIT trap removes every copy (no temp-dir leak) (A4) — closed 2026-10-16, evidence: evidence/G7.md
- [x] 7.5 consistency_rbac_test.sh: reject a malformed exception line (class token without colon) with a clear FAIL instead of a Python traceback (O3) — closed 2026-10-16, evidence: evidence/G7.md

## 8. Stage-B fixes (round 2, no further review)

- [x] 8.1 CFC-1: no `golang:` exemption from the variable-image rule; `golang:1.27.2@${D}` and `golang:1.27.2-${X}` fail; mutants (C1) — closed 2026-10-16, evidence: evidence/G8.md
- [x] 8.2 CFC-1: ignore comment lines inside a `FROM \` continuation so the image is still parsed; mutant with a literal golang:1.99.0 (C2) — closed 2026-10-16, evidence: evidence/G8.md
- [x] 8.3 Fill the Verification table statuses from the evidence (6.2 / post-merge rows stay open) — closed 2026-10-16, evidence: evidence/G8.md

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFC-1 | `consistency_go_toolchain_test.sh` fixtures; real suite under the bumped Go | equal passes; others fail; suite green | pass | evidence/G1.md (fixtures, `task test` under 1.27.2), G7.md, G8.md (variable and continuation mutants) |
| CFC-2 | `consistency_tool_pins_test.sh` fixtures; real pins | newer-Go, too-old analysis tool and unresolved fail; real pins pass | pass | evidence/G2.md |
| CFC-3 | `consistency_rbac_test.sh` with one triple removed; tenant render | fails naming the triple; tenant exceptions listed | pass | evidence/G3.md (self-test `triple removed`; tenant probe), G7.md |
| CFC-4 | `consistency_metrics_test.sh` cases | each fails or passes as specified | pass | evidence/G5.md, G7.md (registry test) |
| CFC-5 | formatter tests | behind and lookup failure red | pass | evidence/G4.md |
| CFC-5 | first scheduled run | green on current go.mod | pass | workflow_dispatch run 38833372718 on main@4e8c7b40d: success. The first scheduled weekly run has not happened yet; the dispatched run exercises the same workflow and script |
| CFC-6 | each script's self-test plus no-op copy, by exit status | mutants red, no-op green | pass | evidence/G1.md–G5.md (per guard), G6.md (CI wiring), G7.md, G8.md |
| all | `lint` job on the PR head; independent review | green, all guards in the log; APPROVE; both revisions recorded | pass | PR #463 head 361a00e6e: all 44 non-skipped checks green, including lint, tool-pins-selftest and SonarCloud; main post-merge CI by headSha 4e8c7b40d: all workflows success; reviews in loop.md (G1–G9, B rounds 1–2) |
