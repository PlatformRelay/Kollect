# Architecture and quality tooling setup

Kollect enforces **package import boundaries** (`go-arch-lint`), **external dependency policy**
(`golangci-lint` `depguard` + `gomodguard`), and optional **SonarCloud** maintainability
dashboards. This page covers what works locally without any SaaS account and what maintainers must
configure once.

!!! tip "Works without accounts"
    `task lint` (golangci-lint + `go-arch-lint`), `task arch-lint`, `task format:check`, and
    `task vulncheck` run fully offline after `go mod download`. No SonarCloud or GitHub secrets
    required for day-to-day development.

## Local commands

| Task | Purpose |
| --- | --- |
| `task lint` | golangci-lint v2 **and** `go-arch-lint check` |
| `task arch-lint` | Import-graph fitness only (`.go-arch-lint.yml`) |
| `task arch-lint:graph` | Render DI dependency graph (`--type di --include-vendors`) to `docs/architecture-graph.svg` |
| `task arch-lint:graph:flow` | Same graph in flow (reverse-DI) view — optional |
| `task format:check` | `gofmt` + `goimports` drift gate (`golangci-lint fmt --diff`) |
| `task vulncheck` | `govulncheck` (module CVE scan) |
| `task sonar` / `task sonar:local` | Local SonarCloud upload (maintainer; needs `SONARCLOUD_TOKEN`) |

