# Local development

This guide covers building, testing, and running **Kollect** on your machine against a local
Kubernetes cluster (typically [kind](https://kind.sigs.k8s.io/)).

!!! tip "Assumptions"
    This guide assumes Go, Docker, kind, kubectl, and [Task](https://taskfile.dev/) are installed.
    New to CRDs or the docs site? Start with [Understand the basics](../concepts/resource-model.md) and
    [getting-started/install.md](../getting-started/install.md).

## Prerequisites

| Tool | Version / notes |
| --- | --- |
| [Go](https://go.dev/dl/) | **1.26.4+** (see `go.mod`) |
| [Docker](https://docs.docker.com/get-docker/) | For container image builds and kind |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | 1.28+ recommended |
| [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) | Local cluster for smoke / e2e |
| [Task](https://taskfile.dev/installation/) | Runs project tasks (`Taskfile.yml`) |
| [Kubebuilder](https://book.kubebuilder.io/quick-start.html#installation) | v4.x CLI (scaffolded with **4.14** — see `PROJECT`) |
| [pre-commit](https://pre-commit.com/#install) | Optional but recommended (`pre-commit install`) |

Optional: `task tools:git-cliff` installs a pinned [git-cliff](https://git-cliff.org/) binary into
`bin/` (also used by `task changelog*`).

### Releases (maintainers)

| Task | Purpose |
| --- | --- |
| `task changelog` | Preview unreleased notes |
| `task changelog:write` | Regenerate `CHANGELOG.md` |
| `task changelog:verify` | Fail if changelog drift (same as preflight CI) |
| `task release-dry-run` | Build `dist/` install YAML + chart (no push) |

Full runbook: [RELEASE.md](../RELEASE.md). Retroactive version anchors (`v0.0.1`–`v0.0.4`, RC series) are
documented in the `CHANGELOG.md` header and `hack/release/cliff.toml`.

**Local dry-run** (`task release-dry-run`) runs `hack/release-assets.sh` with
`VERSION=0.0.0-dry-run` (override with `VERSION=0.1.0 task release-dry-run`). Outputs land in
`dist/`:

| Artifact | Path |
| --- | --- |
| Install manifests | `dist/install.yaml` |
| CRD bundle | `dist/install-crds.yaml` |
| Helm chart tarball | `dist/kollect-<version>.tgz` |
| Checksums | `dist/checksums.txt` |

The task does **not** push images or publish GitHub/OCI assets.

**GitHub Release** — tagged `v*.*.*` pushes run
[`.github/workflows/release.yaml`](https://github.com/platformrelay/kollect/blob/main/.github/workflows/release.yaml): GHCR image
(`ghcr.io/platformrelay/kollect`), Trivy, cosign, SPDX SBOM, Helm OCI chart, GitHub Release assets.

**Manual release test** (`workflow_dispatch`): Actions → **Release** → enter an existing tag;
optional `draft` / `prerelease` flags.

Before the first tag:

```sh
task changelog
VERSION=0.1.0 task release-dry-run
task changelog:verify
```

See [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md) and [SECURITY.md](https://github.com/platformrelay/kollect/blob/main/SECURITY.md).

## Clone and build

```sh
git clone https://github.com/platformrelay/kollect.git
cd kollect
```

### One-shot dev bootstrap

From a fresh clone, a single command downloads modules, builds the manager, creates the
**kollect-dev** kind cluster, installs the operator via Helm, and applies sample CRs:

```sh
task dev-up
# operator only (skip ingress/TLS/Grafana): KOLLECT_DEV_MINIMAL=1 task dev-up
```

`task dev-up` builds then runs **`task demo-up`** (in-cluster Forgejo hero) so the default
evaluator path reaches `ConnectionVerified` without Secrets. Profile/target examples remain under
`config/samples/`; multi-sink / database samples:

```sh
kubectl apply -k config/samples/advanced/
```

Use `task --list-all` to see all targets.

### Build only

```sh
# Download modules and build the manager binary
task build
# equivalent: make build  →  bin/manager
```

The manager binary lands at `bin/manager`.

## Local Kind (dev)

For daily development, use the **kollect-dev** profile (`hack/kind/dev/`). **`task dev-up`**
(above) runs the full flow; the targets below are useful when you need individual steps for
debugging or iteration.

```sh
task kind-dev-up          # cluster + operator (+ addons unless KOLLECT_DEV_MINIMAL=1)
KOLLECT_DEV_MINIMAL=1 task kind-dev-up   # operator only (skip addons)
task kind-dev-load        # rebuild image after code changes
task kind-dev-status      # cluster + pod status
kubectl apply -k config/samples/          # profiles/targets examples (no secret sinks)
task demo-up                              # credential-free Ready Git path
# optional multi-sink / Postgres / S3 / Kafka:
# kubectl apply -k config/samples/advanced/
task kind-dev-down
```

**Prerequisites beyond Docker/kind/kubectl/helm:** [mkcert](https://github.com/FiloSottile/mkcert)
for trusted `*.localhost` HTTPS (skipped gracefully if not installed). Certs are generated under
`hack/kind/dev/certs/` (git-ignored).

Optional env vars:

| Variable | Effect |
| --- | --- |
| `KOLLECT_DEV_MINIMAL=1` | Skip ingress, TLS, Grafana, Prometheus |
| `KOLLECT_DEV_PROMETHEUS=1` | Install lightweight Prometheus in dev cluster |

See [hack/kind/README.md](https://github.com/platformrelay/kollect/blob/main/hack/kind/README.md) for architecture and cluster comparison.

### Run the manager on the host (alternative)

Useful for fast iteration with a debugger:

```sh
make run
# or after codegen: go run ./cmd/main.go
```

Ensure your kubeconfig points at `kind-kollect-dev` (`kubectl config use-context kind-kollect-dev`).

### Manual / kustomize deploy (legacy)

If you prefer raw manifests instead of Helm:

```sh
kind create cluster --name kollect-dev
task install:crds
task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-dev
task deploy:operator
kubectl apply -k config/samples/
```

Default image tag is `kollect-controller-manager:dev` (see `Taskfile.yml`).

## E2E Kind (CI)

The **kollect-e2e** profile (`hack/kind/e2e/`) is minimal: single node, no ingress or monitoring
addons. It mirrors `.github/workflows/e2e-nightly.yaml` via shared scripts.

```sh
task kind-e2e-up
bash hack/kind/e2e/smoke.sh    # sample CRs, nginx seed, bounded waits, HTTP probe
task kind-e2e-down
```

Helm values: `charts/kollect/ci/e2e-tenant-values.yaml`. Kubernetes version is pinned from
`go.mod` in `hack/kind/common.sh` (same pin as dev and envtest).

## Multi-cluster fleet (shared sink)

Multi-cluster is **N independent single-mode operators** — one Helm release per cluster — exporting
to a **shared sink** (Postgres, Git, Kafka, NATS) with `spec.cluster` row partitioning. There is
**no** hub/spoke runtime tier, ingest API, or queue transport between clusters
([ADR-0501](../adr/0501-multi-cluster-fleet.md)).

Walkthrough: [Multi-cluster fleet example](../examples/multi-cluster-fleet.md) ·
[Deployment topology matrix](../deployment/topology-matrix.md).

## Code generation workflow

Kollect commits generated artifacts. After changing API types or `+kubebuilder` markers:

```sh
make generate    # deepcopy (api/*/zz_generated.deepcopy.go)
make manifests   # CRDs (config/crd/bases), RBAC (config/rbac/role.yaml)
```

Or via Task:

```sh
task generate
task manifests
```

**Verify nothing drifted** (CI and pre-commit run this):

```sh
task verify
```

`hack/verify.sh` regenerates into a temp dir and diffs against the tree. If it fails, run
`make generate manifests`, commit the updated files, and re-run `task verify`.

### controller-gen paths

`Makefile` invokes controller-gen with explicit paths:

```makefile
paths="./api/..." paths="./internal/..." paths="./cmd/..."
```

If you add packages outside these trees, extend the `paths=` list or RBAC / CRD generation will
miss your types.

## Tests

Test pyramid (L0–L5), coverage floors, and CI gates:
[Testing strategy](testing.md) · [ADR-0706](../adr/0706-testing-merge-gate-architecture.md) ·
[coding-standards.md](coding-standards.md#testing).

| Task | Purpose |
| --- | --- |
| `task test` | Unit + envtest (no coverage floor) |
| `task coverage` | Unit/envtest + `coverage.out` + floor check |
| `task test-integration` | L3 sink/transport integration (Docker) |
| `task test:e2e` | L4 kind smoke (setup → smoke → teardown) |

`make test` runs `setup-envtest`, which downloads Kubernetes API server/etcd binaries into `bin/`
for controller-runtime envtest. First run may take a minute. Controller tests live under
`internal/controller/` (`suite_test.go` sets up envtest).

E2E scripts, nightly workflows, multi-tenant fixtures, and tenantMode RBAC asserts are documented in
[testing.md](testing.md) and `hack/kind/README.md`.

### Benchmarks (micro, safe default)

Run extractor and collection hot-path benchmarks without heavy synthetic clusters:

```sh
task bench
# equivalent:
go test -short -bench=. -benchmem ./internal/collect/...
```

Uses `-short` so long sub-benchmarks are skipped on laptops. Suitable for CI and quick regression
checks. See [operator-manual/performance.md](../operator-manual/performance.md) and [ADR-0603](../adr/0603-performance-scalability.md).

### Load tests (opt-in, bounded)

**Not** part of default `task test`. Requires explicit opt-in and caps at **2000** synthetic objects:

```sh
KOLECT_LOAD_TEST=1 task load-test
# equivalent:
KOLECT_LOAD_TEST=1 go test -tags=load -count=1 -timeout=15m ./test/load/...
```

Never run 10k-object load tests locally unless you have dedicated hardware and understand API-server
load. Default envtest suites cap synthetic objects at **500**.

### Performance report (`task perf-report`)

Runs `hack/perf-report.sh`: micro-benchmarks under `internal/collect/`, a quick unit-test pass, and
writes a markdown summary useful when comparing regressions on a laptop. Output is written to a
**gitignored** local path (`agent-context/PERF-SNAPSHOT.md`); in CI the same script writes
`artifacts/perf-snapshot.md` and uploads it as a workflow artifact — never commit either path.

```sh
task perf-report
```

See [operator-manual/performance.md](../operator-manual/performance.md) for operator tuning and the metrics catalog.

## Lint and format

Go conventions, lint policy, and CI gates:
[coding-standards.md](coding-standards.md) ·
[tooling-setup.md](tooling-setup.md).

```sh
task lint          # golangci-lint v2 + go-arch-lint
task arch-lint     # import-graph fitness only
task vulncheck     # govulncheck (CI vulncheck job)
task format        # go fmt ./...
task format:check  # fail if gofmt or goimports would change files
task helm-test     # helm lint + helm-docs drift + unittest
task helm-docs     # regenerate charts/kollect/README.md
task lint:markdown # markdownlint-cli2 on docs/**/*.md
```

Install hooks once: `pre-commit install` (gitleaks, scrub, verify, golangci-lint, markdownlint).

## Pre-commit and scrub before push

Before committing:

```sh
git add ...
task scrub
gitleaks protect --staged --no-banner
```

`task scrub` scans **staged** files for forbidden company/legacy strings (see `hack/scrub.sh`).

## Common pitfalls

### Generated artifact drift

Symptom: CI `verify` job or `task verify` fails after editing `api/v1alpha1/*_types.go`.

Fix: `make generate manifests`, review diff, commit generated YAML and deepcopy files.

### `references/` and `agent-context/` are local-only

These directories are **gitignored** and hold private planning material and OSS reference clones.
They are not in the public module graph.

- Do **not** expect `go mod tidy` to resolve imports from `references/`.
- Do not commit paths under `agent-context/`, `references/`, or `AGENTS.md`.
- If present locally, `references/IBM-Message-Queue-Operator/` is an **example reference only**
  (not production-grade). Borrow layout, Taskfiles, CI, and docs/ADR structure selectively; do not
  copy MQ-specific logic.

### Confusing `go mod tidy` with local-only trees

If tidy fails or pulls unexpected modules, check that no Go file imports from ignored reference
paths. The module root is `github.com/platformrelay/kollect` only.

### Image tag mismatch

- **Task** defaults: `kollect-controller-manager:dev`
- **Make deploy**: `IMG=controller:latest` unless you set `IMG=...`

For kind, build and load the same tag Task uses, or override consistently:

```sh
make docker-build docker-push IMG=ghcr.io/platformrelay/kollect:dev
make deploy IMG=ghcr.io/platformrelay/kollect:dev
```

### Sample CRs vs controller maturity

Controllers reconcile namespaced and cluster-scoped inventory CRs today — see
[getting-started/install.md](../ROADMAP.md) for phase-level status. Applying samples validates
CRD schema, webhook rules, and end-to-end export when sinks are configured.

## Documentation site (MkDocs)

Preview locally:

```sh
python3 -m venv .venv-docs && . .venv-docs/bin/activate
pip install mkdocs-material
mkdocs serve
```

Open http://127.0.0.1:8000/

Strict build (matches CI):

```sh
task docs:verify
```

This also checks tracked Markdown, documentation truth and freshness contracts, committed samples,
and the browser layout when Chrome/Chromium is available. Use
`DOCS_REQUIRE_CHROME=1 task docs:verify` to require the browser exactly as CI does.

Configuration: `mkdocs.yml` at the repository root. GitHub Pages workflow:
`.github/workflows/docs.yaml`. See [ADR-0701](../adr/0701-mkdocs-github-pages.md).

| Doc | Audience |
| --- | --- |
| [getting-started/install.md](../getting-started/install.md) | First install on kind, sample CRs |
| [concepts/architecture.md](../concepts/architecture.md) | CRD model, reconciliation, phasing |
| [REQUIREMENTS.md](../REQUIREMENTS.md) | Product requirements and NFRs |
| [getting-started/first-inventory.md](../getting-started/first-inventory.md) | Annotated YAML walkthroughs |
| [adr/README.md](../adr/README.md) | Architecture decision records |
| [operator-manual/performance.md](../operator-manual/performance.md) | Scale targets, metrics, pprof, bounded load tests |

## Further reading

- [getting-started/install.md](../getting-started/install.md) — operator install and first CRs
- [concepts/architecture.md](../concepts/architecture.md) — CRD model and reconciliation flow
- [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md) — commits, PR checks
- [ADRs](../adr/README.md) — architecture decision records
