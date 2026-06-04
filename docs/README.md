# kollect documentation

Source for user and contributor docs. Markdown files in this directory are the canonical content.

## Local preview (MkDocs)

GitHub Pages deployment is scaffolded but may not be live yet. Preview locally:

```sh
pip install mkdocs-material
mkdocs serve
```

Open http://127.0.0.1:8000/

Build static site output to `site/`:

```sh
mkdocs build
```

Site configuration: [`mkdocs.yml`](../mkdocs.yml) at the repository root.

## GitHub Pages

When enabled, the site publishes from the `gh-pages` branch or GitHub Pages artifact via
`.github/workflows/docs.yaml`. Default URL pattern:

`https://<org>.github.io/kollect/`

See [ADR-0021](adr/0021-mkdocs-github-pages.md) for tooling choices.

## Document map

| Doc | Audience |
| --- | --- |
| [QUICKSTART.md](QUICKSTART.md) | First install on kind, sample CRs |
| [DEVELOPMENT.md](DEVELOPMENT.md) | Build, test, codegen, lint |
| [ARCHITECTURE.md](ARCHITECTURE.md) | CRD model, reconciliation, phasing |
| [examples/](examples/) | Annotated YAML walkthroughs |
| [adr/](adr/) | Architecture decision records |
