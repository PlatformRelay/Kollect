# ADR-0701: MkDocs Material for documentation site

> Build the docs site from in-repo Markdown with MkDocs Material; deploy to GitHub Pages via CI.

**Theme:** 07 · Project & meta · **Status:** Current

## Context

Kollect needs contributor and user documentation that is easy to maintain in-repo, preview
locally, and publish to GitHub Pages without a heavy custom frontend. Docs already live as Markdown
under `docs/` (architecture, ADRs, guides).

Alternatives considered:

- **Docsify / VitePress** — lighter but less structure for multi-page ADR corpora
- **Hugo** — fast, but adds Go/Ruby toolchain friction beside the Go operator repo
- **GitHub wiki** — decoupled from PR review and versioning

## Decision

Use **[MkDocs](https://www.mkdocs.org/)** with the **[Material](https://squidfunk.github.io/mkdocs-material/)**
theme. Configuration in `mkdocs.yml` at repo root; content stays in `docs/`. CI builds with
`mkdocs build` and deploys via GitHub Actions (`docs.yaml`) to GitHub Pages.

Mermaid diagrams in existing docs are supported via `pymdownx.superfences`.

## Consequences

- Contributors preview with `pip install mkdocs-material && mkdocs serve`
- Navigation is declared in `mkdocs.yml` — new top-level docs must be added to `nav`
- Site URL defaults to `https://konih.github.io/kollect/` until a custom domain is configured
- Python is a **docs-only** dependency; the operator build remains Go-only

## Open questions

- Custom domain (e.g. `docs.kollect.dev`) — see `agent-context/QUESTIONS.md`
- Whether to pin MkDocs/Material versions in a `requirements-docs.txt` (deferred)
