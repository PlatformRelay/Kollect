# Proposal

## Why

Some facts live in two files by necessity, and nothing compares them. Probed on this tree:

- `go.mod` says `go 1.26.6`; `Dockerfile:2` and `Dockerfile.pipeline:4` build with a newer
  `golang:` image. CI tests and runs govulncheck with the go.mod toolchain (`go-version-file:
  go.mod`; `Taskfile.yml:14` sets `GOTOOLCHAIN` from it), so the shipped binary is built by a
  toolchain no test ran under and govulncheck never scanned. Nothing notices.
- Tools run through `go run <pkg>@<version>` or `go install` (go-arch-lint, govulncheck,
  controller-gen, setup-envtest, kustomize) are built with the repo's Go. A tool whose own
  `go` directive is newer than the repo's Go fails to build under `GOTOOLCHAIN=local`, and a Go
  bump can break them silently.
- Nothing tells the maintainer when `go.mod`'s Go falls behind the latest patch of its minor
  (the patch carries stdlib security fixes that govulncheck then reports as red).
- `config/rbac/role.yaml` (generated) and `charts/kollect/templates/{clusterrole,role}.yaml`
  (hand-written) both grant the manager's permissions. Two regression locks exist for single
  rules (`hack/test/core_events_rbac_test.sh`, `cluster_scope_rbac_test.sh`), written after
  outages; there is no general comparison.
- `docs/operator-manual/metrics.md` and `charts/kollect/templates/prometheusrule.yaml` name
  `kollect_*` metrics; the registered set lives in `internal/metrics/`, mirrored by hand in
  `registeredMetricNames` (`internal/metrics/metrics_catalog_test.go:11`, "update both").

attune checks this kind of invariant in its CI. Only the invariants that exist in kollect are
taken.

## What Changes

- Go: `go.mod` is bumped to the Go version the Dockerfiles ship (`go 1.27.1`, no separate
  `toolchain` line); a guard keeps the Dockerfiles and `go.mod` equal; Renovate groups the `go`
  directive with the golang image so they move in one PR.
- A guard builds each pinned `go run`/`go install` tool under `GOTOOLCHAIN=local` with the go.mod
  Go (reading each tool's Go directive from `go mod download -json`).
- A scheduled sensor goes red when `go.mod`'s Go is behind the latest patch of its minor.
- Guards for RBAC (kustomize role vs rendered chart roles) and metrics (names in docs and chart vs
  names registered in code).

Each guard is a `hack/test/consistency_*_test.sh` with a negative self-test (the pattern of
`hack/test/dev_mise_pin_drift_test.sh`), run in the `lint` job and in `task check`.

## Capabilities

### New Capabilities

- `cross-file-consistency`: facts stated in more than one file that must agree, and the gates
  that fail when they do not.

### Modified Capabilities

None.

## Impact

- Entry points: the `lint` job of `ci.yaml` (a required check after change 1), `task check`, and
  a new scheduled workflow `.github/workflows/go-patch-lag.yaml`.
- The Go bump touches `go.mod` and every module's compile; the PR runs the whole suite.

## Dependencies

Landing order across the ten proposed changes: (1) ci-workflow-hardening, (2) task-check-entrypoint,
(3) golangci-lint-bump, (4) cross-file-consistency-gates, (5) developer-toolchain-pins,
(6) dependency-update-automation, (7) ci-failure-reporting, (8) test-depth-signals,
(9) mutation-testing-signal, (10) public-agent-contract. Needs 1 (guards block merges), 2 (guards
in `task check`) and 3 (a golangci-lint built with an older Go cannot lint a module that declares a
newer one). The sensor's red runs are reported once change 7 lists it.

## Non-goals

- CRD vs chart `crds/`: already gated by `hack/verify.sh:58`.
- `appVersion` vs the artifacthub image tag: `hack/test/dist_artifacthub_chart_test.sh:48-74`.
- `values.yaml` vs `values.schema.json`: the schema is `additionalProperties: true` at the top, so a
  key-by-key gate would be vacuous; helm-docs drift is already gated (`helm-docs:verify`). No
  Helm-values-vs-CRD gate: values configure the operator, not the CRDs.
- Dashboards and alert files other than `prometheusrule.yaml`: none are committed (probe:
  `git ls-files | grep -i 'dashboard\|grafana\|alert'` is empty).

## Assumptions

- Go 1.27.1 builds the module at current dependency versions; probe `go build ./... && go vet
  ./...` and the test suite before the bump (task 1.1).
- `go mod download -json <pkg>@<version>` returns the module's `GoMod` file path, whose `go` line
  gives the tool's required Go. Source: `go help mod download`; probe in task 3.1.
- The latest patch of a Go minor can be read from `https://go.dev/dl/?mode=json`. Probe in
  task 4.1; the sensor needs network and is allowed to fail only by going red, never by passing
  when the lookup fails.
- `helm template charts/kollect` renders the manager roles (CFC-3), with default values and
  `tenantMode=true`. Probe in task 5.1.
- Histogram series appear as `<name>_bucket|_sum|_count` in docs and rules (probe in task 6.1).
- Registered names are readable from Go source: string literals `"kollect_[a-z0-9_]+"` in the
  non-test files of `internal/metrics` EXCLUDING `metrics_catalog.go`, whose `Catalog` is
  documentation, not registration (probe in task 6.1).
