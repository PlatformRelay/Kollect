# Multi-tenancy and scopes

The recommended boundary is one namespaced pipeline per team. `KollectScope` constrains allowed
GVKs, namespaces, and sinks; admission and SubjectAccessReview checks enforce the boundary before
collection begins. Watch labels provide an additional opt-in or opt-out filter, not an RBAC bypass.

Use a cluster-scoped resource only when cluster-wide ownership is intentional. Prefer workload
identity and namespace-local references, and keep the operator's watched namespaces as narrow as
the platform model permits.

See [ADR-0203](../adr/0203-namespaced-multi-tenancy.md),
[ADR-0205](../adr/0205-watch-labels.md), and the
[production checklist](../operator-manual/production-checklist.md).
