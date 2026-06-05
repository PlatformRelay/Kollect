# ADR-0030: Connection test — no dedicated CR

## Status

Accepted (2026-06-05)

## Context

Product requirements call for a **first-class connection test** with clear, discoverable feedback
when sinks are misconfigured ([REQUIREMENTS.md](../REQUIREMENTS.md), [GUIDELINES.md](../../GUIDELINES.md)).

[ADR-0015](0015-static-vs-reconciled.md) originally assumed static `KollectSink` objects with **no
controller**, probing via annotations and surfacing `SinkReachable` on reconciled
`KollectTarget` / `KollectInventory`. Implementation added a **minimal `KollectSink` reconciler**
that runs connectivity checks and writes `ConnectionVerified` on the sink
(`internal/controller/kollectsink_controller.go`).

An alternative is a dedicated **`KollectConnectionTest`** CR (apply a test object, wait on status,
garbage-collect). That pattern helps composite or cross-cluster probes but adds API surface, RBAC,
webhooks, and orphan CR lifecycle.

## Decision

### Reject `KollectConnectionTest` CR (Phase 0–2)

Do **not** add a dedicated connection-test kind. Use **declarative spec** + **imperative annotation**
on `KollectSink`, plus pipeline conditions on reconciled objects.

### Sink connectivity (implemented)

| Trigger | When probe runs |
| --- | --- |
| **`spec.connectionTest: true`** | On create/update while true (**samples and CI only**) |
| **Annotation `kollect.dev/test-connection: "true"`** | One-shot re-test without editing spec |

Probe uses the same TLS trust and secret resolution as export (`caBundle` / `caSecretRef`,
`secretRef`). Supported types today: `git`, `postgres`, `kafka` (extend per sink).

**Status on `KollectSink`:**

| Condition | Meaning |
| --- | --- |
| **`ConnectionVerified`** `True` | Last probe succeeded |
| **`ConnectionVerified`** `False` | Probe failed (reason e.g. `ConnectionTestFailed`, `SecretResolveFailed`) |
| **`Degraded`** `True` | Set alongside failed probe |
| **`TLSInsecure`** `True` | `insecureSkipVerify` enabled (dev warning) |

**Operator metrics:** `kollect_sink_connection_test_total{type,result}`.

**`kubectl` example:**

```sh
kubectl wait --for=condition=ConnectionVerified kollectsink/git-inventory \
  -n default --timeout=60s
```

Re-run without spec change:

```sh
kubectl annotate kollectsink git-inventory -n kollect-system \
  kollect.dev/test-connection=true --overwrite
```

### Pipeline reachability (follow-up, not blocking)

End-to-end export health belongs on **reconciled** objects, not only the sink:

| Condition | Object | Meaning |
| --- | --- | --- |
| **`SinkReachable`** (or export-specific condition per [ADR-0020](0020-error-taxonomy.md)) | `KollectInventory`, `KollectTarget` | Last export or sink resolution succeeded; includes latency / last attempt time |

Sink-only `ConnectionVerified` proves **credentials and network to the backend**; it does not
prove the full collect → aggregate → export path.

### Minimal sink reconciler (exception to “static config”)

`KollectProfile` and `KollectScope` remain **webhook-validated static config** (no controller).

`KollectSink` has a **narrow reconciler** whose sole job is connection test status — not
collection or export. This is an intentional exception to full static-config purity, documented in
[ADR-0015](0015-static-vs-reconciled.md).

### When to revisit a dedicated CR

Only if a later phase needs **bundled probes** in one shot, for example:

- Sink + `KollectProfile` list permission + SAR in one audited object
- Hub → spoke cross-cluster probe independent of mutating `KollectSink`
- CI job that must not flip `spec.connectionTest` on shared cluster sinks

Until then, prefer annotation + optional **Job** manifest in docs over a new API kind.

## Consequences

### Positive

- No extra CRD, RBAC, or garbage-collection policy.
- Matches Flux-style config + imperative debug annotation.
- `spec.connectionTest: true` gives GitOps-friendly “always verify on change” for CI/samples.
- Aligns with existing controller and samples.

### Negative

- Annotation-based re-test is weaker for audit than a dedicated test CR (mitigate with audit logs
  on annotation patches if required).
- Composite “does my pipeline work?” still needs `SinkReachable` on Inventory/Target (follow-up).
- `KollectProfile` connectivity (can I list this GVK?) is out of scope for sink probe — separate
  feature or future bundled test if needed.

### Production default

- **`spec.connectionTest` defaults to `false`** in production sink manifests and Helm-documented
  examples. Use **`kollect.dev/test-connection: "true"`** annotation for on-demand probes to avoid
  status/etcd churn on unrelated spec edits.
- **Samples and CI** may keep `connectionTest: true` for regression coverage.

## Open questions

- **OPEN:** Exact condition name and fields on Inventory/Target (`SinkReachable` vs reuse
  [ADR-0020](0020-error-taxonomy.md) export conditions only)?
- **OPEN:** Auto-clear `kollect.dev/test-connection` annotation after successful probe to avoid
  re-probing every reconcile?
