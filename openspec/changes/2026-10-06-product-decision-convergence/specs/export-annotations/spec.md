# Spec Delta

## ADDED Requirements

### Requirement: ERA-1 Changing kollect.dev/requestedAt forces one immediate export past the debounce

The inventory reconcilers SHALL read `kollect.dev/requestedAt` from the reconciled
`KollectInventory` / `KollectClusterInventory` object and treat a change of its value (including
its appearance on an object that had none, and its removal after being set) as an invalidation of
the per-sink export debounce: the next reconcile SHALL export to every sink binding that would
otherwise be debounced. After that export, the steady-state debounce SHALL resume unchanged: same
content, same generation and no further change to the annotation SHALL debounce again as before.
The annotation documentation SHALL scope the key to exactly the kinds whose reconcilers honour it
(the two inventory kinds), not to "reconciled Kollect CRs" generally.

#### Scenario: Changed annotation re-exports unchanged content

- **WHEN** an inventory exported to a sink at t0, its content checksum and generation are unchanged, and the `requestedAt` annotation value is changed
- **THEN** the next reconcile exports to that sink instead of debouncing
- **AND** after that export, a reconcile with no further change debounces again

#### Scenario: Unchanged annotation keeps the debounce

- **WHEN** the annotation value is unchanged across reconciles with unchanged content
- **THEN** the per-sink debounce behaves exactly as without the annotation

#### Scenario: Cluster path behaves like the namespaced path

- **WHEN** the annotation changes on a `KollectClusterInventory`
- **THEN** the same one-export bypass applies on the cluster export path

#### Scenario: Preview does not report a debounce the export will not do

- **WHEN** the annotation changes and a preview is rendered before the next export
- **THEN** the preview does not report the affected bindings as debounced

#### Scenario: Absence is a value, not a wildcard

- **WHEN** the annotation is removed after being set
- **THEN** the next reconcile exports once (absence-to-set and set-to-absence are both changes)
- **AND** the `Synced` condition message and requeue cadence stay as for any other export

### Requirement: ERA-2 Exported source-object copies record the collected generation

When a profile exports in Resource mode, the embedded pruned copy of the source object SHALL carry
`kollect.dev/collectedGeneration: "<n>"` in its `metadata.annotations` whenever the copy retains a
metadata section, where `<n>` is the source object's `metadata.generation` at collection time. The
stamp SHALL be applied after pruning and scrubbing, so profile prune paths and scrub rules cannot
remove it. A profile whose include section drops metadata (the default `SpecAndStatus` does) SHALL
export the copy without the stamp: there is no metadata section to carry it. The annotation
documentation SHALL state that the stamp requires a metadata-retaining include section.

#### Scenario: Resource-mode copy is stamped

- **WHEN** a target whose profile sets `export.mode: Resource` collects an object with generation 42
- **THEN** the exported Item's embedded copy has `metadata.annotations["kollect.dev/collectedGeneration"] == "42"`

#### Scenario: Stamp survives profile pruning of annotations

- **WHEN** the profile's prune spec removes `metadata.annotations` from the embedded copy
- **THEN** the exported copy still carries the `kollect.dev/collectedGeneration` annotation

#### Scenario: No metadata section, no stamp

- **WHEN** the profile's include section excludes metadata so the embedded copy has no `metadata` map
- **THEN** the copy carries no stamp and the export succeeds

#### Scenario: Attributes mode is untouched

- **WHEN** a profile uses the default Attributes mode
- **THEN** the exported Item is byte-identical to today's output for the same input
