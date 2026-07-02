# ADR-0801: Pipeline CLI mode — kubeconfig-based collection without operator deployment

**Theme:** 08 · Pipeline & CLI · **Status:** Accepted (2026-07-02, maintainer directive)

## Context

Kollect currently runs exclusively as an in-cluster Kubernetes operator. That model works well when
you have admin access to a cluster (to install CRDs, create the Deployment, grant RBAC), but it
excludes a common CI/CD adoption pattern:

> **"We have a kubeconfig with read access to a cluster, and a GitLab CI pipeline. We want to
> collect Helm release versions, image versions, ingress hostnames, and namespace lists, then
> commit the result to a git repo — without asking anyone for permission to install an operator."**

No prior ADR covers this deployment model. The closest related decision is ADR-0501 (multi-cluster
fleet), which explicitly notes that "single Git commit fan-in across clusters requires external CI
or a merge job — not built into the operator." This ADR covers that gap: making kollect the merge
job.

### What the operator model requires that the pipeline cannot provide

| Operator requirement | Why it blocks CI adoption |
|---|---|
| CRD installation (cluster-admin) | CRDs are cluster-scoped; most teams have namespace-scoped RBAC only |
| Long-running Deployment + RBAC | Needs review/approval cycle; overkill for a periodic snapshot job |
| Webhook TLS serving | Requires cert-manager or manual Secret management |
| Leader election (Lease) | Needs `coordination.k8s.io` RBAC |
| KollectTarget/Profile CRDs readable via API | With no operator, these CRDs are not installed |

### Why not fork or create a new project

A fork would duplicate ~6,000 lines of active, tested code:
- `internal/collect.Extractor` (CEL + JSONPath + Helm decode)
- `internal/collect.Store` + `MarshalTargetExport`
- `internal/sink.Registry` + all 9 Backend implementations (git, gitlab, s3, gcs, postgres,
  bigquery, mongodb, kafka, nats)
- `api/v1alpha1` CRD type definitions

The architecture is already port/adapter: controllers depend on interfaces, the sink `Backend`
interface is `Export(ctx context.Context, payload []byte, path string) error`, and the extractor
and store are pure functions with no controller-runtime dependency. The reuse surface is real and
verified by the architecture tests (`test/arch/arch_test.go`).

### "Configured using the same CRs"

The user wants config that looks like the existing CRD schema. Two readings:

- **(A) Config from local YAML files** — CR-shaped YAML files on disk, decoded by the CLI.
  No cluster access needed beyond reading workload resources. Works even if CRDs are not installed.
- **(B) Read CRDs from cluster** — apply `KollectProfile`/`KollectTarget` to the cluster via the
  kubeconfig and read them back. Requires CRD install + RBAC to read `kollect.dev` resources.

This ADR adopts **(A)** as primary. "No CRD install" is the invariant constraint.
Option B can be layered on later without breaking the CLI design.

---

## Decision

### Add a `kollect-pipeline` CLI binary

A second binary (`cmd/cli/main.go`) is added alongside the existing operator binary. It shares all
internal packages but has no controller-runtime manager, no reconcilers, no webhooks, no informers,
and no in-cluster identity.

```
kollect-pipeline collect \
  --kubeconfig $KUBECONFIG \          # path or KUBECONFIG env; falls back to ~/.kube/config
  --config ./collect-config/ \        # directory of KollectProfile + KollectTarget + Sink YAML
  --output ./inventory/               # local filesystem output directory (type: local sink)
```

The binary exits `0` on full success, `1` on partial failure (some targets skipped/forbidden),
`2` on configuration or connection error.

### Architecture: what is reused vs. what is new

