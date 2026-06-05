# ADR-0403: Connection test — sink probes and the KollectConnectionTest CR

> First-class sink connectivity testing: a sink probe for quick checks plus a `KollectConnectionTest`
> CR for audited/CI/composite probes.

**Theme:** 04 · Export & sinks · **Status:** Current · **Evolution:** an initial decision to reject a
dedicated CR was reversed — the `KollectConnectionTest` CR is now part of the model; sink-only probe
mechanisms remain valid.

## Context

Product requirements call for a **first-class connection test** with clear, discoverable feedback
when sinks are misconfigured ([REQUIREMENTS.md](../REQUIREMENTS.md), [GUIDELINES.md](../../GUIDELINES.md)).

[ADR-0202](0202-static-vs-reconciled.md) originally assumed static `KollectSink` objects with **no
controller**, probing via annotations and surfacing `SinkReachable` on reconciled
`KollectTarget` / `KollectInventory`. Implementation added a **minimal `KollectSink` reconciler**
that runs connectivity checks and writes `ConnectionVerified` on the sink
(`internal/controller/kollectsink_controller.go`).

An alternative is a dedicated **`KollectConnectionTest`** CR (apply a test object, wait on status,
garbage-collect). That pattern helps composite or cross-cluster probes but adds API surface, RBAC,
webhooks, and orphan CR lifecycle.

## Decision

### ~~Reject `KollectConnectionTest` CR~~ → **Accepted in ADR-0703**

Add namespaced **`KollectConnectionTest`** for audited/CI/composite probes. Keep **declarative
spec** + **imperative annotation** on `KollectSink` for quick sink-only retests, plus pipeline
conditions on reconciled objects.

### Sink connectivity (implemented)

| Trigger | When probe runs |
| --- | --- |
| **`spec.connectionTest: true`** (default) | On create/update while enabled |
| **`spec.connectionTest: false`** | Opt out of automatic probes |
| **Annotation `kollect.dev/test-connection: "true"`** | One-shot re-test without editing spec |

Probe uses the same TLS trust and secret resolution as export (`caBundle` / `caSecretRef`,
`secretRef`). Supported types today:

| `spec.type` | Probe wired |
| --- | --- |
| `git`, `gitlab`, `postgres`, `kafka`, `s3` | ✅ |
| `nats`, `gcs` | ⬜ not wired ([kollectsink.md](../crds/kollectsink.md)) |

Extend per sink as backends mature.

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

### Pipeline reachability (implemented)

End-to-end export health belongs on **reconciled** objects, not only the sink:

| Condition | Object | Meaning |
| --- | --- | --- |
| **`SinkReachable`** | `KollectInventory`, `KollectTarget` | Sink resolution (`ConnectionVerified` / sink found) before export; **`ExportSucceeded`** / **`ExportFailed`** after inventory export attempts. `Synced` remains the primary export condition per [ADR-0602](0602-error-taxonomy.md). |

`KollectTarget` derives sink refs from **`KollectInventory` in the same namespace** (targets have no
direct `sinkRefs`). Inventory reconciler watches **`KollectSink`** status changes to requeue affected
inventories.

Sink-only `ConnectionVerified` proves **credentials and network to the backend**; it does not
prove the full collect → aggregate → export path.

### Minimal sink reconciler (exception to “static config”)

`KollectProfile` and `KollectScope` remain **webhook-validated static config** (no controller).

`KollectSink` has a **narrow reconciler** whose sole job is connection test status — not
collection or export. This is an intentional exception to full static-config purity, documented in
[ADR-0202](0202-static-vs-reconciled.md).

### `KollectConnectionTest` CR (ADR-0703)

Namespaced CR with `spec.sinkRef`, optional `spec.profileRef`, status conditions (latency,
sanitized errors). Use for CI, audit, and composite probes. Sink annotation/spec remain for ad-hoc
sink checks.

## Consequences

### Positive

- **`KollectConnectionTest` CR** gives audited, CI-friendly composite probes with status conditions.
- Sink annotation + `spec.connectionTest` remain for quick sink-only retests (Flux-style imperative debug).
- `spec.connectionTest: true` gives GitOps-friendly “always verify on change” for CI/samples only.
- Minimal sink reconciler + dedicated test CR share TLS/secret resolution with export.

### Negative

- `KollectConnectionTest` adds CRD surface, RBAC, webhooks, and garbage-collection via
  `spec.ttlSecondsAfterFinished` (default **300s**).
- Annotation-based re-test is weaker for audit than the dedicated test CR (mitigate with audit logs
  on annotation patches if required).
- Composite “does my pipeline work?” uses `SinkReachable` on Inventory/Target plus `Synced` on export.
- `KollectProfile` connectivity (can I list this GVK?) is optional on `KollectConnectionTest` — not
  covered by sink-only probes alone.

### Production default

- **`spec.connectionTest` defaults to `true`** (CRD OpenAPI default). Set **`connectionTest: false`**
  to opt out of automatic probes on every spec change.
- **`kollect.dev/test-connection: "true"`** remains for one-shot re-tests when probes are disabled.

## Resolved (2026-06-05)

- **Garbage collection:** `spec.ttlSecondsAfterFinished` — default **300**; controller deletes CR
  after probe completes and TTL elapses.
- **Re-run on spec change:** any `spec` edit bumps `metadata.generation`; when
  `status.observedGeneration != generation`, the reconciler **re-probes** (resets `completed` /
  `completedAt` on the new run). TTL clock restarts after the new probe finishes.
- **`SinkReachable`** on `KollectInventory` and `KollectTarget`; export outcomes set
  `ExportSucceeded` / `ExportFailed` reasons; `Synced` unchanged for export progress.
- **Annotation auto-clear:** after a successful probe triggered only by
  `kollect.dev/test-connection`, the reconciler removes the annotation (kept when
  `spec.connectionTest: true`).
