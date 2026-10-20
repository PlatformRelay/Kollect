# Spec Delta

## ADDED Requirements

### Requirement: SCR-1 A changed referenced Secret replaces the pooled backend before the next export

The operator's backend pool SHALL key each pooled backend on the sink spec and on the identity and
`resourceVersion` of every Secret the backend build reads for that spec: `spec.secretRef`,
`git.auth.secretRef`, `kafka.secretRef`, `nats.secretRef`, `postgres.databaseRef`,
`mongodb.databaseRef`, `bigquery.secretRef` and `tls.caSecretRef`. When any of them changed since
the pooled backend was built, the next export or cleanup that acquires the sink's backend SHALL
build a new backend from the current Secret values and SHALL NOT use the old one. The pool SHALL
read these Secrets from the API server, not from the informer cache, so a Secret change SHALL take
effect on the first export attempt that starts after the change was written, without a restart, a
spec change or a sink delete. For a binding with an export interval > 0 that attempt comes within
the interval. For a binding whose interval is `0s` ("export only on material change") a Secret
change alone SHALL NOT trigger an export; the change takes effect at the next export that a
material change, a spec generation change or a `kollect.dev/requestedAt` change causes.

#### Scenario: Rotated database password is used by the next export

- **GIVEN** a Postgres sink whose pooled backend was built from `databaseRef` Secret `db` with password `p1`
- **WHEN** Secret `db` is updated to password `p2` and the next export runs
- **THEN** that export runs on a backend built from `p2`
- **AND** the backend built from `p1` is no longer in the pool

#### Scenario: Rotated CA is trusted by the next export

- **GIVEN** an S3 sink with `tls.caSecretRef: ca` whose pooled backend trusts CA X
- **WHEN** Secret `ca` is updated to CA Y and the next export runs against an HTTPS server whose certificate CA Y signed
- **THEN** the TLS handshake succeeds against CA Y, where the backend built from CA X fails it

#### Scenario: Change visible before the cache sees it

- **GIVEN** a sink whose pooled backend was built from Secret value A
- **WHEN** the Secret is updated to B and an export starts before the operator's informer has delivered the update
- **THEN** that export runs on a backend built from B

#### Scenario: Cleanup reads uncached too

- **GIVEN** a Postgres sink whose `databaseRef` Secret was rotated, with the operator's cache still holding the old value
- **WHEN** an inventory with Delete is deleted and the relational prune runs
- **THEN** the prune's backend is built from the new value

#### Scenario: Cross-namespace Secret under a restricted cache

- **GIVEN** the manager cache restricted to namespace `team-a` and a sink in `team-a` whose allowlisted `secretRef` names a Secret in `shared-creds`
- **WHEN** an export runs
- **THEN** the backend is built from that Secret and SHALL NOT fail with an unknown-namespace cache error

#### Scenario: Every reference kind is covered

- **WHEN** any field of type `SecretReference` exists in `KollectSinkSpec`
- **THEN** the pool fingerprint includes the Secret it names for the sink types that read it, and a test fails if a reference field is added without that coverage

#### Scenario: Rotate and rotate back

- **GIVEN** a sink whose backend was built from Secret value A
- **WHEN** the Secret changes to B, an export runs, the Secret changes back to A, and another export runs
- **THEN** each export runs on a backend built from the value current at its acquire, three builds in total

#### Scenario: Unchanged Secrets do not rebuild

- **WHEN** two exports run with no change to the sink spec or any referenced Secret
- **THEN** both use the same pooled backend instance
- **AND** the pool SHALL NOT build a second backend

#### Scenario: An unrelated Secret does not rebuild

- **WHEN** a Secret that the sink does not reference changes between two exports
- **THEN** both exports use the same pooled backend instance

### Requirement: SCR-2 A deleted or emptied referenced Secret fails the next export

When a Secret from SCR-1's list is missing, or exists with no non-empty value in its data, the
acquire SHALL fail with an error that names the Secret and its namespace, SHALL retire the sink's
pooled backend, and the export or cleanup SHALL fail without contacting the sink. Cleanup for a
stream sink (Kafka, NATS), which never contacts the sink, SHALL decide retained before reading
any Secret. The error SHALL
be retryable (not terminal), so the export succeeds again once the Secret is restored, without a
change to the sink; this holds for the uncached metadata read too, whose not-found result SHALL
carry the same retryable error and not a terminal API status. For an inventory whose interval is
`0s`, the retry comes from SCR-7. An S3 or GCS sink whose referenced Secret is empty SHALL NOT fall back to
ambient credentials. A metadata read that fails for any other reason (timeout, cancelled
context, forbidden) observed no change: it SHALL fail that acquire only and SHALL NOT retire the
pooled backend on its own account; a mismatch with the pooled entry that an earlier successful
read of the same acquire saw still retires that entry.

#### Scenario: Deleted credential Secret