```
┌──────────────────────────────────────────────────────────────────┐
│                      cmd/cli/main.go  (NEW)                      │
│  cobra · kubeconfig flag · config dir flag · exit codes          │
└──────────────────────────┬───────────────────────────────────────┘
                           │
         ┌─────────────────▼───────────────────┐
         │  internal/pipeline/loader.go  (NEW)  │
         │  Deserializes KollectProfile +       │
         │  KollectTarget + Sink YAML from disk │
         │  using k8s.io/apimachinery scheme    │
         └─────────────────┬───────────────────┘
                           │
         ┌─────────────────▼───────────────────┐
         │  internal/collect/runner.go  (NEW)  │
         │  One-shot List-based collection.    │
         │  Builds dynamic.Client from         │
         │  kubeconfig. Calls dynamic.List()   │
         │  per GVK + label selector.          │
         │  Feeds Extractor (REUSED).          │
         │  Populates Store (REUSED).          │
         └─────────────────┬───────────────────┘
                           │
         ┌─────────────────▼───────────────────┐
         │  internal/collect.Store (REUSED)    │
         │  MarshalTargetExport → []byte       │
         └─────────────────┬───────────────────┘
                           │
         ┌─────────────────▼───────────────────┐
         │  internal/sink.Registry (REUSED)    │
         │  git / gitlab / s3 / gcs / …        │
         │  + local (NEW)                      │
         └─────────────────────────────────────┘
```

**Reused as-is (zero changes to existing packages):**

| Package | What is reused |
|---|---|
| `api/v1alpha1` | `KollectProfile`, `KollectTarget`, `KollectSnapshotSink` Go types — deserialized from disk YAML |
| `internal/collect.Extractor` | CEL + JSONPath + Helm decode; operates on `*unstructured.Unstructured` |
| `internal/collect.Store` | In-memory item store + `MarshalTargetExport` |
| `internal/export` | `MarshalEnvelope`, `ItemsFingerprint` |
| `internal/sink.Registry` | All 9 backend factories (git, gitlab, s3, gcs, postgres, bigquery, mongodb, kafka, nats) |
| `internal/sink/git` | Git/GitLab backend (clone → write → commit → push) |

**Net-new (no changes to existing code):**

| Package | Purpose |
|---|---|
| `cmd/cli/` | CLI entry point, cobra wiring, kubeconfig flag, exit codes |
| `internal/pipeline/loader.go` | Deserialize YAML files from a config directory into typed CRD objects |
| `internal/collect/runner.go` | One-shot List-based collection runner (replaces informer Engine for this mode) |
| `internal/sink/local/backend.go` | `type: local` — writes export JSON/YAML to a local directory; CI owns the git commit |

**Why not reuse `collect.Engine` directly:** The Engine wraps
`dynamicinformer.DynamicSharedInformerFactory`, a bounded dispatch channel, N drain workers, and
resync periods. A one-shot CLI run does not benefit from any of this. The extractor and store are
the reusable core; the runner is a thin, synchronous `List()`-based loop over targets.

### The local filesystem sink (`type: local`)

The GitLab CI case: kollect writes files; the CI job does `git add . && git commit && git push`.

```yaml
# collect-config/sink.yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectSnapshotSink
metadata:
  name: ci-local
spec:
  type: local
  pathTemplate: "{namespace}/{name}.yaml"
```

The backend writes the export envelope (same format as git/s3 sinks, ADR-0419) to
`<outputDir>/<pathTemplate>`, creating parent directories. If the payload hash matches the
existing file on disk, the write is skipped (no unnecessary CI diffs). Content is YAML by default.

The `file://` git sink (go-git with a local repo path) is deliberately **not** the primary
recommendation here. It creates git commits, requiring a pre-cloned working tree, and the CI
pipeline's own commit step would create a double-commit. The `local` sink cleanly separates
collection from version control.

The SSRF policy (EC-P1-01/EC-P2-03) does not apply to `type: local`: there are no network calls
and no URL handling. No allowlist or opt-in needed.

### Config directory layout

```
collect-config/
  profile-helmrelease.yaml    # KollectProfile targeting helm.sh/v1 Secret (Helm release data)
  profile-ingress.yaml        # KollectProfile targeting networking.k8s.io/v1/Ingress
  profile-namespace.yaml      # KollectProfile targeting v1/Namespace
  target-helmrelease.yaml     # KollectTarget referencing profile-helmrelease
  target-ingress.yaml         # KollectTarget referencing profile-ingress
  target-namespace.yaml       # KollectTarget (cluster-wide)
  sink.yaml                   # KollectSnapshotSink (type: local OR git/gitlab)
```

