# Coding standards

Binding standards for Go code, tests, security, commits, architecture, and merge gates in Kollect.
Engineering expectations (error taxonomy, robustness, definition of done) live in
[GUIDELINES.md](../../GUIDELINES.md). This page is the contributor-facing checklist enforced by
lint, CI, and review.

!!! tip "Before you open a PR"
    `task lint` · `task coverage` · `task verify` · `task scrub` — see
    [CONTRIBUTING.md](../../CONTRIBUTING.md#pull-request-process) and
    [Testing strategy](testing.md).

## Go style and lint

| Tool | Config / command | Purpose |
| --- | --- | --- |
| **golangci-lint v2** | [`.golangci.yml`](../../.golangci.yml) · `task lint` | Static analysis, formatters, dependency policy |
| **go-arch-lint** | [`.go-arch-lint.yml`](../../.go-arch-lint.yml) · `task arch-lint` | Package import boundaries |
| **gofmt / goimports** | `task format:check` | Formatting drift gate |

Run `task lint` locally before every PR. It includes golangci-lint **and** `go-arch-lint check`.

### Logging

Use **structured logging** via `log/slog` or `controller-runtime/log` (`logr`). Do **not** introduce
`github.com/sirupsen/logrus` — blocked by `depguard` and `gomodguard`.

The **`logcheck`** linter enforces [Kubernetes logging conventions](https://github.com/kubernetes-sigs/logcheck):
stable message strings with variable data in key/value pairs; never log secrets, tokens, or full
payloads. See [GUIDELINES.md § 1](../../GUIDELINES.md#1-error-handling) for the error-wrapping
and logging rules that pair with `logcheck`.

### Dependency policy

`depguard` denies deprecated stdlib shims (`io/ioutil`), non-standard error/logging packages, and
legacy protobuf imports. `gomodguard` blocks `logrus` and `pkg/errors` in `go.mod`. When adding a
legitimate dependency, extend `gomodguard.allowed.modules` in `.golangci.yml` and keep `task lint`
green. Details: [tooling-setup.md](tooling-setup.md#golangci-lint-dependency-policy).

### Tests and matchers

In `_test.go` files, prefer **Ginkgo/Gomega** matchers over `testify/assert` (`depguard` on tests).

## Testing

Follow the six-tier pyramid (L0–L5) in [Testing strategy](testing.md) and
[ADR-0706](../adr/0706-testing-merge-gate-architecture.md).

| Requirement | Detail |
| --- | --- |
| **Coverage floor** | 65% statement coverage on `./internal/...` today (`task coverage`, `COVERAGE_MIN`) |
| **Pre-v0.1.0 target** | 80% — ratchet the floor only when coverage is sustained |
| **Behavior tests** | Table-driven unit tests; envtest for controllers/webhooks; golden contracts for extractors |
| **New sink backends** | Must reach L3 integration (`task test-integration`) before merge |
| **Codegen drift** | `task verify` must pass (manifests, deepcopy, RBAC, mocks committed) |

Integration-tagged tests (`-tags=integration`) and e2e packages are excluded from the default
coverage profile.

## Security

| Control | Where |
| --- | --- |
| **CodeQL** | `.github/workflows/codeql.yaml` — Go analysis on `main` and PRs |
| **Secret scan** | `gitleaks` in CI; `task scrub` + `gitleaks protect --staged` before commit |
| **Vulnerability scan** | `task vulncheck` (`govulncheck`) in CI |
| **Threat model** | [SECURITY.md](../../SECURITY.md) |
| **Security ADR** | [ADR-0104](../adr/0104-security-model.md) — TLS, RBAC, redaction, secrets |

### Secrets and scrubbing

- Credentials only via `secretRef`; never in CR spec/status, logs, or committed files.
- Run `task scrub` on staged diffs before commit to catch private strings and old project identities.
- Profile redaction uses `scrubKeys` before items enter the store ([ADR-0303](../adr/0303-helm-release-inventory.md)).

### Git sink validation

Git and GitLab sink paths, refs, and config values are validated at admission and export time to
block path traversal, shell injection, and unsafe refs:

- `ValidateGitRef` — safe ref charset; rejects leading `-` and `..` segments
- `validateObjectPath` / workdir containment — object paths cannot escape the workdir
- `ValidateGitSinkWarnings` — admission warnings for risky git sink settings (webhook)

Implementation: `internal/sink/git/validate.go`, `internal/validation/git.go`. Extend these
validators when adding git-related fields; do not bypass with `#nosec` without an ADR note.

### Transport and hub ingest

Sink and doc endpoints must use **verified TLS** with org CA support (`caBundle` / `caSecretRef`).
Hub HTTP ingest listens in plain HTTP inside the pod — **terminate TLS at the ingress or service
mesh** before traffic reaches the operator ([ADR-0503](../adr/0503-hub-cluster-auth-istio-pattern.md)).

## Commits

[CONTRIBUTING.md § Commit messages](../../CONTRIBUTING.md#commit-messages) defines the format:

- **[Conventional Commits](https://www.conventionalcommits.org/)** types and scopes
- Optional **[gitmoji](https://gitmoji.dev/) shortcode** prefix (`:sparkles:`, not Unicode emoji)
- Breaking changes only when a tagged release exists and adopters must migrate

Changelog entries are generated with [git-cliff](https://git-cliff.org/) (`cliff.toml`). Only
`feat`, `fix`, `perf`, `refactor`, and breaking commits appear in the user-facing changelog.

## Architecture

Package layout and dependency flow are documented in
[ARCHITECTURE.md § Package boundaries](../ARCHITECTURE.md#package-boundaries).

Import rules are enforced by [`.go-arch-lint.yml`](../../.go-arch-lint.yml) (`task arch-lint`).
When introducing a new `internal/` package or cross-component import:

1. Update `.go-arch-lint.yml` to describe the intended graph.
2. Run `task arch-lint` and legalize existing violations incrementally (see
   [tooling-setup.md](tooling-setup.md#go-arch-lint-baseline-workflow)).
3. Record non-trivial decisions in `docs/adr/`.

Non-trivial API, tenancy, sink, or multi-cluster changes require an ADR before merge — see
[ADR index](../adr/README.md).

## Pull request and CI gates

`main` is protected: linear history, required checks **`preflight`** and **`test`**, no force-push.
Use **Rebase and merge** on PRs ([CONTRIBUTING.md § Changelog and releases](../../CONTRIBUTING.md#changelog-and-releases)).

| Gate | Workflow / task | Blocks merge? |
| --- | --- | --- |
| **Preflight** | `.github/workflows/preflight.yaml` | Yes |
| `go mod tidy` drift | preflight job | Yes |
| Codegen drift | `task verify` | Yes |
| Stale changelog | `task changelog:verify` | Yes |
| **CI** | `.github/workflows/ci.yaml` | Yes |
| Lint + arch fitness | `task lint` | Yes |
| Coverage floor | `task coverage` | Yes |
| Integration (L3) | `task test-integration` | Yes |
| Secret scan | gitleaks | Yes |
| Helm / image smoke | `task helm-test`, `task docker:build` | Yes |

Optional jobs (perf-report, SonarCloud, e2e-nightly) do not block merge unless noted in ADR-0706.
