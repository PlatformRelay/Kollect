# KollectDatabaseSink

**Scope:** Namespace · **Reconciled:** Yes (connection test) · **Short name:** `kdb`

Platform-shared backends: publish a `KollectDatabaseSink` in `kollect-system` and reference it from a
`KollectClusterInventory` sink ref by `name` + `namespace` — there is no cluster-scoped sink kind
([ADR-0208](../adr/0208-cluster-static-refs-via-namespace.md)).

## What it is for

A `KollectDatabaseSink` configures **relational / document** export backends — PostgreSQL, BigQuery
([ADR-0420](../adr/0420-bigquery-database-sink.md)), and MongoDB
([ADR-0417](../adr/0417-mongodb-database-sink.md)). Inventories reference database sinks via
`KollectInventory.spec.databaseSinkRefs`. Postgres uses a bulk upsert path for high-row exports;
BigQuery uses atomic `MERGE` jobs with stale-row delete reconciliation.

## Spec highlights

| Field | Purpose |
| --- | --- |
| `spec.type` | Backend: `postgres`, `bigquery`, `mongodb` |
| `spec.postgres` / `spec.bigquery` / `spec.mongodb` | Type-specific connection settings |
| `spec.tls` | Private CA trust for Postgres and MongoDB (`caBundle` or `caSecretRef`), see [TLS trust](#tls-trust-spectls) |
| `spec.provisioning.mode` | `ensure` (create table, default) or `existing` (never issue DDL) ([ADR-0416](../adr/0416-sink-config-layering.md)) |
| `spec.options` | Non-secret pass-through tuning (secret-like keys rejected by the webhook) |
| `spec.exportMinInterval` | Default per-ref debounce when inventory ref omits override |
| `spec.connectionTest` | Automatic probe on create/update (default `true`) |

MongoDB upserts one document per inventory item into `spec.mongodb.database`/`spec.mongodb.collection`,
keyed on `(inventory_namespace, inventory_name, target_name, source_uid)`
([ADR-0417](../adr/0417-mongodb-database-sink.md)).

## TLS trust (`spec.tls`)

`spec.tls.caBundle` (inline PEM) or `spec.tls.caSecretRef` (a Secret, read from the first non-empty
key of `tls.crt`, `ca.crt`, `ca.pem`) adds a private CA. Set one of the two. Postgres and MongoDB
trust the CA **in addition to** the system roots; BigQuery ignores it. There is no strict pinning
option.

**Postgres.** The CA is checked according to the effective `sslmode`: the DSN's (`ssl=true` in a
URL DSN counts as `require`), else the one in the service-file entry the DSN or `PGSERVICE`
selects, else `PGSSLMODE`:

| `sslmode` with a CA | Result |
| --- | --- |
| unset everywhere | `verify-full`: chain and hostname verified; a DSN host that does not match the certificate SAN fails |
| `verify-full` | chain and hostname verified |
| `verify-ca` | chain verified, hostname not checked (set it explicitly when the DSN host is not in the SAN) |
| `require` | chain verified (the libpq rule for `require` with a root certificate) |
| `disable`, `allow`, `prefer` set explicitly | terminal error naming the mode and its source |

A second root certificate source is a terminal error naming the source: `sslrootcert` in the DSN
or in the selected service-file entry, `PGSSLROOTCERT`, or `~/.postgresql/root.crt`. Use
`spec.tls` or the DSN, not both.

**MongoDB.** The URI must use TLS (`tls=true` or `ssl=true`, a TLS option such as
`tlsCertificateKeyFile`, or `mongodb+srv://`). With a CA, these are terminal errors naming the URI
option, never the URI: TLS off (`mongodb://` without TLS, or `tls=false`/`ssl=false`), a CA file in
the URI (`tlsCAFile`, `sslCertificateAuthorityFile`), and verification off (`tlsInsecure`,
`sslInsecure`, `tlsAllowInvalidCertificates`, `tlsAllowInvalidHostnames` set to anything but
`false`). Option names match in any letter case. Client certificates from the URI are kept.

