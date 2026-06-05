# ADR-0702: Doc-sync, templating, and Confluence publication

> Kollect collects and exports; rendering/publishing to a CMS stays out of the operator and belongs in
> external CI that consumes the exports.

**Theme:** 07 · Project & meta · **Status:** Dropped (binding scope guardrail)

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
| Collect + export inventory | **Kollect** (`KollectTarget` → `KollectInventory` → sinks) |
| Template + publish to Confluence/wiki | **External** CI or doc tool consuming Git/object-store export |

Rationale (single responsibility):

- The operator **collects and exports** auditable inventory snapshots.
- **Templating and Confluence push** add CMS credentials, idempotent page merge, and content drift
  unrelated to Kubernetes watches.
- Git (or Postgres/Kafka) export already satisfies portal and audit needs ([ADR-0102](0102-prior-art.md)).

## Consequences

### Positive

- Smaller security surface (no Confluence tokens in the operator).
- Faster path to production sinks (Git, S3, Postgres, Kafka — [ADR-0402](0402-sink-backends-database-kafka.md)).
- Clear boundary for platform teams: export contract in Git JSON; render elsewhere.

### Negative

- No one-click Confluence update from a CR; teams must wire CI to exported artifacts.
- `kpub` short name and any Publication samples remain **documentation-only rejected** references.

## Scope creep guardrails (binding)

Any feature that smells like doc-sync must be **rejected** unless it is plain inventory export:

| In scope (Kollect) | Out of scope (external CI / portal) |
| --- | --- |
| Deterministic JSON/YAML/row export to Git, S3, GCS, Postgres, Kafka | Go templates, Confluence REST, wiki merge, CMS credentials |
| Read-only HTTP `GET /inventory` | Rich portal UI, auth beyond [ADR-0404](0404-inventory-api-auth.md) |
| `checksum`, `generation`, `itemCount` in export metadata | Rendered HTML/Markdown publication pipelines |
| Stable sort keys for `git diff` | Idempotent page upsert, attachment handling |

**Review gate:** before adding a reconciler or sink field, ask: *does this render or publish content
to a human CMS?* If yes → **reject** and document the external pipeline pattern instead.

**Platform review (2026-06-05):** guardrails **reaffirmed** — no scope creep into templating,
Confluence, or rendered publication inside the operator. See [PLATFORM-DECISIONS.md](../PLATFORM-DECISIONS.md).

Rejected kind names and short names (`KollectPublication`, `kpub`) must not reappear in codegen,
samples, or public docs except as historical rejection notes.

## See also

- [ADR-0201: CRD model](0201-crd-model.md) — `KollectPublication` listed under rejected kinds
- [ADR-0102: Prior art](0102-prior-art.md) — Publication stance updated to rejected
- [ADR-0402: Database and Kafka sinks](0402-sink-backends-database-kafka.md) — in-scope export targets
