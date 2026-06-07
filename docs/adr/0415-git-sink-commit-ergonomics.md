# ADR-0415: Git sink commit ergonomics

> Rich commit messages and templates for Git snapshot exports — readable `git log` without
> opening every JSON diff.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

Git snapshot sinks are audit / GitOps projections, not the query system of record
([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)). Operators and reviewers interact with exports
primarily through **commit history**, merge request titles, and diffs.

[ADR-0407](0407-git-object-store-layout.md) documents object paths and push workflow. Commit
identity and message templates were under-specified relative to data already present in the export
envelope ([ADR-0405](0405-export-data-contract.md)).

## Decision

### Commit context from export envelope

At export time the operator builds a `CommitContext` from:

- Inventory namespace and name (from object path)
- `spec.cluster` and envelope `cluster`
- Inventory CR `metadata.generation` / envelope `generation`
- Envelope `itemCount`, `checksum`, `exportedAt`
- Rendered object path and sink name

Commit rendering **never** infers `{generation}` from Parquet hive path segments alone.

### Default subject (no configuration required)

When `spec.git.commitMessage` is unset:

```text
chore({cluster}/{namespace}/{name}): export {itemCount} items @ {checksumShort}
```

`{checksumShort}` is the first 12 hex characters of the envelope checksum. PoC git sinks produce
actionable log lines without extra CR fields.

### Template placeholders

Subject (`commitMessage`), body (`commitBody`), and trailers (`commitTrailers`) support:

| Placeholder | Source |
| --- | --- |
| `{cluster}` | envelope or `spec.cluster` |
| `{namespace}` | inventory namespace |
| `{name}` | inventory name |
| `{generation}` | inventory / envelope generation |
| `{exportGeneration}` | envelope generation (alias for future split) |
| `{itemCount}` | envelope |
| `{checksum}` | full envelope checksum |
| `{checksumShort}` | first 12 hex of checksum |
| `{exportedAt}` | RFC3339 UTC |
| `{sink}` | sink ref name |
| `{path}` | rendered object path |

### Commit body and trailers

- `spec.git.commitBody` — optional multi-line body (same placeholders).
- `spec.git.commitTrailers` — optional git trailers (e.g. `Kollect-Checksum: {checksum}`).
- go-git and CLI engines write subject + body + trailers as separate commit paragraphs.

### Author identity

Default author remains `kollect <kollect@kollect.dev>`; overridable via `spec.git.author`.

## Consequences

- Git log is searchable by inventory identity, item count, and checksum prefix.
- Breaking default message change is acceptable in pre-alpha (`v0.3.x`).
- GitLab MR description templates and shared `internal/sink/attribution` package are deferred
  ([ADR-0407](0407-git-object-store-layout.md) cross-link).

## Deferred (future ADRs / versions)

| Item | Notes |
| --- | --- |
| `status.sinkExports[].lastCommit` | v0.4.x — SHA write-back after push |
| `branchWorkflow` on plain git | v0.4.x — feature branch parity with GitLab MR mode |
| Persistent mirror (`spec.git.mirror`) | PERF-10 — replace clone-per-export |
| GitLab `descriptionTemplate` | shared template engine with git |
| Coalesced fleet commits | one commit per inventory export (no hub tier) |

## Related

- [ADR-0407](0407-git-object-store-layout.md) — paths, push policy, branch workflow overview
- [ADR-0405](0405-export-data-contract.md) — envelope fields wired into commits
- [ADR-0413](0413-export-interval-scheduling.md) — debounced vs material-change exports
