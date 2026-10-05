# Tasks

## 1. Go toolchain (CFC-1, CFC-4)

- [ ] 1.1 Write `hack/test/consistency_go_toolchain_test.sh` with fixtures (equal, image newer, image older, Dockerfiles disagree, none found); watch the "newer" and "older" cases fail on the assertion
- [ ] 1.2 Implement the check; wire into `lint` (requires the toolchain change to have landed, so the tree is green)

## 2. RBAC (CFC-2, CFC-4)

- [ ] 2.1 Probe `helm template` for default and `tenantMode=true`; record the real current difference between `role.yaml` and the chart
- [ ] 2.2 Write `consistency_rbac_test.sh` with a fixture missing a triple; watch it fail on the assertion; add the exceptions file with reasons for any genuine difference found in 2.1
- [ ] 2.3 Implement; wire into `lint` (needs the pinned Helm: reuse `hack/install-helm.sh`)

## 3. Metrics (CFC-3, CFC-4)

- [ ] 3.1 Probe histogram suffix use and the literal scan on `internal/metrics`
- [ ] 3.2 Write `consistency_metrics_test.sh` with renamed-metric, undocumented-metric, stale-mirror, suffix and empty-scan cases; watch them fail
- [ ] 3.3 Implement; fix any real drift it finds in its own commit; wire into `lint`

## 4. Land

- [ ] 4.1 Archive the change (`openspec archive`) as the last commit of the PR, after review and green CI; the review record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| CFC-1 | `consistency_go_toolchain_test.sh` fixtures | equal passes; newer, older, disagree and empty fail | not-run | |
| CFC-2 | `consistency_rbac_test.sh` with one triple removed from the chart copy; tenant render | fails naming the triple; cluster-scoped tenant exceptions listed with reasons | not-run | |
| CFC-3 | `consistency_metrics_test.sh` rename, undocumented, stale mirror, suffix, empty | each fails or passes as specified | not-run | |
| CFC-4 | each script's self-test plus no-op copy, judged by exit status | mutants red, no-op green | not-run | |
| all | `lint` job on the PR head | green with all three checks in the log | not-run | |
| all | independent review | APPROVE; reviewed and archive revisions recorded | not-run | |