Architecture rules live in [`.go-arch-lint.yml`](https://github.com/platformrelay/kollect/blob/main/.go-arch-lint.yml).
See [concepts/architecture.md](../concepts/architecture.md#package-boundaries) for the intended dependency direction.

### go-arch-lint baseline workflow

1. Edit `.go-arch-lint.yml` to describe the **target** graph (components + `mayDependOn`).
2. Run `task arch-lint`.
3. For each existing violation, **legalize** it in config (add `mayDependOn` or `# todo(arch-NN)`
   comment) rather than fixing all imports in one PR.
4. Remove `todo` entries incrementally as refactors land.

Generate a dependency graph (optional):

```sh
task arch-lint:graph          # DI view + vendors (default, linked from concepts/architecture.md)
task arch-lint:graph:flow     # reverse-DI / execution-flow view
```

Output: `docs/architecture-graph.svg` (both tasks write the same path; run only one before commit).
Flags are fixed in `Taskfile.yml`: `--type di|flow`, `--include-vendors`. Vendor nodes require the
`vendors` and per-component `canUse` entries in `.go-arch-lint.yml`. Pinned version:
`GO_ARCH_LINT_VERSION` in `Taskfile.yml` (invoked via `go run …@version`, not linked
into the operator module — go-arch-lint's module graph conflicts with `go mod tidy`).

See [concepts/architecture.md § Package boundaries](../concepts/architecture.md#package-boundaries) for the rendered
graph.

### golangci-lint dependency policy

`depguard` (deny deprecated / non-standard logging and errors) and `gomodguard` (block `logrus`,
`pkg/errors` in `go.mod`) are configured in [`.golangci.yaml`](https://github.com/platformrelay/kollect/blob/main/.golangci.yaml). The
**logcheck** plugin is built via [`hack/tooling/.custom-gcl.yml`](https://github.com/platformrelay/kollect/blob/main/hack/tooling/.custom-gcl.yml) when `make lint` runs.
Configuration changes should keep `task lint` green — adjust `depguard` / `gomodguard.blocked`
rules if a legitimate new dependency is blocked.

## SonarCloud (maintainer setup)

SonarCloud is **optional** until `SONAR_TOKEN` is configured. CI runs the scan with
`continue-on-error: true` so missing tokens do not block merges.

### 1. Create organization and project

1. Sign in at [SonarCloud](https://sonarcloud.io/) with a GitHub account that administers the `platformrelay` organization.
2. Create or import organization **`platformrelay`** (must match `sonar.organization` in
   `sonar-project.properties`).
3. Add project **`PlatformRelay_Kollect`** (must match `sonar.projectKey`).
4. Set visibility to **Public** (free for OSS).

### 2. Tokens (GitHub secret + local `.envrc`)

SonarCloud exposes one **project analysis token**. Use the same value in two places — different
env var names by convention:

| Where | Variable | Notes |
| --- | --- | --- |
| GitHub Actions | `SONAR_TOKEN` | Repository secret (`Settings` → `Secrets and variables` → `Actions`) |
| Local (direnv) | `SONARCLOUD_TOKEN` | Export in `.envrc` (gitignored); run `direnv allow` after edit |

Steps:

1. SonarCloud → **My Account** → **Security** → **Generate Tokens** (project analysis token).
2. GitHub → `platformrelay/kollect` → add repository secret **`SONAR_TOKEN`** with that token.
3. Locally, add `export SONARCLOUD_TOKEN="<same token>"` to `.envrc` (never commit).

### 3. Local Sonar scan

After `SONARCLOUD_TOKEN` is in `.envrc` and direnv is loaded:

```sh
task sonar          # alias for sonar:local
task sonar:local    # runs task coverage, then sonar-scanner via Docker
```

Requires Docker. Uploads to org **`platformrelay`**, project **`PlatformRelay_Kollect`** per
[`sonar-project.properties`](https://github.com/platformrelay/kollect/blob/main/sonar-project.properties). First run may take a few minutes;
confirm the project appears at [SonarCloud](https://sonarcloud.io/project/overview?id=PlatformRelay_Kollect).

### 4. Quality gate (recommended)

After the first successful scan:

1. SonarCloud → project **kollect** → **Quality Gates**.
2. Prefer **Sonar way** or a custom gate that fails on **new** issues only (not legacy debt).
3. Enable **Pull Request decoration** (GitHub App) when ready for PR comments.

### 5. CI wiring

| Workflow | When | Coverage |
| --- | --- | --- |
| `.github/workflows/ci.yaml` job `sonarcloud` | Every push / PR (after `test`) | Downloads `coverage` artifact (`coverage.out`) |
| `.github/workflows/sonarcloud.yaml` | `workflow_dispatch` manual | Re-runs `task coverage` then scans |

Properties file: [`sonar-project.properties`](https://github.com/platformrelay/kollect/blob/main/sonar-project.properties).

## Codecov (maintainer setup)

Codecov complements the merge gate (`task coverage` / `COVERAGE_MIN` in CI) with PR patch
coverage comments, project trends, and the README badge. Uploads are **non-blocking**
(`fail_ci_if_error: false`); the **`test`** job enforces the coverage floor.

### 1. Install the Codecov GitHub App (required for reliable PR comments)

Codecov uploads can succeed without the app, but PR comments and status checks are rate-limited
when Codecov calls the GitHub API with a shared token instead of app credentials. If you see
*“install Codecov GitHub App for reliable uploads/comments”* on a PR, complete this step once:

1. Open [github.com/apps/codecov](https://github.com/apps/codecov) and click **Configure**.
2. Select the **`platformrelay`** organization (owns `platformrelay/kollect`).
3. Grant access to **`kollect`** (or all repositories if you prefer org-wide setup).
4. Confirm the repo appears at [codecov.io/gh/platformrelay/kollect](https://codecov.io/gh/platformrelay/kollect).

No repository secret is required for uploads when CI uses OIDC (see below).

### 2. CI wiring

| Item | Location |
| --- | --- |
| Upload step | `.github/workflows/ci.yaml` job **`test`** — `codecov/codecov-action` v6 with `use_oidc: true` |
| Project / patch targets | [`codecov.yml`](https://github.com/platformrelay/kollect/blob/main/codecov.yml) at repo root |
| Merge gate (blocking) | `COVERAGE_MIN` env on the same job — independent of Codecov |

The **`test`** job requests `id-token: write` so GitHub Actions can mint an OIDC token for
Codecov upload authentication ([Codecov OIDC docs](https://docs.codecov.com/docs/codecov-tokens)).

Legacy **`CODECOV_TOKEN`** repository secrets are optional and ignored when `use_oidc: true`; you
may remove the secret after verifying uploads on `main`.

### 3. Local coverage (contributors)

Contributors do not need Codecov accounts. Run `task coverage` before opening a PR; CI uploads
`coverage.out` automatically when the **`test`** job passes.

## Renovate dependency updates

Renovate is self-hosted: the **Renovate** workflow (`.github/workflows/renovate.yaml`) runs it
hourly (cron `17 * * * *`), one run at a time, and a maintainer can start a run by hand with
**Run workflow** (manual runs log at debug level). It groups updates for Go modules, Kubernetes
libraries, GitHub Actions, container images, the docs npm and pip-compile locks, and the pinned
build tools. Dependabot vulnerability alerts and security updates stay enabled in the repository
settings; there are no Dependabot version-update jobs.

**Token.** Renovate authenticates with the repository secret `RENOVATE_TOKEN`, a token owned by
the maintainer (**Settings → Secrets and variables → Actions**). There is no fallback: when the
secret is empty the first step fails the run and Renovate does not start, because pull requests
opened with the workflow's own token never start CI and could never merge. The job's
`GITHUB_TOKEN` is read-only and only checks out the repository.

**Pinned Renovate.** The workflow runs a Renovate image pinned by tag and digest (the
`renovate-image` input). Renovate updates that pin itself, in a PR of its own that is opened or
updated only on Mondays (UTC), manual runs included, and is never auto-merged: a new Renovate
changes what the bot does, so a maintainer reads it first.

**Dependency dashboard.** Renovate keeps a Dependency Dashboard issue open with the pending,
rate-limited and failed updates. Its checkboxes take effect on the next run.

**What auto-merges.** Only Docker image digest updates of the Dockerfiles and the devcontainer
(same tag, new digest). Renovate merges such a PR itself, by rebase, once every check on its head
is green, and rebases it first when it is behind `main`; it merges at most one branch per run.
Everything else waits for a maintainer: Go modules, the `go` directive and the golang image tag
(the `go toolchain` group), GitHub Actions, the pinned tools, the docs packages, the Renovate
image, and any image tag change. A group PR that mixes a digest with any other update is not
auto-merged. The scope is narrow on purpose: `RENOVATE_TOKEN` is a bypass actor of the `main`
ruleset, so GitHub would not stop its merge even if a required check never started. A new
third-party image added to a Dockerfile or the devcontainer joins the auto-merge scope.

**Rebasing a bot PR.** Bot PRs outside the auto-merge scope are rebased only when they conflict.
The exception is a group PR whose first update in Renovate's order is an image digest: it takes
that update's setting, so Renovate rebases it whenever it falls behind `main`, but never merges
it. Rebase a bot PR before merging with `gh pr update-branch --rebase <number>`; the rebase
checkbox in the PR body works too, but only on the next hourly run.

**Pausing.** `gh workflow disable renovate.yaml` stops the schedule;
`gh workflow enable renovate.yaml` turns it back on.

**Validating the config.** Repository rules live in `renovate.json`, the self-hosted global
config in `.github/renovate-config.json`. Validate both with
`bash hack/ci/renovate-config-validate.sh`. It runs `renovate-config-validator` from the same
pinned image as the bot, so it needs Docker, and the required `lint` job runs it on every PR.
`hack/test/renovate_policy_test.sh` guards the workflow, both configs and this section.

**The hosted Renovate App is unused.** It is still installed on the organisation and has never
opened a PR here; removing it from the repository is the maintainer's job. If it ever starts
working on this repository, two bots race on the same `renovate/*` branches.

**Before a release.** Auto-merged digest bumps reach `main` without a human. Before approving a
release, the maintainer reviews the bot merges since the last tag: the PRs labelled
`dependencies` merged after it.

## What maintainers configure vs contributors

| Item | Contributor | Maintainer |
| --- | --- | --- |
| `task lint` / `arch-lint` | Run before PR | Keep `.go-arch-lint.yml` todos current |
| `SONAR_TOKEN` / `SONARCLOUD_TOKEN` | — | GitHub secret + `.envrc` (same token, different names) |
| Codecov GitHub App | — | [Install app](https://github.com/apps/codecov) on `platformrelay/kollect` for reliable PR comments |
| `CODECOV_TOKEN` | — | Legacy optional; CI uses OIDC — safe to delete after upload verified |
| `RENOVATE_TOKEN` | — | Repository secret: the maintainer-owned token Renovate runs as (contents and pull-request write), so bot PRs start CI; no fallback, the Renovate run fails without it |
| SonarCloud quality gate blocking | — | Enable after baseline scan (Phase 1) |

## Further reading

- [Testing strategy](testing.md) — coverage floors, CI matrix, Sonar as trend dashboard
- [CONTRIBUTING.md](https://github.com/platformrelay/kollect/blob/main/CONTRIBUTING.md) — PR lint checklist
- [development/setup.md](../development/setup.md) — local dev commands
