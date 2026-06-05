# ADR-0004: CRD model — prefixed kinds, static vs reconciled split

## Status

Accepted (updated 2026-06-05 — `KollectInventory` namespaced; reserved cluster kinds)

## Context

kollect replaces a hardcoded batch collector schema with CRD-driven configuration. Operators in
this space use different tenancy and config patterns:

- **external-secrets** splits `SecretStore` (namespaced) vs `ClusterSecretStore` (cluster) with
  namespace `conditions` on the cluster variant; provider config is a discriminated union in spec.
- **Flux notification-controller** keeps `Provider` and `Alert` as **static** objects (no status
  subresource, no dedicated reconciler) while `Receiver` is reconciled.
- **Argo CD** uses `AppProject` as a tenancy boundary and `Application` as the reconciled unit.

kollect must combine generic attribute selection, resource selection, aggregation, and multi-backend
export — no single OSS project covers all of this. Templated documentation sync (Confluence, etc.)
is **explicitly rejected** ([ADR-0011](0011-doc-sync-templating.md)).

Platform users commonly terminate TLS to internal Git/GitLab with **custom CAs**; sink configuration
must support that from early phases, not as a later bolt-on.

## Decision

API group `kollect.dev/v1alpha1`. All kinds are **prefixed** (`Kollect*`) to avoid collisions.

### Static config (validated, no controller)

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectProfile` | **Namespace** (breaking; was cluster) | Reusable extraction schema: GVK + named CEL/JSONPath attributes ([ADR-0031](0031-namespaced-profiles.md)) |
| `KollectSink` | **Namespace** (breaking; was cluster) | Backend config: `type` (`git`, `gitlab`, `s3`, `gcs`, `postgres`, `kafka`) + endpoint + `secretRef` + TLS trust ([ADR-0032](0032-platform-architecture-pivot.md)) |
| `KollectScope` | **Namespace** (Phase 1 priority) | Tenancy boundary: allowed GVKs, namespaces, sinks for a team ([ADR-0016](0016-namespaced-multi-tenancy.md)) |

### Reconciled (controller + dynamic informers)

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectTarget` | Namespaced | `profileRef` + selectors + optional CEL predicate; drives collection |
| `KollectInventory` | **Namespaced** | Aggregates targets **in the same namespace**; dispatches to sinks |

### Rejected (never ship)

| Kind | Rationale |
| --- | --- |
| `KollectPublication` | Doc-sync / Confluence / in-operator templating — out of scope; use Git export + external CI ([ADR-0011](0011-doc-sync-templating.md)) |
| `KollectHub` | Hub Deployment lifecycle via CRD — use Helm **`mode: hub`** instead ([ADR-0032](0032-platform-architecture-pivot.md)) |

### Reconciled (connection test)

| Kind | Scope | Role |
| --- | --- | --- |
| `KollectConnectionTest` | Namespace | One-shot / CI probe of sink (+ optional profile); [ADR-0032](0032-platform-architecture-pivot.md) |

### Reserved (designed, not built yet)

- `KollectReceiver` — inbound webhook → trigger (Flux Receiver pattern).
- `KollectTargetSet` — generator templating many Targets (ApplicationSet pattern).
- **`KollectClusterProfile`** (cluster) — platform-shared extraction schemas ([ADR-0031](0031-namespaced-profiles.md)).
- **`KollectClusterSink`** (cluster) — platform-shared export backends ([ADR-0032](0032-platform-architecture-pivot.md)).
- **`KollectClusterInventory`** (cluster) — platform-wide rollup across namespaces. **No controller in Phase 0–1** — ADR + plan only.
- **`KollectClusterScope`** (cluster) — platform tenancy boundary when namespaced `KollectScope` is
  insufficient; addition after namespaced scope enforcement ships (Phase 3).

Short names: `kprof`, `ksink`, `kscope`, `ktgt`, `kinv` (reserved: `kcinv`, `kcscope`). `kpub` was
reserved for rejected `KollectPublication` — do not use.

All reconciled kinds support `spec.suspend` and `kollect.dev/requestedAt` manual-trigger annotation.

### Validating admission (early)

**Validating webhooks** (Phase 0/1, not post-hoc workarounds) must reject at apply time:

- Invalid or non-compilable **CEL** and **JSONPath** expressions on `KollectProfile` attributes
- Unknown `KollectSink.type` values (enum aligned with [ADR-0020](0020-error-taxonomy.md))
- Cross-field constraints already expressed as CEL `x-kubernetes-validations` in OpenAPI

Runtime collection may still surface `ErrTerminal` for GVK/API discovery failures; expression syntax
is webhook-validated first.

### `KollectSink` TLS trust (custom CA)

Git and GitLab sink specs include explicit trust material (at least one of):

- `tls.caBundle` — inline PEM (base64) for org-internal CA
- `tls.caSecretRef` — namespaced or cluster secret key containing CA bundle

Default remains **verify TLS**; `insecureSkipVerify` is opt-in only for dev and must be flagged in
status when used.

Credential material stays in `secretRef` (tokens, SSH keys) — never in spec/status.

### JSONPath filters on targets

**Deferred:** sink-side or target-side JSONPath *filters* (post-extraction row filtering) are not
Phase 1 API. Schema clarity and aggregation matter more than where filtering runs ([REQUIREMENTS.md](../REQUIREMENTS.md)).

## Consequences

### Positive

- Static Profile/Sink cuts moving parts (validated at admission, read at reconcile time).
- Namespaced Profiles and Sinks align with team ownership and `tenantMode` installs.
- Prefix naming is grep-friendly and avoids generic kind collisions in multi-operator clusters.
- Early webhooks prevent bad profiles from wedging reconcilers.
- **`KollectClusterInventory`** reserved for platform portal without blocking team-scoped MVP.

### Negative

- Namespaced inventory requires one object per namespace (or explicit federation via hub — [ADR-0022](0022-multi-cluster-sync-rfc.md)).
- Breaking scope migration for Profile and Sink requires sample and RBAC sweep.
- Webhook + CEL maintenance cost on every new attribute type rule.

## Open questions

- **RESOLVED (2026-06-05):** **`KollectSink` is namespaced**; **`KollectClusterSink`** reserved for platform-shared backends ([ADR-0032](0032-platform-architecture-pivot.md)).
- **OPEN:** Single `caBundle` field vs only `secretRef` for CA — size limits on CRD spec?
- **OPEN:** `KollectClusterInventory` selector model — all namespaces vs explicit namespace list?
