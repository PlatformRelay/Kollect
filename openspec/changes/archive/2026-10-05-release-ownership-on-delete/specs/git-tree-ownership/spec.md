# Spec Delta

## ADDED Requirements

### Requirement: ROD-1 Deleting an inventory releases its ownership record

When a `KollectInventory` or `KollectClusterInventory` is deleted, the cleanup SHALL remove its
ownership record `.kollect-prune/<sha256(owner)>.json` from every bound Git or GitLab sink it can
reach, under `deletionPolicy: Retain` and `Delete`, including when the deletion skips the retraction
because another live inventory shares its export identity. The owner SHALL be derived from the
inventory's kind, namespace and name with the function the export uses. The finalizer SHALL NOT be
released while the record is still present on a reachable sink.

#### Scenario: Retain deletion of a cluster inventory

- **WHEN** `KollectClusterInventory` `platform` with a recorded tree is deleted and its git sink has `deletionPolicy: Retain`
- **THEN** its record is absent from the branch after the cleanup
- **AND** the record SHALL NOT survive the deletion

#### Scenario: Delete deletion of a namespaced inventory

- **WHEN** `KollectInventory` `team-a/apps` with a recorded tree is deleted and its git sink has `deletionPolicy: Delete`
- **THEN** its record is absent from the branch after the cleanup

#### Scenario: Shared export identity

- **WHEN** `KollectClusterInventory` `platform` is deleted with `deletionPolicy: Delete` while `KollectInventory` `cluster/platform` exists
- **THEN** the cluster inventory's record is removed and no exported file is deleted

#### Scenario: No identity

- **WHEN** a git sink cleanup request carries no inventory identity
- **THEN** it fails with a terminal error before the backend is built

### Requirement: ROD-2 Retain keeps every exported file

Under `deletionPolicy: Retain` the release SHALL change nothing but the deleting inventory's record
file, in one commit.

#### Scenario: Retain release commit

- **WHEN** an inventory with a recorded tree and a document is deleted under `Retain`
- **THEN** every file outside `.kollect-prune/` has the same bytes as before
- **AND** the release SHALL NOT delete any exported file

### Requirement: ROD-3 A release never touches another inventory's record or files

A release, under either policy, SHALL NOT remove or rewrite another inventory's record or any path
another inventory's record lists, including when the two inventories render the same object path.

#### Scenario: Same object path, other kind

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` both have records and the cluster inventory is deleted under either policy
- **THEN** the namespaced inventory's record and files are unchanged
- **AND** the release SHALL NOT remove another inventory's record

### Requirement: ROD-4 A Delete retraction is ownership-correct

Under `deletionPolicy: Delete` the retraction SHALL remove, in one commit, the inventory's candidate
paths (document, parts, layout sidecars), every path its own record lists, and its record. It SHALL
NOT delete a path another inventory's record lists. A deletion without an owner (`DeleteExport`) and
an ownerless directory-scoped tree prune SHALL NOT delete a path any record lists.

#### Scenario: Recorded tree retracted

- **WHEN** an inventory whose record lists per-resource files is deleted under `Delete`
- **THEN** those files, its document and its record are removed in one commit

#### Scenario: Candidate path recorded by another inventory

- **WHEN** a candidate path of the deleting inventory is listed by another inventory's record
- **THEN** that file is kept with its bytes
- **AND** the retraction SHALL NOT delete it

#### Scenario: Ownerless deletion

- **WHEN** `DeleteExport` without an owner matches a path some record lists
- **THEN** that file is kept

### Requirement: ROD-5 The release is idempotent

An absent record SHALL count as released: the cleanup succeeds, commits nothing and releases the
finalizer. A retried release after a successful one SHALL succeed without a commit.

#### Scenario: No record

- **WHEN** an inventory that never wrote a record is deleted under `Retain`
- **THEN** the cleanup succeeds and the branch tip does not move

#### Scenario: Retry after success

- **WHEN** a release succeeded and the cleanup runs again
- **THEN** it succeeds and SHALL NOT fail the finalizer

### Requirement: ROD-6 A failed release keeps the finalizer's retry semantics

A release that cannot reach or update the backend SHALL return an error classified like a failed
retraction (transient unless the backend classified it terminal), so the finalizer stays and the
deletion is retried. It SHALL NOT report the record released.

#### Scenario: Unreachable backend under Retain

- **WHEN** the git sink's backend cannot be built or reached during a `Retain` deletion
- **THEN** the cleanup returns a transient error and the finalizer is kept
- **AND** the outcome SHALL NOT be reported as retained by policy

### Requirement: ROD-7 A released slot can be taken over

After a release, an inventory of another kind or name SHALL be able to export the paths the released
record listed, and the released owner SHALL no longer count toward the owner cap.

#### Scenario: Successor of another kind

- **WHEN** `KollectClusterInventory` `platform` owned a path, was deleted under `Retain`, and `KollectInventory` `cluster/platform` exports that path
- **THEN** the export succeeds and its record lists the path

## RENAMED Requirements

- FROM: `### Requirement: IEI-8 Delete and recreate continues the record`
- TO: `### Requirement: IEI-8 Delete and recreate starts a new record`

## MODIFIED Requirements

### Requirement: IEI-8 Delete and recreate starts a new record

The owner SHALL carry no UID. An inventory recreated with the same kind, namespace and name after a
deletion that released its record (ROD-1) SHALL start a new record and SHALL NOT delete any file its
predecessor exported. Only when the deletion could not reach the sink (the sink was gone or the
finalizer was forced) does the record survive, and a recreated inventory with the same identity then
continues it. An inventory of another kind or name SHALL NOT inherit a surviving record.

#### Scenario: Recreate with the same identity

- **WHEN** cluster inventory `platform` is deleted with `deletionPolicy: Retain` and recreated, and its new snapshot omits a resource the old one exported
- **THEN** that resource's file is kept, and the new record lists only the new snapshot's files

#### Scenario: Recreate as the other kind

- **WHEN** cluster inventory `platform` is deleted and `KollectInventory` `cluster/platform` exports
- **THEN** the namespaced inventory SHALL NOT delete any file the cluster inventory exported