- **GIVEN** a sink whose backend is pooled
- **WHEN** its `secretRef` Secret is deleted and the next export runs
- **THEN** the export fails with an error naming that Secret and namespace, and the pooled backend is retired and closed
- **AND** the export SHALL NOT run on the cached backend

#### Scenario: Emptied credential Secret

- **WHEN** the referenced Secret is updated so that its data has no keys, or only empty values, and the next export runs
- **THEN** the export fails with an error naming that Secret as having no data, and the pooled backend is retired

#### Scenario: Emptied S3 Secret does not use ambient credentials

- **GIVEN** an S3 sink with `secretRef` and an AWS identity available to the process
- **WHEN** every key of the Secret is removed and an export runs
- **THEN** the export fails naming the Secret, and no S3 client is built

#### Scenario: Deleted CA Secret

- **WHEN** the `tls.caSecretRef` Secret of a sink with a pooled backend is deleted and the next export runs
- **THEN** the export fails naming that Secret

#### Scenario: Delete, then restore

- **GIVEN** a sink whose export failed because its Secret was deleted
- **WHEN** the Secret is created again and the export is retried
- **THEN** the export succeeds on a newly built backend, with no change to the sink object

#### Scenario: An observed change retires even when the rebuild read fails

- **WHEN** the metadata read shows a changed `resourceVersion` and the full read then times out
- **THEN** that acquire fails and the old pooled backend is retired

#### Scenario: A cancelled read does not retire a shared backend

- **GIVEN** a pooled backend used by two inventories
- **WHEN** one inventory's acquire fails because its context was cancelled during the Secret read
- **THEN** that acquire fails, and the other inventory's next acquire gets the same pooled backend

#### Scenario: Stream sink cleanup with a deleted Secret

- **GIVEN** a Kafka or NATS sink whose `secretRef` Secret was deleted
- **WHEN** an inventory exported to it is deleted
- **THEN** the cleanup returns retained without reading the Secret or building a backend, and the inventory deletion completes

#### Scenario: Cleanup after a rotated-away Git credential

- **GIVEN** a Git sink whose `git.auth.secretRef` Secret was deleted, and an inventory with Retain is deleted
- **WHEN** the release runs
- **THEN** it fails naming the Secret and the inventory stays `Terminating`, until the Secret is restored or `kollect.dev/force-cleanup: "true"` is set

### Requirement: SCR-3 Retiring a pooled backend never cuts an export in flight

Every path that removes a pooled backend (a referenced Secret change, a spec change, a deleted or
emptied Secret, the delete eviction) SHALL remove it from the pool at once, so no later acquire
receives it, and SHALL close it exactly once, after the last export or cleanup that acquired it
has released it. A retired backend SHALL NOT be closed while an acquirer holds it, and the idle
TTL SHALL NOT remove an entry that is held. A Secret change observed by a successful read SHALL
retire the old backend even when the rebuild then fails, and the old backend SHALL NOT be used as
a fallback. A backend built after a pool miss that came before another acquire stored or
retired the sink's entry SHALL NOT be pooled.

#### Scenario: Rotation during a long export

- **GIVEN** export E1 holds the pooled backend B1 and is still writing
- **WHEN** the Secret changes and export E2 acquires
- **THEN** E2 gets a new backend B2, E1 completes on B1 without a closed-client error, and B1 is closed once, after E1 releases it

#### Scenario: Spec change during a long export

- **WHEN** the sink spec changes while E1 holds B1 and E2 acquires
- **THEN** B1 is closed only after E1 releases it

#### Scenario: Concurrent acquires after a rotation

- **WHEN** several exports acquire concurrently after one Secret change
- **THEN** they all receive the same new backend, every surplus build is closed, and each retired backend is closed exactly once (race detector clean)

#### Scenario: Failed rebuild retires the old backend

- **GIVEN** a pooled backend built from Secret value A
- **WHEN** the Secret changes to a value the constructor rejects and an export runs
- **THEN** the export fails, the old backend is retired, and no export SHALL use the old backend afterwards

#### Scenario: Sink deleted during an export

- **WHEN** the sink is deleted while E1 holds its backend
- **THEN** the pool no longer holds the entry, no later acquire re-pools a backend for that sink, and the backend is closed after E1 releases it

#### Scenario: A slower builder does not replace a newer entry

- **GIVEN** acquire G1 missed and is building from Secret version 5
- **WHEN** the Secret moves to version 6, acquire G2 builds and stores B6, and then G1's build completes
- **THEN** B6 stays pooled, G1 exports on its own backend under an owning release that closes it, and the next acquire gets B6

#### Scenario: The TTL does not close a held backend

- **WHEN** the TTL prune runs while an export holds an entry
- **THEN** the entry is not closed under that export

#### Scenario: Real Postgres pool across a password rotation

- **GIVEN** a Postgres sink exporting as role `u1`, then its `databaseRef` Secret switched to role `u2` and `u1` set to `NOLOGIN`
- **WHEN** the next export runs
- **THEN** the rows are written in a session of `u2`, and after release no session of `u1` from the operator remains

