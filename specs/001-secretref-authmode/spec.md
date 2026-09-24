# Feature Specification: B2 — Admission secretRef guard + auth-mode enum

**Feature Branch**: `fm/kollect-b2-secretref` (target `fix/secretref-scope-and-authmode`)

**Created**: 2026-09-24

**Status**: Draft

**Input**: Findings K-04 and K-12 of the six-review unification register
(`data/kollect-unify/report.md`, §4.1), with captain decision C-1(a): reject
cross-namespace `secretRef` at admission with an explicit operator opt-in
allowlist. Batch B2; test lock: webhook envtest cross-ns reject/allow + startup
flag enum validation.

## Context

Two independent admission/startup defects turn a narrowly-scoped grant into a
cluster-wide capability:

- **K-04 (CRITICAL).** `SecretReference.Namespace` is a free-form, schema-admitted
  field on every family sink (`spec.secretRef`, `spec.tls.caSecretRef`,
  `spec.git.auth.secretRef`, `spec.postgres.databaseRef`, `spec.mongodb.databaseRef`,
  `spec.bigquery.secretRef`, `spec.nats.secretRef`, `spec.kafka.secretRef`).
  `sink.ResolveSecret` honours the namespace verbatim while the manager holds
  cluster-wide `secrets get;list;watch`. A principal who may create a sink in
  their own namespace can point it at an attacker endpoint and name another
  namespace's Secret, causing the operator to present the victim's credentials
  to that endpoint. Five of six review legs confirmed it.
- **K-12 (HIGH).** `--inventory-auth-mode` is an unvalidated free string.
  `RequireInventoryGet` is set only when the value is exactly `kubernetes`; any
  other value (a capitalisation, a trailing space, an alias) leaves the
  inventory HTTP server authenticating tokens but performing **no**
  authorization, so every valid bearer token in the cluster can read the whole
  inventory. The HTTP server is off by default.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cross-namespace Secret references are rejected at admission (Priority: P1)

A tenant who may create a `KollectSnapshotSink` in their own namespace cannot
make the operator read a Secret belonging to another namespace. On create or
update, a sink whose `secretRef` (or `caSecretRef` / `databaseRef` /
`git.auth.secretRef` / nested family `secretRef`) names a namespace other than
the sink's own is rejected by the validating webhook with an error naming the
offending field.

**Why this priority**: This is the exfiltration primitive. Closing it is the
whole point of the batch.

**Independent Test**: envtest: create a `KollectSnapshotSink` in namespace A
whose `spec.secretRef.namespace` is B (with the allowlist empty) → admission
fails and the error names `spec.secretRef.namespace`.

**Acceptance Scenarios**:

1. **Given** the allowlist is empty, **When** a family sink in namespace `A`
   sets a Secret reference namespace of `B` (`B != A`), **Then** admission
   rejects it and the error names the field.
2. **Given** the allowlist is empty, **When** a family sink sets a Secret
   reference with an empty namespace, **Then** admission accepts it (implicit
   same-namespace reference).
3. **Given** the allowlist is empty, **When** a family sink sets a Secret
   reference namespace equal to its own namespace, **Then** admission accepts it.

---

### User Story 2 - Operator opt-in allowlist for deliberate cross-namespace refs (Priority: P1)

An operator who deliberately runs the fleet fan-in pattern (sinks in many
namespaces reading credentials from a small set of shared namespaces) can
allowlist those target namespaces process-wide. Cross-namespace references to an
allowlisted namespace are admitted; every other cross-namespace reference stays
rejected.

**Why this priority**: The captain chose (a) precisely to preserve the fan-in
pattern behind a flag; without the allowlist the fix breaks that deployment.

**Independent Test**: envtest with the allowlist set to `shared-creds`:
reference to `shared-creds` is admitted; reference to `other` is rejected.

**Acceptance Scenarios**:

1. **Given** the allowlist contains `shared-creds`, **When** a sink in `A`
   references namespace `shared-creds`, **Then** admission accepts it.
2. **Given** the allowlist contains `shared-creds`, **When** a sink in `A`
   references namespace `other`, **Then** admission rejects it.
3. **Given** the allowlist is configured, **Then** it is process-wide, set from
   the manager flag, and not tenant-controllable through any CRD field.

---

### User Story 3 - Invalid inventory auth mode fails startup (Priority: P2)

An operator who mistypes `--inventory-auth-mode` (for example `Kubernetes` or
`k8s`) gets a hard startup failure naming the valid values, instead of a server
that authenticates but never authorizes.

