# Tasks

## 1. Go toolchain (CFC-1, CFC-5)

- [ ] 1.1 Write `hack/test/consistency_go_toolchain_test.sh` with fixtures (equal, newer, older, two Dockerfiles disagree, none found); watch the "older" and "disagree" cases fail on the assertion
- [ ] 1.2 Implement the check; wire into `lint`
- [ ] 1.3 Decide the existing 1.26.6 vs 1.27.1 gap (design question in proposal: it passes CFC-1 as information; the operator may prefer equality) and record the choice in `docs/development/testing.md`

## 2. RBAC (CFC-2, CFC-5)

- [ ] 2.1 Probe `helm template` for default and `tenantMode=true`; record the real current difference between `role.yaml` and the chart
- [ ] 2.2 Write `consistency_rbac_test.sh` with a fixture missing `leases`; watch it fail on the assertion; add the exceptions file with reasons for any genuine difference found in 2.1
- [ ] 2.3 Implement; wire into `lint` (needs the pinned Helm: reuse `hack/install-helm.sh`)

## 3. Metrics (CFC-3, CFC-5)

- [ ] 3.1 Probe histogram suffix use and the literal scan on `internal/metrics`
- [ ] 3.2 Write `consistency_metrics_test.sh` with renamed-metric, undocumented-metric, stale-mirror, suffix and empty-scan cases; watch them fail
- [ ] 3.3 Implement; fix any real drift it finds in its own commit; wire into `lint`

## 4. Chart version (CFC-4, CFC-5)

- [ ] 4.1 Write `consistency_chart_appversion_test.sh` with the mismatch fixture; watch it fail
- [ ] 4.2 Implement; wire into `lint`

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFC-1 | `consistency_go_toolchain_test.sh` fixtures | older and disagree fail, newer passes, empty fails | not-run | |
| CFC-2 | `consistency_rbac_test.sh` with `leases` removed from the chart copy; tenant render | fails naming the triple; tenant mode accounted | not-run | |
| CFC-3 | `consistency_metrics_test.sh` rename, undocumented, stale mirror, suffix, empty | each fails or passes as specified | not-run | |
| CFC-4 | `consistency_chart_appversion_test.sh` mismatch fixture | fails | not-run | |
| CFC-5 | each script's self-test plus no-op copy, judged by exit status | mutants red, no-op green | not-run | |
| all | `lint` job on the PR head | green with all four checks in the log | not-run | |
| all | independent review | APPROVE | not-run | |
