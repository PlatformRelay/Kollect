# kollect

Generic Kubernetes **inventory export operator** (`kollect.dev/v1alpha1`).

Platform and application teams need **stakeholder-visible inventory** without bespoke collectors
per resource type: what is deployed, where, and which attributes matter for audits, cost, or
developer portals. Batch scripts and hardcoded schemas do not scale across clusters and CRDs.

**kollect** watches arbitrary API resources, extracts attributes with CEL or JSONPath, aggregates
results, and exports to pluggable sinks (Git, GitLab, S3, GCS, Postgres, Kafka). Exporting cluster
state to Git gives auditable, diffable snapshots that
developer portals and compliance workflows can consume alongside live API access — so stakeholders
without `kubectl` or repo access still see versioned, traceable system state.

## Quick start

**Prerequisites:** Docker, [kind](https://kind.sigs.k8s.io/), kubectl, Go, and [Task](https://taskfile.dev/).

```sh
kind create cluster --name kollect-dev
task build
task install:crds
task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-dev
task deploy:operator
kubectl apply -k config/samples/
```

Full step-by-step guide with verification and maturity notes:
**[docs/QUICKSTART.md](docs/QUICKSTART.md)**

## Documentation

| Guide | Description |
| --- | --- |
| [Quick start](docs/QUICKSTART.md) | Install on kind, apply samples |
| [Development](docs/DEVELOPMENT.md) | Build, test, codegen, lint |
| [Architecture](docs/ARCHITECTURE.md) | CRD model and reconciliation |
| [Example walkthrough](docs/examples/deployment-inventory.md) | Profile → sink → target → inventory |
| [ADRs](docs/adr/) | Architecture decisions |

Preview docs locally: `mkdocs serve` (see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md#documentation-site-mkdocs)).

## Project layout

| Path | Purpose |
| --- | --- |
| `api/v1alpha1/` | CRD Go types (`KollectProfile`, `KollectSink`, `KollectTarget`, `KollectInventory`) |
| `internal/controller/` | Reconcilers for `KollectTarget` and `KollectInventory` |
| `config/crd/bases/` | Generated CRD YAML |
| `config/samples/` | Example CR instances |
| `hack/verify.sh` | Codegen drift gate (also `task verify`) |

Static config kinds (`KollectProfile`, `KollectSink`) are validated via the API; reconciled
kinds (`KollectTarget`, `KollectInventory`) run controllers. See [GUIDELINES.md](GUIDELINES.md) for
engineering rules and [docs/adr/](docs/adr/) for architecture decisions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `task lint`, `task test`, and `task verify` before
opening a PR.

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

Copyright (c) 2026 Konrad Heimel. Licensed under the [MIT License](LICENSE).
