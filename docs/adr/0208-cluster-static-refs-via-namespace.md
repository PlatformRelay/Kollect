# ADR-0208: Cluster work references namespaced static configuration

> Cluster targets and inventories resolve profiles and family sinks by namespace plus name.

**Theme:** 02 · API & tenancy · **Status:** Current

## Context

Profiles and destinations contain team-owned configuration and namespaced Secret references.
Duplicating them into cluster-scoped kinds would create competing sources of truth and broaden
credential access. Cluster-wide collection still needs an explicit way to consume approved shared
configuration.

## Decision

- `KollectProfile`, `KollectSnapshotSink`, `KollectDatabaseSink`, and `KollectEventSink` are only
  namespaced.
- `KollectClusterTarget.spec.profileRef` identifies a profile with `name` and `namespace`.
- Sink references on `KollectClusterInventory` identify family sinks with `name` and `namespace`.
- Admission and reconciliation require explicit namespaces, enforce `KollectClusterScope` ceilings,
  and report missing, denied, or forbidden references with conditions and Events.
- Shared platform configuration normally lives in an operator namespace such as `kollect-system`;
  tenant configuration remains in tenant namespaces.

## Security boundary

Reference permission is not implied by object existence. Scope policy constrains allowed
configuration namespaces, and the manager's Kubernetes RBAC must permit the referenced reads.
Credentials remain in the sink namespace and are resolved only for an authorized reference.

## Consequences

- Platform teams publish one reusable profile or sink for both namespaced and cluster-wide work.
- Secret ownership and tenancy stay visible in the Kubernetes namespace model.
- Cluster manifests are slightly more explicit because every static-config reference includes a
  namespace.
