# kollect

**kollect** is a generic Kubernetes **inventory export operator** (`kollect.dev/v1alpha1`).

Use these docs to install the operator, apply sample CRs, and understand how collection and export
fit together.

## Start here

- **[Quick start](QUICKSTART.md)** — kind cluster, operator install, sample CRs
- **[Development guide](DEVELOPMENT.md)** — build, test, codegen, lint
- **[Architecture](ARCHITECTURE.md)** — CRD model, reconciliation, phasing
- **[Requirements](REQUIREMENTS.md)** — product priorities (TLS, aggregation, HTTP API, Helm)

## Examples

- [Deployment inventory → Git](examples/deployment-inventory.md)

## Decisions

Architecture decision records live in [adr/README.md](adr/README.md).
