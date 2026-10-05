# Spec Delta

## Purpose

Defines the facts that kollect states in more than one file and the gates that fail when those
statements disagree, so drift is found by CI and not by a user or an outage.

## ADDED Requirements

### Requirement: CFC-1 The shipped binary is built with a toolchain no older than the tested one

The Go version in the `FROM golang:<version>` line of every `Dockerfile*` SHALL be greater than or
equal to the `go` directive of `go.mod`, and all `Dockerfile*` that build Go SHALL use the same tag.

#### Scenario: Image newer than go.mod

- **WHEN** `go.mod` says `go 1.26.6` and the Dockerfiles say `golang:1.27.1`
- **THEN** the check passes, and its output states the difference as information

#### Scenario: Image older than go.mod

- **WHEN** `go.mod` is bumped to `go 1.27.2` and a Dockerfile still says `golang:1.27.1`
- **THEN** the check SHALL fail and name the file and both versions

#### Scenario: Two Dockerfiles disagree

- **WHEN** `Dockerfile` and `Dockerfile.pipeline` use different Go tags
- **THEN** the check SHALL fail

#### Scenario: No Go image found

- **WHEN** the scan finds no `golang:` line
- **THEN** the check SHALL fail; an empty scan never passes

### Requirement: CFC-2 The chart grants at least what the kustomize role grants

Every (apiGroup, resource, verb) triple in `config/rbac/role.yaml` SHALL be granted by the manager
roles rendered from `charts/kollect` with default values, unless listed with a reason in
`hack/test/testdata/consistency-rbac-exceptions.txt`.

#### Scenario: Chart matches

- **WHEN** the rendered roles cover every triple of `role.yaml`
- **THEN** the check passes

#### Scenario: A kubebuilder marker adds a permission the chart lacks

- **WHEN** a controller gains `+kubebuilder:rbac` for `coordination.k8s.io/leases` and `make manifests` updates `role.yaml`, but the chart template is not changed
- **THEN** the check SHALL fail and name the missing triple

#### Scenario: Exception without a reason

- **WHEN** an exception line has no reason text
- **THEN** the check SHALL fail

#### Scenario: Tenant mode

- **WHEN** the chart is rendered with `tenantMode=true`
- **THEN** the union of the rendered `Role` and `ClusterRole` still covers every triple, or the tenant-mode difference is a listed exception with a reason

### Requirement: CFC-3 Metric names agree between code, docs and chart

Every `kollect_*` metric name in `docs/operator-manual/metrics.md` and in
`charts/kollect/templates/prometheusrule.yaml` (histogram suffixes `_bucket`, `_sum`, `_count`
stripped) SHALL be registered in `internal/metrics`, every registered metric SHALL appear in
`metrics.md`, and `registeredMetricNames` SHALL equal the set found in the source.

#### Scenario: Alert on a renamed metric

- **WHEN** a metric is renamed in code and `prometheusrule.yaml` still queries the old name
- **THEN** the check SHALL fail and name the rule file and the unknown metric

#### Scenario: New metric without docs

- **WHEN** a metric is registered and absent from `metrics.md`
- **THEN** the check SHALL fail and name the metric

#### Scenario: Mirror list not updated

- **WHEN** a metric is added to `metrics.go` but not to `registeredMetricNames`
- **THEN** the check SHALL fail

#### Scenario: Histogram suffix

- **WHEN** a rule queries `kollect_export_duration_seconds_bucket`
- **THEN** it is accepted as the registered histogram `kollect_export_duration_seconds`

#### Scenario: Empty scan

- **WHEN** the source scan returns no names
- **THEN** the check SHALL fail

### Requirement: CFC-4 Chart appVersion and the image annotation agree

`appVersion` in `charts/kollect/Chart.yaml` SHALL equal the tag, without the leading `v`, of the
image in the `artifacthub.io/images` annotation.

#### Scenario: Version bumped in one place

- **WHEN** `appVersion` is `0.22.0` and the annotation says `v0.21.0`
- **THEN** the check SHALL fail

### Requirement: CFC-5 Each consistency gate is proven able to fail

Each check SHALL contain a self-test that corrupts a copy of the tree to break its invariant and
asserts a non-zero exit, plus a no-op copy that must pass.

#### Scenario: Mutant survives

- **WHEN** corrupting the copy does not make the check exit non-zero
- **THEN** the self-test SHALL fail
