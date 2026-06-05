# kollect

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/konih/kollect/badge)](https://securityscorecards.dev/viewer/?uri=github.com/konih/kollect)
[![CI](https://github.com/konih/kollect/actions/workflows/ci.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/konih/kollect/graph/badge.svg)](https://codecov.io/gh/konih/kollect)
[![Docs](https://github.com/konih/kollect/actions/workflows/docs.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/docs.yaml)
[![License: MIT](https://img.shields.io/github/license/konih/kollect)](https://github.com/konih/kollect/blob/main/LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/konih/kollect)](https://pkg.go.dev/github.com/konih/kollect)
[![Docs site](https://img.shields.io/badge/docs-konih.github.io%2Fkollect-blue)](https://konih.github.io/kollect/)
[![Container](https://img.shields.io/badge/ghcr.io-konih%2Fkollect-2496ED?logo=docker&logoColor=white)](https://github.com/konih/kollect/pkgs/container/kollect)

**kollect** is a Kubernetes operator that turns selected, live cluster state into a **durable,
queryable, diffable inventory** — decoupled from the apiserver's availability, RBAC, and scale
limits. Portals, automation, and auditors read **export data**, not unbounded list/watch against the
live API.

Kubernetes is the source of truth for *what is running*; it is a poor *system of record* for
stakeholder inventory. kollect maintains a **read model**: **select** resources by GVK → **extract**
the attributes that matter (CEL or JSONPath) → **aggregate** across targets → **debounce** →
**export** to pluggable sinks. Inventory is **configuration, not code** — owned per team in its own
namespace.

**Read the docs:** **[konih.github.io/kollect](https://konih.github.io/kollect/)** — architecture,
quick start, CR reference, ADRs, and examples. This README is the front door; the site is the map.

> **Pre-beta.** APIs and defaults may change until the first release candidate. See the
> [roadmap](https://konih.github.io/kollect/ROADMAP/) for current status.

## How it works

```text
Kubernetes API  →  shared informer (per GVK)  →  in-memory collect store
       →  KollectInventory debounce  →  sink projection(s)
```

The in-memory snapshot per inventory is **canonical**; every sink is a **projection** of it — no
single backend is privileged ([sink roles](https://konih.github.io/kollect/adr/0401-sink-taxonomy-state-vs-stream/)).

| Sink role | Examples | Good for |
| --- | --- | --- |
| **Snapshot store** | Git, GitLab, S3/GCS (JSON today) | Audit, diff, GitOps-friendly history |
| **Relational store** | Postgres | Rich SQL for portals and dashboards |
| **Event emitter** | Kafka / Redpanda / NATS | Change streams, downstream consumers |

Full payload lives in sinks; CR `.status` holds summaries only ([etcd limits](https://konih.github.io/kollect/adr/0103-etcd-limit/)).

## Quick start

Try kollect on a local kind cluster:

```sh
kind create cluster --name kollect-dev
git clone https://github.com/konih/kollect.git && cd kollect
task build
task install:crds && task docker:build
kind load docker-image kollect-controller-manager:dev --name kollect-dev
task deploy:operator
kubectl apply -k config/samples/
```

Watch `KollectInventory` status and check your sink (Git demo repo, Postgres, Kafka, etc.).

**Full walkthrough** — prerequisites, Helm options, maturity notes:
**[Quick start →](https://konih.github.io/kollect/QUICKSTART/)**

## Why kollect?

| | |
| --- | --- |
| **Event-driven** | Shared informers per GVK — inventory stays current without polling loops ([ADR-0301](https://konih.github.io/kollect/adr/0301-event-driven-informers/)). |
| **Schema-flexible** | Declare attributes in `KollectProfile`; no bespoke collector per CRD. |
| **CRD-native & GitOps-friendly** | Profiles, sinks, targets, and inventory are Kubernetes resources in team namespaces. |
| **Multi-tenant** | `KollectScope` gates which teams and namespaces may export to which sinks. |
| **Fleet-ready** | Default path: spokes write to **shared sinks** with a cluster label. Optional **hub mode** (`mode: hub\|spoke` on the same image) for Git fan-in or credential centralization — no hub CRD required. |

Default install for new teams: **namespaced Helm** with `tenantMode: true` and scoped
`watchNamespaces`. Platform-wide cluster operators remain supported.

## Learn more

| Topic | Link |
| --- | --- |
| Problem statement, CRD model, reconciliation | [Architecture](https://konih.github.io/kollect/ARCHITECTURE/) |
| Locked platform decisions | [Platform decisions](https://konih.github.io/kollect/PLATFORM-DECISIONS/) |
| CR fields, RBAC, failure modes | [CR reference](https://konih.github.io/kollect/CR-REFERENCE/) |
| Multi-cluster & hub/spoke | [ADR-0501](https://konih.github.io/kollect/adr/0501-multi-cluster-sync-rfc/) |
| Sink taxonomy (state vs stream) | [ADR-0401](https://konih.github.io/kollect/adr/0401-sink-taxonomy-state-vs-stream/) |
| Build-order phases and status | [Roadmap](https://konih.github.io/kollect/ROADMAP/) |
| Examples index | [Examples](https://konih.github.io/kollect/examples/) |
| Example: Deployment → Git export | [Walkthrough](https://konih.github.io/kollect/examples/deployment-inventory/) |
| Live demo inventory (Git sink) | [kollect-inventory-demo](https://github.com/konih/kollect-inventory-demo) |

Developers: run `task lint`, `task test`, and `task verify` before opening a PR —
[CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md).

## License

Copyright (c) 2026 Konrad Heimel. Licensed under the [MIT License](LICENSE).
