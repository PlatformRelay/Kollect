# ADR-0201: CRD model — prefixed kinds and explicit scope

> The current `Kollect*` API, its scopes, and the split between configuration and work resources.

**Theme:** 02 · API & tenancy · **Status:** Current

## Context

Kollect needs reusable extraction configuration, typed destinations, collection selectors, and
durable inventory composition without placing exported payloads in etcd. Public kind names are
prefixed to avoid collisions in clusters with many operators.

## Decision

The API group is `kollect.dev/v1alpha1` and the shipped kinds are:

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectProfile` | Namespace | Reusable GVK and CEL/JSONPath extraction schema |
| `KollectSnapshotSink` | Namespace | Git, GitLab, S3, or GCS snapshot destination |
| `KollectDatabaseSink` | Namespace | Postgres, MongoDB, or BigQuery state destination |
| `KollectEventSink` | Namespace | Kafka or NATS event destination |
| `KollectScope` | Namespace | Tenant allowlists for GVKs, namespaces, and sink references |
| `KollectTarget` | Namespace | Profile reference plus resource selectors; drives collection |
| `KollectInventory` | Namespace | Composes targets and exports their canonical snapshot |
| `KollectClusterTarget` | Cluster | Cross-namespace collection with explicit namespace selection |
| `KollectClusterInventory` | Cluster | Composes namespaced targets/snapshots for platform rollups |
| `KollectClusterScope` | Cluster | Collection ceiling for cluster-scoped work |
| `KollectConnectionTest` | Namespace | Explicit one-shot destination probe |

Cluster-scoped work resources reference namespaced profiles and sinks using a name plus namespace;
there are no cluster-scoped profile or sink kinds ([ADR-0208](0208-cluster-static-refs-via-namespace.md)).
The old unified `KollectSink` is not a public kind; the similarly named Go struct is only an internal
adapter for family sink implementations ([ADR-0414](0414-sink-family-crds.md)).

All reconciled work kinds expose status conditions and `observedGeneration`. Full collected payloads
are exported to sinks; status contains only bounded summaries and references.

## Validation and trust

- CRD schema and validating webhooks reject invalid expressions, unknown sink types, incompatible
  family fields, and unsafe endpoints.
- Sink credentials use `secretRef`; outbound TLS trust uses `tls.caSecretRef` or a size-bounded
  inline `tls.caBundle`. TLS verification remains the default.
- Namespace and scope checks are enforced before collection and export.

## Rejected kinds

- `KollectPublication`: documentation publication belongs in external CI
  ([ADR-0702](0702-doc-sync-templating.md)).
- `KollectHub`: fleets use shared-sink fan-in ([ADR-0501](0501-multi-cluster-fleet.md)).

## Consequences

- Namespaced configuration aligns ownership and Secret access with teams.
- Cluster rollups remain possible without duplicating platform configuration into cluster-scoped
  APIs.
- Family-specific schemas and webhooks prevent invalid destination combinations at admission.