**Both types:** CA PEM that holds no certificate, a `caSecretRef` Secret that has data but none of
the three keys, and a spec that sets both `caBundle` and `caSecretRef` are terminal errors. An
entirely empty `caSecretRef` Secret is a retryable error instead, see
[Secret rotation](#secret-rotation).

A failed backend build is not cached, but no controller watches the CA Secret and a terminal
error does not requeue. After fixing a Secret that failed the sink, change any annotation on
the sink, for example `kubectl annotate kollectdatabasesink <name> ca-fixed-at="$(date +%s)" --overwrite`:
the probe runs again, and the inventories that reference the sink reconcile and build its
backend on their next export.

## Secret rotation

Each export and each row-prune cleanup reads the metadata of the Secrets the sink references
(`spec.postgres.databaseRef`, `spec.mongodb.databaseRef`, `spec.bigquery.secretRef`,
`spec.secretRef`, `spec.tls.caSecretRef`) from the API server. When one of them changed since the
backend was built, that export builds a new backend, and a new connection pool, from the current
values. No restart, spec change or annotation is needed. Exports already running finish on the
old backend, which is closed after the last of them ends, so a rotated database password is used
from the next export on and the old sessions end with the old backend.

- **When it takes effect.** A Secret change alone does not trigger an export. With an export
  interval above `0s`, the next export comes within the interval. With `exportMinInterval: 0s`
  ("export only on material change"), it comes with the next material change, spec generation
  change or `kollect.dev/requestedAt` change.
- **Deleted or emptied Secret.** A missing Secret, or one with no non-empty value in its data,
  fails the export with a retryable error that names the Secret and its namespace
  (`secret "<name>" not found in namespace "<namespace>"` or `… has no data`) and never a value.
  The database is not contacted and the backend is dropped. Once the Secret is restored, the next
  retry exports again with no change to the sink; an inventory with `exportMinInterval: 0s`
  retries every 30 s.
- **Terminal errors** do not requeue. After fixing the Secret, change an annotation on the sink
  as above.
- **Inventory deletion.** The row prune builds the backend, so a deleted or emptied Secret keeps
  the inventory `Terminating` until the Secret is restored or `kollect.dev/force-cleanup: "true"`
  is set on the inventory.

BigQuery writes one row per inventory item into `spec.bigquery.dataset`/`spec.bigquery.table`,
partitioned on `exported_at` and clustered by inventory identity fields; each export performs
upsert + stale delete in a single `MERGE` statement
([ADR-0420](../adr/0420-bigquery-database-sink.md)).

## Example

A Postgres sink that upserts into an existing table and never issues DDL
([`config/samples/advanced/kollect_v1alpha1_kollectdatabasesink.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectdatabasesink.yaml)):

```yaml
apiVersion: kollect.dev/v1alpha1
kind: KollectDatabaseSink
metadata:
  name: postgres-inventory-demo
  namespace: default
spec:
  type: postgres
  cluster: kind-kollect-dev
  connectionTest: true
  provisioning:
    mode: existing          # 'ensure' (default) creates the table; 'existing' never touches DDL
  options:
    statement_timeout: "30000"
  postgres:
    databaseRef:            # DSN lives in a Secret — never inline credentials
      name: inventory-postgres-dsn
      namespace: kollect-system
    schema: public
    table: inventory_items
```

MongoDB variant:
[`config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_mongodb.yaml).

BigQuery variant:
[`config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_bigquery.yaml`](https://github.com/platformrelay/kollect/blob/main/config/samples/advanced/kollect_v1alpha1_kollectdatabasesink_bigquery.yaml).

## Status

`status.conditions` includes `ConnectionVerified` after the family sink reconciler runs an optional
connectivity probe ([ADR-0403](../adr/0403-connection-test.md)).

### Preview (`status.preview`)

Annotate a sink with `kollect.dev/preview: "true"` to have the reconciler render a side-effect-free
preview of its export implications under `status.preview`
([ADR-0416](../adr/0416-sink-config-layering.md) §8): the effective provisioning mode and
serialization format, plus the expected Postgres DDL or MongoDB index keys for the configured
backend (BigQuery currently surfaces general warnings only), and any warnings. Removing the
annotation clears `status.preview`.

See [ADR-0414](../adr/0414-sink-family-crds.md) for the family CRD model and
[ADR-0416](../adr/0416-sink-config-layering.md) for provisioning and option layering.
