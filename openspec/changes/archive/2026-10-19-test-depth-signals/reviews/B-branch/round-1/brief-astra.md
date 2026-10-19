# Cold review brief: stage-B whole-branch adversarial review of OpenSpec change `test-depth-signals` (kollect)

You are an independent, adversarial reviewer with read-only access to this repository (cwd is the
repo root, branch `feat/test-depth-signals`, HEAD `548d0d973`, base `origin/main` @ `b05651d20`).
You have no network and cannot write `/tmp` or the tree. Read the files; do not guess. Change
nothing. The range is `git diff b05651d20..HEAD` (`git log --oneline b05651d20..HEAD`).

## Context

kollect is a Go Kubernetes operator. This change adds three test-depth signals:

- TDS-1: the `fuzz` job of `.github/workflows/ci.yaml` calls `hack/ci/fuzz-retry.sh <target> <pkg>`:
  fail closed, retry at most once and only the known Go fuzz deadline flake (an exact line
  sequence), deny markers fail at once, a zero-target or nothing-fuzzed run is red. A
  `go mod download` step runs before it so `go: downloading` lines never reach the runner.
- TDS-2: a nightly `.github/workflows/test-shuffle.yaml` runs `task test:shuffle`
  (`hack/ci/test-shuffle.sh`): every `go list ./...` package three times with `-shuffle=<seed>`;
  Ginkgo (`RunSpecs`) packages as three `-count=1` invocations, the rest `-count=3`; envtest
  provisioned by the script; the workflow is listed in `ci-failure-report.yaml`'s
  `on.workflow_run.workflows` (the change-7 failure reporter, which reports scheduled runs only).
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README `## Quick start` `sh` fences:
  `task <name>` exists, `kubectl apply -k <path>` exists, `kubectl get` names are CRD names.

Operator-validated decisions: shuffle scope A (envtest in, e2e only out; Ginkgo spec order is a
non-goal); fuzz retry A (fail closed; retry only the known deadline-flake text).

Repo conventions: every `hack/test/*_test.sh` guard has a plain mode (checks the real tree) and a
`--self-test` mode (mutates a throwaway copy, asserts each mutant red). `hack/check.sh` (`task
check`) sweeps all guards in both modes; CI guards run in the required `lint` job of `ci.yaml` in
both modes (CWS-6 in `hack/test/ci_workflow_security_test.sh`). Fixtures captured from real runs
are in `hack/test/testdata/fuzz-retry/`.

Already known and deferred to this stage (do not re-report as new; you may say whether each is
worse than it looks): F16 in `ci_workflow_security_test.sh` ignores `if:` and `continue-on-error:`
on the module-download step; a `make setup-envtest` failure exits without an `::error::` line; no
empty-pkg F0 case/mutant and no case replaying the real crash fixture `crash-local.txt`; the
README guard's round-2 fixes (backslash continuation, indented fence) were not re-reviewed;
`internal: true` tasks count as existing tasks.

## Target questions (answer each explicitly)

1. Can `hack/ci/fuzz-retry.sh` retry a real crash, or pass a run that fuzzed zero targets or
   nothing at all? Try: deny markers split across lines or prefixed; a crash whose output also
   matches the flake sequence; a corpus file under a nested `testdata/fuzz/`; `pkg` with `/...`
   matching several packages; `go test` output on stderr vs stdout; a retry whose second attempt
   prints `no fuzz tests to fuzz`; exit codes other than 0/1.
2. Can the shuffle job pass without actually shuffling (seed ignored, `-shuffle=off`, empty seed),
   without running some packages (go list failure, a package silently dropped, an exclusion), or
   without envtest (assets empty, `make setup-envtest` failing, `KUBEBUILDER_ASSETS` unset in the
   `go test` environment)? Does a failure in invocation 1 of 3 stop or get masked by a later pass?
3. Is the reported seed enough to reproduce a failure? Check against
   `$(go env GOROOT)/src/testing/testing.go` (how `-shuffle` and `-count` interact) and the Ginkgo
   seed note in the spec.
4. Can `hack/test/readme_quickstart_test.sh` pass on a broken quick start? Try: commands after
   `;`, `||`, pipes, `$(...)`, `sudo`/`env X=1` prefixes, `task -d`/flags, `kubectl -n x get`,
   `kubectl get kinv/name`, `kubectl apply -k` with a path that is a file not a kustomization,
   `make` targets, a second `Quick start` heading, a heading at `###` level, CRLF line endings.
5. Is the CI wiring correct: the `fuzz` job permissions, matrix, step order and `go mod download`
   step; the `lint` job running each new guard in both modes; `test-shuffle.yaml` permissions,
   pins, concurrency, timeout, triggers (does it also run on PRs or `workflow_dispatch`?), and does
   it set `SHUFFLE_SEED`? Is the change-7 reporter entry (`ci-failure-report.yaml` and its test)
   correct, and does the workflow name match exactly?
6. What does a scheduled run of `test-shuffle` do on failure: does the reporter open an issue, is
   the seed reachable from the issue or the log, and does it collide with the E2E nightly's issue?
7. Are the two Go test fixes (`internal/collect/metrics_snapshot_test.go`,
   `internal/metrics/metrics_test.go`) real fixes for order dependence, or do they weaken the
   assertion?
8. Anything else in the diff that fails open, is a tautological test, or claims more than the code
   does.

For each CRITICAL/WARNING you are about to file, argue the opposite in one line; keep only what
survives.

## Files

- `hack/ci/fuzz-retry.sh`, `hack/test/fuzz_retry_test.sh`, `hack/test/testdata/fuzz-retry/`
- `hack/ci/test-shuffle.sh`, `hack/test/test_shuffle_test.sh`, `hack/test/testdata/shuffle/`,
  `hack/task/Taskfile.test.yml`
- `hack/test/readme_quickstart_test.sh`, `README.md`, `Taskfile.yml`, `config/crd/bases/`
- `.github/workflows/ci.yaml`, `.github/workflows/test-shuffle.yaml`,
  `.github/workflows/ci-failure-report.yaml`, `hack/ci/ci-failure-report.sh`
- `hack/test/ci_workflow_security_test.sh`, `hack/test/ci_failure_report_test.sh`, `hack/check.sh`
- `hack/coverage.sh`, `Makefile`
- spec: `openspec/changes/test-depth-signals/specs/test-depth/spec.md`, `tasks.md`, `proposal.md`

## Output format

Markdown. Start with `## Verdict` and one of APPROVE / CONCERNS / BLOCK and one sentence.
Then `## Answers` with one short paragraph per target question 1-8.
Then `## Findings`, each as:

```
### <ID A1..An> <CRITICAL|WARNING|NOTE>: <one-line title>
- Where: <file:line>
- Evidence: what the file says (quote briefly)
- Why it matters: how it fails open or misses the spec
- Fix: the smallest change
```

CRITICAL = a guard or runner that can pass while its requirement is violated (fails open), or CI
wiring that does not run what it claims. WARNING = a real gap that should be fixed before merge.
NOTE = minor. End with `## Could not check`.
