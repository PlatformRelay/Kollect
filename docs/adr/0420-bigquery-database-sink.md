# ADR-0420: BigQuery database sink

> A real `type: bigquery` backend for the database sink family: snapshot upserts with delete
> reconciliation into a partitioned, clustered BigQuery table via atomic `MERGE` DML — Workload
> Identity first, service-account key as the explicit fallback. Replaces the ADR-0414 stub.

**Theme:** 04 · Export & sinks · **Status:** Accepted (design 2026-06-09 — implementation pending)

## Context

The database sink family ([ADR-0414](0414-sink-family-crds.md)) ships two real backends — Postgres
([ADR-0402](0402-sink-backends-database-kafka.md)) and MongoDB ([ADR-0417](0417-mongodb-database-sink.md)) —
plus a `bigquery` stub that passes CRD/webhook validation and fails at export with *not implemented*.
The stub admission is being removed in a parallel change; **`bigquery` re-enters the CRD enum and the
webhook allowlist only together with this real backend**, so a `type: bigquery` sink is never
admissible without a working export path.

Why BigQuery as the next database backend:

- Fleet teams on GCP want **SQL analytics and dashboards over inventory** (Looker/Data Studio, ad-hoc
  Standard SQL) without operating a Postgres instance — a serverless analytics projection of the
  same canonical snapshot.
- It exercises a genuinely different corner of the family contract: no enforced primary keys, no
  transactions across statements, partition/clustering instead of indexes, job-based execution.
  Surviving that without changing the family CRD is further proof the abstraction holds.

The backend must honor the locked database-family contracts:

- **Identity** `(inventory_namespace, inventory_name, target_name, source_uid)` and **delete
  reconciliation** — stale rows for an `(inventory, cluster)` partition are removed each export; an
  empty snapshot clears the partition ([ADR-0401](0401-sink-taxonomy-state-vs-stream.md),
  [ADR-0402](0402-sink-backends-database-kafka.md)). The Postgres backend implements this in
  `internal/sink/postgres/backend.go`; column naming below mirrors its DDL exactly.
- **Credentials never in spec/status/logs** — secret material only via Secret references.
- **Cross-cutting `provisioning`** ([ADR-0416](0416-sink-config-layering.md)): `ensure` (default)
  creates the destination table if missing; `existing` never issues DDL and preflights existence.
- **Capabilities** are the relational-store profile (`cap.RelationalStore()`: state store with
  `SupportsDelete`), so empty snapshots still reach `Export` as `[]` and prune stale rows.
- **Connection-test parity** — automatic probe plus `KollectConnectionTest`
  ([ADR-0403](0403-connection-test.md)), wired through `internal/sink/probe.go` like
  `postgres.TestConnection` / `mongodb.TestConnection`.

## Decision

### 1. `type: bigquery` on the database family — no new kind

`KollectDatabaseSink` / `KollectClusterDatabaseSink` gain a real `bigquery` backend behind the
existing `spec.type` enum. The placeholder `BigQuerySpec` (`api/v1alpha1/sink_common_types.go`)
becomes a full config block:

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectDatabaseSink
metadata:
  name: fleet-analytics
  namespace: team-a
spec:
  type: bigquery
  cluster: prod-west            # optional — labels rows; clustering key
  exportMinInterval: 5m         # optional — also the cost lever (see Consequences)
  provisioning:
    mode: ensure                # ensure (default) creates the table; never the dataset
  bigquery:
    project: acme-fleet-analytics
    dataset: inventory          # must already exist — kollect never creates datasets
    table: inventory_items
    location: EU                # optional — job placement; defaults to dataset location
    secretRef:                  # optional — omit to use ADC / Workload Identity
      name: bigquery-sa-key
      namespace: kollect-system
