# Local development

This guide covers building, testing, and running **kollect** on your machine against a local
Kubernetes cluster (typically [kind](https://kind.sigs.k8s.io/)).

## Prerequisites

| Tool | Version / notes |
| --- | --- |
| [Go](https://go.dev/dl/) | **1.25+** (see `go.mod`; project targets 1.26) |
| [Docker](https://docs.docker.com/get-docker/) | For container image builds and kind |
| [kubectl](https://kubernetes.io/docs/tasks/tools/) | 1.28+ recommended |
| [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) | Local cluster for smoke / e2e |
| [Task](https://taskfile.dev/installation/) | Runs project tasks (`Taskfile.yml`) |
| [Kubebuilder](https://book.kubebuilder.io/quick-start.html#installation) | v4.x CLI (scaffolded with **4.14** — see `PROJECT`) |
| [pre-commit](https://pre-commit.com/#install) | Optional but recommended (`pre-commit install`) |

Optional: [git-cliff](https://git-cliff.org/) for changelog previews (`task changelog`).

## Clone and build

```sh
git clone https://github.com/konih/kollect.git
cd kollect

# Download modules and build the manager binary
task build
# equivalent: make build  →  bin/manager
```

The manager binary lands at `bin/manager`. Use `task --list-all` to see all targets.

## Run against kind

### 1. Create a cluster

```sh
kind create cluster --name kollect-dev
kubectl cluster-info --context kind-kollect-dev
```

### 2. Install CRDs

```sh
task install:crds
# or: make install
```

### 3. Build and load the operator image

```sh
task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-dev
```

Default image tag is `kollect-controller-manager:dev` (see `Taskfile.yml`).

### 4. Deploy the manager

```sh
task deploy:operator
```

This applies `config/default` (namespace `kollect-system`, deployment `kollect-controller-manager`).

### 5. Apply sample CRs

```sh
kubectl apply -k config/samples/
```

See [QUICKSTART.md](QUICKSTART.md) and [examples/deployment-inventory.md](examples/deployment-inventory.md)
for what each sample does and what to expect at the current project phase.

### 6. Run the manager on the host (alternative)

Useful for fast iteration with a debugger:

```sh
make run
# or after codegen: go run ./cmd/main.go
```

Ensure your kubeconfig points at the target cluster (`KUBECONFIG` or default `~/.kube/config`).

### Teardown

```sh
kind delete cluster --name kollect-dev
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

### End-to-end (kind)

```sh
make test-e2e
```

Creates (or reuses) kind cluster `kollect-test-e2e`, runs `test/e2e/`, then deletes the cluster.
E2E is also available as a manual GitHub Actions workflow (`.github/workflows/test-e2e.yaml`).

### Coverage

```sh
make test
# cover.out at repo root
go tool cover -func=cover.out
```

## Lint and format

```sh
task lint          # golangci-lint v2 (custom plugins via .custom-gcl.yml if present)
task format        # go fmt ./...
task format:check  # fail if gofmt would change files
```

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

## Further reading

- [QUICKSTART.md](QUICKSTART.md) — operator install and first CRs
- [ARCHITECTURE.md](ARCHITECTURE.md) — CRD model and reconciliation flow
- [CONTRIBUTING.md](https://github.com/konih/kollect/blob/main/CONTRIBUTING.md) — commits, PR checks
- [ADRs](adr/README.md) — architecture decision records
