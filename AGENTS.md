# AGENTS.md

Kollect turns Kubernetes state into durable inventory: you declare what matters once, and Kollect
keeps it current and delivers the same canonical rows to Git, object storage, databases and event
streams. This file is the working contract for AI coding assistants and new contributors: the
commands, the gates, what CI enforces and where else to change when you add something.
[CONTRIBUTING.md](CONTRIBUTING.md) owns the process.

## Commands

| Command | What it does |
| --- | --- |
| `task build` | Build the manager binary |
| `task test` | Run unit tests with envtest |
| `task lint` | Run golangci-lint v2 and go-arch-lint |
| `task verify` | Fail if generated artifacts (manifests, deepcopy, RBAC) are stale |
| `task check` | Run the full local gate (see below) |
| `task scrub` | Fail if staged files contain forbidden strings |
| `task lint:markdown` | Lint tracked Markdown source with markdownlint-cli2 |
| `task spec:validate` | Validate `openspec/config.yaml`, every OpenSpec change and capability spec (strict) |
| `task lint:shell` | Lint shell scripts with ShellCheck |
| `task helm-test` | Lint the chart, verify helm-docs drift, run helm-unittest |
| `task test-integration` | Run integration-tagged tests; fails unless every one of them ran and passed (needs Docker) |

## The full local gate

`task check` runs `hack/check.sh`. It runs every required CI gate that can run on a developer
machine, reports each failed gate, and exits non-zero if any failed. The gates, in order:

<!-- gates:start -->
- `verify`
- `format:check`
- `lint:shell`
- `lint:markdown`
- `lint`
- `vulncheck`
- `test`
- `scrub`
- `spec:validate`
- `audit-rbac`
- `build`
- `helm`
- `go-mod`
- `gitleaks`
- `gitleaks-detect`
- `workflow-security`
- `workflow-security-audit`
<!-- gates:end -->

After the gates it runs the guard sweep: every `hack/test/*_test.sh` and
`hack/test/lab_harness_meta_suite.sh`, plain, plus `--self-test` for each guard whose code
mentions that flag. A guard whose header declares that it requires Docker is skipped with a
printed reason. Last, it prints an exclusion table of the required gates it does not run locally,
each with its reason: test-integration, kind-smoke, docker-build, pipeline-cli-smoke and
dependency-review.

## CI-enforced rules

A release is eligible only when these check runs are green on its exact commit
(`required_checks` in `hack/release/verify-eligibility.sh`):

- `ci.yaml`
  - `gitleaks`: installs gitleaks with `hack/install-gitleaks.sh`, then
    `gitleaks detect --source . --config .github/gitleaks.toml --verbose --redact --no-git`.
  - `workflow-security`: installs zizmor with `hack/install-zizmor.sh` and audits `.github/`
    offline at high severity.
  - `verify`: `task verify`.
  - `audit-rbac`: `bash hack/audit-rbac.sh` (Polaris and kubeaudit).
  - `vulncheck`: `task vulncheck` (govulncheck).
  - `lint`: `task format:check`, `task lint:shell`, the `hack/test` guards (plain, and with
    `--self-test` for most guards that have one), then `task lint`.
  - `tool-pins-selftest`: the `--self-test` of `hack/test/consistency_tool_pins_test.sh` and of
    `hack/test/consistency_toolchain_pins_test.sh`.
  - `test`: a report job; on a change outside the documentation paths it fails unless the
    `test-suite` job (`task coverage`, unit tests with the coverage floor) succeeded.
  - `build`: `task build`.
  - `test-integration`: `task test-integration` (testcontainers).
  - `helm`: installs Helm and the helm-unittest plugin, then `task helm-test`.
  - `docker-build`: `task docker:build`, `task docker:build:pipeline`, and a smoke run of the
    pipeline image, which must run as a non-root user.
- `preflight.yaml`
  - `preflight`: on pull requests, the commit identity check (`hack/check-commit-identity.sh`);
    then `task lint:markdown`, `go mod tidy` with no drift in `go.sum`, `go mod verify` and
    `task verify`.
