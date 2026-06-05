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
| `task format:check` | `gofmt` drift gate |
| `task vulncheck` | `govulncheck` (module CVE scan) |

Architecture rules live in [`.go-arch-lint.yml`](../../.go-arch-lint.yml) at the repo root.
See [ARCHITECTURE.md](../ARCHITECTURE.md#package-boundaries) for the intended dependency direction.

### go-arch-lint baseline workflow

1. Edit `.go-arch-lint.yml` to describe the **target** graph (components + `mayDependOn`).
2. Run `task arch-lint`.
3. For each existing violation, **legalize** it in config (add `mayDependOn` or `# todo(arch-NN)`
   comment) rather than fixing all imports in one PR.
4. Remove `todo` entries incrementally as refactors land.

Generate a dependency graph (optional):

```sh
go run github.com/fe3dback/go-arch-lint@v1.15.0 graph
```

Pinned version: `GO_ARCH_LINT_VERSION` in `Taskfile.yml` (invoked via `go run …@version`, not linked
into the operator module — go-arch-lint's module graph conflicts with `go mod tidy`).

### golangci-lint dependency policy

`depguard` (deny deprecated / non-standard logging and errors) and `gomodguard` (block `logrus`,
`pkg/errors` in `go.mod`) extend the existing `.golangci.yml` profile. Configuration changes should
keep `task lint` green — expand `gomodguard.allowed.modules` if a legitimate new dependency is
blocked.

## SonarCloud (maintainer setup)

SonarCloud is **optional** until `SONAR_TOKEN` is configured. CI runs the scan with
`continue-on-error: true` so missing tokens do not block merges.

### 1. Create organization and project

1. Sign in at [SonarCloud](https://sonarcloud.io/) with GitHub (`konih` account).
2. Create or import organization **`konih`** (must match `sonar.organization` in
   `sonar-project.properties`).
3. Add project **`konih_kollect`** (must match `sonar.projectKey`).
4. Set visibility to **Public** (free for OSS).

### 2. GitHub secret

1. SonarCloud → **My Account** → **Security** → **Generate Tokens** (project analysis token).
2. GitHub → `konih/kollect` → **Settings** → **Secrets and variables** → **Actions**.
3. Add repository secret **`SONAR_TOKEN`** with the analysis token.

### 3. Quality gate (recommended)

After the first successful scan:

1. SonarCloud → project **kollect** → **Quality Gates**.
2. Prefer **Sonar way** or a custom gate that fails on **new** issues only (not legacy debt).
3. Enable **Pull Request decoration** (GitHub App) when ready for PR comments.

### 4. CI wiring

| Workflow | When | Coverage |
| --- | --- | --- |
| `.github/workflows/ci.yaml` job `sonarcloud` | Every push / PR (after `test`) | Downloads `coverage` artifact (`coverage.out`) |
| `.github/workflows/sonarcloud.yaml` | `workflow_dispatch` manual | Re-runs `task coverage` then scans |

Properties file: [`sonar-project.properties`](../../sonar-project.properties).

## What maintainers configure vs contributors

| Item | Contributor | Maintainer |
| --- | --- | --- |
| `task lint` / `arch-lint` | Run before PR | Keep `.go-arch-lint.yml` todos current |
| `SONAR_TOKEN` | — | GitHub secret + SonarCloud project |
| `CODECOV_TOKEN` | — | Optional; separate from Sonar |
| Quality gate blocking | — | Enable after baseline scan (Phase 1) |

## Further reading

- [Testing strategy](testing.md) — coverage floors, CI matrix, Sonar as trend dashboard
- [CONTRIBUTING.md](../../CONTRIBUTING.md) — PR lint checklist
- [DEVELOPMENT.md](../DEVELOPMENT.md) — local dev commands
- Local maintainer checklist (secrets, no tokens in repo): `agent-context/TOOLING-SETUP-MAINTAINER.md`
  (not committed — maintainer copy)
