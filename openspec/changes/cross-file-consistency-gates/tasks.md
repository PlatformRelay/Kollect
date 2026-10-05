# Tasks

## 1. Go equality and bump (CFC-1, CFC-6)

- [ ] 1.1 Probe: `go build ./... && go vet ./...` and `task test` under the target Go; probe whether Renovate bumps the `go` directive and which group rule wins (the golang-image group must come after the existing "go minor and patch dependencies" and "docker images" rules); probe CodeQL's Go extractor on the target Go (`codeql.yaml`)
- [ ] 1.2 Write `hack/test/consistency_go_toolchain_test.sh` (plain and `--self-test`) with fixtures (equal, image newer, image older, disagree, toolchain line, literal, none found, group rule missing); watch the newer/older cases fail on the assertion
- [ ] 1.3 Bump `go.mod` to the version the Dockerfiles ship (version-only commit, no `toolchain` line); add the Renovate group; guard passes; `task test`, `task lint`, `task vulncheck` recorded

## 2. Tool pins (CFC-2, CFC-6)

- [ ] 2.1 Write `consistency_tool_pins_test.sh` with a fixture pin whose Go directive is newer and an unresolvable pin; watch them fail; implement through `go mod download -json`

## 3. RBAC (CFC-3, CFC-6)

- [ ] 3.1 Probe `helm template` for default and `tenantMode=true`; record the real current difference
- [ ] 3.2 Write `consistency_rbac_test.sh` with a fixture missing a triple; add the exceptions file with reasons; implement; fix real drift in its own commit

## 4. Sensor (CFC-5)

- [ ] 4.1 Probe `https://go.dev/dl/?mode=json`; write the formatter test (behind, current, lookup failure, newer minor); implement `hack/ci/go-patch-lag.sh` and `.github/workflows/go-patch-lag.yaml` (weekly, `contents: read`); the workflow satisfies the zizmor gate

## 5. Metrics (CFC-4, CFC-6)

- [ ] 5.1 Probe histogram suffix use and the literal scan (excluding `metrics_catalog.go`)
- [ ] 5.2 Write `consistency_metrics_test.sh` with renamed-metric, undocumented, catalog-only, stale-mirror, suffix and empty-scan cases; watch them fail; implement; fix real drift in its own commit

## 6. Wiring and land

- [ ] 6.1 Run all guards in `lint` (plain and `--self-test` as separate steps) and, by the glob, in `task check`
- [ ] 6.2 Merge with the `post-merge` row open; archive in a follow-up evidence PR once it passes

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFC-1 | `consistency_go_toolchain_test.sh` fixtures; real suite under the bumped Go | equal passes; others fail; suite green | not-run | |
| CFC-2 | `consistency_tool_pins_test.sh` fixtures; real pins | newer-Go and unresolved fail; real pins pass | not-run | |
| CFC-3 | `consistency_rbac_test.sh` with one triple removed; tenant render | fails naming the triple; tenant exceptions listed | not-run | |
| CFC-4 | `consistency_metrics_test.sh` cases | each fails or passes as specified | not-run | |
| CFC-5 | formatter tests | behind and lookup failure red | not-run | |
| CFC-5 | first scheduled run | green on current go.mod | post-merge | follow-up evidence PR |
| CFC-6 | each script's self-test plus no-op copy, by exit status | mutants red, no-op green | not-run | |
| all | `lint` job on the PR head; independent review | green, all guards in the log; APPROVE; both revisions recorded | not-run | |
