# sink-tls-trust Specification

## Purpose
Defines how a sink's configured CA (`spec.tls.caBundle` or `spec.tls.caSecretRef`) is applied to
the S3, GCS, Postgres and MongoDB clients, in the operator and in `kollect-pipeline`: the CA is
added to the system roots, verification is never turned off, and a CA that cannot take effect is an
error instead of being ignored.

## Requirements

### Requirement: SCT-1 S3 and GCS trust the configured CA in addition to the system roots

When an `s3` or `gcs` sink configures a CA, its HTTPS client SHALL verify the server certificate
against the system roots plus that CA, for export and for the connection probe. The client SHALL
keep the netguard dialer and SHALL NOT use a proxy, as without a CA. When the effective endpoint
(the spec endpoint, or for `s3://` the one the AWS SDK takes from `AWS_ENDPOINT_URL`,
`AWS_ENDPOINT_URL_S3` or shared config) has scheme `http` and a CA is configured, backend build
SHALL fail with a terminal error that says the CA is never checked over plain HTTP. With a CA
configured, the client SHALL NOT follow a redirect to a non-HTTPS URL. Without a CA, behaviour is
unchanged.

#### Scenario: Private-CA endpoint is trusted

- **GIVEN** an S3-compatible HTTPS server whose certificate is signed only by a private CA X
- **WHEN** an `s3` sink with `caSecretRef` naming X exports and probes
- **THEN** the TLS handshake succeeds and the object is written

#### Scenario: GCS path uses the same trust

- **GIVEN** the same server and a `gcs` sink with CA X
- **WHEN** it exports and probes
- **THEN** both succeed

#### Scenario: Public roots still trusted

- **GIVEN** an HTTPS server whose certificate chains to a root in the system pool, not to X
- **WHEN** an `s3` or `gcs` sink with CA X exports
- **THEN** the handshake succeeds

#### Scenario: Unknown CA still refused

- **GIVEN** a server signed by CA Y, which is neither X nor in the system pool
- **WHEN** an `s3` sink with CA X exports
- **THEN** the export fails with a certificate verification error
- **AND** the client SHALL NOT fall back to an unverified connection

#### Scenario: Plain HTTP endpoint with a CA is rejected

- **WHEN** an `s3` sink with a CA is built with endpoint `http://host/bucket`, or with endpoint
  `s3://bucket` while `AWS_ENDPOINT_URL_S3` (or `AWS_ENDPOINT_URL`) is an `http://` URL
- **THEN** the build fails with a terminal error naming `spec.tls` and the `http` scheme
- **AND** no request is sent

#### Scenario: Redirect to plain HTTP is refused

- **GIVEN** a private-CA HTTPS server that answers with a redirect to an `http://` URL
- **WHEN** an `s3` sink with CA X exports
- **THEN** the export fails and the `http://` URL SHALL NOT receive a request

### Requirement: SCT-2 Postgres trusts the configured CA within the DSN's sslmode

