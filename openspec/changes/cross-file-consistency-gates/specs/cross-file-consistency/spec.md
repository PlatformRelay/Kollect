# Spec Delta

## Purpose

Defines facts that kollect states in more than one file and the gates that fail when those
statements disagree, so drift is found by CI and not by a user or an outage.

## ADDED Requirements

"Manager roles" means the `Role` and `ClusterRole` objects the chart renders for the manager's
service account (`templates/clusterrole.yaml`, `templates/role.yaml` and
`templates/role-leader-election.yaml`); the metrics-reader roles are excluded.

### Requirement: CFC-1 The shipped binary is built with the tested toolchain

The Go version in the `FROM golang:<version>` line of every `Dockerfile*` that builds Go SHALL
equal the `go` directive of `go.mod`. `go.mod` SHALL carry no separate `toolchain` line, and every
Go setup in CI SHALL use `go-version-file: go.mod`. Renovate SHALL move the `go` directive and the
golang image tag in one group.

#### Scenario: Equal

- **WHEN** `go.mod` and every Dockerfile name the same Go version
- **THEN** the check passes

#### Scenario: Image differs from go.mod

- **WHEN** a Dockerfile names a newer or older Go than `go.mod`
- **THEN** the check SHALL fail and name the file and both versions, because tests and govulncheck run the go.mod toolchain and never see the one that ships

#### Scenario: Lone image PR

- **WHEN** Renovate opens a PR bumping only the golang image (group rule missing, mis-ordered after a more general rule, or the manager does not bump the `go` directive)
- **THEN** the check turns red on that PR, and the guard also checks the group rule's presence and order

#### Scenario: Toolchain line or literal

- **WHEN** `go.mod` gains a `toolchain` line or a workflow adds a Go version literal
- **THEN** the check SHALL fail

#### Scenario: No Go image found

- **WHEN** the scan finds no `golang:` line
- **THEN** the check SHALL fail; an empty scan never passes

### Requirement: CFC-2 Pinned tools build with the repository's Go

Every tool pinned through `go run <pkg>@<version>` or `go install` SHALL build under
`GOTOOLCHAIN=local` with the Go of `go.mod`; the check reads each tool's required Go from
`go mod download -json` and fails when it exceeds `go.mod`'s.

#### Scenario: Tool needs a newer Go

- **WHEN** a pinned tool's `go` directive is newer than `go.mod`'s
- **THEN** the check SHALL fail and name the tool, version and both Go versions

#### Scenario: Tool lookup fails

- **WHEN** `go mod download` cannot resolve a pin (network or typo)
- **THEN** the check SHALL fail; an unresolved pin is never a pass

#### Scenario: Transition after a Go bump

- **WHEN** `go.mod`'s Go is raised
- **THEN** every tool pin is rechecked in the same PR

### Requirement: CFC-3 The chart grants at least what the kustomize role grants

Every (apiGroup, resource, verb) triple in `config/rbac/role.yaml` SHALL be granted by the manager
roles rendered from `charts/kollect` with default values, unless listed with a reason in
`hack/test/testdata/consistency-rbac-exceptions.txt`.

#### Scenario: Chart matches

- **WHEN** the rendered roles cover every triple of `role.yaml`
- **THEN** the check passes

#### Scenario: A kubebuilder marker adds a permission the chart lacks

- **WHEN** a controller gains `+kubebuilder:rbac` for a new resource and `make manifests` updates `role.yaml`, but the chart templates are not changed
- **THEN** the check SHALL fail and name the missing triple

#### Scenario: Exception without a reason

- **WHEN** an exception line has no reason text
- **THEN** the check SHALL fail

#### Scenario: Tenant mode

- **WHEN** the chart is rendered with `tenantMode=true`
- **THEN** namespaced triples are covered by the rendered namespaced `Role`; cluster-scoped triples (namespaces, tokenreviews, subjectaccessreviews, `kollectcluster*`) appear as listed exceptions with the reason that tenant mode drops cluster scope

### Requirement: CFC-4 Metric names agree between code, docs and chart

Every `kollect_*` metric name in `docs/operator-manual/metrics.md` and in
`charts/kollect/templates/prometheusrule.yaml` (histogram suffixes `_bucket`, `_sum`, `_count`
stripped) SHALL be registered in the non-test source of `internal/metrics` other than
`metrics_catalog.go`, every registered metric SHALL appear in `metrics.md`, and
`registeredMetricNames` SHALL equal that source set.

#### Scenario: Alert on a renamed metric

- **WHEN** a metric is renamed in code and `prometheusrule.yaml` still queries the old name
- **THEN** the check SHALL fail and name the rule file and the unknown metric

#### Scenario: New metric without docs

- **WHEN** a metric is registered and absent from `metrics.md`
- **THEN** the check SHALL fail and name the metric

#### Scenario: Catalog is not registration

- **WHEN** a name appears only in `metrics_catalog.go`
- **THEN** it does not count as registered

#### Scenario: Mirror list not updated

- **WHEN** a metric is added to `metrics.go` but not to `registeredMetricNames`
- **THEN** the check SHALL fail

#### Scenario: Histogram suffix

- **WHEN** a rule queries `kollect_export_duration_seconds_bucket`
- **THEN** it is accepted as the registered histogram

#### Scenario: Empty scan

- **WHEN** the source scan returns no names
- **THEN** the check SHALL fail

### Requirement: CFC-5 Falling behind the latest Go patch is a red signal

A scheduled workflow SHALL fail when the `go` directive of `go.mod` is behind the latest released
patch of its minor, and SHALL fail when the latest patch cannot be determined.

#### Scenario: Behind

- **WHEN** `go.mod` names patch N and the latest release of that minor is N+1
- **THEN** the scheduled run is red and says so (the CI-failure reporter, once present, files an issue)

#### Scenario: Current

- **WHEN** `go.mod` equals the latest patch
- **THEN** the run is green

#### Scenario: Lookup fails

- **WHEN** the release listing is unreachable or unparsable
- **THEN** the run SHALL fail, not pass

#### Scenario: Newer minor exists

- **WHEN** a newer minor than `go.mod`'s exists
- **THEN** the sensor does not fail on that alone

### Requirement: CFC-6 Each consistency gate is proven able to fail

Each check SHALL contain a self-test that corrupts a copy of the tree to break its invariant and
asserts a non-zero exit, plus a no-op copy that must pass, and SHALL run on `pull_request` in a
required job, plain and `--self-test` as separate steps.

#### Scenario: Mutant survives

- **WHEN** corrupting the copy does not make the check exit non-zero
- **THEN** the self-test SHALL fail
