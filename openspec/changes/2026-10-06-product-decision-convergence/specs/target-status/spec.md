# Spec Delta

## ADDED Requirements

### Requirement: TSP-1 A KollectClusterTarget exposes the collected count and when it last changed

`KollectClusterTarget` status SHALL carry `collectedCount` and `collectedCountUpdatedAt` with the
same semantics as `KollectTarget`: the count is the machine-readable number of resources the
cluster target was collecting at its last healthy refresh, `null` means never computed, and the
timestamp moves only when the number changes. Both fields SHALL be exposed as kubectl printer
columns (Collected / Updated) mirroring the namespaced target.

#### Scenario: Cluster target persists its count

- **WHEN** a `KollectClusterTarget` reaches Ready while its engine-registered synthetic targets hold 17 items
- **THEN** `status.collectedCount == 17` and `status.collectedCountUpdatedAt` is set

#### Scenario: Steady count keeps its timestamp

- **WHEN** two consecutive reconciles derive the same count
- **THEN** `collectedCountUpdatedAt` does not move and the count value is unchanged

#### Scenario: Count change is persisted independently of the condition text

- **WHEN** the derived count changes while the Ready condition payload stays byte-identical (the shared no-op-skip writer would skip the write)
- **THEN** the new count is still persisted (the PERF-FIX-05 escape hatch holds on the cluster path)

#### Scenario: Degraded target keeps its last count

- **WHEN** a previously Ready cluster target goes Degraded
- **THEN** its last known count and timestamp are not cleared
