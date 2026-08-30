# Planned features

Forward-looking work that has not shipped. This page is intentionally shorter than the engineering
backlog: it describes user-visible directions, not every implementation task.

**Last verified:** 2026-08-30 against **v0.19.0**. BigQuery and NATS already ship; completed work
belongs in the [changelog](https://github.com/platformrelay/kollect/blob/main/CHANGELOG.md).

## Status legend

| Status | Meaning |
| --- | --- |
| **Next** | Active near-term direction; release assignment waits for implementation evidence |
| **Exploring** | Design or spike exists; scope may change |
| **Deferred** | Valid direction with no near-term commitment |
| **Frozen** | No feature development; maintenance or retirement only |

## Next

### Launch-quality documentation

**Status: Next**

- Keep the published security architecture visible and synchronized with trust-boundary, NetGuard,
  RBAC, redaction, transport, runtime, and release-control changes.
- Replace inaccurate raster architecture diagrams with maintainable Mermaid sources.
- Improve information architecture, newcomer paths, visual contrast, and document freshness checks.
- Incorporate lab results only after the method and evidence are reproducible. Public claims for
  **v0.14.0** multi-node evidence are bounded (**ready with conditions**): HA failover and selected
  sinks — not Wave-4 load, 100k, or a full scenario catalogue
  ([testing](../development/testing.md#multi-node-lab-evidence)).

### Pre-1.0 stabilization

**Status: Next**

Focus on compatibility guidance, upgrades, failure recovery, observability, and production
validation. New surface area should clear a higher bar than hardening existing workflows.

## Sinks & export

### S3/GCS Parquet follow-ups

**Status: Shipped; hardening continues**

S3 and GCS already accept `serialization.format: parquet`. Remaining work is compaction guidance,
schema-evolution documentation, manifest metadata, and broader interoperability evidence.

### Azure Blob Storage sink

**Status: Deferred**

No Azure backend ships today. Graduation requires a concrete authentication model, emulator or
integration coverage, connection tests, redacted errors, and operator documentation.

### Multipart export completeness for Git sinks

**Status: Planned**

Sharded (large) exports written to Git/GitLab in the default YAML layout will emit a per-set
manifest sidecar recording the export generation and expected part count, so operators and CI can
verify from the YAML output that a snapshot set is complete rather than torn or stale. The
JSON/object-store sinks already carry this completeness marker; this extends the same guarantee to
the default YAML sinks.

## API & tenancy

### API v1beta1 and conversion webhook

**Status: Exploring**

Define the compatibility contract, storage-version migration, conversion strategy, and supported
upgrade paths before promoting the API beyond `v1alpha1`.

### Additional reusable policy abstractions

**Status: Deferred**

Potential collection-rule or receiver abstractions must demonstrate that they simplify real
multi-team configurations without weakening `KollectScope` boundaries.

## Read API & UI (frozen)

### Read API contract

**Status: Design-only**

The optional HTTP Read API is not the current adoption path. Its contract is not stable; there is
no shipped browser client, and no client should receive Kubernetes, database, or event-bus credentials.
See [ADR-0418](../adr/0418-fleet-console-read-plane.md) for the fleet read-plane design.

## Observability & performance

### Prometheus metrics scoped to targets / inventory rows

**Status: Exploring**

Expose bounded-cardinality health, row-count, duration, and error signals useful to operators.
Avoid resource identity or extracted attribute values as labels.

### Prometheus metrics from collected attribute values

**Status: Deferred**

This remains high risk because user-selected values can create unbounded cardinality and expose
sensitive data. Any proposal needs explicit allowlists, budgets, and safe defaults.

### OpenTelemetry tracing

**Status: Deferred**

Trace reconciliation and export flow only after metric names, error taxonomy, and cardinality
policy stabilize. See [ADR-0605](../adr/0605-opentelemetry-tracing.md).

## How items graduate

1. Explore in an issue or RFC; use an ADR for a cross-cutting or hard-to-reverse decision.
2. Accept the design and identify compatibility, security, and operational consequences.
3. Implement with the required test layers and documentation.
4. Move shipped behavior to the [roadmap](../ROADMAP.md) and changelog; remove stale planning copy.

## See also

- [Roadmap](../ROADMAP.md)
- [ADR index](../adr/README.md)
- [ADR / RFC process](../development/adr-rfc-process.md)
- [Contributing](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md)
