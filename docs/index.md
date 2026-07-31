---
hide:
  - navigation
  - toc
  - title
---

<div class="kollect-hero" markdown="1">

![Kollect — durable Kubernetes inventory](assets/branding/kollect-wordmark-dark.svg){ .kollect-hero-logo }

<p class="kollect-badges" markdown="1">

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/PlatformRelay/Kollect/badge)](https://securityscorecards.dev/viewer/?uri=github.com/PlatformRelay/Kollect)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13106/badge)](https://www.bestpractices.dev/projects/13106)
[![CodeQL](https://github.com/platformrelay/kollect/actions/workflows/codeql.yaml/badge.svg)](https://github.com/platformrelay/kollect/actions/workflows/codeql.yaml)
[![Release](https://img.shields.io/github/v/release/platformrelay/kollect?label=release)](https://github.com/platformrelay/kollect/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/platformrelay/kollect/blob/main/LICENSE)

</p>

**Kubernetes knows what's running. Kollect makes it a record.** Declare what matters in a few CRs
and get a durable, always-current inventory wherever your platform needs it — a Git history you
can `diff`, a database your portal can query, an event stream your automation can react to. Start
with one Git repo; grow to multi-tenant fan-out across teams without rebuilding anything.

Record the hero demo locally: [DEMO-GIF-GUIDE.md](DEMO-GIF-GUIDE.md).

*Git-simple to start · platform-grade to grow* — `kollect.dev/v1alpha1` · event-driven · CRD-native · fleet-ready

[Quick start :octicons-arrow-right-24:](getting-started/install.md){ .md-button .md-button--primary }
[CR reference :octicons-arrow-right-24:](crds/index.md){ .md-button }

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

!!! warning "Pre-1.0 API"
    Kollect uses a `v1alpha1` API. Breaking API or default changes may ship in minor releases
    before 1.0; release notes and migration guidance call them out. See the
    [roadmap](ROADMAP.md) for current maturity.

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

```mermaid
flowchart LR
  API["Kubernetes API"] -->|watch| Informers["Shared informers<br/>per GVK"]
  Informers --> Store["Canonical<br/>inventory snapshot"]
  Store -->|debounce| Inventory["KollectInventory"]
  Inventory --> Snapshot["Git · GitLab<br/>S3 · GCS"]
  Inventory --> Database["Postgres · MongoDB<br/>BigQuery"]
  Inventory --> Event["Kafka · NATS"]
```

The in-memory snapshot per inventory is **canonical**; every sink is a **projection** of it — no
single backend is privileged. Sink roles (snapshot store, relational store, event emitter) are
documented in [ADR-0401](adr/0401-sink-taxonomy-state-vs-stream.md); reconciliation detail in
[Architecture](concepts/architecture.md) and [Data flows](concepts/export-pipeline.md).

### Supported & planned sinks

| Family CRD | `spec.type` | Status |
| --- | --- | --- |
| `KollectSnapshotSink` | `git`, `gitlab`, `s3` | **Core** — production-ready |
| `KollectSnapshotSink` | `gcs` | **Beta** — shipped, maturing |
| `KollectDatabaseSink` | `postgres` | **Core** |
| `KollectDatabaseSink` | `mongodb`, `bigquery` | **Beta** |
| `KollectEventSink` | `kafka`, `nats` | **Beta** |
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

Full fields: [CR reference](crds/index.md) · model rationale: [ADR-0201](adr/0201-crd-model.md).

## Performance

Kollect is designed for **large single clusters** and **multi-cluster fleets**. The
[performance guide](operator-manual/performance.md) distinguishes reproducible results from design targets and
catalogues tuning knobs. Fleet fan-in uses shared sinks rather than a hub merge tier
([ADR-0603](adr/0603-performance-scalability.md)).

## Documentation map

| Section | Start here |
| --- | --- |
| **Getting started** | [Quick start](getting-started/install.md) · [Development setup](development/setup.md) · [Examples](examples/README.md) |
| **Core concepts** | [CRD model](adr/0201-crd-model.md) · [CR reference](crds/index.md) · [Multi-cluster fleet](adr/0501-multi-cluster-fleet.md) |
| **Operator manual** | [Install & ops](operator-manual/index.md) · [Upgrading](operator-manual/upgrading.md) · [Helm values](operator-manual/helm-values.md) |
| **Performance & ops** | [Performance tuning](operator-manual/performance.md) · [Scaling & fleet](operator-manual/performance.md) · [Best practices](operator-manual/production-checklist.md) · [Troubleshooting](operator-manual/troubleshooting.md) |
| **Background** | [Prerequisites & basics](concepts/resource-model.md) · [Architecture](concepts/architecture.md) ([package graph](architecture-graph.svg)) · [Data flows](concepts/export-pipeline.md) |
| **Reference** | [Custom resources](crds/index.md) · [FAQ](operator-manual/troubleshooting.md) · [ADRs](adr/README.md) · [RFCs](rfc/README.md) |
| **Contributing** | [Roadmap](ROADMAP.md) · [Planned features](roadmap/planned-features.md) · [ADR/RFC process](development/adr-rfc-process.md) · [Release process](RELEASE.md) |

## Try an example

- [Deployment inventory → Git / Postgres / Kafka](getting-started/first-inventory.md) — the end-to-end walkthrough
- [Postgres state store (relational SoR)](examples/postgres-state-store.md)
- [NATS event sink](examples/nats-event-sink.md)
- [Helm release inventory (Argo primary; Flux secondary)](examples/helm-release-inventory.md)
- [Live demo inventory exported to Git](https://github.com/konih/kollect-inventory-demo) — see real output

## Go deeper

- [Platform decisions](PLATFORM-DECISIONS.md) — the locked design summary
- [Sink taxonomy: state vs stream](adr/0401-sink-taxonomy-state-vs-stream.md) — why no backend is privileged
- [Read-only UI console (frozen preview)](operator-manual/ui.md) — maintenance-only and disabled by default
- [Roadmap](ROADMAP.md) — shipped, next, and later work
