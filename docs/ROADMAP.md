# Kollect roadmap

Kollect is a Kubernetes inventory exporter: select resources by GVK, extract attributes with CEL
or JSONPath, aggregate a canonical snapshot, and send it to one or more sinks.

**Last verified:** 2026-08-03 against **v0.15.0**. For exact shipped changes, use the
[changelog](https://github.com/platformrelay/kollect/blob/main/CHANGELOG.md). For proposals that
have not entered a release, see [Planned features](roadmap/planned-features.md).

!!! warning "Pre-1.0 API"
    Kollect uses a `v1alpha1` API. Breaking API or default changes may ship in minor releases
    before 1.0. Release notes and migration guidance call them out.

## Shipped in v0.14.0

The current release includes:

- Optional `--allow-private-sinks` opt-in (default off, cluster-admin only) so in-cluster
  (RFC1918/ULA) sink backends can be targeted without disabling SSRF protections
  ([security policy](security/resolved-address-policy.md)).
- Git snapshot export that falls back to a writable temp directory when the process cache root is
  read-only (Talos / `readOnlyRootFilesystem` installs).
- Controller image tags published under the `v`-prefixed layout so Helm OCI chart tags no longer
  collide with the manager image on GHCR.

- Event-driven collection with shared dynamic informers.
- Namespaced and cluster-wide inventory pipelines with `KollectScope` policy boundaries.
- CEL and JSONPath extraction, aggregation, deduplication, redaction, and full-resource export.
- Parallel fan-out to snapshot, database, and event sink families.
- Git, GitLab, S3, GCS, Postgres, MongoDB, BigQuery, Kafka, and NATS backends.
- Per-sink retries, circuit breakers, export intervals, connection testing, and status summaries.
- Helm packaging, signed release artifacts, SBOMs, provenance, and CI security gates.
- Pipeline CLI for kubeconfig-based collection without installing the operator
  ([guide](guides/pipeline-cli.md), [ADR-0801](adr/0801-pipeline-cli-mode.md)), including streaming
  collected output to standard output for local inspection and debugging.

The [CR reference](crds/index.md) describes the supported API. The
[operator manual](operator-manual/index.md) covers installation, operation, and failure modes.

## Independent lab validation (v0.14.0)

A multi-node Talos lab (1 control plane + 2 workers) exercised published **v0.14.0** and returned
**ready with conditions** — not a full catalogue pass:

| Proven on that pin | Explicitly **not** claimed |
| --- | --- |
| HA: two replicas on distinct workers; leader failover observed | Wave-4 / Tier-S load (500–2k) |
| ClusterIP Postgres / MinIO / NATS with `allowPrivateSinks: true` | 100k rows/cluster design proof |
| Inventory export to Postgres; GitHub + GitLab snapshot export | Full Ubuntu D-suite / every sink backend |
| Certificate scrape non-zero (partial count vs live cluster) | NetPol/Cilium deny-path, worker drain, Argo scrape |

See [testing](development/testing.md#multi-node-lab-evidence) and
[resolved-address policy](security/resolved-address-policy.md). Broader schedules and redacted
protocol publication remain follow-up work.

## Next

Near-term work is deliberately narrow:

1. **Launch documentation** — keep public claims and the published security architecture aligned
   with releases, improve navigation and diagrams, and establish freshness checks.
2. **Independent validation** — deepen multi-node lab coverage (load, failure injection, NetPol)
   and publish only reproducible, bounded claims.
3. **Pre-1.0 stabilization** — prioritize compatibility, operator ergonomics, upgrade guidance,
   and production evidence over adding broad new product surfaces.

Items are not assigned to a release until their implementation, tests, and documentation are ready.

## Supported & planned sinks

“Core” and “Beta” describe maturity, not whether code exists. All Core and Beta rows below ship in
v0.14.0.

| Family CRD | `spec.type` | Maturity |
| --- | --- | --- |
| `KollectSnapshotSink` | `git`, `gitlab`, `s3` | **Core** |
| `KollectSnapshotSink` | `gcs` | **Beta** |
| `KollectDatabaseSink` | `postgres` | **Core** |
| `KollectDatabaseSink` | `mongodb`, `bigquery` | **Beta** |
| `KollectEventSink` | `kafka`, `nats` | **Beta** |
| `KollectSnapshotSink` | `azureblob` | **Planned** — no backend ships today |
| `KollectSnapshotSink` | S3/GCS `serialization.format: parquet` | **Beta** — shipped output mode |

The source of truth for accepted values is the
[CR reference](crds/index.md#kinds). Planned backends are not accepted by admission
until a real implementation and test coverage land.

## Read API (design-only)

The optional HTTP Read API and the fleet read-plane design ([ADR-0418](adr/0418-fleet-console-read-plane.md))
are **not** part of the recommended adoption path. There is no shipped browser console; operators
should use sink exports (Git, databases, object storage) as the product surface.

## Later

These directions remain useful but are not scheduled:

- `v1beta1` API design and conversion-webhook strategy.
- Parquet compaction and richer query guidance for the shipped S3/GCS output mode.
- Azure Blob Storage after a real backend, emulator coverage, and credential model exist.
- Richer target- and inventory-scoped metrics.
- OpenTelemetry tracing after the metric surface and cardinality policy stabilize.
- Additional collection filters and reusable policy abstractions.

See [Planned features](roadmap/planned-features.md) for scope and graduation rules.

## Performance and scalability

Performance targets and test methods live in the [performance guide](operator-manual/performance.md). Treat scale
figures as design targets unless the page links a reproducible run for the exact release. Multi-node
lab evidence for **v0.14.0** covers HA and selected sinks under the conditions above — **not**
Wave-4 load or the 100k design proof.

The architecture scales through shared informers, debounced exports, bounded concurrency, export
sharding, and fleet fan-in through shared sinks rather than a central control-plane hub.

## Rejected or intentionally absent

- Storing full inventory payloads in CR `.status` or etcd.
- A central fleet hub that watches every member cluster.
- Prometheus as an inventory export sink.
- In-tree Confluence or documentation-sync publication.
- Browser access to Kubernetes, database, or event-bus credentials.

The [ADR index](adr/README.md) records the reasoning and current status of architectural choices.

## How roadmap changes

A roadmap item graduates only when:

1. its design is accepted where an ADR or RFC is warranted;
2. implementation and relevant test layers are complete;
3. user and operator documentation match the shipped behavior; and
4. the changelog and release notes identify compatibility impact.

Historical release detail belongs in the
[changelog](https://github.com/platformrelay/kollect/blob/main/CHANGELOG.md), not this page.

## Further reading

- [Architecture](concepts/architecture.md)
- [Platform decisions](PLATFORM-DECISIONS.md)
- [Planned features](roadmap/planned-features.md)
- [Release process](RELEASE.md)
- [ADR index](adr/README.md)
