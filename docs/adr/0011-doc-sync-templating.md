# ADR-0011: Doc-sync, templating, and Confluence publication

## Status

**Rejected** (2026-06-05) — out of scope for kollect

## Context

Early plans included a **`KollectPublication`** reconciled kind: render aggregated inventory with Go
templates and push to Confluence or a documentation Git repo (patterns ported from a prior batch
collector). That combined **inventory collection**, **templating**, and **CMS sync** in one operator.

Stakeholders still need rendered documentation, but that workflow fits a **separate pipeline** (e.g.
GitLab CI job reading exported JSON from the Git sink) rather than in-cluster doc backends.

## Decision

**Do not implement** `KollectPublication`, Confluence REST clients, or in-operator Go-template doc
sync.

| Approach | Owner |
| --- | --- |
| Collect + export inventory | **kollect** (`KollectTarget` → `KollectInventory` → sinks) |
| Template + publish to Confluence/wiki | **External** CI or doc tool consuming Git/object-store export |

Rationale (single responsibility):

- The operator **collects and exports** auditable inventory snapshots.
- **Templating and Confluence push** add CMS credentials, idempotent page merge, and content drift
  unrelated to Kubernetes watches.
- Git (or Postgres/Kafka) export already satisfies portal and audit needs ([ADR-0013](0013-prior-art.md)).

## Consequences

### Positive

- Smaller security surface (no Confluence tokens in the operator).
- Faster path to production sinks (Git, S3, Postgres, Kafka — [ADR-0025](0025-sink-backends-database-kafka.md)).
- Clear boundary for platform teams: export contract in Git JSON; render elsewhere.

### Negative

- No one-click Confluence update from a CR; teams must wire CI to exported artifacts.
- `kpub` short name and any Publication samples remain **documentation-only rejected** references.

## See also

- [ADR-0004: CRD model](0004-crd-model.md) — `KollectPublication` listed under rejected kinds
- [ADR-0013: Prior art](0013-prior-art.md) — Publication stance updated to rejected
- [ADR-0025: Database and Kafka sinks](0025-sink-backends-database-kafka.md) — in-scope export targets
