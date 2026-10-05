# git-tree-ownership Specification

## Purpose
Defines which files a Git or GitLab tree export (`perResource` or `split` layout) may delete and
which inventory identity owns them, so that pruning removes an inventory's own stale files and never
another inventory's.

## Requirements

### Requirement: IEI-1 Export requests carry the inventory kind, namespace and name

Both inventory reconcilers SHALL pass the inventory's kind, namespace (empty for
`KollectClusterInventory`) and name with every snapshot export, and the sink SHALL derive the prune
owner of every inventory from that identity as `["v2", kind, cluster, namespace, name]`, never from
the object path. The owner of a `KollectClusterInventory` `X` and of a `KollectInventory` `X` in
namespace `cluster` SHALL differ.

#### Scenario: Cluster inventory export

- **WHEN** `KollectClusterInventory` `platform` exports to a git sink with a `perResource` layout and no `spec.cluster`
- **THEN** the ownership engine receives owner `["v2","KollectClusterInventory","default","","platform"]` with prune requested

#### Scenario: Namespaced inventory export

- **WHEN** `KollectInventory` `default/team-inventory` exports to the same kind of sink
- **THEN** the ownership engine receives owner `["v2","KollectInventory","default","default","team-inventory"]`

#### Scenario: Same name, different kind

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` export to the same sink
- **THEN** their owners and ownership record paths differ
- **AND** generation and multipart suffixes SHALL NOT change either owner

### Requirement: IEI-2 A cluster inventory and a namespaced inventory in namespace cluster never delete each other's files

An export by one of these two inventories SHALL NOT delete or rewrite a file recorded for the other,
on any snapshot, including an empty or shrinking one and the final part of a multipart set. When both
project the same file path, the export that would claim a path the other owns SHALL be rejected
without writing or deleting anything.

#### Scenario: Disjoint trees, then an empty snapshot

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` export disjoint resource trees, then the cluster inventory exports an empty snapshot
- **THEN** the cluster inventory's files are removed
- **AND** the namespaced inventory's files SHALL NOT be deleted

#### Scenario: Shared projected path

- **WHEN** both inventories project the same file and one of them already owns it
- **THEN** the other's export fails with a terminal error naming the path
- **AND** it SHALL NOT change any file's bytes or any record

### Requirement: IEI-3 Cluster inventories prune their own stale files

A complete tree export by a `KollectClusterInventory` SHALL delete exactly the paths its previous
record lists and the current export no longer projects, and an empty snapshot SHALL remove all of them.
The export SHALL NOT be a no-op because the namespace component is `cluster`.

#### Scenario: A resource disappears

- **WHEN** a cluster inventory exported Deployments `api` and `web`, and the next snapshot holds only `api`
- **THEN** the `web` file is deleted in the same commit that updates the record

#### Scenario: Empty snapshot

- **WHEN** a cluster inventory with recorded files exports an empty snapshot to an explicit tree layout
- **THEN** its recorded files are deleted and an empty record is committed

#### Scenario: Return to an earlier snapshot

- **WHEN** a cluster inventory exports snapshot A, then B, then A again
- **THEN** after the second A exactly A's files are present for that inventory

#### Scenario: Interrupted multipart set

- **WHEN** a cluster inventory sends part 1 of 2 and part 2 never arrives
- **THEN** no file is deleted and its record is unchanged

### Requirement: IEI-4 The identity is mandatory and must match the object path

A git layout export without an inventory identity SHALL fail with a terminal error. An identity whose
namespace or name differs from those parsed from the object path (multipart suffix removed), a
`KollectClusterInventory` whose path namespace is not `cluster`, or an identity with an unknown kind or
a namespace that does not fit its kind SHALL fail with a terminal error before the backend is reached.

#### Scenario: No identity

- **WHEN** a `perResource` git export request carries no identity
- **THEN** it fails with a terminal error, and the backend SHALL NOT be called

#### Scenario: Identity on another inventory's path

- **WHEN** a request names `KollectInventory` `team-b/apps` on object path `inventory/team-a/apps.json`
- **THEN** it fails with a terminal error, and SHALL NOT change any file or record

### Requirement: IEI-5 No commit claims one path twice

Before writing, the engine SHALL refuse a commit whose ownership records would claim one path twice,
with a terminal error naming the path and both owners.

#### Scenario: Colliding records

- **WHEN** the records to be committed list the same path for two owners
- **THEN** the export fails with a terminal error and SHALL NOT write anything

### Requirement: IEI-6 A multipart set whose manifest belongs to another inventory commits nothing

Every non-final part of a prune-bearing multipart set SHALL claim-check the set-manifest path the
final part will write. A set whose manifest path another inventory's record lists SHALL be rejected
on part 1.

#### Scenario: Foreign manifest

- **WHEN** `KollectInventory` `cluster/platform` owns `inventory/cluster/platform.manifest.json` and `KollectClusterInventory` `platform` starts a two-part set
- **THEN** part 1 fails with a terminal error naming the manifest path
- **AND** no file of the set SHALL be committed

### Requirement: IEI-7 The ownership rejection names the owner

The "belongs to another inventory" error SHALL name the owning inventory (kind, namespace/name and
sink cluster, decoded from its owner) and the path of its record file.

#### Scenario: Rejection text

- **WHEN** `KollectInventory` `cluster/platform` owns a path that `KollectClusterInventory` `platform` now writes
- **THEN** the error contains `KollectInventory cluster/platform (cluster "default")` and `.kollect-prune/<sha256(owner)>.json`

### Requirement: IEI-8 Delete and recreate continues the record

The owner SHALL carry no UID. An inventory recreated with the same kind, namespace and name SHALL
continue its predecessor's record, including after a `deletionPolicy: Retain` deletion. An inventory
of another kind or name SHALL NOT inherit it.

#### Scenario: Recreate with the same identity

- **WHEN** cluster inventory `platform` is deleted with `deletionPolicy: Retain` and recreated, and its new snapshot omits a resource the old one exported
- **THEN** that resource's file is deleted, and no file outside the old record is deleted

#### Scenario: Recreate as the other kind

- **WHEN** cluster inventory `platform` is deleted and `KollectInventory` `cluster/platform` exports
- **THEN** the namespaced inventory SHALL NOT delete any file the cluster inventory's record lists

### Requirement: IEI-9 The record format does not change

Ownership records SHALL keep `version: 1` and the fields `version`, `owner` and `paths`, at
`.kollect-prune/<sha256(owner)>.json`.

#### Scenario: Record for a cluster inventory

- **WHEN** a cluster inventory writes its record
- **THEN** the record decodes with the strict reader and its path is the SHA-256 of its owner
