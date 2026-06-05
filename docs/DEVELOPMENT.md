# Local development

This guide covers building, testing, and running **kollect** on your machine against a local
Kubernetes cluster (typically [kind](https://kind.sigs.k8s.io/)).

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

Full runbook: [RELEASE.md](RELEASE.md). Retroactive version anchors (`v0.0.1`–`v0.1.0`) are
documented in the `CHANGELOG.md` header and `cliff.toml`.

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
[`.github/workflows/release.yaml`](../.github/workflows/release.yaml): GHCR image
(`ghcr.io/konih/kollect`), Trivy, cosign, SPDX SBOM, Helm OCI chart, GitHub Release assets.

**Manual release test** (`workflow_dispatch`): Actions → **Release** → enter an existing tag;
optional `draft` / `prerelease` flags.

Before the first tag:

```sh
task changelog
VERSION=0.1.0 task release-dry-run
task changelog:verify
```

See [CONTRIBUTING.md](../CONTRIBUTING.md) and [SECURITY.md](../SECURITY.md).

## Clone and build

```sh
git clone https://github.com/konih/kollect.git
cd kollect
```

### One-shot dev bootstrap

From a fresh clone, a single command downloads modules, builds the manager, creates the
**kollect-dev** kind cluster, installs the operator via Helm, and applies sample CRs:

```sh
task dev-up
# operator only (skip ingress/TLS/Grafana): KOLLECT_DEV_MINIMAL=1 task dev-up
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
kubectl apply -k config/samples/
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

See [hack/kind/README.md](../hack/kind/README.md) for architecture and cluster comparison.

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

## Multi-cluster hub auth (ADR-0028)

Hub spoke ingest validates **`TokenReview`** then **`SubjectAccessReview`** (non-resource `POST`
`/hub/v1alpha1/reports`, or `create`/`patch` on `kollectremoteclusters`). Spokes send
`Authorization: Bearer` plus `X-Kollect-Cluster-Id`. See
[ADR-0028](adr/0028-hub-cluster-auth-istio-pattern.md).

### Generate Istio-style remote credential secrets

Stub CLI for GitOps registration on the hub (optional pull path):

```sh
go run ./cmd/kollect create-remote-secret --cluster spoke-a --namespace platform
# or:
hack/create-remote-secret.sh --cluster spoke-a --api-server https://spoke-a.example:6443
```

Omitted `--token` / `--ca-file` emit placeholders; pipe to `kubectl apply -f -` after editing or
substituting real spoke credentials. Pair with `KollectRemoteCluster.spec.credentialsSecretRef`.

Build the helper binary:

```sh
go build -o bin/kollect ./cmd/kollect
```

## Code generation workflow

kollect commits generated artifacts. After changing API types or `+kubebuilder` markers:

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

### Unit tests + envtest

```sh
task test
# equivalent: make test
```

`make test` runs `setup-envtest`, which downloads Kubernetes API server/etcd binaries into `bin/`
for controller-runtime envtest. First run may take a minute.

Controller tests live under `internal/controller/` (`suite_test.go` sets up envtest).

### Integration tests (testcontainers)

Sink integration tests use the `integration` build tag and Docker:

- **Git:** bare `file://` remote
- **S3:** MinIO module
- **Postgres:** official Postgres image (`internal/sink/postgres/`)
- **Kafka:** Redpanda module (`internal/sink/kafka/`)

```sh
task test-integration
# equivalent:
go test -tags=integration -count=1 ./internal/sink/...
```

If Docker is unavailable, MinIO tests skip; unit tests under `internal/sink/` still run via
`task test`.

### End-to-end (kind)

```sh
make test-e2e
```

Creates (or reuses) kind cluster `kollect-e2e`, runs `test/e2e/`, then deletes the cluster.
E2E is also available as a manual GitHub Actions workflow (`.github/workflows/test-e2e.yaml`).

### Nightly kind smoke (CI)

Scheduled and manual workflow `.github/workflows/e2e-nightly.yaml` uses `hack/kind/e2e/setup.sh`
and `hack/kind/e2e/smoke.sh` on cluster **kollect-e2e**, then runs multi-tenant isolation,
`task bench` with artifact upload, and a local bare-repo git export assert. Remote git clone/SHA
assert in `hack/e2e/git-export-assert.sh` is skipped when `GITHUB_TOKEN` is unset (inventory HTTP
hash is still verified); set `GIT_EXPORT_TEST_REPO` to override the default demo repo URL.

### tenantMode RBAC e2e

Nightly installs a second Helm release (`kollect-tenant`) in `kollect-tenant-ops` with
`tenantMode: true` and asserts a namespaced `Role` (no manager `ClusterRole`):

```sh
chmod +x hack/e2e/tenant-mode.sh
REPO_ROOT="$(pwd)" hack/e2e/tenant-mode.sh
```

### Multi-tenant e2e (default pattern)

Nightly smoke uses **dynamic tenant namespaces** (`kollect-tenant-a`, `kollect-tenant-b`) — not
shared `default` — to prove per-namespace inventory rollup isolation:

```sh
# After operator is running on kind (see nightly workflow or local Helm install):
chmod +x hack/e2e/multitenant.sh
REPO_ROOT="$(pwd)" hack/e2e/multitenant.sh
```

The script:

1. Creates two tenant namespaces with distinct label selectors on `KollectTarget`.
2. Seeds one Deployment per tenant.
3. Asserts each `KollectInventory.status.itemCount` is **1** (no cross-tenant leakage).
4. Probes `GET /inventory?namespace=<tenant>` and verifies HTTP payloads stay scoped.

Fixtures live under `test/e2e/fixtures/multitenant/`. Helm CI values:
`charts/kollect/ci/e2e-tenant-values.yaml`.

Unit tests: `TestKollectInventoryReconciler_aggregatesSameNamespaceOnly`,
`TestStoreNamespaceIsolation`, `TestCacheOptionsForWatchNamespaces_scopedNamespaces`.

### Coverage

```sh
make test
# cover.out at repo root
go tool cover -func=cover.out
```

### Benchmarks (micro, safe default)

Run extractor and collection hot-path benchmarks without heavy synthetic clusters:

```sh
task bench
# equivalent:
go test -short -bench=. -benchmem ./internal/collect/...
```

Uses `-short` so long sub-benchmarks are skipped on laptops. Suitable for CI and quick regression
checks. See [PERFORMANCE.md](PERFORMANCE.md) and [ADR-0026](adr/0026-performance-scalability.md).

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
**gitignored** local path (`agent-context/PERF-SNAPSHOT.md`); never commit it.

```sh
task perf-report
```

See [PERFORMANCE.md](PERFORMANCE.md) for operator tuning and the metrics catalog.

## Lint and format

```sh
task lint          # golangci-lint v2 (custom plugins via .custom-gcl.yml if present)
task vulncheck     # govulncheck ./... (same as CI vulncheck job)
task format        # go fmt ./...
task format:check  # fail if gofmt would change files
task helm-test     # helm lint + unittest (transport default, tenantMode RBAC)
```

`task vulncheck` uses the [Go vulnerability database](https://vuln.go.dev/) and fails when known
issues affect imported packages in this module. See [SECURITY.md](../SECURITY.md) for policy and
documented exceptions.

Install hooks once:

```sh
pre-commit install
```

Pre-commit runs gitleaks, scrub, verify, and golangci-lint on relevant changes.

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

### Confusing `go mod tidy` with local-only trees

If tidy fails or pulls unexpected modules, check that no Go file imports from ignored reference
paths. The module root is `github.com/konih/kollect` only.

### Image tag mismatch

- **Task** defaults: `kollect-controller-manager:dev`
- **Make deploy**: `IMG=controller:latest` unless you set `IMG=...`

For kind, build and load the same tag Task uses, or override consistently:

```sh
make docker-build docker-push IMG=ghcr.io/konih/kollect:dev
make deploy IMG=ghcr.io/konih/kollect:dev
```

### Sample CRs vs controller maturity

As of Phase 0/early Phase 1, reconcilers may still be scaffolds. Applying samples validates CRD
schema and wiring; export to Git sinks requires implemented controller logic. See
[QUICKSTART.md](QUICKSTART.md#current-maturity).

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
mkdocs build --strict
```

Configuration: `mkdocs.yml` at the repository root. GitHub Pages workflow:
`.github/workflows/docs.yaml`. See [ADR-0021](adr/0021-mkdocs-github-pages.md).

| Doc | Audience |
| --- | --- |
| [QUICKSTART.md](QUICKSTART.md) | First install on kind, sample CRs |
| [ARCHITECTURE.md](ARCHITECTURE.md) | CRD model, reconciliation, phasing |
| [REQUIREMENTS.md](REQUIREMENTS.md) | Product priorities |
| [examples/deployment-inventory.md](examples/deployment-inventory.md) | Annotated YAML walkthroughs |
| [adr/README.md](adr/README.md) | Architecture decision records |
| [PERFORMANCE.md](PERFORMANCE.md) | Scale targets, metrics, pprof, bounded load tests |

## Further reading

- [QUICKSTART.md](QUICKSTART.md) — operator install and first CRs
- [ARCHITECTURE.md](ARCHITECTURE.md) — CRD model and reconciliation flow
- [CONTRIBUTING.md](https://github.com/konih/kollect/blob/main/CONTRIBUTING.md) — commits, PR checks
- [ADRs](adr/README.md) — architecture decision records
