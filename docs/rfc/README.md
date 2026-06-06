# Request for Comments (RFCs)

Optional **pre-ADR** design documents for Kollect. RFCs hold exploration, option comparisons, and
review feedback before a decision is locked in a numbered
[Architecture Decision Record](../adr/README.md).

**Process guide:** [ADR and RFC process](../development/adr-rfc-process.md)

## When to add an RFC here

| Situation | Prefer |
| --- | --- |
| Comparing multiple architectures with long trade-off tables | RFC in `docs/rfc/` |
| Decision ready to merge with the codebase | ADR in `docs/adr/0Txx-….md` |
| Small extension of an existing decision | Update the existing ADR |

Many Kollect records skip this directory and land directly as ADRs with status **Exploring** — that
is fine. Use `docs/rfc/` when the write-up would clutter the ADR index or needs iteration before a
number is assigned.

## Naming and lifecycle

- **Filename:** `docs/rfc/<kebab-case-topic>.md` — no numeric prefix.
- **Status:** `Draft` → `Review` → `Accepted` | `Withdrawn` | `Superseded`.
- **Promotion:** when accepted, add **ADR-0Txx** in the appropriate theme, link from the RFC header,
  and set RFC status to **Superseded**.

## Active RFCs

| RFC | Topic | Status |
| --- | --- | --- |
| [Prometheus attribute metrics](prometheus-attribute-metrics.md) | Scalar gauge/counter export from collected numeric attributes (Tier C′) | Proposed (Exploring) |

Exploring ADRs that began as RFC-style work:

| ADR | Title |
| --- | --- |
| [ADR-0304](../adr/0304-custom-resource-aggregation-rfc.md) | Custom-resource metrics and richer aggregation |
| [ADR-0501](../adr/0501-multi-cluster-sync-rfc.md) | Multi-cluster sync topology |

## Backlog without RFCs yet

Items tracked on [Planned features](../roadmap/planned-features.md) until a draft opens here or in
`docs/adr/`:

- BigQuery sink (spec needed)
- Azure Blob Storage sink (spec needed)

Recently promoted to Exploring ADRs (formerly backlog):

| ADR | Topic |
| --- | --- |
| [ADR-0604](../adr/0604-target-scoped-prometheus-metrics.md) | Target- and inventory-scoped Prometheus metrics |
| [ADR-0605](../adr/0605-opentelemetry-tracing.md) | OpenTelemetry tracing |

## See also

- [ADR and RFC process](../development/adr-rfc-process.md)
- [Planned features](../roadmap/planned-features.md)
- [ADR index](../adr/README.md)
