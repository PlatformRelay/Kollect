# kollect

Generic Kubernetes **inventory + doc-sync operator** (`kollect.dev/v1alpha1`).

kollect watches arbitrary API resources, extracts attributes with CEL or JSONPath, aggregates
results, and exports to pluggable sinks (Git, GitLab, S3, GCS, Prometheus) and documentation
backends (Confluence, Git). It is event-driven (dynamic informers), designed for robust
multi-tenant collection with SAR-aware RBAC degradation.

## Problem

Platform and application teams need **stakeholder-visible inventory** without bespoke
collectors per resource type: what is deployed, where, and which attributes matter for audits,
cost, or developer portals. Batch scripts and hardcoded schemas do not scale across clusters
and CRDs.

kollect replaces one-off inventory jobs with declarative CRDs: define a **profile** (GVK +
attributes), attach **targets** (selectors), aggregate in **inventory**, and dispatch to
**sinks**.

## Quickstart

**Prerequisites:** Go (see `go.mod`), Docker (for image build), kubectl, and a Kubernetes 1.28+
cluster for deployment.

```sh
# Build and test locally
task build
task test

# Generate / verify CRDs and RBAC
make generate manifests
task verify

# Install CRDs and deploy the manager (set IMG for your registry)
make install
make docker-build docker-push IMG=ghcr.io/konih/kollect:dev
make deploy IMG=ghcr.io/konih/kollect:dev

# Apply sample CRs
kubectl apply -k config/samples/
```

For a consolidated install manifest:

```sh
make build-installer IMG=ghcr.io/konih/kollect:dev
kubectl apply -f dist/install.yaml
```

## Project layout

| Path | Purpose |
|------|---------|
| `api/v1alpha1/` | CRD Go types (`KollectProfile`, `KollectSink`, `KollectTarget`, `KollectInventory`) |
| `internal/controller/` | Reconcilers for `KollectTarget` and `KollectInventory` |
| `config/crd/bases/` | Generated CRD YAML |
| `config/samples/` | Example CR instances |
| `hack/verify.sh` | Codegen drift gate (also `task verify`) |

Static config kinds (`KollectProfile`, `KollectSink`) are validated via the API; reconciled
kinds (`KollectTarget`, `KollectInventory`) run controllers. See `GUIDELINES.md` for engineering
rules and `docs/adr/` for architecture decisions (as they land).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `task lint`, `task test`, and `task verify` before
opening a PR.

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

Copyright (c) 2026 Konrad Heimel. Licensed under the [MIT License](LICENSE).
