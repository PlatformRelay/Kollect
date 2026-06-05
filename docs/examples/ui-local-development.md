# UI local development (mock vs live)

!!! tip "Prerequisites"
    Mock mode needs Node.js and `pnpm`/`npm` only. Live mode requires a running operator with the
    Read API enabled — see [Kind local lab](kind-local-lab.md).

The kollect-ui SPA can run **without a cluster** using MSW mocks, or against a live Read API when
the operator is running.

## Mock mode (default)

```bash
task ui-dev
```

Open http://localhost:5173 — MSW intercepts `/v1alpha1/*` with contract-faithful fixtures (team-a
inventory, degraded targets, mixed export status, 120-row pagination catalog).

Append `?debug=true` to show the connection banner. See [ADR-0412](../adr/0412-mock-read-api-for-ui-development.md).

## Live Read API

Port-forward the operator inventory HTTP server (default `:8082`), then:

```bash
cd ui
VITE_MOCK_API=false VITE_READ_API_URL=http://127.0.0.1:8082 npm run dev
```

Requires a populated cluster with `KollectInventory` / `KollectTarget` CRs — see
[Kind local lab](kind-local-lab.md).

## Prism (real HTTP, e2e)

```bash
task ui-mock-prism
VITE_MOCK_API=false VITE_READ_API_URL=http://127.0.0.1:4010 npm run dev
```

More detail: [`ui/README.md`](../../ui/README.md) · [ADR-0412](../adr/0412-mock-read-api-for-ui-development.md).