The loader resolves `profileRef` by name within the loaded set (not via cluster API). Order within
the directory does not matter; references are resolved after all files are loaded.

### Scope enforcement in pipeline mode

`KollectScope` enforcement is **skipped** in pipeline mode. The scope policy lives in cluster state
that requires the operator. The enforcement boundary in pipeline mode is RBAC: the kubeconfig's
subject can only List what it has RBAC permission to List. Per-target SAR pre-checks from the
collect extractor are preserved — a List that fails RBAC degrades to `skipped:forbidden` (logged,
counted in exit code) rather than an error exit.

### Multi-cluster and CI integration

```yaml
# .gitlab-ci.yml — reference example
collect-inventory:
  image: ghcr.io/konih/kollect-pipeline:v0.8.0
  script:
    - kollect-pipeline collect
        --kubeconfig "$KUBECONFIG"
        --config ./collect-config/
        --output ./inventory/
    - git add inventory/
    - git diff --cached --quiet ||
        git commit -m "chore: inventory snapshot [skip ci]"
    - git push
  artifacts:
    paths: [inventory/]
```

Two supported patterns for multi-cluster, depending on how clusters are reached:

1. **Separate kubeconfigs** (different clouds/accounts, no single merged kubeconfig available) —
   one CI job per kubeconfig; set `spec.cluster` on the sink explicitly (consistent with ADR-0501's
   `pathTemplate: {cluster}/...` convention):

   ```yaml
   # collect-config/sink-cluster-a.yaml
   spec:
     type: local
     cluster: cluster-a
     pathTemplate: "{cluster}/{namespace}/{name}.yaml"
   ```

2. **One kubeconfig, many contexts** (the common case for `eksctl`/`gcloud container clusters
   get-credentials`-style tooling, or a team-merged kubeconfig) — one CI job fans out across a
   named list or wildcard-matched subset of contexts via `--context`. See "Multi-context selection"
   below.

### Multi-context selection (`--context`)

`--context` accepts repeated values and/or glob patterns (`*`/`?`, `path.Match` semantics — no
`**`), matched against the context names present in the loaded kubeconfig:

```
kollect-pipeline collect --context prod-eu-1 --context prod-us-1      # explicit list
kollect-pipeline collect --context "prod-*"                           # wildcard
kollect-pipeline collect --context "prod-*" --context staging-canary  # mixed
kollect-pipeline collect                                              # default: current-context only
```

Resolution rules:

- No `--context` flag → unchanged from single-context behavior: only `kubeconfig.CurrentContext`.
- One or more `--context` values → each is matched against the kubeconfig's context names. A
  pattern containing `*`/`?` is a glob; a literal name is matched exactly. The result is the
  de-duplicated union of all matches, processed in **sorted order** (deterministic, diffable CI
  output).
- A **literal** pattern matching nothing is a fatal config error (`ExitFatalError`) — almost always
  a typo.
- A **wildcard** pattern matching nothing is a warning, not fatal — e.g. `staging-*` matching zero
  contexts during a staging teardown should not break the job.
- An empty resolved set after all patterns are applied **is** fatal (nothing to collect).

**Run semantics:** each resolved context runs the full collect → export pass independently and
sequentially. One context's failure (unreachable API server, RBAC denial) does not abort the
others — it is recorded and folded into the aggregate exit code (worst-of:
`ExitFatalError` > `ExitPartialFailure` > `ExitSuccess`, across all contexts).

**`{cluster}` placeholder default:** in multi-context mode, the `{cluster}` path-template
placeholder (ADR-0501 convention) defaults to **the context name** when `spec.cluster` is unset, so
output naturally partitions per cluster without per-cluster sink YAML:

```yaml
spec:
  type: local
  pathTemplate: "{cluster}/{namespace}/{name}.yaml"
  # spec.cluster left unset — each context run substitutes its own context name
```

If `spec.cluster` is set explicitly alongside multiple `--context` values, that is rejected at
startup (before any collection begins) — an explicit single cluster identity conflicts with a
multi-cluster run by construction, and silently picking one would produce misleading output paths.

Sequential (not parallel) execution is the v0.8.0 choice — simplicity over throughput; N clusters
× one CI job's wall-clock budget is the explicit trade-off. Parallel per-context execution (bounded
worker pool) is deferred (see Post-MVP).