**Why this priority**: Catastrophic when hit, but the inventory HTTP server is
off by default, so it is one notch below the exfiltration fix.

**Independent Test**: unit: parse flags with `--inventory-auth-mode=k8s` and run
the startup validator → it returns an error; `kubernetes` and `disabled` pass.

**Acceptance Scenarios**:

1. **Given** the mode is `kubernetes` or `disabled`, **When** startup validates
   it, **Then** validation passes.
2. **Given** the mode is anything else, **When** startup validates it, **Then**
   startup fails with an error naming the accepted values.
3. **Given** the mode is a valid non-disabled value, **Then** the inventory HTTP
   server requires authorization (never authentication-only).

### Edge Cases

- A reference namespace that differs from the sink namespace only by surrounding
  whitespace is treated as the literal value and rejected unless allowlisted;
  the namespace itself is validated by the existing DNS-1123 rules.
- Cluster-scoped kinds (`KollectClusterInventory`) resolve sinks through
  `spec.sinkNamespace`; that path is out of scope for this batch (the referenced
  Secret is still the sink's own namespace's Secret). Only the namespaced family
  sinks carry the vulnerable references.
- The allowlist is empty by default; no configuration means deny-by-default.
- Disabling the webhook server (`--validating-webhooks-enabled=false`) removes
  this admission guard; that is an existing operator choice, noted as residual
  risk rather than silently accepted.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The validating webhook for each namespaced family sink
  (`KollectSnapshotSink`, `KollectDatabaseSink`, `KollectEventSink`) MUST reject,
  on create and update, any `SecretReference` whose `namespace` is non-empty and
  differs from the sink's own namespace, unless that namespace is in the
  process-wide allowlist.
- **FR-002**: The guard MUST cover every Secret reference reachable from a family
  sink spec: `spec.secretRef`, `spec.tls.caSecretRef`, `spec.git.auth.secretRef`,
  `spec.postgres.databaseRef`, `spec.mongodb.databaseRef`,
  `spec.bigquery.secretRef`, `spec.nats.secretRef`, `spec.kafka.secretRef`.
- **FR-003**: The rejection MUST be a `field.ErrorList` `Forbidden` entry at the
  offending `…secretRef.namespace` / `…databaseRef.namespace` path so the API
  server reports the precise field.
- **FR-004**: A process-wide allowlist of permitted cross-namespace Secret
  namespaces MUST exist, default empty, set from the manager flag
  `--allow-secret-ref-namespaces` (comma-separated), and MUST NOT be settable
  through any CRD field.
- **FR-005**: The Helm chart MUST expose the allowlist (`allowSecretRefNamespaces`)
  and pass the flag to the manager only when it is non-empty.
- **FR-006**: Startup MUST fail with an error naming the accepted values when
  `--inventory-auth-mode` is not one of `kubernetes`, `disabled`.
- **FR-007**: When the inventory HTTP server is enabled with a valid non-disabled
  mode, authorization MUST be required (the server MUST NOT run in
  authentication-only mode through a valid flag value).
- **FR-008**: Existing same-namespace and empty-namespace Secret references MUST
  keep working unchanged.

### Key Entities

- **SecretReference**: name plus optional namespace. The namespace is the
  security-relevant field; empty means "the referencing sink's own namespace".
- **Allowlist**: a process-wide set of namespaces that may be referenced
  cross-namespace, derived from one operator flag.
- **Inventory auth mode**: `kubernetes` (TokenReview + SubjectAccessReview) or
  `disabled` (dev/CI only).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A cross-namespace Secret reference on any namespaced family sink is
  rejected by the real API server + webhook when the allowlist is empty, proven
  by an envtest create that returns an admission error naming the field.
- **SC-002**: The same reference is admitted when its namespace is in the
  allowlist, proven by an envtest create that succeeds.
- **SC-003**: An invalid `--inventory-auth-mode` fails startup validation and a
  valid one passes, proven by unit tests.
- **SC-004**: `go test ./...` (repo gates) stays green, including the existing
  webhook and startup-flag suites.

## Assumptions

- The captain's C-1 answer (a) is fixed input; this spec does not revisit the
  policy choice.
- "Operator opt-in allowlist" is implemented as a flat list of *target*
  namespaces permitted for cross-namespace references, mirroring the
  `allowPrivateSinks` process-wide-flag pattern. Per-source-namespace scoping is
  deliberately out of scope as over-engineering for the 80% case; the residual
  risk (an allowlisted namespace can be read by any sink author) is recorded in
  the batch report.
- Findings outside B2 are owned by other lanes and are not touched.