```

```go
// BigQuerySpec configures BigQuery relational export (ADR-0420).
type BigQuerySpec struct {
    // project is the GCP project id that owns the dataset.
    // +required
    Project string `json:"project"`

    // dataset is the BigQuery dataset id; it must already exist.
    // +required
    Dataset string `json:"dataset"`

    // table is the destination table name.
    // +required
    Table string `json:"table"`

    // location pins job placement (for example EU); defaults to the dataset location.
    // +optional
    Location string `json:"location,omitempty"`

    // secretRef references a Secret holding a service-account JSON key
    // (key credentials.json). When absent, Application Default Credentials are used.
    // +optional
    SecretRef *SecretReference `json:"secretRef,omitempty"`
}
```

Pre-GA breaking change to the stub shape: `dataset`/`table` were optional on the stub and become
required; `project`, `location`, and `secretRef` are new. No conversion machinery — `v1alpha1`
posture per [ADR-0414](0414-sink-family-crds.md).

### 2. Authentication — ADC first, key file as the explicit alternative

Exactly two modes, resolved in this order:

| Mode | Trigger | Mechanism |
| --- | --- | --- |
| **ADC / Workload Identity Federation** (primary) | `bigquery.secretRef` absent | Client built with no explicit credentials; the manager pod's bound service account resolves via Application Default Credentials (GKE Workload Identity or federated WIF elsewhere) |
| **Service-account JSON key** (alternative) | `bigquery.secretRef` set | Secret key `credentials.json` passed as client credentials JSON |

No other auth modes (no API keys, no OAuth user flows, no access-token fields). The builder
(`internal/sink/build_context.go`) gains a `bigquery` branch resolving `spec.bigquery.secretRef`
into `BuildContext.DatabaseSecretData` — the same seam `spec.postgres.databaseRef` uses — except
the ref is optional, and an absent ref is valid (ADC), not an error.

Required IAM on the bound principal (documented on the CRD page, not enforced by kollect):
`roles/bigquery.dataEditor` on the dataset plus `roles/bigquery.jobUser` on the project.

### 3. Write path — one atomic `MERGE` per export, no streaming inserts

Three candidates were evaluated against the export semantics (idempotent whole-snapshot upserts,
debounced cadence via `exportMinInterval`, payload bounded by the ~1.5 MiB `maxExportBytes` envelope
cap from [ADR-0103](0103-etcd-limit.md)):

| Path | Verdict | Why |
| --- | --- | --- |
| **Streaming inserts** (legacy `insertAll` / Storage Write API) | **Rejected** | Append-only — every export would duplicate rows instead of upserting; rows in the streaming buffer cannot be touched by DML for up to ~90 minutes, which breaks delete reconciliation outright; per-byte ingest cost on every export cycle |
| **Load job into staging + `MERGE`** | Fallback, deferred | Free ingest and unbounded size, but two jobs per export plus staging-table lifecycle/GC to manage |
| **Parameterized `MERGE` DML with `UNNEST(@rows)`** | **Chosen** | One atomic, idempotent statement per export; upsert **and** stale-delete in a single job; no staging artifacts; works against the emulator |

The chosen statement binds the snapshot as an `ARRAY<STRUCT<...>>` query parameter and mirrors the
Postgres `unnest($4::text[], $5::text[])` stale-delete pattern:

```sql
MERGE `project.dataset.table` AS t
USING UNNEST(@rows) AS s
ON  t.inventory_namespace = @inv_ns AND t.inventory_name = @inv_name
AND t.cluster = @cluster
AND t.target_name = s.target_name AND t.source_uid = s.source_uid
WHEN MATCHED THEN UPDATE SET
  payload = s.payload, exported_at = @exported_at,
  resource_namespace = s.resource_namespace
WHEN NOT MATCHED BY TARGET THEN INSERT
  (inventory_namespace, inventory_name, target_name, source_uid,
   cluster, resource_namespace, payload, exported_at)
  VALUES (@inv_ns, @inv_name, s.target_name, s.source_uid,
          @cluster, s.resource_namespace, s.payload, @exported_at)
WHEN NOT MATCHED BY SOURCE
  AND t.inventory_namespace = @inv_ns AND t.inventory_name = @inv_name
  AND t.cluster = @cluster