### Git sink in pipeline mode

The existing `KollectSnapshotSink` with `spec.type: git` works unchanged in pipeline mode.
The CLI loads the sink YAML, resolves `secretRef` from a co-located Secret manifest in the config
dir (first iteration), builds the `git.Backend`, and exports. This is the right choice when the
team wants kollect to own the commit rather than the CI script.

### Upgrade path from pipeline to operator

Because the config YAML is identical to what the operator reads from the cluster API, adoption
is incremental with no config migration:

1. Run `kollect-pipeline` with local config files + CI-owned git commit.
2. When admin access is available: `kubectl apply -f collect-config/` installs the same CRs.
3. Deploy the operator — it picks up the CRs and takes over event-driven collection.
4. Remove the CI job.

---

## Consequences

### Positive

- **Zero cluster footprint** — no CRD install, no Deployment, no RBAC beyond read access to
  workload resources. Can be set up with namespace-scoped read permissions.
- **Same CRD schema** — teams learn one config language; migration to full operator is opt-in.
- **CI/CD native** — one-shot execution model, exit codes, file-artifact output; composable with
  existing CI commit steps.
- **No existing API surface changed** — `kollect-pipeline` is a separate binary; the operator API
  and existing Helm chart are unchanged.
- **All snapshot sink backends available in pipeline mode** — git, gitlab, s3, gcs — at no
  additional implementation cost (the `Backend` interface is reused verbatim).

### Negative / trade-offs

- **No event-driven freshness** — collection is triggered by CI schedule or push, not by
  resource change events. Staleness is bounded by CI cadence (typically 5–60 minutes).
- **List calls on every run** — no informer cache means N API server List calls per target per
  run. At CI cadence (hourly or per-push) this is negligible; a `--min-interval` guard prevents
  accidental tight loops.
- **Two binaries to release** — the release supply chain (ADR-0705) gains a second GHCR image
  (`kollect-pipeline`), Trivy scan, SBOM, and cosign attestation path.
- **Secret resolution is simpler** — `secretRef` in operator mode reads from the cluster Secret
  store; in pipeline mode, secrets must come from local files (Secret manifests in the config dir)
  or environment variables. The initial implementation uses local files. A `secretRef.env` variant
  is deferred.

### Out of scope for this ADR

- `KollectScope` enforcement in pipeline mode — deferred; RBAC is the boundary.
- `secretRef.env` — environment variable secret binding.
- Incremental / fingerprint-caching pipeline — pipeline mode is always full List; caching the
  prior run's fingerprint to skip unchanged targets is deferred.
- CRDs-in-cluster config reading (Option B) — deferred; the upgrade path covers the need.
- Database and event sink types (postgres, bigquery, kafka, nats) in pipeline mode — technically
  free (backends are reused), but not in scope for v0.8 stories.
- **Parallel multi-context execution** — v0.8.0 runs resolved contexts sequentially; a bounded
  worker pool for concurrent per-context collection is deferred (see pipeline-cli Post-MVP table).

---

## Open questions

| # | Question | Status |
|---|---|---|
| 1 | Separate binary (`kollect-pipeline`) or a subcommand (`kollect pipeline collect`) on the main binary? | Leaning separate: cleaner image, no operator flags in `--help`, avoids arch-lint pressure on `cmd`. |
| 2 | Secret resolution: local Secret YAML file vs env-variable mapping? | Local file for v0.8; `secretRef.env` deferred. |
| 3 | Should `--output` be a zero-config shorthand that implies `type: local`, or must the user always supply a Sink YAML? | `--output` as zero-config shorthand; explicit Sink YAML in config dir overrides it. |
| 4 | Ship `kollect-pipeline` in v0.8.0 alongside the operator, or a separate tag/workflow? | Same tag; second image built in the same `release.yaml` workflow job. |
| 5 | Single context only, or a list/wildcard selection across contexts in one kubeconfig? | Added 2026-06-30 (maintainer request): `--context` is repeatable and glob-aware; resolved set runs sequentially with worst-of exit-code aggregation. See "Multi-context selection" above. |
