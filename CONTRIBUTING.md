# Contributing to Kollect

Thank you for helping improve Kollect.

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md) and is governed per
[GOVERNANCE.md](GOVERNANCE.md).

## Standards map

Pull requests must meet the linked standards before merge. Each document owns one concern — do not
duplicate prose across them.

| Document | Owns |
| --- | --- |
| [REQUIREMENTS.md](docs/REQUIREMENTS.md) | Product *what* — functional requirements and NFR targets |
| [GUIDELINES.md](GUIDELINES.md) | Operator *how well* — error taxonomy, robustness, security model, perf, definition of done |
| [Coding standards](docs/development/coding-standards.md) | Go *how* — lint, formatting, modules, race detector, CI gates |
| [Testing strategy](docs/development/testing.md) | Test pyramid (L0–L5), coverage floors, integration/e2e tiers |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, PR workflow, changelog, doc PR checklist |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Community behavior standards (Contributor Covenant v2.1) |
| [GOVERNANCE.md](GOVERNANCE.md) | Roles, decision making, continuity, security contact |
| [SECURITY.md](SECURITY.md) | Vulnerability reporting and threat model |
| [Assurance case](docs/ASSURANCE-CASE.md) | Security claims, trust boundaries, countermeasures |
| [Security review](docs/SECURITY-REVIEW.md) | Dated self-review findings and residual risks |
| [SCA remediation policy](docs/security/sca-remediation-policy.md) | Dependency CVE and license remediation thresholds (OSPS-VM-05.01) |
| [Architecture decision records](docs/adr/) | Locked design decisions — update or add ADRs for non-trivial changes |
| [tooling-setup.md](docs/development/tooling-setup.md) | Maintainer setup for arch-lint, depguard, SonarCloud |

**Merge policy:** use **Rebase and merge** on pull requests (see
[Changelog and releases](#changelog-and-releases)). `main` requires green **`preflight`** and
**`test`** CI checks and linear history.

**Local preflight** (before opening a PR): `task lint` · `task coverage` · `task coverage:race`
(recommended) · `task verify` · `task scrub` · `gitleaks protect --staged --no-banner`. Technical gate
details: [coding-standards.md § Pull request and CI gates](docs/development/coding-standards.md#pull-request-and-ci-gates).

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

## Developer Certificate of Origin (DCO)

By contributing, you certify the [Developer Certificate of Origin (DCO)](DCO) (version 1.1).

Include a `Signed-off-by` line in every commit message when you are able:

```text
:sparkles: feat(sink): add example validation

Signed-off-by: Your Name <your.email@example.com>
```

Git can append this automatically: `git commit -s`. The DCO is a statement of license on your
contribution; it complements the MIT license in [LICENSE](LICENSE). A DCO bot is not required —
maintainers may ask you to amend commits if sign-off is missing on substantive contributions.

## Reporting bugs

**Open a [GitHub Issue](https://github.com/konih/kollect/issues/new)** for bugs, regressions, and
feature requests. Do **not** use issues for security vulnerabilities — email
**konrad.heimel@gmail.com** per [SECURITY.md](SECURITY.md).

Include: Kollect version or commit, Kubernetes version, minimal repro YAML or steps, expected vs
actual behavior, and relevant operator logs (redact secrets).

## Good first contributions

Looking for a small, review-friendly change? Try one of these:

| Area | Ideas |
| --- | --- |
| **Docs** | Fix typos, clarify [QUICKSTART](docs/QUICKSTART.md), improve admonitions on procedural pages |
| **ADRs** | Typo or link fixes in `docs/adr/` |
| **Golden tests** | Add or extend extractor golden fixtures under `test/` |
| **Markdown lint** | Run `task lint:markdown` and fix warnings |
| **Sample YAML** | Improve `config/samples/` or `docs/examples/` manifests |
| **UI** | Small accessibility or copy fixes in `ui/src/` (keep PRs focused) |

Search issues labeled
[`good first issue`](https://github.com/konih/kollect/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
If none exist, pick a row above and mention it in your PR description.

## Code review

All pull requests need **green CI** and **maintainer approval** before merge to `main`.

### Required checks

| Check | Task / workflow |
| --- | --- |
| Lint and format | `task lint`, `task format:check` (`CI`) |
| Tests and coverage floor | `task coverage` (`CI`) |
| Integration (when sink/backend touched) | `task test-integration` |
| Codegen drift | `task verify` (`preflight`) |
| Changelog drift | `task changelog:verify` (`preflight`) |
| Secret scan | gitleaks (`CI`) |
| UI (when `ui/` changed) | `task ui-ci` |

### Review expectations

Reviewers (currently the **maintainer only**) verify:

- CI is green and local preflight steps were run
- Tests cover behavior changes; ADRs updated for architectural decisions
- No secrets, private strings, or forbidden identities (`task scrub`, gitleaks)
- User-facing changes have conventional commits suitable for the changelog
- Security-sensitive paths follow [ADR-0104](docs/adr/0104-security-model.md)

**External contributions** always require maintainer review before merge. Maintainer-authored
changes may merge without a second human reviewer today (solo-maintainer policy — see
[GOVERNANCE.md](GOVERNANCE.md) and [ADR-0705](docs/adr/0705-release-supply-chain.md)).

## Pull request process

1. Fork or branch from `main`.
2. Run the [local preflight](#standards-map) checklist (`task lint` · `task coverage` ·
   `task coverage:race` · `task verify` · `task scrub`).
3. Keep changes focused; update ADRs in `docs/adr/` when making architectural decisions.
4. Ensure CI is green (`preflight` + `CI` workflows).
5. Request review; address feedback with additional commits (avoid force-push to `main`).

For test pyramid tiers, coverage floors, and integration expectations see
[Testing strategy](docs/development/testing.md). For Go conventions and CI gate matrix see
[Coding standards](docs/development/coding-standards.md).

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

By contributing, you agree that your contributions are licensed under the project [MIT
license](LICENSE) and that you certify the [DCO](DCO) as described above.

All participants are expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
