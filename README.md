<p align="center">
  <a href="https://konih.github.io/kollect/">
    <img src="docs/assets/branding/kollect-logo-light.png" alt="Kollect — durable Kubernetes inventory" width="360">
  </a>
</p>

<p align="center">
<a href="https://securityscorecards.dev/viewer/?uri=github.com/konih/kollect"><img src="https://api.securityscorecards.dev/projects/github.com/konih/kollect/badge" alt="OpenSSF Scorecard"></a>
<a href="https://www.bestpractices.dev/projects/13106"><img src="https://www.bestpractices.dev/projects/13106/badge" alt="OpenSSF Best Practices"></a>
<a href="https://konih.github.io/kollect/"><img src="https://img.shields.io/badge/docs-konih.github.io%2Fkollect-blue?style=flat-square&logo=readthedocs&logoColor=white" alt="Documentation"></a>
<a href="https://github.com/konih/kollect/actions/workflows/ci.yaml"><img src="https://github.com/konih/kollect/actions/workflows/ci.yaml/badge.svg" alt="CI"></a>
<a href="https://github.com/konih/kollect/actions/workflows/preflight.yaml"><img src="https://github.com/konih/kollect/actions/workflows/preflight.yaml/badge.svg" alt="Preflight"></a>
<br />
<a href="https://github.com/konih/kollect/actions/workflows/docs.yaml"><img src="https://github.com/konih/kollect/actions/workflows/docs.yaml/badge.svg" alt="Docs"></a>
<a href="https://github.com/konih/kollect/actions/workflows/codeql.yaml"><img src="https://github.com/konih/kollect/actions/workflows/codeql.yaml/badge.svg" alt="CodeQL"></a>
<a href="https://github.com/konih/kollect/releases"><img src="https://img.shields.io/github/v/tag/konih/kollect?label=release" alt="Release"></a>
<a href="https://codecov.io/gh/konih/kollect"><img src="https://codecov.io/gh/konih/kollect/graph/badge.svg" alt="codecov"></a>
<br />
<a href="https://github.com/konih/kollect/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
<a href="https://pkg.go.dev/github.com/konih/kollect"><img src="https://img.shields.io/github/go-mod/go-version/konih/kollect" alt="Go"></a>
<a href="https://github.com/konih/kollect/pkgs/container/kollect"><img src="https://img.shields.io/badge/ghcr.io-konih%2Fkollect-2496ED?logo=docker&logoColor=white" alt="Container"></a>
</p>

# Kollect

> **Your cluster, in Git, diffable.** Declare GVK + CEL in CRDs and get a clean, Git-committed
> inventory of anything running in your cluster — no scripts, no apiserver hammering. When the
> cluster changes, the inventory commits change; `git log` is your audit trail and `git diff` is
> your drift report. The same snapshot fans out to Postgres, object stores, and event streams —
> consumers read **export data**, never unbounded list/watch against the live cluster.

<!-- Hero GIF (generate locally): docs/assets/demo/hero-git-only.gif — see docs/DEMO-GIF-GUIDE.md -->

Kubernetes is the source of truth for *what is running*; it is a poor *system of record* for
stakeholder inventory. Kollect closes that gap: **select** resources by GVK → **extract** attributes
(CEL or JSONPath) → **aggregate** across targets → **debounce** → **export** to pluggable sinks.
Inventory is **configuration, not code** — owned per team in its own namespace, GitOps-friendly from
day one.

