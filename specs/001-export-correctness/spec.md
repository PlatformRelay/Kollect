# Feature Specification: Export correctness — no silent drops, no torn relational sets

**Feature Branch**: `fm/kollect-b1-export` (target: `fix/export-spill-silent-drop`)

**Created**: 2026-09-24

**Status**: Draft

**Input**: Unify report batch B1 — findings K-01, K-02, K-03, K-31
(`data/kollect-unify/report.md` §4, §6). Captain calls C-3 = (b): rename
"spill" to "inline cap" in docs and make the skip loud; real object-store spill
is deferred to a later feature.

## Context

Kollect's export pipeline has one hard-coded inline ceiling (`SpillMandatoryBytes`
= 1 MiB) and a configurable per-binding `maxExportBytes` ceiling (default global
1.5 MiB). Above 1 MiB, non-object-store sinks are supposed to require an
object-store spill. There is no spill write path: the code simply skips the
export and returns success. The controller then records `Exported`/`Ready`,
suppressing re-export for up to 24 h. Separately, multipart partitioning runs for
every sink family, but only snapshot (git/s3/gcs) backends understand
`partIndex`/`partTotal`; relational backends diff-delete per part and event
backends emit independent markerless streams.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Oversize non-spillable exports fail loudly (Priority: P1)

As an operator, when a collected snapshot cannot be delivered to a non-object-store
sink because it exceeds the inline cap, I want the inventory to report
`Degraded`/`SpillRequired` (never `Exported`/`Ready`), so I do not believe data
was persisted when it was silently dropped.

**Why this priority**: K-01 is the headline CRITICAL — silent data loss reported
as success. It is the reproduced defect and the smallest blast radius.

**Independent Test**: A git-only binding with a payload of `SpillMandatoryBytes+1`
(1.2 MiB) yields `Degraded` with reason `SpillRequired`; the sink status is not
`Exported`; the coalesce checksum is not recorded.

**Acceptance Scenarios**:

1. **Given** a git-only inventory binding and a 1.2 MiB payload, **When** the
   export runs, **Then** the inventory is `Degraded`/`SpillRequired` and the git
   sink reports `Synced=False`.
2. **Given** a mixed git+postgres binding and a 1.2 MiB payload, **When** the
   export runs, **Then** the git sink fails with `SpillRequired` and the overall
   inventory is not `Ready=True reason=Exported`.
3. **Given** an s3 object-store binding and a 1.2 MiB payload, **When** the
   export runs, **Then** the export succeeds (object stores accept the full
   payload).

---

### User Story 2 - Relational/event sinks receive the complete set (Priority: P1)

As an operator, when a database or event sink has a per-binding ceiling smaller
than the snapshot, I want every row delivered in one complete snapshot, so the
sink's diff-delete does not erase the rows from an earlier part.

**Why this priority**: K-02 is the destructive face of the same defect; it is
HIGH and must ship with K-01.

**Independent Test**: A postgres-only binding with `maxExportBytes: 500KiB` and a
snapshot between 500 KiB and 1 MiB yields all rows at the sink, not only the last
part.

**Acceptance Scenarios**:

1. **Given** a postgres-only binding with a 500 KiB per-binding ceiling and a
   ~700 KiB snapshot, **When** the export runs, **Then** the sink receives a
   single complete payload and every row survives.
2. **Given** a snapshot (git) binding, **When** the payload exceeds its ceiling,
   **Then** multipart partitioning still applies (git multipart is a shipped
   feature and handles part markers + union prune).

---

### User Story 3 - Documentation names the real mechanism (Priority: P2)

As an operator reading the docs, I want the "spill" concept to be described as an
inline cap with a loud skip, so the documentation does not advertise a capability
that does not exist.

**Why this priority**: K-03 (WARNING) is a truthfulness fix that ships with the
K-01 behaviour change.

**Independent Test**: `docs/REQUIREMENTS.md` and `docs/adr/0103-etcd-limit.md` no
longer claim an object-store spill mechanism exists; they describe the inline cap
and the `SpillRequired` degradation.

**Acceptance Scenarios**:

1. **Given** the updated docs, **When** an operator reads the export-size policy,
   **Then** it states that payloads above the inline cap degrade with
   `SpillRequired` unless an object-store sink is bound, and that no automatic
   spill write path exists.

---

### Edge Cases

- Payload exactly at `SpillMandatoryBytes` (1 MiB): not `RequiresSpill`
  (strictly above). Exports normally.
- Payload above the inventory ceiling: pre-export gate reports `PayloadTooLarge`
  (existing behaviour, unchanged).
- Single item larger than the per-binding ceiling: `PartitionEnvelopes` returns a
  terminal error; only that binding fails (existing behaviour, unchanged).
- Cluster `KollectClusterInventory`: same partition decision and same spill error
  mapping as the namespaced path.
- K-31: the register assigns this ID to batch B1 but contains no K-31 entry (the
  register omits K-27, K-31, K-43). The export path fix covers both the namespaced
  and cluster call sites; the missing definition is recorded in `plan.md`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: `RunExportEnvelope` MUST return a terminal, classifiable error when
  a non-object-store backend cannot accept a payload above `SpillMandatoryBytes`
  (instead of returning `nil`).
- **FR-002**: The error MUST be identifiable so controllers map it to reason
  `SpillRequired`, on both the per-sink status and the aggregate inventory
  condition.
- **FR-003**: A failed oversize export MUST NOT record a coalesce checksum or set
  the sink `Synced=True reason=Exported`.
- **FR-004**: Multipart partitioning MUST apply only to snapshot-family bindings
  (git/s3/gcs). Database and event families MUST receive a single complete part.
- **FR-005**: The inline-cap decision MUST be enforced at the sink boundary so it
  holds for every call site (namespaced and cluster inventories, cleanup).
- **FR-006**: Documentation MUST describe the mechanism as an inline cap with a
  loud `SpillRequired` skip and MUST NOT claim an object-store spill write path
  exists.
- **FR-007**: Existing snapshot multipart behaviour (part markers, union prune,
  `PartitionsChecksum` debounce) MUST be preserved.

### Key Entities

- **SpillAssessment**: payload size vs warn / spill-mandatory / hard-cap
  thresholds (unchanged).
- **EnvelopePartition**: one bounded envelope slice with index/total/checksum.
- **ErrSpillRequired**: new sentinel terminal error raised at the sink boundary.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A git-only 1.2 MiB export never reports `Exported`/`Ready`; it
  reports `Degraded`/`SpillRequired` (end-to-end test).
- **SC-002**: A postgres-only binding with a 500 KiB ceiling delivers all rows
  (end-to-end test).
- **SC-003**: All existing export, partition, controller, and sink tests remain
  green; coverage floor (90% aggregate) holds.
- **SC-004**: `golangci-lint` and the repository's own gates pass.

## Assumptions

- Captain call C-3 = (b) is authoritative: rename in docs, loud skip now, real
  spill later.
- Snapshot family = git + s3 + gcs; only these understand `partIndex`/`partTotal`
  and run a union prune.
- The 1 MiB inline cap remains process-wide; operators who need larger relational
  payloads must wait for real spill or use an object store.
- K-31 has no recoverable definition in the available artefacts; the fix covers
  the full export surface and the gap is recorded rather than guessed.
