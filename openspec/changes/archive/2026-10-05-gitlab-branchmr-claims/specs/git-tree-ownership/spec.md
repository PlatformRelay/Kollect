# Spec Delta

## ADDED Requirements

### Requirement: IEI-10 A merge-request export never pushes a claim the target already gives another inventory

An export that pushes to a branch other than the one it cloned SHALL, before writing anything, check
every path it writes, records or pre-claims against the ownership records of the freshly fetched
merge target tip, in addition to the checked-out branch. A path that another inventory's record on the
target lists SHALL fail the export with a terminal error naming the path, the target branch, the
owning inventory and its record file. Malformed ownership metadata on the target SHALL fail the
export. A target branch that does not exist yet SHALL hold no claims.

#### Scenario: Claim merged after the feature branch was pushed

- **WHEN** inventory B pushed its merge-request branch, inventory A's claim on path P then merged into the target, and B's next export from the warm mirror also writes P
- **THEN** B's export fails with a terminal error containing P, `belongs to another inventory`, A's kind and namespace/name and A's record file
- **AND** B's feature branch SHALL NOT move
- **AND** no branch SHALL exist whose merge into the target claims P twice

#### Scenario: Own claims stay exportable

- **WHEN** B's next export after the refusal no longer writes P
- **THEN** it is pushed to B's feature branch

#### Scenario: Claim already on the target at the first export

- **WHEN** A's claim on P is on the target and inventory C's first merge-request export writes P
- **THEN** C's export fails with a terminal error naming A
- **AND** C's feature branch SHALL NOT be created

#### Scenario: Set manifest claimed on the target

- **WHEN** a non-final part pre-claims a set manifest that another inventory's record on the target lists
- **THEN** the part fails with a terminal error

### Requirement: IEI-11 Each inventory has its own merge-request branch

The GitLab sink SHALL push the merge request of a `KollectInventory` to `<prefix>/<namespace>/<name>`
and that of a `KollectClusterInventory` to `<prefix>/_cluster/<name>`, using the kind the export
pipeline passes with every export. A `KollectClusterInventory` `X` and a `KollectInventory` `X` in
namespace `cluster` SHALL NOT share a branch, and no branch SHALL carry the ownership records of two
inventories. A caller without a kind SHALL get the cluster inventory's branch for path namespace
`cluster` and the namespaced inventory's branch otherwise.

#### Scenario: Same object path, two kinds

- **WHEN** `KollectClusterInventory` `platform` and `KollectInventory` `cluster/platform` both export the same file in merge-request mode
- **THEN** the remote holds two feature branches, each carrying exactly one inventory's record
- **AND** neither branch SHALL claim the shared file twice

### Requirement: IEI-12 The duplicate-ownership error names both owners and their records

When the ownership records on a branch already claim one path twice, the export SHALL fail with a
terminal error naming the path, both owning inventories and both record files, so that an operator
can repair the branch by deleting one record.

#### Scenario: Polluted branch

- **WHEN** the records of `KollectInventory` `team-a/apps` and `KollectClusterInventory` `platform` both list path P
- **THEN** every export to that branch fails with a terminal error containing P, both inventories' kind and name, and both `.kollect-prune/<sha256(owner)>.json` paths