**Read the docs:** **[konih.github.io/kollect](https://konih.github.io/kollect/)** — architecture,
quick start, CR reference, ADRs, and examples. This README is the front door; the site is the map.

> **Pre-beta.** APIs and defaults may change until the first release candidate. See the
> [roadmap](https://konih.github.io/kollect/ROADMAP/) for current status.

## Why Kollect?

- **Decoupled read model** — consumers query a sink, not the apiserver. No RBAC blast radius, no
  watch-storm risk, no etcd size limits ([why](https://konih.github.io/kollect/adr/0103-etcd-limit/)).
- **Event-driven, no polling** — one shared informer per GVK keeps inventory current as the cluster
  changes ([ADR-0301](https://konih.github.io/kollect/adr/0301-event-driven-informers/)).
- **Schema-flexible** — declare the attributes you want in a `KollectProfile`; no bespoke collector
  per resource kind.
- **Pluggable sinks, no privileged backend** — the same snapshot fans out to Git, Postgres, object
  store, or an event stream ([sink taxonomy](https://konih.github.io/kollect/adr/0401-sink-taxonomy-state-vs-stream/)).
- **Multi-tenant by design** — `KollectScope` gates which teams, namespaces, and sinks each tenant
  may use.
- **Fleet-ready** — **N single-mode operators → one shared sink**, partitioned by `spec.cluster`; no
  central hub tier to operate ([ADR-0501](https://konih.github.io/kollect/adr/0501-multi-cluster-fleet/)).
- **Built for scale** — a **10,000-row baseline validated in CI**, a **100,000-row design target**
  per cluster with export sharding, plus tunable reconcile/dispatch concurrency
  ([performance](https://konih.github.io/kollect/PERFORMANCE/)).

## See it end-to-end

A real pipeline is a handful of Kubernetes resources. This is the
[Deployment-inventory walkthrough](https://konih.github.io/kollect/examples/deployment-inventory/) —
collect container images from Deployments and export them to Postgres (for portals) and Git (for
audit) at the same time:

```mermaid
flowchart LR
  Profile["<b>KollectProfile</b><br/>Deployment schema"]
  Target["<b>KollectTarget</b><br/>select Deployments"]
  Inv["<b>KollectInventory</b><br/>aggregate · debounce · export"]
  Snap["<b>KollectSnapshotSink</b>"]
  Db["<b>KollectDatabaseSink</b>"]
  Ev["<b>KollectEventSink</b>"]
  K8s[("Kubernetes API")]

  Profile --> Target
  K8s -- "informer per GVK" --> Target
  Target --> Inv
  Inv --> Snap
  Inv --> Db
  Inv --> Ev
  Snap --> SnapOut["Git · GitLab · S3 · GCS"]
  Db --> DbOut["Postgres · MongoDB"]
  Ev --> EvOut["Kafka"]
```

## Quick start (MVP)

Spin up the full pipeline on a local kind cluster in one command (needs Docker, kind, kubectl, and
[Task](https://taskfile.dev/)):

```sh
git clone https://github.com/konih/kollect.git && cd kollect
task dev-up                       # build, create kind cluster, install operator + sample CRs
kubectl get kinv,ktgt,ksnap,kdb -A    # watch the pipeline come up
```

`task dev-up` builds the manager, boots a `kollect-dev` kind cluster, installs the operator, and
applies the sample `Profile → Sink → Target → Inventory` pipeline. Watch the `KollectInventory`
`Ready` condition, then read your sink — the [live demo repo](https://github.com/konih/kollect-inventory-demo)
shows what the Git export looks like.

**Full walkthrough** — prerequisites, Helm install, maturity notes:
**[Quick start →](https://konih.github.io/kollect/QUICKSTART/)**

## How it works

![Kollect operator pipeline from Kubernetes API through shared informers, in-memory collect store, and debounced KollectInventory export to Git, GitLab, S3, GCS, Postgres, MongoDB, and Kafka sink projections.](docs/assets/illustrations/readme-how-it-works-dark.webp)

The in-memory snapshot per inventory is **canonical**; every sink is a **projection** of it — no
single backend is privileged ([sink roles](https://konih.github.io/kollect/adr/0401-sink-taxonomy-state-vs-stream/)).
Sinks are split into three CRD families ([ADR-0414](https://konih.github.io/kollect/adr/0414-sink-family-crds/)):

| Sink family | Examples | Good for |
| --- | --- | --- |
| **`KollectSnapshotSink`** | Git, GitLab, S3, GCS | Audit, diff, GitOps-friendly history |
| **`KollectDatabaseSink`** | Postgres, MongoDB | Rich queries for portals and dashboards |
| **`KollectEventSink`** | Kafka | Change streams, downstream consumers |

Full payload lives in sinks; CR `.status` holds summaries only ([etcd limits](https://konih.github.io/kollect/adr/0103-etcd-limit/)).

## Performance

Kollect is built for **large single clusters** and **multi-cluster fleets**, with honest, tested
targets ([ADR-0603](https://konih.github.io/kollect/adr/0603-performance-scalability/)):

| Tier | Scope | Collected rows | Status |
| --- | --- | --- | --- |
| **Baseline** | 1 cluster | **10,000+** | Validated in nightly load tests |
| **Design target** | 1 cluster | **100,000** | Requires export sharding + Postgres bulk upsert + `resourcesProfile: large` |
| **Fleet** | Shared sink | 10k–100k × **N** operators | Partitioned by `spec.cluster`; no hub merge tier |

Tuning knobs — reconcile/dispatch concurrency, export debounce (`exportMinInterval`, default `30s`),
namespace-scoped informers, Git commit fingerprinting, and `maxExportBytes` caps — are catalogued in
the **[performance guide](https://konih.github.io/kollect/PERFORMANCE/)**.

## Learn more

| Topic | Link |
| --- | --- |
| Problem statement, CRD model, reconciliation | [Architecture](https://konih.github.io/kollect/ARCHITECTURE/) |
| Locked platform decisions | [Platform decisions](https://konih.github.io/kollect/PLATFORM-DECISIONS/) |
| CR fields, RBAC, failure modes | [CR reference](https://konih.github.io/kollect/CR-REFERENCE/) |
| Multi-cluster fleet | [ADR-0501](https://konih.github.io/kollect/adr/0501-multi-cluster-fleet/) |
| Sink taxonomy (state vs stream) | [ADR-0401](https://konih.github.io/kollect/adr/0401-sink-taxonomy-state-vs-stream/) |
| Build-order phases and status | [Roadmap](https://konih.github.io/kollect/ROADMAP/) |
| Examples index | [Examples](https://konih.github.io/kollect/examples/) |
| Example: Deployment → Git export | [Walkthrough](https://konih.github.io/kollect/examples/deployment-inventory/) |
| Live demo inventory (Git sink) | [kollect-inventory-demo](https://github.com/konih/kollect-inventory-demo) |

Developers: run `task lint`, `task test`, and `task verify` before opening a PR —
[CONTRIBUTING.md](CONTRIBUTING.md).

## Community

| | |
| --- | --- |
| **Contributing** | [CONTRIBUTING.md](CONTRIBUTING.md) — DCO, PR workflow, good first tasks |
| **Code of Conduct** | [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — Contributor Covenant v2.1 |
| **Governance** | [GOVERNANCE.md](GOVERNANCE.md) — roles, decisions, continuity |

## Security

Report vulnerabilities privately — see [SECURITY.md](SECURITY.md). Security architecture:
[docs/ASSURANCE-CASE.md](docs/ASSURANCE-CASE.md).

## License

Copyright (c) 2026 Konrad Heimel. Licensed under the [MIT License](LICENSE).
