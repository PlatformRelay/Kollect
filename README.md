# kollect

[![CI](https://github.com/konih/kollect/actions/workflows/ci.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/konih/kollect/graph/badge.svg)](https://codecov.io/gh/konih/kollect)
[![Docs](https://github.com/konih/kollect/actions/workflows/docs.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/docs.yaml)
[![License: MIT](https://img.shields.io/github/license/konih/kollect)](https://github.com/konih/kollect/blob/main/LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/konih/kollect)](https://pkg.go.dev/github.com/konih/kollect)
[![Docs site](https://img.shields.io/badge/docs-konih.github.io%2Fkollect-blue)](https://konih.github.io/kollect/)
[![Container](https://img.shields.io/badge/ghcr.io-konih%2Fkollect-2496ED?logo=docker&logoColor=white)](https://github.com/konih/kollect/pkgs/container/kollect)

**kollect** is a Kubernetes inventory operator. Point it at any API resource (any GVK), describe
the fields you care about with CEL or JSONPath, and export live cluster state to **Postgres**,
**Kafka**, **Git**, and more — no bespoke collector per CRD, no batch cron jobs scraping the API.

Stakeholders who never touch `kubectl` still get auditable, versioned snapshots they can diff,
query, or feed into developer portals and compliance workflows.

**Read the docs:** **[konih.github.io/kollect](https://konih.github.io/kollect/)** — install guides,
CR references, architecture, and examples. This README is the front door; the site is the map.

## Quick start

Try kollect on a local kind cluster in a few minutes:

1. **Create a cluster** — `kind create cluster --name kollect-dev`
2. **Build and deploy** — `task build`, `task install:crds`, `task docker:build`, load the image, `task deploy:operator`
3. **Apply samples** — `kubectl apply -k config/samples/` (profile → sink → target → inventory)
4. **Verify** — watch `KollectInventory` status and check your sink (Postgres, Kafka, or the [demo Git repo](https://github.com/konih/kollect-inventory-demo))

Full copy-paste commands, prerequisites, and maturity notes:
**[Quick start on the docs site →](https://konih.github.io/kollect/QUICKSTART/)**

## Why kollect?

| | |
| --- | --- |
| **Event-driven** | Dynamic informers react to changes — inventory stays current without polling loops. |
| **CRD-native** | Declare profiles, sinks, targets, and inventory in Kubernetes; GitOps-friendly from day one. |
| **Multi-tenant** | `KollectScope` gates which teams and namespaces can export to which sinks. |
| **Hub / spoke** | Run a central hub that aggregates inventory from spoke clusters via `KollectClusterTarget`. |

kollect is early and moving fast — issues, ideas, and PRs are welcome. See where we're headed in the
[roadmap](https://konih.github.io/kollect/ROADMAP/).

## Learn more

| Topic | Link |
| --- | --- |
| CR fields, RBAC, failure modes | [CR reference](https://konih.github.io/kollect/CR-REFERENCE/) |
| Debouncing, collection, scope gates | [Data flows](https://konih.github.io/kollect/DATA-FLOWS/) |
| Build-order phases and status | [Roadmap](https://konih.github.io/kollect/ROADMAP/) |
| Example: Deployment → Git export | [Walkthrough](https://konih.github.io/kollect/examples/deployment-inventory/) |
| Live demo inventory (Git sink) | [kollect-inventory-demo](https://github.com/konih/kollect-inventory-demo) |

Developers: `task lint`, `task test`, and `task verify` before opening a PR — details in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

Copyright (c) 2026 Konrad Heimel. Licensed under the [MIT License](LICENSE).