- `e2e-smoke.yaml`
  - `kind-smoke`: a report job; on a change outside the documentation paths it fails unless
    `kind-smoke-run` (the kind smoke scenario `hack/kind/e2e/smoke.sh`) succeeded.
  - `pipeline-cli-smoke`: the pipeline CLI scenario `hack/kind/e2e/pipeline-cli-smoke.sh` on
    kind.

## Conventions

- Commits: Conventional Commits with an optional gitmoji shortcode, merged with rebase and merge;
  see [CONTRIBUTING.md](CONTRIBUTING.md).
- Behavioural changes are specified and verified with OpenSpec; see
  [docs/development/spec-workflow.md](docs/development/spec-workflow.md).

## Checklists

### When adding a CRD field

- Add the field to the spec type, for example `KollectProfileSpec` in `api/v1alpha1/kollectprofile_types.go`.
- Regenerate with `make generate manifests`, then `bash hack/helm-sync-crds.sh`; this updates `config/crd/bases/` and `charts/kollect/crds/`.
- Run `python3 hack/gen-glossary.py` to refresh the generated CRD block in `docs/GLOSSARY.md` (`hack/test/glossary_drift_test.sh` checks it).
- Update the schema golden with `UPDATE_GOLDEN=1 go test ./test/schema/ -run TestCRDSpecOpenAPIFragments`; it rewrites `test/schema/golden/<kind>.spec.openapi.yaml`, which `task verify` checks.
- Document the field on its CR page, `docs/crds/<kind>.md`, and in `docs/crds/index.md`.
- Finish with `task verify` green.

### When adding a metric

- Declare the collector in `internal/metrics/metrics.go` and add it to `registerTo` there.
- Add an entry to `Catalog` in `internal/metrics/metrics_catalog.go`; the catalog test requires every registered metric there.
- Add the name to `registeredMetricNames` in `internal/metrics/metrics_catalog_test.go` (`TestRegisterMatchesRegisteredMetricNames` fails without it).
- Add a row to `docs/operator-manual/metrics.md`; `hack/test/consistency_metrics_test.sh` checks it.
- Only when the metric should alert: add a rule to `charts/kollect/templates/prometheusrule.yaml`; the same guard checks its metric name.
- Finish with `go test ./internal/metrics/` and `bash hack/test/consistency_metrics_test.sh` green.

### When adding a sink

- Create the package `internal/sink/<name>` with its type name and backend.
- Register its factory in `NewRegistry` in `internal/sink/registry.go`.
- Add exactly one row for the type to the primary matrix of `docs/operator-manual/lab-backend-fidelity.md` (`hack/test/docs_lab_doc_04_fidelity_test.sh` checks it).
- Accept the type in a family CRD enum, for example `KollectEventSinkSpec` in `api/v1alpha1/kollecteventsink_types.go`, regenerate as for a CRD field, and update `test/schema/golden/<kind>.spec.openapi.yaml` the same way.
- Check that `ToKollectSinkSpec` in `api/v1alpha1/sink_adapter.go` maps the family to the sink spec.
- Check that `internal/validation/family_sink.go` validates the family's type.
- No integration package edit is needed: `Taskfile.yml` already covers `./internal/sink/...`.
- Finish with `task verify` and `task lint` green.

## Gotchas

- `task test-integration` needs Docker; a new integration-tagged package outside the listed ones
  must be added to `INTEGRATION_PACKAGES` in `Taskfile.yml`. Without Docker,
  `task test-integration:explore` reports what it could not run (CONTRIBUTING.md, `Taskfile.yml`).
- `task scrub` checks staged files only, and `hack/scrub-patterns.txt` holds case-insensitive
  regular expressions, so stage your change before you run it (`hack/scrub.sh`).
- `task changelog:verify` is deliberately not a pull-request gate: rebase merges change commit
  hashes, so it always drifts there; run it locally before tagging (`.github/workflows/preflight.yaml`).
- The required `test` and `kind-smoke` contexts report on every pull request, including
  documentation-only ones; the real work runs in `test-suite` and `kind-smoke-run`
  (`.github/workflows/ci.yaml`, `.github/workflows/e2e-smoke.yaml`).
- A changed CRD schema keeps `task verify` red until the schema golden is updated
  (`test/schema/contract_test.go`).
