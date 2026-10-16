You are reviewing a whole git branch, independently of whoever implemented it, with an
ADVERSARIAL lens. You run read-only in the repository root. You have not seen the authors'
reasoning; do not look for it in `openspec/changes/cross-file-consistency-gates/loop.md`,
`evidence/` or `reviews/` (those are the authors' narrative). Judge the code.

## Inputs

1. TASK — OpenSpec change `cross-file-consistency-gates`. Requirements CFC-1..CFC-6 are in
   `openspec/changes/cross-file-consistency-gates/specs/cross-file-consistency/spec.md`
   (read it; it is the contract). Summary: CFC-1 Go version equality (go.mod `go` directive vs
   every `FROM golang:` in `Dockerfile*` and the devcontainer image; no `toolchain` line; CI uses
   `go-version-file: go.mod`; Renovate groups directive + image). CFC-2 every `go run`/`go install`
   tool pin builds under exactly go.mod's Go, plus a smoke run of golangci-lint, govulncheck,
   go-arch-lint. CFC-3 chart-rendered manager roles cover every triple of `config/rbac/role.yaml`
   (default and tenant mode, with a reasoned exceptions file). CFC-4 metric names agree between
   `internal/metrics`, `docs/operator-manual/metrics.md`, the chart PrometheusRule and the
   `registeredMetricNames` mirror. CFC-5 a weekly workflow fails when go.mod is behind the latest
   patch of its minor or when the lookup fails. CFC-6 every guard has a self-test (mutants must
   go red, a no-op copy must stay green) and runs on pull_request in a required job, plain and
   `--self-test` as separate steps.
2. DIFF — `git diff 0c1456b5d..HEAD` (base is origin/main 0c1456b5d). Ignore `openspec/`
   except the spec. The code under review: `hack/test/consistency_*_test.sh`,
   `hack/test/go_patch_lag_test.sh`, `hack/ci/go-patch-lag.sh`,
   `.github/workflows/go-patch-lag.yaml`, `.github/workflows/ci.yaml`, `hack/check.sh`,
   `hack/test/ci_workflow_security_test.sh`, `hack/test/ci_docs_gate_test.sh`,
   `hack/release/verify-eligibility.sh`, `renovate.json`, `go.mod`, devcontainer, `mise.toml`,
   `Makefile`, `Taskfile.yml`, `hack/tooling/.custom-gcl.yml`,
   `hack/test/testdata/consistency-rbac-exceptions.txt`, `docs/operator-manual/metrics.md`.
3. SENSORS (reported, do not rerun multi-minute tiers): each guard's plain run and `--self-test`
   were reported green per task; the holistic `task check` / `task coverage` / `task vulncheck`
   run happens in parallel with you and is not your concern.
4. RULES — `AGENTS.md` and `agent-context/GUIDELINES.md` if present (fail closed, no silent
   fallbacks, tests must be able to fail).

## What to attack (whole branch, across task boundaries)

a. For EACH guard: how could it pass while its invariant is broken? Look for fail-open paths:
   empty scans that count as pass, parse errors swallowed, `|| true`, pipelines whose exit
   status is lost (pipefail, SIGPIPE, subshells, `$(...)` swallowing failures, `set -e`
   suppressed inside `if`/`&&`), regexes that miss real syntax (quotes, line continuations,
   comments, YAML anchors, multiple `FROM` stages, `ARG`-templated versions, `golang:` with
   `-alpine`/digest suffixes), version comparisons that are lexical, numbers that overflow,
   files outside the scanned glob that would carry the same fact.
b. Self-tests (CFC-6): is each mutant actually exercising the clause it names, or would it go
   red for an unrelated reason (so the clause could be deleted and the self-test still pass)?
   Does the no-op copy prove anything? Are mutants applied to a copy, and could a failed `sed`
   (pattern not found) leave the copy unchanged yet still be counted as killed?
c. Interactions: the Go bump (go.mod to the Dockerfiles' Go), the golangci-lint / govulncheck /
   go-arch-lint bumps, the Renovate group rule, and the CI wiring. Would the next Renovate PR
   (golang image patch bump, or a tool bump) be caught, or slip, or be blocked forever? Does any
   guard depend on network in a way that makes CI flaky or that a cache could mask? Does the
   tool-pins guard really test under exactly go.mod's Go (GOTOOLCHAIN), never the host Go?
d. CI wiring: are all guards actually run on `pull_request`, in a job that is required
   (`hack/release/verify-eligibility.sh` lists required checks), in both modes, as separate
   steps? Can a path filter, `if:` condition, `continue-on-error`, or job dependency skip one?
   Does `hack/check.sh` really run them all locally?
e. Security of the new workflow `.github/workflows/go-patch-lag.yaml` and of `ci.yaml` changes:
   permissions, triggers, pinned actions by SHA, `persist-credentials`, script injection via
   `${{ }}` in `run:`, untrusted input (the go.dev JSON) reaching a shell, curl without timeouts
   or TLS enforcement, temp-file handling, cleanup traps, anything that writes outside a temp dir.
f. AI-code failure modes: tautological tests, weakened assertions, prose that claims more than
   the code does, exception lists that whitelist too much (read the RBAC exceptions file
   against the spec's allowed classes).

Mutation-verify where you can cheaply in your head or with a quick read-only probe; you cannot
write files. For each P0/P1 you are about to file, argue the opposite for one line; keep only
what survives. An APPROVE with no findings is valid when substantiated.

## Return (at most 25 lines, nothing else)

verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [CRITICAL|WARNING|NIT] <one line, with path:line> — <how the invariant breaks while the gate stays green, or the concrete risk>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity: CRITICAL = a guard can pass with its invariant broken on a realistic change, a security
hole, or a vacuous gate presented as proof. WARNING = correctness gap, untested behaviour, flaky
or fail-open path needing an unlikely trigger. NIT = note only.
