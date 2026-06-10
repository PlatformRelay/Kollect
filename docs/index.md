---
hide:
  - navigation
  - toc
  - title
---

<div class="kollect-hero" markdown="1">

![Kollect — durable Kubernetes inventory](assets/branding/kollect-logo-stacked-dark.png){ .kollect-hero-logo }

<p class="kollect-badges" markdown="1">

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/konih/kollect/badge)](https://securityscorecards.dev/viewer/?uri=github.com/konih/kollect)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13106/badge)](https://www.bestpractices.dev/projects/13106)
[![CI](https://github.com/konih/kollect/actions/workflows/ci.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/ci.yaml)
[![Preflight](https://github.com/konih/kollect/actions/workflows/preflight.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/preflight.yaml)
<br />
[![Docs](https://github.com/konih/kollect/actions/workflows/docs.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/docs.yaml)
[![CodeQL](https://github.com/konih/kollect/actions/workflows/codeql.yaml/badge.svg)](https://github.com/konih/kollect/actions/workflows/codeql.yaml)
[![Release](https://img.shields.io/github/v/release/konih/kollect?label=release)](https://github.com/konih/kollect/releases)
[![codecov](https://codecov.io/gh/konih/kollect/graph/badge.svg)](https://codecov.io/gh/konih/kollect)
<br />
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/konih/kollect/blob/main/LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/konih/kollect)](https://pkg.go.dev/github.com/konih/kollect)
[![Container](https://img.shields.io/badge/ghcr.io-konih%2Fkollect-2496ED?logo=docker&logoColor=white)](https://github.com/konih/kollect/pkgs/container/kollect)

</p>

**Your cluster, in Git, diffable.** Declare GVK + CEL in CRDs and get a clean, Git-committed
inventory — when the cluster changes, the inventory commits change; `git log` is your audit trail
and `git diff` is your drift report. **Git export is the hero** — Postgres and every other sink get
the same rows in parallel, for free. Portals, automation, and auditors read **export data**, not
unbounded list/watch against the live API.

Record the hero demo locally: [DEMO-GIF-GUIDE.md](DEMO-GIF-GUIDE.md).

`kollect.dev/v1alpha1` · event-driven · CRD-native · fleet-ready

[Quick start :octicons-arrow-right-24:](QUICKSTART.md){ .md-button .md-button--primary }
[CR reference :octicons-arrow-right-24:](CR-REFERENCE.md){ .md-button }

</div>

## What Kollect does

Kubernetes is the source of truth for *what is running*; it is a poor *system of record* for
stakeholder inventory. Kollect maintains a **read model** — live state captured once, then served
from export data:

**Scope** and **Target** select resources by GVK and namespace; **Profile** extracts the attributes
that matter (CEL or JSONPath); **Inventory** rolls up matching objects, **debounces** churn, and
**exports** snapshots to pluggable sinks (Git, object stores, databases, event streams). Every
backend sees the same aggregated rows; sinks are interchangeable projections.

Inventory is **configuration, not code** — owned per team in its own namespace.

!!! warning "Pre-beta"
    APIs and defaults may change until the first release candidate. See the
    [roadmap](ROADMAP.md) for current status.

## Why Kollect?

<div class="kollect-grid" markdown="1">

<div class="kollect-card" markdown="1">

### :material-radar: Event-driven

Shared informers per GVK — inventory stays current without polling loops
([ADR-0301](adr/0301-event-driven-informers.md)).

</div>

<div class="kollect-card" markdown="1">

### :material-cube-outline: CRD-native

Declare profiles, sinks, targets, and inventory in Kubernetes; GitOps-friendly from day one.

</div>

<div class="kollect-card" markdown="1">

### :material-account-group: Multi-tenant

`KollectScope` gates which teams and namespaces can export to which sinks.

</div>

<div class="kollect-card" markdown="1">

### :material-hub: Fleet-ready

Each cluster runs `mode: single` and exports to **shared sinks** with a cluster label
([ADR-0501](adr/0501-multi-cluster-fleet.md)).

</div>

</div>

## How it works

![Left-to-right operator pipeline from Kubernetes API through shared per-GVK informers and an in-memory collect store, KollectInventory debounce, to fan-out sink projections for Git, GitLab, S3, GCS, Postgres, MongoDB, and Kafka.](assets/illustrations/how-it-works-informer-sink-dark.webp){ .kollect-illus .kollect-illus--wide }

The in-memory snapshot per inventory is **canonical**; every sink is a **projection** of it — no
single backend is privileged. Sink roles (snapshot store, relational store, event emitter) are
documented in [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md); reconciliation detail in
[Architecture](ARCHITECTURE.md) and [Data flows](DATA-FLOWS.md).

### Supported & planned sinks

| Family CRD | `spec.type` | Status |
| --- | --- | --- |
| `KollectSnapshotSink` | `git`, `gitlab`, `s3` | **Core** — production-ready |
| `KollectSnapshotSink` | `gcs` | **Beta** — shipped, maturing |
| `KollectDatabaseSink` | `postgres` | **Core** |
| `KollectDatabaseSink` | `mongodb`, `bigquery` | **Beta** — `bigquery` v0.7.x hardening |
| `KollectEventSink` | `kafka`, `nats` | **Beta** — `nats` v0.7.x hardening |
| `KollectSnapshotSink` | `azureblob` | **Planned** |
| Object-store sinks | Parquet layout | **Planned** — on S3/GCS |

Release timing and deferred backends: [Roadmap — Supported & planned sinks](ROADMAP.md#supported-planned-sinks).

## The resource model

A pipeline is just a handful of Kubernetes resources: **config you declare** (`KollectProfile`,
family sinks — `KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink`, `KollectScope`)
and **objects the operator reconciles** (`KollectTarget`, `KollectInventory`). Cluster-scoped
`KollectCluster*` variants add cross-namespace rollup.

```mermaid
flowchart LR
  K8s(["Kubernetes API"]):::api

  subgraph declare["You declare — static config"]
    direction TB
    Profile["<b>KollectProfile</b><br/>what to extract"]
    Scope["<b>KollectScope</b><br/>guardrails"]
    Snap["<b>KollectSnapshotSink</b><br/>snapshot store"]
    Db["<b>KollectDatabaseSink</b><br/>relational SoR"]
    Ev["<b>KollectEventSink</b><br/>event emitter"]
  end

  subgraph run["Operator reconciles"]
    direction TB
    Target["<b>KollectTarget</b><br/>what to watch"]
    Inv["<b>KollectInventory</b><br/>aggregate · debounce · export"]
  end

  subgraph out["Sink projections — choose any"]
    direction TB
    SnapOut["Git · GitLab · S3 · GCS<br/><i>snapshot store</i>"]
    Rel["Postgres · MongoDB<br/><i>relational SoR</i>"]
    EvtOut["Kafka<br/><i>event emitter</i>"]
  end

  K8s -- "informer per GVK" --> Target
  Profile --> Target
  Target --> Inv
  Scope -. gates .-> Target
  Scope -. gates .-> Inv
  Inv --> Snap
  Inv --> Db
  Inv --> Ev
  Snap --> SnapOut
  Db --> Rel
  Ev --> EvtOut

  classDef api fill:#1F2937,stroke:#6B7280,color:#fff;
  classDef config fill:#326CE5,stroke:#1b3a8c,color:#fff;
  classDef work fill:#18B6A3,stroke:#0e6f63,color:#fff;
  classDef proj fill:#7FB3FF,stroke:#326CE5,color:#081A4B;

  class Profile,Scope,Snap,Db,Ev config;
  class Target,Inv work;
  class SnapOut,Rel,EvtOut proj;
```

| Kind | You set | Role |
| --- | --- | --- |
| `KollectProfile` | GVK + CEL / JSONPath attributes | **What to extract** from each object |
| `KollectTarget` | selectors + `profileRef` | **What to watch** and collect |
| `KollectInventory` | family sink refs + cadence | **Aggregate, debounce, and export** |
| `KollectSnapshotSink` | type + endpoint + `secretRef` | **Snapshot store** (Git, GitLab, S3, GCS) |
| `KollectDatabaseSink` | type + credentials | **Relational SoR** (Postgres, MongoDB) |
| `KollectEventSink` | type + brokers | **Event emitter** (Kafka) |
| `KollectScope` | allowed GVKs / namespaces / sinks | **Guardrails** for the team namespace |

Full fields: [CR reference](CR-REFERENCE.md) · model rationale: [ADR-0201](adr/0201-crd-model.md).

## Performance

Kollect is built for **large single clusters** and **multi-cluster fleets**, with honest, tested
targets ([ADR-0603](adr/0603-performance-scalability.md)) — **10,000+** rows validated in nightly
load tests, **100,000-row** design target per cluster, and fleet fan-in with no hub merge tier.
Tuning knobs are catalogued in the [performance guide](PERFORMANCE.md).

## Documentation map

| Section | Start here |
| --- | --- |
| **Getting started** | [Quick start](QUICKSTART.md) · [Development setup](DEVELOPMENT.md) · [Examples](examples/README.md) |
| **Core concepts** | [CRD model](adr/0201-crd-model.md) · [CR reference](CR-REFERENCE.md) · [Multi-cluster fleet](adr/0501-multi-cluster-fleet.md) |
| **Operator manual** | [Install & ops](OPERATOR-MANUAL.md) · [Upgrading](operator-manual/upgrading.md) · [Helm values](operator-manual/helm-values.md) |
| **Performance & ops** | [Performance tuning](PERFORMANCE.md) · [Scaling & fleet](operator-manual/scaling-and-fleet.md) · [Best practices](BEST-PRACTICES.md) · [Troubleshooting](TROUBLESHOOTING.md) |
| **Background** | [Prerequisites & basics](UNDERSTAND-THE-BASICS.md) · [Architecture](ARCHITECTURE.md) ([package graph](architecture-graph.svg)) · [Data flows](DATA-FLOWS.md) |
| **Reference** | [Custom resources](CR-REFERENCE.md) · [FAQ](FAQ.md) · [ADRs](adr/README.md) · [RFCs](rfc/README.md) |
| **Contributing** | [Roadmap](ROADMAP.md) · [Planned features](roadmap/planned-features.md) · [ADR/RFC process](development/adr-rfc-process.md) · [Release process](RELEASE.md) |

## Try an example

- [Deployment inventory → Git / Postgres / Kafka](examples/deployment-inventory.md) — the end-to-end walkthrough
- [Postgres state store (relational SoR)](examples/postgres-state-store.md)
- [NATS event sink](examples/nats-event-sink.md)
- [Helm release inventory (Argo primary; Flux secondary)](examples/helm-release-inventory.md)
- [Live demo inventory exported to Git](https://github.com/konih/kollect-inventory-demo) — see real output

## Go deeper

- [Platform decisions](PLATFORM-DECISIONS.md) — the locked design summary
- [Sink taxonomy: state vs stream](adr/0401-sink-taxonomy-state-vs-stream.md) — why no backend is privileged
- [Read-only UI console (frozen preview)](operator-manual/ui.md) — early adopter SPA; program frozen until v0.7.x+
- [Roadmap](ROADMAP.md) — build-order phases and current status
