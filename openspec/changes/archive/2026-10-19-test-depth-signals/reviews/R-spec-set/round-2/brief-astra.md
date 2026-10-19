# Cold review brief: stage-R round 2 of OpenSpec change `test-depth-signals` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/test-depth-signals`, HEAD `9a42d7591`). You have no network. Read the files; do not
guess. Change nothing.

## Context

kollect is a Go Kubernetes operator. This change adds three test-depth signals:

- TDS-1: the inline fuzz retry loop of the `fuzz` job in `.github/workflows/ci.yaml` moves to
  `hack/ci/fuzz-retry.sh`: fail closed, retry only the known Go fuzz deadline flake (an allow-list
  of its lines), and a zero-target fuzz run is red.
- TDS-2: a nightly `.github/workflows/test-shuffle.yaml` runs `task test:shuffle`: every unit
  package three times with `-shuffle=<seed>`, Ginkgo (`RunSpecs`) packages as three `-count=1`
  invocations, the rest with `-count=3`; envtest provisioned by the script; reported by
  `.github/workflows/ci-failure-report.yaml`.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands.

Operator-validated decisions (2026-10-16/18): shuffle scope A (envtest in, all unit packages except
e2e; Ginkgo specs are not reordered, a non-goal); fuzz retry A (fail closed; retry only the known
deadline-flake text).

Round 1 of this review produced findings V1-V17, recorded with their dispositions in
`openspec/changes/test-depth-signals/reviews/R-spec-set/verification.md` (the leg files beside it
are the raw reviews). Commit `9a42d7591` applied them (`git show 9a42d7591` shows the diff). V15 was
rejected and V17 deferred to task 1.1, both with reasons in that file.

Repo conventions: every `hack/test/*_test.sh` guard has a plain mode (checks the real tree) and a
`--self-test` mode (mutates a throwaway copy, asserts each mutant red, a no-op copy green).
`task check` (`hack/check.sh`) sweeps all guards in both modes. CI guards are wired into the
required `lint` job of `ci.yaml` in both modes (CWS-6 in `hack/test/ci_workflow_security_test.sh`)
and named in its CFC-6 `cfc_guards` list. Red-first TDD: each test task records a red that fails on
the target assertion; a group's tests and implementation land in one commit.

## Target questions

1. For each round-1 finding V1-V16: is it closed by the current spec set? Answer per id: closed,
   partly closed (what is missing), or not closed. Check that the fix matches what
   `verification.md` describes and that it holds against the code it cites.
2. Is anything new broken by the fixes: contradictions between proposal, spec and tasks; a case
   that cannot be red against the stub it names, or cannot kill the mutant that names it; a
   requirement with no case; a task over ~15 testable claims; an interface the tasks assume but
   the spec does not fix; a stale `path:line`.
3. The whole-spec question: would executing `tasks.md` as written deliver
   `specs/test-depth/spec.md`?

Check specifically: the Ginkgo split against `go.mod`'s pinned Ginkgo source in the module cache
(`go env GOMODCACHE`); the shuffle/count claims against `$(go env GOROOT)/src/testing/testing.go`;
the fuzz allow-list against the deny markers (does a deny-marker case still isolate its mutant
now that unknown lines are rejected anyway?); the corpus check in a package with no tracked
`testdata/`; envtest provisioning against `hack/coverage.sh` and `Makefile`; the reporter citations
in `hack/test/ci_failure_report_test.sh`; and every `path:line` in the three spec files.

## Files

- `openspec/changes/test-depth-signals/proposal.md`
- `openspec/changes/test-depth-signals/specs/test-depth/spec.md`
- `openspec/changes/test-depth-signals/tasks.md`
- `openspec/changes/test-depth-signals/reviews/R-spec-set/verification.md` (round 1)
- `.github/workflows/ci.yaml` (jobs `fuzz`, `lint`), `.github/workflows/ci-failure-report.yaml`,
  `.github/actions/go-cache/action.yml`
- `hack/check.sh`, `hack/coverage.sh`, `Makefile`, `Taskfile.yml`, `hack/task/Taskfile.test.yml`
- `hack/test/ci_workflow_security_test.sh`, `hack/test/ci_failure_report_test.sh`
- `README.md` (section `## Quick start (MVP)`), `config/crd/bases/`
- `internal/controller/suite_test.go`, `internal/webhook/v1alpha1/suite_test.go`,
  `internal/pipeline/integration_test.go`

## Output format

Markdown. Start with `## Verdict` and one of APPROVE / CONCERNS / BLOCK and one sentence.
Then `## Round-1 closure` as a table: id, status (closed / partly / not), one-line reason.
Then `## Findings` (new ones only), each as:

```
### <ID> <CRITICAL|WARNING|NOTE>: <one-line title>
- Where: <file:line>
- Evidence: what the file says (quote briefly)
- Why it matters: how executing tasks.md as written would miss the spec, or fail open
- Fix: the exact edit to the spec set
```

CRITICAL = executing the tasks as written would ship a guard that can pass while the spec is
violated, or a task that cannot be executed. WARNING = a real gap or stale claim that will cost a
task round. NOTE = minor. Finish with `## Stale file:line claims` (claim, cited, actual), or "none".
