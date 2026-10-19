# Spec Delta

## Purpose

Defines how a sink's configured CA (`spec.tls.caBundle` or `spec.tls.caSecretRef`) is applied to
the S3, GCS, Postgres and MongoDB clients, in the operator and in `kollect-pipeline`: the CA is
added to the system roots, verification is never turned off, and a CA that cannot take effect is an
error instead of being ignored.

## ADDED Requirements

### Requirement: SCT-1 S3 and GCS trust the configured CA in addition to the system roots

When an `s3` or `gcs` sink configures a CA, its HTTPS client SHALL verify the server certificate
against the system roots plus that CA, for export and for the connection probe. The client SHALL
keep the netguard dialer and SHALL NOT use a proxy, as without a CA. An `http://` endpoint with a CA
configured SHALL be rejected at backend build with a terminal error that says the CA is never
checked over plain HTTP. Without a CA, behaviour is unchanged.

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

- **WHEN** an `s3` sink with endpoint `http://host/bucket` and a CA is built
- **THEN** the build fails with a terminal error naming `spec.tls` and the `http` scheme
- **AND** no request is sent

### Requirement: SCT-2 Postgres trusts the configured CA within the DSN's sslmode

When a `postgres` sink configures a CA, every TLS configuration derived from the DSN (the main
connection and every fallback) SHALL verify against the system roots plus that CA. `sslmode=verify-full`
SHALL verify chain and hostname; `verify-ca` SHALL verify the chain; `require` SHALL verify the
chain as `verify-ca` does (libpq's rule when a root certificate is configured). An effective
sslmode of `disable`, `allow` or `prefer`, including the default when the DSN sets none, SHALL be
rejected at backend build with a terminal error that names the sslmode and the accepted values. A DSN
that sets `sslrootcert` (or a `PGSSLROOTCERT` default) while the sink configures a CA SHALL be
rejected the same way, naming `sslrootcert` and `spec.tls`. No error text SHALL contain the DSN,
host, user, password or database name. Without a CA, behaviour is unchanged.

#### Scenario: verify-full with a private CA

- **GIVEN** a Postgres server with a certificate for its hostname signed only by private CA X
- **WHEN** a `postgres` sink with CA X and `sslmode=verify-full` exports and probes
- **THEN** both succeed

#### Scenario: require is upgraded to a chain check, not left unverified

- **GIVEN** a Postgres server signed by CA Y (not X, not in the system pool)
- **WHEN** a sink with CA X and `sslmode=require` connects
- **THEN** the connection fails with a certificate verification error
- **AND** the connection SHALL NOT succeed unverified

#### Scenario: Public roots still trusted

- **GIVEN** a Postgres server whose certificate chains to a system-pool root, not to X
- **WHEN** a sink with CA X and `sslmode=verify-full` connects
- **THEN** it succeeds

#### Scenario: A sslmode that would ignore the CA is rejected

- **WHEN** a sink with a CA has a DSN with no sslmode, or `sslmode=prefer`, `allow` or `disable`
- **THEN** the build fails with a terminal error naming the sslmode and `verify-full`, `verify-ca`,
  `require`
- **AND** no connection is attempted

#### Scenario: DSN CA file conflicts with the sink CA

- **WHEN** a sink with a CA has a DSN containing `sslrootcert=/path` (keyword or URL form)
- **THEN** the build fails with a terminal error naming `sslrootcert` and `spec.tls`
- **AND** the error SHALL NOT contain the DSN, host, user or password

#### Scenario: Wrong CA is diagnosable without leaking the target

- **GIVEN** a server signed by Y and a sink with CA X and `verify-full`
- **WHEN** the probe runs
- **THEN** the error says TLS certificate verification failed against the configured CA
- **AND** it SHALL NOT contain the host, user or database name

### Requirement: SCT-3 MongoDB trusts the configured CA within the URI's TLS settings

When a `mongodb` sink configures a CA, the driver's TLS configuration SHALL verify against the
system roots plus that CA, keeping the URI's other TLS settings (client certificate). A URI that
sets `tlsCAFile` or `sslCertificateAuthorityFile`, a URI on which TLS is off (`mongodb://` without
`tls=true`/`ssl=true`, or `tls=false`/`ssl=false` on either scheme), or a URI with
`tlsInsecure=true` SHALL be rejected at backend build with a terminal error naming the URI
parameter and `spec.tls`. No error text SHALL contain the URI, host or credentials. Without a CA,
behaviour is unchanged.

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

- **WHEN** a sink with a CA has a URI with `tlsCAFile=...`, or `mongodb://` without `tls=true`, or
  `tls=false`, or `tlsInsecure=true`
- **THEN** the build fails with a terminal error naming that parameter and `spec.tls`
- **AND** the error SHALL NOT contain the URI or credentials

### Requirement: SCT-4 An unusable CA is a terminal error, never ignored

For `s3`, `gcs`, `postgres` and `mongodb`, CA PEM that yields no certificate (garbage, empty after
trimming, or only non-certificate blocks such as a private key) SHALL fail backend build with a
terminal error that names `spec.tls` and does not echo the PEM. For every sink type, a `caSecretRef`
Secret with none of the keys `tls.crt`, `ca.crt`, `ca.pem` SHALL fail with a terminal error naming
the Secret and the expected keys, and a spec that sets both `caBundle` and `caSecretRef` SHALL fail
with a terminal error. A terminal build failure SHALL NOT be cached: after the Secret or spec is
fixed, the next backend acquisition (`acquireBackend`, `internal/sink/backend_pool.go:145-153`)
builds the backend.

#### Scenario: Garbage PEM

- **WHEN** a sink of each of the four types has `caBundle: "not a certificate"`
- **THEN** the build fails with a terminal error naming `spec.tls`, before any connection

#### Scenario: Secret without a CA key

- **WHEN** `caSecretRef` names a Secret with only key `other`
- **THEN** the operator and the CLI both fail with a terminal error naming the Secret and
  `tls.crt`, `ca.crt`, `ca.pem`
- **AND** the client SHALL NOT be built with the system roots alone

#### Scenario: Both CA sources set

- **WHEN** a sink sets `caBundle` and `caSecretRef`
- **THEN** the build fails with a terminal error, and neither CA is used

#### Scenario: Failure then recovery

- **GIVEN** a `postgres` sink with a CA whose DSN Secret sets `sslrootcert`, so the build fails
- **WHEN** the DSN Secret is corrected and the backend is acquired again, with no change to the sink spec
- **THEN** the backend builds and the export succeeds

### Requirement: SCT-5 Operator and CLI resolve and apply the CA identically

The operator's export path, the operator's connection probe and `kollect-pipeline collect` SHALL
select the CA through one shared function (same key order, same errors) and SHALL build the S3,
GCS, Postgres and MongoDB clients through the same constructors with the CA passed. For the same
sink spec, Secrets and DSN/URI, the operator and the CLI SHALL either both succeed with the same
trust or both fail with the same error text. The CLI SHALL exit 2 (`ExitFatalError`) on a CA
error, before any export.

#### Scenario: Same result on both paths

- **GIVEN** a table of inputs (no CA; `caBundle`; `caSecretRef` with `ca.crt`; with `tls.crt` and
  `ca.crt`; with no CA key; both sources set)
- **WHEN** the operator resolver and the CLI resolver run on each
- **THEN** each pair returns the same PEM or the same error

#### Scenario: Probe and export agree

- **WHEN** a sink with a CA and a conflicting DSN/URI is probed and exported
- **THEN** both fail with the same terminal error text

#### Scenario: CLI fails fast

- **WHEN** `kollect-pipeline collect` runs a `postgres` sink with a CA and a DSN without sslmode
- **THEN** it exits 2 and writes nothing to the sink

### Requirement: SCT-6 Documentation states the trust rules

The sink reference pages for `KollectSnapshotSink` and `KollectDatabaseSink`, the upgrade guide,
`docs/REQUIREMENTS.md` FR-EXP-7, the security architecture TLS section and the pipeline CLI guide
SHALL describe: append to system roots, the Postgres sslmode and `sslrootcert` rules, the MongoDB
URI rules, the terminal errors of SCT-4, and that strict pinning is not offered. The v0.22.0
release notes SHALL carry one line naming the behaviour change.

#### Scenario: Requirement claim matches the code

- **WHEN** FR-EXP-7 is read after this change
- **THEN** it lists every sink type that applies the CA (Git, GitLab, Kafka, NATS, S3, GCS,
  Postgres, MongoDB) and none that does not
