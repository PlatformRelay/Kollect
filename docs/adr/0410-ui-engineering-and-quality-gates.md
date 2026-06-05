# ADR-0410: UI engineering and quality gates

> Monorepo `ui/` with React 19 + Vite 6, a frontend test pyramid mirroring [ADR-0706](0706-testing-merge-gate-architecture.md),
> CI bundle budget enforcement, and Kollect branding — so the read-only console ships with the same
> quality bar as the operator.

**Theme:** 07 · Project & meta (frontend) · **Status:** Current (accepted 2026-06-05)

## Context

[ADR-0408](0408-read-api-ui-architecture.md) accepts a read-only SPA fed by the Read API. Maintainer
locked decisions (OQ-5–8, OQ-12) specify monorepo layout, Tailwind v4, bundle budget CI failure, nightly
visual regression, and MVP scope (observability only — no Target create forms, no frontend RBAC shell).

Backend quality gates live in [ADR-0706](0706-testing-merge-gate-architecture.md). Frontend gates must be
**explicit** so UI PRs do not rely on ad-hoc checks.

## Decision

### 1. Monorepo layout (OQ-5)

The SPA lives in the **kollect monorepo**, not a separate repository:

```text
ui/                      # React SPA
  src/
  public/
  e2e/
  openapi/               # symlink or copy from repo root openapi/v1alpha1/
  package.json
  pnpm-lock.yaml         # committed
  vite.config.ts
charts/kollect-ui/       # static Deployment subchart ([ADR-0409](0409-kollect-ui-deployment.md))
hack/ci/ui-verify.sh     # contract drift + bundle budget
.github/workflows/ui-ci.yaml
```

**Taskfile targets:** `task ui-dev`, `task build-ui`, `task ui-ci`, `task ui-mock-prism`, `task ui-visual`.

**Mock stack (Phase 1):** MSW intercepts `/v1alpha1/*` in dev and Vitest by default
(`VITE_MOCK_API=true`); optional Prism on port 4010 for real HTTP e2e — see
[ADR-0412](0412-mock-read-api-for-ui-development.md).

OpenAPI changes require regenerated TS types — `task verify` (or `hack/verify-ui-contract.sh`) fails
on drift, matching backend codegen discipline.

### 2. Stack (locked)

| Layer | Choice | Rationale |
| --- | --- | --- |
| **Framework** | React 19 | Prior art (Argo CD UI); [ADR-0408](0408-read-api-ui-architecture.md) assumption |
| **Build** | Vite 6 | Fast dev; content-hashed assets |
| **Language** | TypeScript | OpenAPI codegen; typed routes |
| **Routing** | `@tanstack/react-router` | Code-splitting, typed loaders |
| **Server state** | `@tanstack/react-query` | Cache, retry, SSE invalidation |
| **Client UI state** | **Zustand** (`ui/src/store/`) | Filter prefs, column visibility, drawer/selection — unit-tested slices; **not** XState |
| **Tables** | `@tanstack/react-table` + `@tanstack/react-virtual` | 10k-row virtualization ([NFR-PERF-1](../REQUIREMENTS.md)) |
| **Styling** | **Tailwind v4** (OQ-6) | Ops-density layouts; repo-wide |
| **Primitives** | Radix UI | a11y-friendly dialogs, menus |
| **Icons** | Lucide | Tree-shakeable |
| **Forms (deferred)** | `react-hook-form` + `zod` | Target create forms post-MVP |
| **API client** | `openapi-typescript` or Orval from `openapi/v1alpha1/inventory.yaml` | Contract-first |

**Non-goals (v0.2):** i18n, Service Worker offline, heavy charting libraries, mobile-native apps,
XState/state-machine client layer (deferred — Zustand is the default global store).

**Client state split (v0.2):** TanStack Query owns server cache; Zustand owns UI prefs and ephemeral
selection (`connection`, `inventory` column/filter prefs, `selection` drawer state). URL search params
are primary for inventory filters ([ADR-0408](0408-read-api-ui-architecture.md)).

### 3. Testing pyramid (mirrors ADR-0706 spirit)

```
                         ┌─────────────────────────────────────┐
                    L4   │  E2E — Playwright (kind + Helm)      │
                         │  operator + kollect-ui               │
                         └──────────────────┬──────────────────┘
                                            │
                         ┌──────────────────▼──────────────────┐
                    L3   │  Contract — MSW + OpenAPI diff       │
                         └──────────────────┬──────────────────┘
                                            │
         ┌──────────────────────────────────▼──────────────────────────────────┐
    L2   │  Component — RTL + user-event; badges, virtualized table           │
         └──────────────────────────────────┬──────────────────────────────────┘
                                            │
         ┌──────────────────────────────────▼──────────────────────────────────┐
    L1   │  Unit — Vitest: parsers, sort keys, formatters, route loaders       │
         └──────────────────────────────────┬──────────────────────────────────┘
                                            │
         ┌──────────────────────────────────▼──────────────────────────────────┐
    L0   │  Static — ESLint, tsc, stylelint, knip                             │
         └──────────────────────────────────┬──────────────────────────────────┘
                                            │
         ┌──────────────────────────────────▼──────────────────────────────────┐
   L4+   │  Visual regression — Percy/Chromatic (nightly, OQ-8)                 │
         │  a11y — @axe-core/playwright (PR gate)                               │
         └─────────────────────────────────────────────────────────────────────┘
```

