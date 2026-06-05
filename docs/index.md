---
hide:
  - navigation
  - toc
---

<div class="kollect-hero" markdown="1">

# Kollect

**Generic Kubernetes inventory export** — watch any GVK, extract fields with CEL or JSONPath, and
export live cluster state to **Postgres**, **Kafka**, **Git**, and more.

`kollect.dev/v1alpha1` · event-driven · CRD-native · hub/spoke ready

[Quick start :octicons-arrow-right-24:](QUICKSTART.md){ .md-button .md-button--primary }
[CR reference :octicons-arrow-right-24:](CR-REFERENCE.md){ .md-button }

</div>

## What Kollect does

Kollect is a Kubernetes operator that **collects inventory from arbitrary resources**, **aggregates
across targets (and clusters)**, and **exports auditable snapshots** so portals and automation query
durable export data instead of scraping the API at scale.

<div class="kollect-grid" markdown="1">

<div class="kollect-card" markdown="1">

### :material-radar: Event-driven

Dynamic informers react to changes — inventory stays current without polling loops.

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

### :material-hub: Hub / spoke

Run a central hub that aggregates inventory from spoke clusters via `KollectClusterTarget`.

</div>

</div>

## Documentation map

| Section | Start here |
| --- | --- |
| **Understand the basics** | [Architecture](ARCHITECTURE.md) · [Data flows](DATA-FLOWS.md) · [Platform decisions](PLATFORM-DECISIONS.md) |
| **Core concepts** | [CRD model](adr/0201-crd-model.md) · [CR reference](CR-REFERENCE.md) · [Hub and spoke](adr/0501-multi-cluster-sync-rfc.md) |
| **Getting started** | [Quick start](QUICKSTART.md) · [Development setup](DEVELOPMENT.md) |
| **User guide** | [Deployment inventory example](examples/deployment-inventory.md) · [Performance tuning](PERFORMANCE.md) |
| **Reference** | [Custom resources](CR-REFERENCE.md) · [ADRs](adr/README.md) |
| **Contributing** | [Roadmap](ROADMAP.md) · [Release process](RELEASE.md) |

## Examples

- [Deployment inventory → Postgres/Kafka](examples/deployment-inventory.md)
- [Helm release inventory (Argo primary; Flux secondary)](examples/helm-release-inventory.md)