THEN DELETE
```

Size budget: the snapshot payload is capped at ~1.5 MiB ([ADR-0103](0103-etcd-limit.md)), far below
the 10 MB query-request limit, so the array parameter always fits. Should a future change lift the
envelope cap, the load-job + staging `MERGE` variant is the designated escape hatch — a backend
implementation detail, no CRD change.

### 4. Schema mapping — mirror the Postgres column set

Columns replicate the Postgres DDL in `internal/sink/postgres/backend.go` one-to-one so the two
backends stay interchangeable behind the family CRD:

| Column | Postgres | BigQuery | Notes |
| --- | --- | --- | --- |
| `inventory_namespace` | `TEXT NOT NULL` | `STRING REQUIRED` | identity |
| `inventory_name` | `TEXT NOT NULL` | `STRING REQUIRED` | identity |
| `target_name` | `TEXT NOT NULL` | `STRING REQUIRED` | identity |
| `source_uid` | `TEXT NOT NULL` | `STRING REQUIRED` | identity |
| `cluster` | `TEXT NOT NULL DEFAULT ''` | `STRING REQUIRED` | from `spec.cluster`; empty string when unset |
| `resource_namespace` | `TEXT NOT NULL DEFAULT ''` | `STRING REQUIRED` | item namespace, inventory namespace fallback |
| `payload` | `JSONB NOT NULL` | `JSON REQUIRED` | full `Item` row ([ADR-0405](0405-export-data-contract.md)) |
| `exported_at` | `TIMESTAMPTZ NOT NULL` | `TIMESTAMP REQUIRED` | UTC export time |

Physical layout (`ensure` DDL):

- **Time partitioning** on `TIMESTAMP_TRUNC(exported_at, DAY)`.
- **Clustering** on `cluster, inventory_namespace, inventory_name, resource_namespace` (BigQuery
  maximum of four clustering columns).
- **No enforced uniqueness** — BigQuery has no enforced primary keys. Row identity is maintained
  purely by the `MERGE` semantics above; the `ensure` DDL may declare
  `PRIMARY KEY (...) NOT ENFORCED` as an optimizer hint, but correctness never depends on it.

Honest caveat: because upserts rewrite `exported_at`, live rows migrate to the current day's
partition on every export and stale rows being deleted live in *older* partitions — so the
delete side of the `MERGE` cannot be partition-pruned. Clustering on `cluster` + inventory columns
is what bounds scanned bytes, and partitioning chiefly benefits *downstream* analytical queries and
optional retention (partition expiration is out of scope for v1 — see Open questions).

### 5. Provisioning and delete semantics

- **`provisioning.mode: ensure`** (default) creates the **table** with the partitioning/clustering
  spec if missing — once at backend construction, like the Postgres `ensureTable` (PERF-02: pooled
  backends do not repeat DDL per export). It **never creates the dataset**: a dataset is a
  billing/location-scoped container, the same way the Postgres backend never creates the database.
- **`provisioning.mode: existing`** never issues DDL and preflights that the table exists, failing
  loudly with a terminal error when it does not.
- **Delete reconciliation** is carried entirely by the `MERGE` (`WHEN NOT MATCHED BY SOURCE …
  THEN DELETE`, scoped to the `(inventory, cluster)` partition). An **empty snapshot** still reaches
  the backend as `[]` (`cap.RelationalStore()` semantics in `internal/sink/cap`) and degenerates to
  a plain scoped `DELETE`, exactly matching the Postgres empty-snapshot branch. Inventory deletion
  therefore clears its rows on the final empty export — no extra finalizer machinery beyond what
  Postgres has today.

### 6. Connection-test probe

`RunConnectionTest` (`internal/sink/probe.go`) gains a `bigquery` case, surfaced through the
standard sink condition and `KollectConnectionTest` flow ([ADR-0403](0403-connection-test.md)).
The probe is side-effect-free (no DDL, regardless of provisioning mode) and checks, in order:

1. **Credential resolution** — ADC chain or `credentials.json` from the Secret resolves to a token
   source (catches missing Workload Identity bindings and malformed keys).
2. **Dataset existence** — dataset metadata `GET` (catches wrong project/dataset and missing
   `dataEditor` grants).
3. **Job execution** — a **dry-run** `SELECT 1` query job (validates `jobUser` and job placement
   without scanning bytes or incurring cost).

Success message: `BigQuery dataset metadata and dry-run query succeeded`.

### 7. Error classification

Mapped onto the typed reconcile classes from [ADR-0602](0602-error-taxonomy.md)
(`internal/errors`), keyed on `googleapi.Error` HTTP codes and job-status error reasons:

| Signal | Class | Rationale |
| --- | --- | --- |
| 400 `invalid` / `invalidQuery` | **terminal** | bad config or schema drift; retry cannot help |
| 401 / 403 `accessDenied` | **terminal** | credential/IAM misconfiguration until spec or binding changes |
| 404 `notFound` (dataset; table in `existing` mode) | **terminal** | preflight contract violated |
| 409 `duplicate` on `ensure` DDL | success | benign create race — treat as already-exists |
| 429 `rateLimitExceeded` / `quotaExceeded` | **transient** | back off and retry; circuit breaker absorbs sustained throttling |
| 5xx `backendError` / `internalError`, network timeouts | **transient** | standard retry-with-backoff |

Unknown reasons default to transient, consistent with `errors.ClassOf`.

### 8. Validation and webhook rules

`ValidateDatabaseSinkSpec` (`internal/validation/family_sink.go`) already requires `spec.bigquery`
and forbids the `postgres`/`mongodb` sibling blocks for `type: bigquery`. This ADR adds:

- `bigquery.project`, `bigquery.dataset`, `bigquery.table` **required**, non-blank.
- `bigquery.secretRef` optional; when set, the same Secret-reference shape rules as
  `postgres.databaseRef`. No mutually exclusive auth fields exist by construction — ADC is simply
  the absence of `secretRef`.
- `spec.layout` stays forbidden and `serialization.format` stays JSON-only for the database family
  (capability matrix, [ADR-0419](0419-git-export-serialization-layout.md)).
- Sequencing: the stub registration (`internal/sink/stub_backends.go`) and the `bigquery` entries in
  the CRD enum / `validDatabaseSinkTypes` are being **removed** in a parallel change; they
  **re-enter only in the change that ships this backend**, keeping "admissible implies exportable"
  true at every commit.

### 9. Test plan (merge gate)

Per the [ADR-0706](0706-testing-merge-gate-architecture.md) ladder — every new sink backend must
reach L3 before merge:

- **L0 unit:** config resolution (required fields, secret-key lookup, ADC default), `MERGE`/DDL SQL
  builders (golden statement fixtures), error-classification table tests, webhook validation cases.
- **L3 integration** (`-tags=integration`, testcontainers): the
  [goccy/bigquery-emulator](https://github.com/goccy/bigquery-emulator) image — export rows and
  assert content, re-export mutated snapshot and assert upsert + stale delete, empty snapshot clears
  the partition, `existing` mode fails on a missing table, probe path. **Spike gate:** emulator
  support for `MERGE … UNNEST(@rows)` must be validated *first*; if its ZetaSQL coverage falls
  short, the L3 suite drives the load-job + staging variant and the primary write path is
  re-decided before implementation proceeds.
- **Schema/manifests:** golden OpenAPI spec fragment for the database sink CRD under
  `test/schema/golden/` (extending the cases in `test/schema/extract.go`), a
  `config/samples/kollect_v1alpha1_kollectdatabasesink_bigquery.yaml` sample, and a refreshed
  `docs/crds/kollectdatabasesink.md` page.
- **Live GCP e2e** (real project, WIF, real quotas) is **maintainer-only and never runs in CI** —
  there is no hermetic, free, secret-less way to exercise real IAM from a public repo.

## Consequences

### Positive

- GCP-native analytics projection of inventory with full delete-reconcile and probe parity; the
  family CRD again absorbed a new backend without changing shape or the inventory reference model.
- Serverless destination — no database to operate; partitioned/clustered layout keeps downstream
  dashboard queries cheap.
- The single-statement `MERGE` write path keeps exports atomic and idempotent with no staging
  artifacts to garbage-collect.

### Negative

- **New dependency surface:** `cloud.google.com/go/bigquery` plus its `google.golang.org/api` /
  auth transitive tree — the first google-cloud-go SDK in the operator image (the GCS sink
  deliberately uses the S3-compatible XML API, `internal/sink/gcs`). Image size and `vulncheck`
  scope grow accordingly.
- **Emulator fidelity limits:** the emulator approximates Standard SQL via ZetaSQL bindings and
  does not reproduce IAM, quotas, partition pruning, or job billing. L3 proves the SQL contract,
  not GCP behaviour — hence the maintainer-only live e2e.
- **Cost/quota honesty:** every export is a query job; on-demand billing charges scanned bytes, and
  the delete side of the `MERGE` cannot be partition-pruned (clustering bounds it instead). On
  large tables with aggressive cadence this costs real money — `exportMinInterval` is the lever,
  and the CRD docs must say so plainly. DML statements also occupy slot capacity and per-table
  concurrent-mutation limits; sustained throttling surfaces as transient errors through the circuit
  breaker rather than data loss.
- One more backend to harden and keep green under the "no merge without integration proof" bar.

## Alternatives considered

- **Separate `KollectBigQuerySink` kind** — rejected: [ADR-0414](0414-sink-family-crds.md) settled
  on family CRDs precisely so backends are `type` values, not kinds; a parallel kind would fork
  RBAC, refs, and status handling for zero expressiveness gain.
- **Bigtable** — rejected: wide-column NoSQL with key-range scans, no SQL `MERGE`, and no analytics
  query model; it serves none of the SQL-dashboard use cases that motivate a database-family
  analytics backend.
- **Streaming inserts as the primary write path** — rejected (§3): append-only duplicates conflict
  with snapshot-upsert semantics, and the streaming buffer blocks DML deletes for up to ~90
  minutes, breaking delete-reconcile parity.
- **Keeping the webhook stub until the backend lands** — rejected: an admissible CR that can never
  export is a standing foot-gun; the stub is removed first and `bigquery` returns to the allowlist
  atomically with the real backend.

## Open questions

- **OPEN:** Emulator coverage of `MERGE … UNNEST(@rows)` — the implementation spike must confirm it
  before code lands; on failure, choose between the load-job + staging path as primary or
  emulator-only divergence in L3.
- **OPEN:** Should `ensure` ever create the **dataset** (currently: never, by design)? Creating it
  would need a location decision kollect should arguably not own.
- **OPEN:** Optional partition-expiration / retention field on `BigQuerySpec` (cost control for
  high-churn fleets) — out of scope for v1, revisit with operator feedback.
- **OPEN:** None of the sink-family CRDs have golden OpenAPI fragments today
  (`test/schema/extract.go` covers profiles/targets/inventories) — adding the database sink golden
  here sets the precedent; confirm the other families should follow.

## See also

- [ADR-0401: Sink taxonomy — state stores vs event emitters](0401-sink-taxonomy-state-vs-stream.md)
- [ADR-0402: Postgres and Kafka sink backends](0402-sink-backends-database-kafka.md)
- [ADR-0403: Connection test](0403-connection-test.md)
- [ADR-0414: Sink family CRDs](0414-sink-family-crds.md)
- [ADR-0416: Sink configuration layering](0416-sink-config-layering.md)
- [ADR-0417: MongoDB database sink](0417-mongodb-database-sink.md)
- [ADR-0602: Error taxonomy](0602-error-taxonomy.md)
- [ADR-0706: Testing and merge-gate architecture](0706-testing-merge-gate-architecture.md)
