# kollect-ui

Read-only React SPA for the Kollect inventory Read API ([ADR-0408](../docs/adr/0408-read-api-ui-architecture.md)).

## Stack

- React 19 + Vite 6 + TypeScript
- Tailwind CSS v4 (brand tokens: `#326CE5`, `#081A4B`, `#18B6A3`)
- TanStack Router + TanStack Query
- MSW for mock Read API in dev and Vitest ([ADR-0412](../docs/adr/0412-mock-read-api-for-ui-development.md))

## Quick start (mock — default)

From the repo root:

```bash
task ui-dev
```

Or from `ui/`:

```bash
npm ci
VITE_MOCK_API=true npm run dev
```

Open http://localhost:5173 — MSW serves contract-faithful mock responses with no cluster required.

Append `?debug=true` to show the connection banner (**Mock data** vs **Live Read API**).

## Live Read API

Point at a running operator Read API (port-forward or in-cluster):

```bash
VITE_MOCK_API=false VITE_READ_API_URL=http://127.0.0.1:8082 npm run dev
```

Vite proxies `/v1alpha1/*` to `VITE_READ_API_URL` when MSW is off.

## Prism (real HTTP, optional)

For Playwright or manual testing against a standalone HTTP mock:

```bash
task ui-mock-prism   # serves openapi/v1alpha1/inventory.yaml on :4010
VITE_MOCK_API=false VITE_READ_API_URL=http://127.0.0.1:4010 npm run dev
```

SSE watch fidelity is limited in Prism — use MSW (`VITE_MOCK_API=true`) for watch UX development.

## Environment variables

| Variable | Default (`task ui-dev`) | Purpose |
| --- | --- | --- |
| `VITE_MOCK_API` | `true` | Enable MSW Service Worker |
| `VITE_ENABLE_MSW` | — | Deprecated alias for `VITE_MOCK_API=true` |
| `VITE_READ_API_URL` | `http://127.0.0.1:8082` | Live Read API / Vite proxy target when mock off |
| `VITE_MOCK_WATCH_INTERVAL_MS` | `5000` | SSE mock event interval |
| `E2E_READ_API_URL` | `http://127.0.0.1:4010` | Playwright → Prism or mock server (nightly) |

## Scripts

| Script | Purpose |
| --- | --- |
| `npm run dev` | Vite dev server |
| `npm run build` | Production bundle → `dist/` |
| `npm test` | Vitest unit + MSW handler tests |
| `npm run test:a11y` | a11y gate stub (Playwright axe in nightly) |
| `npm run lint` | ESLint |
| `npm run typecheck` | TypeScript |

## Mock fixtures

Hand-maintained under `src/mocks/fixtures/`:

- `inventory-team-a.json` — sample catalog rows
- `export-status-mixed.json` — ok / degraded / unknown sinks
- `targets-degraded.json` — healthy and Degraded Target conditions
- Programmatic 120-row catalog for pagination tests

Handlers: `src/mocks/handlers/` (inventory, status, SSE watch).

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
