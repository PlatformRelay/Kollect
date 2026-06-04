# kollect — engineering guidelines

Binding guidelines for the kollect operator: error handling, robustness, security, and testing.
Enforced by lint, CI, and review. ADRs in `docs/adr/` capture major decisions.

## 1. Error handling

- **Wrap, don't swallow.** Use `fmt.Errorf("...: %w", err)`; never discard errors from fallible calls.
- **Typed error taxonomy** drives requeue behavior:
  - `ErrTransient` (network, throttling, conflicts) → requeue with backoff; `Synced=False`, `Reason=Progressing`.
  - `ErrTerminal` (bad config, invalid CEL/JSONPath) → no requeue; `Degraded=True` + Warning Event.
  - `ErrForbidden` (SAR/RBAC denied) → degrade scope; record `skipped:forbidden`; do not fail the whole reconcile.
- **No `panic`** in reconcilers or libraries (except `main`).
- **Context deadlines** on every external call; propagate reconcile `ctx`.
- **Structured logs** (`logr`): stable messages + keys. Never log secrets, tokens, or full payloads.

## 2. Robustness and reliability

- **Idempotent, level-based reconcile** — safe to run repeatedly.
- **Event-driven collection** — dynamic informers; long resync as a backstop only.
- **Finalizers** when external cleanup is required.
- **Optimistic concurrency** — on `Conflict`, requeue quietly.
- **Bounded resource use** — paginated `List`, scoped caches, tuned concurrency and rate limits.
- **Circuit breakers** around external sinks and doc backends.
- **Lifecycle** — leader election, graceful shutdown, `/healthz` and `/readyz`, PDB where deployed.
- **etcd size guard** — `status` holds summaries/counts/conditions only; payloads go to sinks.
- **Status discipline** — `Ready` / `Synced` / `Degraded` + `observedGeneration`.
- **Determinism** — stable ordering for sink output and golden tests.

## 3. Security

- **Least-privilege RBAC** — minimal generated roles; SAR pre-check before list/watch.
- **Tenancy** — optional `KollectScope` (future) for allowed GVKs, namespaces, sinks.
- **Secrets** — credentials only via `secretRef`; never in spec/status or logs.
- **Container hardening** — distroless non-root, read-only rootfs, dropped capabilities, seccomp.
- **Network** — restrictive `NetworkPolicy` for production egress.
- **Transport** — TLS verification required for sink and doc endpoints.
- **Input validation** — CEL in CRD OpenAPI + validating webhooks where needed.
- **Supply chain** — pinned dependencies and GitHub Action SHAs, gitleaks, govulncheck, image scanning in release pipeline.

## 4. Testing

- **Tests alongside code** — unit, envtest, golden contracts, integration (testcontainers), kind e2e.
- **Pyramid:** unit → envtest (Ginkgo) → golden OpenAPI fragments → integration → e2e.
- **Gates:** `task verify` for codegen drift; race detector on unit/envtest; coverage goals on `internal/`.
- **Mocks** — mockery on small interfaces only.

## 5. Definition of done (per change)

- Relevant tests green; lint clean; `task verify` shows no drift.
- New external calls have timeouts and backoff where appropriate.
- Status conditions and Events updated; no secrets in logs.
- ADR updated when the decision is non-trivial.
