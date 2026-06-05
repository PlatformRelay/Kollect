# kollect-ui

Read-only React SPA for the Kollect inventory Read API (ADR-0408).

## Stack

- React 19 + Vite 6 + TypeScript
- Tailwind CSS v4 (brand tokens: `#326CE5`, `#081A4B`, `#18B6A3`)
- TanStack Router + TanStack Query
- MSW for local development mocks

## Quick start

```bash
cd ui
npm ci
VITE_ENABLE_MSW=true npm run dev
```

Open http://localhost:5173 — MSW serves mock Read API responses when `VITE_ENABLE_MSW=true`.

Point at a live operator Read API (port-forward or in-cluster):

```bash
VITE_READ_API_URL=http://127.0.0.1:8082 npm run dev
```

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Vite dev server |
| `npm run build` | Production bundle → `dist/` |
| `npm test` | Vitest unit tests |
| `npm run test:a11y` | a11y gate stub (Playwright axe in nightly) |
| `npm run lint` | ESLint |
| `npm run typecheck` | TypeScript |

## OpenAPI contract

`openapi/inventory.yaml` is copied from `../openapi/v1alpha1/inventory.yaml` on `npm ci` / `postinstall`.

## Deployment

Build the container image from the repo root:

```bash
docker build -f ui/Dockerfile -t ghcr.io/konih/kollect-ui:dev ui/
```

Helm subchart: `charts/kollect-ui/` (enable with `ui.enabled: true` on the parent chart).

## Routes (MVP placeholders)

- `/` — Overview
- `/inventory` — collected items table
- `/targets` — KollectTarget status
- `/sinks` — placeholder

Auth is **not** implemented in the SPA (MVP); production uses oauth2-proxy at ingress post-MVP.