### Requirement: SCR-4 Credential values stay out of logs, events, status and the pool

No error or log line that this change adds, and no Kubernetes Event or status condition that
carries one, SHALL contain a value from a referenced Secret. The pool key SHALL be computed from
Secret metadata (namespace, name, UID, `resourceVersion`) and the spec only; an acquire that hits
the pool SHALL NOT read the Secret's `data` or `stringData` and SHALL keep no other metadata
(annotations included) beyond those four fields; and the pool SHALL NOT keep the resolved Secret
data after the backend is built. Errors returned by a third-party client's constructor or connect call are
outside this requirement.

#### Scenario: Sentinel values never surface

- **GIVEN** referenced Secrets whose values are the sentinels `sentinel-pass`, `sentinel-token`, `sentinel-key`
- **WHEN** the Secret is rotated, emptied and deleted, and exports run after each step
- **THEN** no returned error, captured log line, recorded Event or inventory status condition contains a sentinel

#### Scenario: Fingerprint carries no Secret data

- **WHEN** two Secrets differ only in their data and carry the same metadata
- **THEN** the key function cannot tell them apart, because it receives no data

#### Scenario: A pool hit reads no values

- **WHEN** an acquire finds a pooled backend whose key matches, for a Secret whose annotations contain a sentinel value
- **THEN** the Secrets were read as metadata only, and nothing the pool keeps contains the sentinel

### Requirement: SCR-5 Operator and CLI agree where the CLI applies

`kollect-pipeline` SHALL reject a referenced Secret whose effective values (data, `stringData`,
then `${env:VAR}` substitution) hold no non-empty value, with the same message text as the
operator, before building the backend. The CLI SHALL keep building one backend per context per
run from the config directory, so a changed Secret manifest takes effect on the next run.

#### Scenario: Emptied Secret manifest in the CLI

- **WHEN** the config directory holds the referenced Secret with no data and no `stringData`
- **THEN** the run fails naming the Secret as having no data, and no backend is built

#### Scenario: CLI run after a Secret manifest change

- **WHEN** a run uses Secret value A, the manifest changes to B, and the CLI runs again
- **THEN** the second run's backend is built from B

### Requirement: SCR-6 Documentation states the rotation rules

The sink reference pages, the security architecture and the troubleshooting page SHALL state that
a Secret change takes effect on the next export, that a deleted or emptied Secret stops exports
until restored, and that a Secret change alone does not trigger an export. They SHALL state that
for an interval > 0 the next export comes within the interval, and that for `exportMinInterval:
0s` ("export only on material change") it comes only with the next material change, spec
generation or `kollect.dev/requestedAt` change. The upgrade notes
SHALL carry a v0.22.0 entry with the compatibility table from `design.md`, and the sentences that
say a built backend keeps its CA until the spec changes SHALL be removed.

#### Scenario: Stale statement removed

- **WHEN** the docs are searched across line breaks for "keeps its CA" (`rg -U 'keeps its\s+CA'`)
- **THEN** there is no match

### Requirement: SCR-7 A transient total export failure is retried when the interval is 0s

When a `KollectInventory` or `KollectClusterInventory` whose inventory-level export interval is
`0s` has an export in which every sink export failed and the aggregated error is not terminal,
the reconcile SHALL return a requeue after `validation.ZeroIntervalWatchdog` (30 s). A terminal
aggregated error SHALL NOT requeue, as today. An inventory with an interval > 0 SHALL keep its
current requeue (its interval). An unchanged `0s` inventory whose last export succeeded SHALL NOT
be exported again by this requeue or any other timer. Its coalescing state is in memory and ages
out 48 h after its last export or skip, so a controller restart, or more than 48 h on a degraded
path that returns before the export, leads to one export of the current payload.

#### Scenario: Deleted Secret under a zero interval is retried

- **GIVEN** a `KollectInventory` with `exportMinInterval: 0s` bound to one sink
- **WHEN** the sink's Secret is missing and the export fails with the retryable not-found error
- **THEN** the reconcile result requeues after `validation.ZeroIntervalWatchdog`
- **AND** after the Secret is created again the next reconcile exports successfully, with no change to the inventory or the sink

#### Scenario: Cluster inventory under a zero interval is retried

- **GIVEN** a `KollectClusterInventory` with `exportMinInterval: 0s` whose every sink export fails with a retryable error
- **WHEN** it reconciles
- **THEN** the result requeues after `validation.ZeroIntervalWatchdog`

#### Scenario: Terminal failure under a zero interval is not requeued

- **WHEN** every sink export of a `0s` inventory fails with a terminal error
- **THEN** the reconcile result has no requeue

#### Scenario: Unchanged, successfully exported zero-interval inventory is not re-exported

- **GIVEN** a `0s` inventory whose export succeeded
- **WHEN** it reconciles again with an unchanged payload, spec and `kollect.dev/requestedAt`, on any later watchdog requeue
- **THEN** no sink export runs
