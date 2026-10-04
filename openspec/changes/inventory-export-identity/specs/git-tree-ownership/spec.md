# Spec Delta

## Purpose

Defines which files a Git or GitLab tree export (`perResource` or `split` layout) may delete, which
inventory identity owns them, and how ownership records written by earlier builds are treated, so
that pruning removes an inventory's own stale files and never another inventory's or an unproven one.

## ADDED Requirements

### Requirement: IEI-1 Export requests carry the inventory kind, namespace and name

Both inventory reconcilers SHALL pass the inventory's kind, namespace (empty for
`KollectClusterInventory`) and name with every snapshot export, and the sink SHALL derive the prune
owner from that identity rather than from the object path. The owner of a `KollectClusterInventory`
`X` and of a `KollectInventory` `X` in namespace `cluster` SHALL differ.

#### Scenario: Cluster inventory export

- **WHEN** `KollectClusterInventory` `platform` exports to a git sink with a `perResource` layout
- **THEN** the export request names kind `KollectClusterInventory`, an empty namespace and name `platform`

#### Scenario: Same name, different kind

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` export to the same sink
- **THEN** their prune owners and ownership record paths differ
- **AND** neither owner SHALL equal the legacy owner `["default","cluster","platform"]`

### Requirement: IEI-2 A cluster inventory and a namespaced inventory in namespace cluster never delete each other's files

An export by one of these two inventories SHALL NOT delete a file recorded for the other, on any
snapshot, including an empty or shrinking one and the final part of a multipart set. When both project
the same file path, the export that would claim a path the other owns SHALL be rejected without
writing or deleting anything.

#### Scenario: Disjoint trees, then an empty snapshot

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` export disjoint resource trees, then the cluster inventory exports an empty snapshot
- **THEN** the cluster inventory's files are removed
- **AND** the namespaced inventory's files SHALL NOT be deleted

#### Scenario: Shared projected path

- **WHEN** both inventories project the same file (for example the split index `inventory/cluster/platform.yaml`) and one of them already owns it
- **THEN** the other's export fails with an error naming the path, and SHALL NOT change any file or record

### Requirement: IEI-3 Cluster inventories prune their own stale files

A complete tree export by a `KollectClusterInventory` whose request carries an identity SHALL delete
exactly the paths its previous record lists and the current export no longer projects, and an empty
snapshot SHALL remove all of them. The export SHALL NOT be a no-op because the namespace component is
`cluster`.

#### Scenario: A resource disappears

- **WHEN** a cluster inventory exported Deployments `api` and `web`, and the next snapshot holds only `api`
- **THEN** the `web` file is deleted in the same commit that updates the record

#### Scenario: Empty snapshot

- **WHEN** a cluster inventory with recorded files exports an empty snapshot to an explicit tree layout
- **THEN** its recorded files are deleted and an empty record is committed

### Requirement: IEI-4 Owners of other namespaced inventories are unchanged

For a `KollectInventory` in any namespace other than `cluster`, the prune owner and its record path
SHALL be byte-identical to the current owner `[cluster, namespace, name]`, and an existing record for it
SHALL keep governing pruning with no migration step.

#### Scenario: Existing record after upgrade

- **WHEN** a repository holds a record for `team-a/apps` written before the upgrade and `team-a/apps` exports a snapshot that dropped one resource
- **THEN** that resource's file is deleted, using the existing record
- **AND** the export SHALL NOT write a second record for `team-a/apps`

### Requirement: IEI-5 Migration never deletes a file whose ownership cannot be proven

A record under the ambiguous legacy owner `[cluster, "cluster", name]` SHALL NOT be used to delete any
file. The first complete export of either the cluster inventory or the namespaced inventory in
namespace `cluster` with that name SHALL remove that record in the same commit and leave every file it
listed in place. A repository with no record for an owner SHALL adopt only the paths the current export
writes.

#### Scenario: Ambiguous legacy record

- **WHEN** a repository holds a legacy record `["default","cluster","platform"]` listing files `p1` and `p2`, and `KollectClusterInventory` `platform` completes an export that writes only `p1`
- **THEN** the legacy record is removed, the new record lists `p1`, and `p2` remains in the repository
- **AND** no later export of any inventory SHALL delete `p2`

#### Scenario: Legacy claim does not block the same name

- **WHEN** the legacy record lists a path that `KollectInventory` `cluster/platform` now writes
- **THEN** the export is not rejected for that path

#### Scenario: Legacy claim still blocks other names

- **WHEN** the legacy record lists a path that `KollectInventory` `team-a/apps` now writes
- **THEN** that export is rejected as claiming another inventory's path, and nothing is written or deleted

#### Scenario: Repository written by v0.21.0

- **WHEN** a repository has no `.kollect-prune` directory and holds files from earlier exports
- **THEN** the first export records only the paths it writes, and SHALL NOT delete any existing file

### Requirement: IEI-6 Returning to an earlier snapshot replays it

For every owner, including the two of IEI-2, an export sequence A, then B, then A SHALL leave the
repository with exactly the files of A for that owner, and the coalescing cache SHALL NOT skip the
second A.

#### Scenario: A to B to A for a cluster inventory

- **WHEN** a cluster inventory exports snapshot A (`api`, `web`), then B (`api`), then A again
- **THEN** after B the `web` file is gone, and after the second A it is present again

### Requirement: IEI-7 Delete and recreate

An inventory recreated with the same kind, namespace and name (a new UID) SHALL continue its
predecessor's record: its first complete export deletes only the predecessor's recorded paths it no
longer projects. An inventory with a different kind or name SHALL NOT inherit a record.

#### Scenario: Recreate with the same identity

- **WHEN** cluster inventory `platform` is deleted with `deletionPolicy: Retain` and recreated, and its new snapshot omits a resource the old one exported
- **THEN** that resource's file is deleted, and no file outside the old record is deleted

#### Scenario: Recreate as the other kind

- **WHEN** cluster inventory `platform` is deleted and `KollectInventory` `cluster/platform` is created
- **THEN** the namespaced inventory SHALL NOT delete any file the cluster inventory's record lists

### Requirement: IEI-8 Multipart sets prune once and migrate once

For a multipart export with an identity, non-final parts SHALL NOT delete files, advance a record or
remove a legacy record. The final part SHALL prune against the union of all parts, and is the only part
that removes the ambiguous legacy record. An interrupted set SHALL leave all records unchanged.

#### Scenario: Interrupted set over a legacy record

- **WHEN** a cluster inventory sends part 1 of 2 in a repository holding the ambiguous legacy record, and part 2 never arrives
- **THEN** no file is deleted, and the legacy record is still present

#### Scenario: Complete set

- **WHEN** a cluster inventory completes a two-part set whose union drops a previously recorded resource
- **THEN** that file is deleted on the final part only, and files of both parts survive

### Requirement: IEI-9 Requests without an identity keep the fail-safe

An export request that carries no inventory identity SHALL keep today's owner derivation from the object
path, and SHALL NOT prune when the namespace component is `cluster`.

#### Scenario: Identity-less request on the shared path

- **WHEN** a request without an identity exports `inventory/cluster/platform.json` with an empty snapshot
- **THEN** no file is deleted

### Requirement: IEI-10 The record format does not change

Ownership records SHALL keep `version: 1` and the fields `version`, `owner` and `paths`, so a record with
a new owner parses in the current reader.

#### Scenario: Record for a cluster inventory

- **WHEN** a cluster inventory writes its record
- **THEN** the record decodes with the current strict reader and its path is the SHA-256 of its owner
