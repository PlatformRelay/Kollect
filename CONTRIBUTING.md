# Contributing to Kollect

Thank you for helping improve Kollect.

## Acceptable contributions

Pull requests must meet the standards below before they can merge. Reviewers use these documents
as the definition of acceptable work.

| Document | What it covers |
| --- | --- |
| [Coding standards](docs/development/coding-standards.md) | Go lint/logging, testing floor, security controls, commits, architecture boundaries, CI gates |
| [Testing strategy](docs/development/testing.md) | L0–L5 pyramid, coverage targets, integration and e2e expectations |
| [GUIDELINES.md](GUIDELINES.md) | Error taxonomy, robustness, security baselines, definition of done |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting, threat model, supply-chain expectations |
| [Architecture decision records](docs/adr/) | Locked design decisions — update or add ADRs for non-trivial changes |

**Merge policy:** use **Rebase and merge** on pull requests (see
[Changelog and releases](#changelog-and-releases)). `main` requires green **`preflight`** and
**`test`** CI checks and linear history.

**Local preflight** (before opening a PR): `task lint` · `task coverage` · `task verify` ·
`task scrub` · `gitleaks protect --staged --no-banner`.

## Commit messages

We follow **[Conventional Commits](https://www.conventionalcommits.org/)** per
[qoomon's cheatsheet](https://gist.github.com/qoomon/5dfcdf8eec66a051ecd85625518cfd13), with an
optional **[gitmoji](https://gitmoji.dev/) shortcode prefix** (e.g. `:sparkles:`, not Unicode
emoji). Use shortcodes when they help scan history; omit them when they add noise.

Format:

```text
:gitmoji: <type>(<optional scope>): <description>

<optional body>

<optional footer>
```

**Types** — pick the first matching category (do not misuse `feat` for pure docs/ci):

| Type | When to use |
| --- | --- |
| `feat` | Add, adjust, or remove a user-facing feature — CRDs, webhooks, operator behavior |
| `fix` | Fix a bug in API/UI behavior |
| `refactor` | Restructure code without changing API or UI behavior |
| `perf` | Performance improvement |
| `style` | Formatting, whitespace, lint-only — no behavior change |
| `test` | Add or correct tests |
| `docs` | Documentation only — CR references, README, ADRs |
| `build` | Build tools, dependencies, Dockerfile, version bumps |
| `ci` | CI/CD pipelines, GitHub Actions, deployment scripts |
| `chore` | Maintenance — `.gitignore`, scaffolding, non-user-facing tooling |

**Scopes** — optional, lowercase, ≤ 20 chars: `api`, `controller`, `hub`, `sink`, `collect`,
`helm`, `webhook`, `validation`, `transport`, `docs`, `ci`, `build`. Do not use issue IDs as
scopes (reference issues in the footer or description instead).

**Description** — imperative present tense ("add" not "added"); lowercase first letter; no
trailing period; ≤ ~72 chars on the subject line (after gitmoji + type/scope).

**Breaking changes** — pre-v0.x default is **no** breaking marker; CRD/schema pivots use plain
`feat(api):`. Use `feat(scope)!:` / `fix(scope)!:` or a `BREAKING CHANGE:` footer only when a
**tagged release already exists** and adopters must migrate. If `!` is used, footer
`BREAKING CHANGE: <migration note>` is **required**. Do not mark breaking for pre-beta CRD churn,
internal refactors, or dev-only flag removal.

**Examples (good):**

```text
:sparkles: feat(api): make KollectSink namespaced
:bug: fix(hub): reject unlisted cluster when allowlist is set
:recycle: refactor(controller): extract scope check helper
:page_facing_up: docs: expand KollectProfile CR reference
:construction_worker: ci: fix e2e-nightly upload-artifact pin
```

**Examples (avoid):**

```text
Feat(api): Added sink.
feat(api)!: change sink scope
feat: misc fixes
```

Capitalized or past-tense subjects, trailing periods, `!` without a `BREAKING CHANGE:` footer,
and vague subjects.

## Changelog and releases

[`CHANGELOG.md`](CHANGELOG.md) is generated from git history with
[git-cliff](https://git-cliff.org/) (`cliff.toml`); gitmoji tokens are stripped from changelog
headings automatically.

| Task | Purpose |
| --- | --- |
| `task changelog` | Preview the **Unreleased** section |
| `task changelog:write` | Regenerate full `CHANGELOG.md` |
| `task changelog:release` | Print notes for the latest tag |
| `task changelog:verify` | Fail if `CHANGELOG.md` is stale (CI/preflight) |
| `task release-dry-run` | Build `dist/` assets without pushing |

Only `feat`, `fix`, `perf`, `refactor`, and breaking commits appear in the user-facing changelog;
`docs`, `test`, `chore`, `ci`, `build`, and `style` are skipped (`cliff.toml`).

**Maintainer release flow** — full runbook: [docs/RELEASE.md](docs/RELEASE.md). Summary:

1. Merge work on `main` with conventional commits.

**GitHub merge policy:** use **Rebase and merge** on pull requests (merge commits are disabled). **Squash and merge** is allowed when a single commit is clearer (e.g. Dependabot); keep the squash title conventional. `main` is protected (linear history, required CI checks `preflight` and `test`, no force-push). Admins are not included in those restrictions, so the maintainer can still push directly to `main` when needed.
2. `task changelog` — sanity-check grouping.
3. Bump `charts/kollect/Chart.yaml` `version` and `appVersion`.
4. `task changelog:write` — commit `CHANGELOG.md`.
5. `git tag vX.Y.Z && git push origin vX.Y.Z` — CI publishes image and GitHub Release.

Tagged releases (`v*.*.*`) trigger [`.github/workflows/release.yaml`](.github/workflows/release.yaml):
multi-arch images to `ghcr.io/konih/kollect` and `ghcr.io/konih/kollect-ui`, Trivy scan, cosign
signing, SPDX SBOMs, Helm chart (OCI), and GitHub Release assets (`install.yaml`, `install-crds.yaml`,
chart tarball, checksums).

## Test coverage

Kollect follows a six-tier test pyramid (L0–L5) documented in
[ADR-0706](docs/adr/0706-testing-merge-gate-architecture.md) and the
[Testing strategy](docs/development/testing.md) page.

CI runs `task coverage`, which writes `coverage.out` for `./internal/...` and enforces a
**65%** floor on statement coverage today (`COVERAGE_MIN`, see `hack/coverage.sh`). The
**pre-v0.1.0 target is 80%** — ratchet `COVERAGE_MIN` when measured coverage is sustained
above that level. The [Codecov](https://codecov.io/gh/konih/kollect) project target is **70%**
(`codecov.yml`). Integration-tagged tests (`-tags=integration`) and e2e packages are excluded
from the default profile.

**Integration CI** (`task test-integration`) runs testcontainers-backed sinks and transports,
including **S3** (MinIO) and **GCS** (S3-compatible) under `internal/sink/s3/` and
`internal/sink/gcs/`. The **e2e-nightly** and manual **E2E (optional)** workflows re-run those
object-store tests after kind smoke.

| Task | Purpose |
| --- | --- |
| `task coverage` | Unit/envtest + `coverage.out` + floor check |
| `task coverage:report` | `go tool cover -func` summary |
| `task coverage:html` | Write `coverage.html` (open in a browser) |
| `task test-integration` | Postgres, Kafka, Git, S3, GCS, Redis, NATS (Docker required) |
| `task test:e2e` | Kind smoke (`hack/kind/e2e/` — matches nightly workflow) |

Coverage is published to [Codecov](https://codecov.io/gh/konih/kollect) from the CI `test` job
after tests pass, using `codecov/codecov-action` with the repository `CODECOV_TOKEN` secret
(`use_oidc: false`). Regressions below the `COVERAGE_MIN` floor fail CI; ratchet the floor toward the **80%**
project target when coverage has grown sustainably (see ADR-0706).

## Pull request process

1. Fork or branch from `main`.
2. Run locally:
   - `task lint` (golangci-lint + **go-arch-lint** import boundaries)
   - `task arch-lint` (optional — arch fitness only; also runs inside `task lint`)
   - `task coverage` (or `task test` for a quick pass without the floor)
   - `task verify`
   - `task scrub` (after staging) and `gitleaks protect --staged --no-banner` before commit
3. Keep changes focused; update ADRs in `docs/adr/` when making architectural decisions.
4. Ensure CI is green (`preflight` + `CI` workflows).
5. Request review; address feedback with additional commits (avoid force-push to `main`).

## Lint and architecture fitness

`task lint` runs **golangci-lint v2** (including `depguard` / `gomodguard` dependency policy) and
**go-arch-lint** against [`.go-arch-lint.yml`](.go-arch-lint.yml). Package boundaries are documented
in [ARCHITECTURE.md](docs/ARCHITECTURE.md#package-boundaries).

**SonarCloud** (maintainability / debt trends) is optional in CI until `SONAR_TOKEN` is configured;
maintainers can run `task sonar:local` with `SONARCLOUD_TOKEN` in `.envrc` — see
[tooling-setup.md](docs/development/tooling-setup.md#sonarcloud-maintainer-setup).

## Code guidelines

See [Coding standards](docs/development/coding-standards.md) for lint, testing, security, and CI
gates. [GUIDELINES.md](GUIDELINES.md) covers error handling, robustness, and definition of done.

## Documentation

User-facing docs live under `docs/` and publish via MkDocs Material ([ADR-0701](docs/adr/0701-mkdocs-github-pages.md)).
When you change behavior or add features, update the matching pages and `mkdocs.yml` nav.

### Admonitions

Use MkDocs Material admonitions (`!!! type "Title"`) on procedural pages (Getting Started, Operator
Manual, examples):

| Type | Use for |
| --- | --- |
| `tip` | Shortcuts, optional paths, assumptions at page top |
| `note` | Context, why a flag exists, version caveats |
| `warning` | Data loss, security, pre-beta API, destructive steps |
| `info` | Maturity, scope, links to ADR decisions |

**Placement:** one `tip` or `note` at the top of Getting Started / Operator Manual pages (link to
[Understand the basics](docs/UNDERSTAND-THE-BASICS.md)); `warning` before mutating shell blocks when
security-sensitive; `note` after complex YAML pointing to CR field reference. Pre-beta features get
`warning` with a [ROADMAP](docs/ROADMAP.md) link. Aim for 2–4 admonitions per page unless
troubleshooting tables need more.

**Tabs** (optional, for install variants):

```markdown
=== "Helm"
    ```sh
    helm install kollect charts/kollect ...
    ```

=== "kind"
    ```sh
    task kind-dev-up
    ```
```

### Doc PR checklist

- New feature → update [ROADMAP](docs/ROADMAP.md) status
- New CR field → `docs/crds/*.md` and [CR-REFERENCE](docs/CR-REFERENCE.md)
- New label/annotation → [ANNOTATIONS-LABELS](docs/ANNOTATIONS-LABELS.md) and relevant CR page
- Add or move pages in `mkdocs.yml` nav
- At least one admonition on new procedural pages
- Run `task lint:markdown` and `mkdocs build` before opening a PR

Glossary CRD section: regenerate with `python3 hack/gen-glossary.py` after schema description changes.

## License

By contributing, you agree that your contributions are licensed under the project MIT
license.