| Tier | Tooling | CI |
| --- | --- | --- |
| L0 | `pnpm lint`, `pnpm typecheck` | **`ui-ci.yaml` PR gate** |
| L1 | `pnpm test:unit` (Vitest) | PR gate |
| L2 | `pnpm test:component` (RTL) | PR gate |
| L3 | `pnpm test:contract` vs `openapi/v1alpha1/inventory.yaml` | PR gate |
| L4 | `pnpm test:e2e` (Playwright + kind + Helm) | **Nightly** / `workflow_dispatch` |
| Visual | Percy or Chromatic — Overview, Inventory, Target detail, Sink list, onboarding | **Nightly required** (OQ-8) |
| a11y | `@axe-core/playwright` on Overview + Inventory | **PR gate** |

**Coverage floors:** 80% lines on `ui/src/lib/**` and formatters; 60% overall.

E2E MVP injects SA token for Read API auth; OIDC flows added when oauth2-proxy lands
([ADR-0409](0409-kollect-ui-deployment.md)).

### 4. Bundle budget (OQ-7)

| Metric | Target (v0.2) |
| --- | --- |
| Initial JS (shell route) | ≤ **200 KB** gzip |
| Lazy route chunk | ≤ **80 KB** gzip each |
| Time to interactive (kind, warm) | ≤ **2.5 s** throttled Fast 3G |

`pnpm build` **fails CI** when the shell route exceeds 200 KB gzip (`vite-plugin-bundle-stats` or
equivalent). Lazy routes for Inventory and Exports via `React.lazy`.

### 5. CI workflow `ui-ci.yaml`

Proposed jobs (implementation tracked in ROADMAP):

| Job | Trigger | Blocks merge |
| --- | --- | --- |
| `ui` | PR + push `main` | Yes — lint, typecheck, unit, component, contract, build (bundle budget), a11y |
| `ui-nightly` | cron + manual | No on PR — e2e + visual regression |

Supply chain: `pnpm install --frozen-lockfile`; `pnpm audit` + OSV scanner block high/critical;
Renovate groups `ui/` deps separately.

### 6. Branding integration

Canonical public assets: `docs/assets/` (logo SVG, favicons, symbol variants). UI copies or imports
from `docs/assets/branding/` at build time.

| Token | Hex | Role |
| --- | --- | --- |
| Kollekt Blue | `#326CE5` | Primary actions, links |
| Deep Navy | `#081A4B` | Nav bar (dark), wordmark on light |
| Inventory Teal | `#18B6A3` | Export/sync success, healthy chips |
| Sky Accent | `#7FB3FF` | Info states, decorative |
| Stone / Graphite | `#E5E7EB` / `#1F2937` | Borders, body text |

**Typography:** Inter (fallbacks: IBM Plex Sans, Geist). Ops-console density.

**Naming:** Product chrome uses **Kollect** (CRDs, page titles). Optional wordmark lockup may render
**Kollekt** in SVG paths — do not rename CRDs or API paths.

**Themes:** `prefers-color-scheme`; light `#FFFFFF` + Deep Navy; dark `#081A4B` + white wordmark.

### 7. MVP scope and deferred features

| In v0.2 MVP | Deferred |
| --- | --- |
| Read-only Overview, Inventory, Target/Sink lists + detail drawers | Target **create/apply** forms (OQ-12) |
| Onboarding = copy-YAML + docs links | Frontend **RBAC-aware nav masking** |
| Export health per sink on inventory | **Cross-cluster portal auth** (OQ-9) |
| Virtualized catalog + filters (when Read API ships) | Login shell / OIDC UI |
| Kollect branding in nav and empty states | i18n / RTL |
| | Simplified "catalog" mode for non-K8s stakeholders (v0.3) |
| | Optional BFF (Phase 3) |

### 8. Security checklist (pre-release)

- No kube tokens in `localStorage` (production)
- CSP enforced on UI routes ([ADR-0409](0409-kollect-ui-deployment.md))
- React default escaping; no raw `condition.message` as HTML
- `dangerouslySetInnerHTML` forbidden except sanitizer allowlist module
- Dependency audit gate in `ui-ci`

## Consequences

### Positive

- Frontend quality bar is documented and enforceable before `ui/` scaffold lands.
- OpenAPI-driven contract tests prevent Read API / UI drift.
- Bundle budget keeps the ops console lightweight on slow networks.

### Negative

- Nightly visual + e2e infra cost (Percy/Chromatic license, kind runners).
- Monorepo increases repo size and CI matrix surface.
- Hybrid K8s API for CRD status requires separate client tests not covered by OpenAPI contract alone.

## See also

- [ADR-0408: Read API and UI architecture](0408-read-api-ui-architecture.md)
- [ADR-0409: Kollect UI deployment](0409-kollect-ui-deployment.md)
- [ADR-0411: Read API extensions for UI](0411-read-api-extensions-for-ui.md)
- [ADR-0412: Mock Read API for UI development](0412-mock-read-api-for-ui-development.md)
- [ADR-0706: Testing and merge-gate architecture](0706-testing-merge-gate-architecture.md)
- [ROADMAP.md](../ROADMAP.md)