When a `postgres` sink configures a CA, every TLS configuration derived from the DSN (the main
connection and every fallback) SHALL verify against the system roots plus that CA. `sslmode=verify-full`
SHALL verify chain and hostname; `verify-ca` SHALL verify the chain; `require` SHALL verify the
chain as `verify-ca` does (libpq's rule when a root certificate is configured). When neither the
DSN nor `PGSSLMODE` sets an sslmode, the sink SHALL use `verify-full`. An explicit `disable`,
`allow` or `prefer` (in the DSN or `PGSSLMODE`) SHALL be rejected at backend build with a terminal
error that names the sslmode, its source and the accepted values. A root certificate from another source (`sslrootcert` in the DSN or in the
service-file entry it selects, `PGSSLROOTCERT`, `~/.postgresql/root.crt`) while the sink configures a
CA SHALL be rejected the same way, naming the
source and `spec.tls`, whether or not that file is readable. No error this requirement adds, and no
connection error, SHALL contain the DSN, host, user, password or database name. Without a CA,
behaviour is unchanged.

#### Scenario: verify-full with a private CA

- **GIVEN** a Postgres server with a certificate for its hostname signed only by private CA X
- **WHEN** a `postgres` sink with CA X and `sslmode=verify-full` exports and probes
- **THEN** both succeed

#### Scenario: require is upgraded to a chain check, not left unverified

- **GIVEN** a Postgres server signed by CA Y (not X, not in the system pool)
- **WHEN** a sink with CA X and `sslmode=require` connects
- **THEN** the connection fails with a certificate verification error
- **AND** the connection SHALL NOT succeed unverified

#### Scenario: Chain-only modes accept a trusted certificate for another hostname

- **GIVEN** a server certificate signed by X for hostname `other.invalid`, not the dialled host
- **WHEN** a sink with CA X connects with `sslmode=require`, then `verify-ca`, then `verify-full`
- **THEN** `require` and `verify-ca` succeed and `verify-full` fails on the hostname

#### Scenario: Public roots still trusted

- **GIVEN** a Postgres server whose certificate chains to a system-pool root, not to X
- **WHEN** a sink with CA X and `sslmode=verify-full` connects
- **THEN** it succeeds

#### Scenario: Unset sslmode becomes verify-full

- **GIVEN** a server signed by X whose certificate SAN does not match the DSN host
- **WHEN** a sink with CA X has a DSN without sslmode (also with `sslnegotiation=direct`) and
  `PGSSLMODE` is unset
- **THEN** the connection verifies chain and hostname and fails on the hostname
- **AND** the same DSN with explicit `sslmode=verify-ca` connects
- **AND** the connection SHALL NOT fall back to plaintext or to an unverified handshake

#### Scenario: Environment sslmode is respected

- **WHEN** a sink with CA X has a DSN without sslmode while `PGSSLMODE=verify-ca`
- **THEN** the build is accepted as `verify-ca` and the CA applies

#### Scenario: An explicit weak sslmode is rejected

- **WHEN** a sink with a CA has `sslmode=prefer`, `allow` or `disable` in the DSN, or `PGSSLMODE`
  set to one of them with no sslmode in the DSN
- **THEN** the build fails with a terminal error naming the sslmode, its source and `verify-full`,
  `verify-ca`, `require`
- **AND** no connection is attempted

#### Scenario: DSN CA file conflicts with the sink CA

- **WHEN** a sink with a CA has a DSN containing `sslrootcert=/path` (keyword or URL form), or
  `PGSSLROOTCERT` or a selected service-file entry names a file (present, missing or malformed), or
  `~/.postgresql/root.crt` exists
- **THEN** the build fails with a terminal error naming that source and `spec.tls`
- **AND** the error SHALL NOT contain the DSN, host, user or password

#### Scenario: Wrong CA is diagnosable without leaking the target

- **GIVEN** a server signed by Y and a sink with CA X and `verify-full`
- **WHEN** the probe runs
- **THEN** the error says TLS certificate verification failed against the configured CA
- **AND** it SHALL NOT contain the host, user or database name

### Requirement: SCT-3 MongoDB trusts the configured CA within the URI's TLS settings

When a `mongodb` sink configures a CA, the driver's TLS configuration SHALL verify against the
system roots plus that CA, keeping the URI's other TLS settings (client certificate). URI option
names SHALL match case-insensitively, as in the driver. A URI that sets `tlsCAFile` or
`sslCertificateAuthorityFile`, a URI on which TLS is off (`mongodb://` with no option that turns
TLS on, or `tls=false`/`ssl=false` on either scheme), or a URI whose effective `tlsInsecure` or
`sslInsecure` is `true` SHALL be rejected at backend build, without a DNS lookup, with a terminal error naming the
URI parameter and `spec.tls`. A driver-built TLS configuration that is absent or already carries a
root pool or disabled verification SHALL be rejected the same way, so a spelling the key check
misses cannot pass. The errors this requirement adds SHALL NOT contain the URI, host or
credentials. Without a CA, behaviour is unchanged.

#### Scenario: Private-CA server is trusted

- **GIVEN** a MongoDB server requiring TLS with a certificate signed only by private CA X
- **WHEN** a `mongodb` sink with CA X and `tls=true` exports and probes
- **THEN** both succeed

#### Scenario: Public roots still trusted

- **GIVEN** a MongoDB server whose certificate chains to a system-pool root, not to X
- **WHEN** a sink with CA X connects
- **THEN** it succeeds

#### Scenario: Unknown CA refused

- **GIVEN** a server signed by Y and a sink with CA X
- **WHEN** it connects
- **THEN** it fails with a certificate verification error and SHALL NOT connect

#### Scenario: URI settings that would ignore or override the CA are rejected

- **WHEN** a sink with a CA has a URI with `tlsCAFile=...` (any letter case), or `mongodb://`
  without `tls=true`, or `tls=false`, or `tlsInsecure=true`, or `sslInsecure=true`
- **THEN** the build fails with a terminal error naming that parameter and `spec.tls`
- **AND** the error SHALL NOT contain the URI or credentials
- **AND** a `mongodb+srv://` URI is rejected without any DNS lookup

#### Scenario: Default TLS on SRV, ssl=true and explicit verification are accepted

- **WHEN** a sink with CA X has a `mongodb+srv://` URI without `tls`, or `mongodb://` with
  `ssl=true`, or `tls=true&tlsInsecure=false`
- **THEN** the build is accepted and the CA applies

### Requirement: SCT-4 An unusable CA is a terminal error, never ignored

For `s3`, `gcs`, `postgres` and `mongodb`, CA PEM that yields no certificate (garbage, empty after
trimming, or only non-certificate blocks such as a private key) SHALL fail backend build with a
terminal error that names `spec.tls` and does not echo the PEM. For every sink type, a `caSecretRef`
Secret with none of the keys `tls.crt`, `ca.crt`, `ca.pem` SHALL fail with a terminal error naming
the Secret and the expected keys, and a spec that sets both `caBundle` and `caSecretRef` SHALL fail
with a terminal error before the CA Secret is fetched. A terminal build failure SHALL NOT be cached:
after the Secret or spec is fixed, the next backend acquisition (`acquireBackend`,
`internal/sink/backend_pool.go:145-153`) builds the backend. A terminal error does not requeue and
no controller watches Secrets, so that acquisition happens on the next sink or inventory event; the
docs SHALL tell users to touch the sink after fixing a Secret.

#### Scenario: Garbage PEM

- **WHEN** a sink of each of the four types has `caBundle: "not a certificate"`
- **THEN** the build fails with a terminal error naming `spec.tls`, before any connection

#### Scenario: Secret without a CA key

- **WHEN** `caSecretRef` names a Secret with only key `other`
- **THEN** the operator and the CLI both fail with a terminal error naming the Secret and
  `tls.crt`, `ca.crt`, `ca.pem`
- **AND** the client SHALL NOT be built with the system roots alone

#### Scenario: Both CA sources set

- **WHEN** a sink sets `caBundle` and `caSecretRef`, even when the referenced Secret does not exist
- **THEN** the build fails with the both-sources terminal error, and neither CA is used

#### Scenario: Failure then recovery

- **GIVEN** a `postgres` sink with a CA whose DSN Secret sets `sslrootcert`, so the build fails
- **WHEN** the DSN Secret is corrected and the backend is acquired again, with no change to the sink spec
- **THEN** the backend builds and the export succeeds

### Requirement: SCT-5 Operator and CLI resolve and apply the CA identically

The operator's export path, the operator's connection probe and `kollect-pipeline collect` SHALL
check the CA sources and select the CA through the same shared functions (same key order, same
error values) and SHALL build the clients through the same constructors with the CA passed. For the
same sink spec and Secrets, the operator and the CLI SHALL either both succeed with the same CA or
both fail with the same shared error, which each MAY wrap with its own context; Secret-lookup
errors stay source-specific. The CLI loads only `KollectSnapshotSink`, so the CLI requirement
covers `s3` and `gcs` end to end; for `postgres` and `mongodb`, the probe and export paths share
the constructors. The CLI SHALL exit 2 (`ExitFatalError`) on a CA error, before any export.

#### Scenario: Same result on both paths

- **GIVEN** a table of inputs (no CA; `caBundle`; `caSecretRef` with `ca.crt`; with `tls.crt` and
  `ca.crt`; with no CA key; both sources set)
- **WHEN** the operator resolver and the CLI resolver run on each
- **THEN** each pair returns the same PEM, or errors that wrap the same shared error

#### Scenario: Probe and export agree

- **WHEN** a sink with a CA and a conflicting DSN/URI is probed and exported
- **THEN** both fail with the same terminal error

#### Scenario: CLI stops before any export

- **WHEN** `kollect-pipeline collect` runs an `s3` sink with garbage CA PEM, or with a CA and an
  `http://` endpoint
- **THEN** the process exits 2 and writes nothing to the sink
- **AND** the context's fatal error wraps the same shared error the operator returns

### Requirement: SCT-6 Documentation states the trust rules

The sink reference pages for `KollectSnapshotSink` and `KollectDatabaseSink`, the upgrade guide,
`docs/REQUIREMENTS.md` FR-EXP-7, the security architecture TLS section and the pipeline CLI guide
SHALL describe: append to system roots, the Postgres sslmode and `sslrootcert` rules, the MongoDB
URI rules, the terminal errors of SCT-4 (naming that the missing-key and both-sources errors apply
to every sink type, Kafka and NATS included), and that strict pinning is not offered. Shared field
descriptions SHALL NOT claim append semantics for Kafka, NATS or the Git probe, which keep their
pool. The v0.22.0 release notes SHALL carry one line naming the behaviour change.

#### Scenario: Requirement claim matches the code

- **WHEN** FR-EXP-7 is read after this change
- **THEN** it lists every sink type that applies the CA (Git, GitLab, Kafka, NATS, S3, GCS,
  Postgres, MongoDB) and none that does not
