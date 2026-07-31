# Kollect roadmap

Kollect is a Kubernetes inventory exporter: select resources by GVK, extract attributes with CEL
or JSONPath, aggregate a canonical snapshot, and send it to one or more sinks.

**Last verified:** 2026-07-31 against **v0.11.0**. For exact shipped changes, use the
[changelog](https://github.com/platformrelay/kollect/blob/main/CHANGELOG.md). For proposals that
have not entered a release, see [Planned features](roadmap/planned-features.md).

!!! warning "Pre-1.0 API"
    Kollect uses a `v1alpha1` API. Breaking API or default changes may ship in minor releases
    before 1.0. Release notes and migration guidance call them out.

## Shipped in v0.11.0

The current release includes:

- Optional `--allow-private-sinks` opt-in (default off, cluster-admin only) so in-cluster
  (RFC1918/ULA) sink backends can be targeted without disabling SSRF protections
  ([security policy](security/resolved-address-policy.md)).

- Event-driven collection with shared dynamic informers.
- Namespaced and cluster-wide inventory pipelines with `KollectScope` policy boundaries.
- CEL and JSONPath extraction, aggregation, deduplication, redaction, and full-resource export.
- Parallel fan-out to snapshot, database, and event sink families.
- Git, GitLab, S3, GCS, Postgres, MongoDB, BigQuery, Kafka, and NATS backends.
- Per-sink retries, circuit breakers, export intervals, connection testing, and status summaries.
- Helm packaging, signed release artifacts, SBOMs, provenance, and CI security gates.
- Pipeline CLI for kubeconfig-based collection without installing the operator
  ([guide](guides/pipeline-cli.md), [ADR-0801](adr/0801-pipeline-cli-mode.md)).

The [CR reference](CR-REFERENCE.md) describes the supported API. The
[operator manual](OPERATOR-MANUAL.md) covers installation, operation, and failure modes.

## Next

Near-term work is deliberately narrow:

1. **Launch documentation** — keep public claims and the published security architecture aligned
   with releases, improve navigation and diagrams, and establish freshness checks.
2. **Independent validation** — incorporate reproducible lab results when the evidence is merged;
   do not publish provisional numbers.
3. **Pre-1.0 stabilization** — prioritize compatibility, operator ergonomics, upgrade guidance,
   and production evidence over adding broad new product surfaces.

Items are not assigned to a release until their implementation, tests, and documentation are ready.

## Supported & planned sinks

“Core” and “Beta” describe maturity, not whether code exists. All Core and Beta rows below ship in
v0.10.0.

| Family CRD | `spec.type` | Maturity |
| --- | --- | --- |
| `KollectSnapshotSink` | `git`, `gitlab`, `s3` | **Core** |
| `KollectSnapshotSink` | `gcs` | **Beta** |
| `KollectDatabaseSink` | `postgres` | **Core** |
| `KollectDatabaseSink` | `mongodb`, `bigquery` | **Beta** |
| `KollectEventSink` | `kafka`, `nats` | **Beta** |
| `KollectSnapshotSink` | `azureblob` | **Planned** — no backend ships today |
| S3 / GCS layout | Parquet | **Planned** |

The source of truth for accepted values is the
[CR reference](CR-REFERENCE.md#kinds). Planned backends are not accepted by admission
until a real implementation and test coverage land.

## Read API + UI console (frozen)

The bundled preview UI and its wider read-plane program are **frozen**. They are not part of the
recommended adoption path and should not be treated as a stable product surface. The operator,
export contract, and sink integrations remain the focus.

Current installations should leave `ui.enabled: false`, which is the chart default. See the
[UI operator note](operator-manual/ui.md) for the present preview status.

## Later

These directions remain useful but are not scheduled:

- `v1beta1` API design and conversion-webhook strategy.
- Parquet layout for object-store snapshots.
- Azure Blob Storage after a real backend, emulator coverage, and credential model exist.
- Richer target- and inventory-scoped metrics.
- OpenTelemetry tracing after the metric surface and cardinality policy stabilize.
- Additional collection filters and reusable policy abstractions.

See [Planned features](roadmap/planned-features.md) for scope and graduation rules.

## Performance and scalability

Performance targets and test methods live in the [performance guide](PERFORMANCE.md). Treat scale
figures as design targets unless the page links a reproducible run for the exact release; current
lab validation is still in progress.

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

- [Architecture](ARCHITECTURE.md)
- [Platform decisions](PLATFORM-DECISIONS.md)
- [Planned features](roadmap/planned-features.md)
- [Release process](RELEASE.md)
- [ADR index](adr/README.md)
