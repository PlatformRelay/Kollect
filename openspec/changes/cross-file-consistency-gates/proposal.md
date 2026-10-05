# Proposal

## Why

Some facts live in two files by necessity, and nothing compares them. Probed on this tree:

- `go.mod` says `go 1.26.6`; `Dockerfile:2` and `Dockerfile.pipeline:4` build with
  `golang:1.27.1`. CI tests and runs govulncheck with the go.mod toolchain
  (`go-version-file: go.mod`; `Taskfile.yml:14` sets `GOTOOLCHAIN` from it), so the shipped binary
  is built by a toolchain that no test ran under and that govulncheck never scanned. Nobody
  decided that; nothing notices.
- `config/rbac/role.yaml` (generated) and `charts/kollect/templates/{clusterrole,role}.yaml`
  (hand-written) both grant the manager's permissions. Two regression locks exist for single
  rules (`hack/test/core_events_rbac_test.sh`, `cluster_scope_rbac_test.sh`), written after
  outages; there is no general comparison.
- `docs/operator-manual/metrics.md` (32 mentions) and `charts/kollect/templates/prometheusrule.yaml`
  (6) name `kollect_*` metrics; the registered set lives in `internal/metrics/metrics.go`, mirrored
  by hand in `registeredMetricNames` (`internal/metrics/metrics_catalog_test.go:11`, "update both").

attune checks this kind of invariant in its CI (cross-file consistency gates). Only the
invariants that exist in kollect are taken.

## What Changes

Three checks, each a `hack/test/consistency_*_test.sh` with a negative self-test on a throwaway
copy (the pattern of `hack/test/dev_mise_pin_drift_test.sh`), run in the `lint` job:

1. Go toolchain: Dockerfiles and `go.mod` use the same Go version.
2. RBAC: kustomize role vs rendered chart roles.
3. Metrics: names in docs and chart vs names registered in code.

## Capabilities

### New Capabilities

- `cross-file-consistency`: facts that are stated in more than one file and must agree, and the
  gate that fails when they do not.

### Modified Capabilities

None.

## Impact

- Entry point: the `lint` job of `.github/workflows/ci.yaml` (the same place as
  `ci_docs_gate_test.sh`), reached locally as `bash hack/test/consistency_<name>_test.sh`.
- The existing 1.26.6 vs 1.27.1 mismatch is resolved by `developer-toolchain-consistency` (it bumps `go.mod` to 1.27.1 and groups the `go` directive with the golang image in Renovate), which lands first; this change's gate is then green on arrival.

## Dependencies

Landing order across the seven proposed changes: developer-toolchain-consistency (with its Go
bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation,
nightly-failure-reporting, test-depth-signals, public-agent-contract.

## Non-goals

- CRD vs chart `crds/`: already gated by `hack/verify.sh:58` (`helm-sync-crds.sh` drift).
- `values.yaml` vs `values.schema.json` vs `docs/operator-manual/helm-values.md`: the schema is
  `additionalProperties: true` at the top, so a key-by-key gate would be vacuous, and the
  helm-docs drift gate (`helm-docs:verify`) already covers the docs. No Helm-values-vs-CRD gate:
  values configure the operator, not the CRDs.
- Dashboards and alert files other than `prometheusrule.yaml`: none are committed (probe:
  `git ls-files | grep -i 'dashboard\|grafana\|alert'` is empty; the word Grafana appears only
  in docs and `hack/kind` scripts).
- `appVersion` vs the artifacthub image tag: already enforced by `hack/test/dist_artifacthub_chart_test.sh:48-74` (run in `lint`'s dist_* loop). Not duplicated.
- Making the Dockerfile and `go.mod` use one mechanism (a build arg). Possible later.

## Assumptions

- `helm template charts/kollect` renders the manager roles (see CFC-2), with the default
  values and with `tenantMode=true`. Probe in task 2.1 (`hack/install-helm.sh` pins Helm).
- Histogram series appear as `<name>_bucket|_sum|_count` in docs and rules; the metric check must
  strip them. Probe: grep `_bucket` in `metrics.md` and `prometheusrule.yaml` in task 3.1.
- Registered names can be read from the Go source of `internal/metrics` (string literals matching
  `"kollect_[a-z0-9_]+"`) without running the operator. Probe: `metrics.go`, `aggregation*.go`.
- The existing `registeredMetricNames` mirror stays; the new check asserts the mirror equals the
  source scan, so "update both" becomes enforced.
