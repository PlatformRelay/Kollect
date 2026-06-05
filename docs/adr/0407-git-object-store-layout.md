# ADR-0407: Git and object-store export layout and workflow

> Where exported inventory lands in a repo or bucket, and how the Git/GitLab sinks commit, branch,
> and open merge requests.

**Theme:** 04 · Export & sinks · **Status:** Current

## Context

The Git, GitLab, S3, and GCS backends are snapshot stores ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)):
each export writes the whole inventory document to a path. The path layout is a contract (consumers and
GitOps tooling depend on it) and the Git write workflow (commit vs branch vs MR) is an ergonomics
decision. Both were implemented (`internal/sink/git`, `internal/sink/gitlab`) without an ADR.

## Decision

### Object path layout

The inventory controller passes a canonical object path; backends map it to their store:

```
inventory/<inventory-namespace>/<inventory-name>.json
```

- Git/GitLab/S3/GCS write the JSON payload ([ADR-0405](0405-export-data-contract.md)) at that path.
- Multi-cluster fan-in disambiguates by cluster in the path/key
  (`clusters/<cluster>/…` or a cluster column/header — [ADR-0501](0501-multi-cluster-sync-rfc.md)),
  so concurrent spokes never collide on the same object.

### Git write workflow

`internal/sink/git/export_file.go` clones a single branch into a temp dir, writes the file, and:

- **Commit identity** is fixed: `kollect <kollect@kollect.dev>`, message `kollect: export inventory`.
- **No-op guard**: if `git add` produces no change, the export errors rather than pushing an empty
  commit (idempotent exports stay quiet — [ADR-0406](0406-sink-registry.md)).
- **Force-push** to the push branch (`push --force -u origin`); the export is a **snapshot**, not an
  append-only history — the latest commit is the source of truth.
- **Empty-remote bootstrap**: a fresh/empty repo is `git init`'d and the branch created, so the first
  export works against a bare repository.
- **Custom CA**: trusted via resolved `caPEM` from `BuildContext` ([ADR-0104](0104-security-model.md));
  no `insecureSkipVerify` for Git.

### GitLab merge-request mode

`internal/sink/gitlab` adds a review workflow on top of the Git backend:

- **Direct mode**: commit straight to the target branch (same as Git).
- **Branch+MR mode** (`MergeRequestModeBranchMR`): push to a per-inventory feature branch
  (`BranchNameForExport(prefix, ns, name)`) off the target branch, then `EnsureMergeRequest` opens or
  updates one MR per inventory via the GitLab API. One stable branch/MR per inventory keeps churn
  reviewable instead of spawning an MR per export.

## Consequences

- Predictable, diffable layout for GitOps and audit ([ADR-0103](0103-etcd-limit.md)).
- Force-push means Git holds **current state with audit trail**, not a queryable history — relational
  history is Postgres' job ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).
- One branch/MR per inventory bounds review noise but means concurrent exports to the same inventory
  serialize on the remote.

## Open questions

- **DECIDED (2026-06-05):** Make the object path a **`spec.pathTemplate`** (e.g.
  `{cluster}/{namespace}/{name}.json`, default `inventory/{namespace}/{name}.json`) so layout is
  configurable per sink.
- **OPEN:** Optional commit-per-export (no force-push) mode for users who want Git history instead of a
  snapshot HEAD?
- **OPEN:** Object-store (S3/GCS) partition layout for the Parquet snapshot sink — lean toward
  `clusters/<cluster>/date=…/` Hive-style partitioning for DuckDB ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md)).
