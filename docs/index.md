# kollect

**kollect** is a generic Kubernetes **inventory export operator** (`kollect.dev/v1alpha1`).

Use these docs to install the operator, apply sample CRs, and understand how collection and export
fit together.

## Start here

- **[Quick start](QUICKSTART.md)** — kind cluster, operator install, sample CRs
- **[Platform decisions](PLATFORM-DECISIONS.md)** — locked architecture (2026-06-05 pivot)
- **[Development guide](DEVELOPMENT.md)** — build, test, codegen, lint
- **[Architecture](ARCHITECTURE.md)** — CRD model, reconciliation, build order
- **[Requirements](REQUIREMENTS.md)** — MVP export path, Postgres/Kafka primary
- **[Performance](PERFORMANCE.md)** — scale targets, metrics, pprof, bounded load tests

## Examples

- [Deployment inventory → Postgres/Kafka](examples/deployment-inventory.md)
- [Helm release inventory (Argo primary; Flux secondary)](examples/helm-release-inventory.md)

## Decisions

Architecture decision records live in [adr/README.md](adr/README.md).
