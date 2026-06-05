# ADR-0412: Mock Read API for UI development

> MSW intercepts the Read API in browser dev and Vitest by default; optional Prism serves real HTTP
> for e2e — so frontend work does not require a running operator or port-forward.

**Theme:** 04 · Export & sinks (read side) · **Status:** Current (accepted 2026-06-05)

## Context

The Kollect UI is a read-model consumer ([ADR-0408](0408-read-api-ui-architecture.md)). Local
development previously required a running operator, Read API port-forward, and populated cluster
inventory before any screen beyond empty states could be exercised.

[ADR-0410](0410-ui-engineering-and-quality-gates.md) specifies an MSW-based contract tier (L3) in
the frontend test pyramid. [ADR-0411](0411-read-api-extensions-for-ui.md) extended the OpenAPI
contract with pagination, filters, export status, and status projection routes — all of which the
UI must consume without a live backend during scaffold hardening.

Phase 1 of the mock Read API strategy (maintainer proposal, 2026-06-05) closes the gap between
scaffold and operator availability.

## Decision

### 1. MSW as the default dev and test mock layer

| Mode | Trigger | Behaviour |
| --- | --- | --- |
| **Mock (default)** | `task ui-dev` sets `VITE_MOCK_API=true` | MSW Service Worker intercepts same-origin `/v1alpha1/*` |
| **Live Read API** | `VITE_MOCK_API=false` + `VITE_READ_API_URL` | Vite proxy forwards to operator Read API |
| **Vitest** | `setupServer` from `msw/node` in test setup | Same handlers as browser — no duplicate stubs |

`VITE_ENABLE_MSW=true` remains supported as a backward-compatible alias for `VITE_MOCK_API=true`
until docs and CI env vars migrate.

### 2. Handler coverage (OpenAPI paths)

Hand-maintained fixtures and handlers under `ui/src/mocks/` implement all Read API paths from
`openapi/v1alpha1/inventory.yaml`:

| Path | Mock behaviour |
| --- | --- |
| `GET /v1alpha1/inventory` | Filter + paginate fixture catalog; `exportStatus[]` when `inventory` query set |
| `GET /v1alpha1/inventory/{namespace}/{name}` | Scoped inventory snapshot |
| `GET /v1alpha1/inventory/watch` | SSE stream; interval from `VITE_MOCK_WATCH_INTERVAL_MS` (default 5s); `?mockWatch=burst` for rapid events |
| `GET /v1alpha1/status/targets` | Degraded + healthy Target conditions; namespace filter |
| `GET /v1alpha1/status/inventories` | Inventory CR status projection; namespace filter |

Fixtures include multi-tenant `team-a` / `team-b` data, mixed `exportStatus` (ok / degraded /
unknown), and a generated 120-row catalog for pagination tests.

### 3. Optional Prism for real HTTP e2e

`@stoplight/prism-cli` is a pinned devDependency. `task ui-mock-prism` serves
`openapi/v1alpha1/inventory.yaml` on port **4010** for Playwright jobs that need real HTTP without
MSW Service Worker interference.

Prism does **not** replace MSW for SSE watch fidelity — use MSW (or a future Go mock server) for
watch-specific e2e; Prism covers list/get happy paths.

### 4. Connection banner (dev only)

`ui/src/store/connection.ts` (Zustand) hydrates `mockApiEnabled` and `readApiBaseUrl` from Vite
env. When `?debug=true` in dev, the shell shows **Mock data** vs **Live Read API** — no duplicate
server cache; TanStack Query remains the server-state layer ([ADR-0410](0410-ui-engineering-and-quality-gates.md)).

### 5. Phase 2 (v0.2 — shipped)

| Deliverable | Status |
| --- | --- |
| `task ui-mock-sync` | Regenerates `ui/src/mocks/generated/openapi-manifest.json` from OpenAPI sha256 |
| CI drift gate | `hack/verify-ui-mock.sh` invoked from `hack/ci/ui-verify.sh` |
| Hand fixtures | Remain authoritative for degraded/403 overlays; manifest gates OpenAPI hash only |

`pnpm test:contract` schema validation remains a follow-up; backend `test/openapi/` is still the
authoritative Go contract gate.

## Consequences

### Positive

- `task ui-dev` works with zero cluster — filters, pagination, export health, and degraded Target UX
  are locally testable.
- Vitest and browser share one handler set — component tests match dev behaviour.
- Prism gives a real HTTP sidecar path for nightly e2e without kind + operator bootstrap.

### Negative

- Hand-maintained fixtures can drift from OpenAPI until Phase 2 codegen + drift gate ship.
- SSE watch semantics in MSW differ from production flush timing — acceptable for UI invalidation
  tests, not for backend load testing.
- Prism SSE support is limited; watch e2e stays on MSW or full stack.

## See also

- [ADR-0408: Read API and UI architecture](0408-read-api-ui-architecture.md)
- [ADR-0410: UI engineering and quality gates](0410-ui-engineering-and-quality-gates.md)
- [ADR-0411: Read API extensions for UI](0411-read-api-extensions-for-ui.md)
- `ui/README.md` — env vars, `task ui-dev`, `task ui-mock-prism`
- `openapi/v1alpha1/inventory.yaml` · `ui/src/mocks/`
