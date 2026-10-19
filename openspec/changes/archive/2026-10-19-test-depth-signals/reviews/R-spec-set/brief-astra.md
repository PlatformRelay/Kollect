# Cold review brief: spec set of OpenSpec change `test-depth-signals` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/test-depth-signals`). You have no network. Read the files; do not guess. Change nothing.

## Context

kollect is a Go Kubernetes operator. This change (8 of 10 in a CI-hardening series) adds three
test-depth signals:

- TDS-1: move the inline fuzz retry loop of the `fuzz` job in `.github/workflows/ci.yaml` into
  `hack/ci/fuzz-retry.sh`, fail closed, retry only the known Go fuzz deadline flake, and make a
  zero-target fuzz run red.
- TDS-2: a nightly workflow `.github/workflows/test-shuffle.yaml` running `task test:shuffle`
  (`go test -shuffle=<seed> -count=3` on all unit packages except e2e, envtest provisioned),
  reported by `.github/workflows/ci-failure-report.yaml`.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands.

The operator validated two decisions on 2026-10-18:

- shuffle scope A: envtest provisioned, all unit packages except e2e; Ginkgo specs are not
  reordered (an explicit non-goal);
- fuzz retry A: fail closed; retry only the known deadline-flake text.

Repo conventions you need: every `hack/test/*_test.sh` guard has a plain mode (checks the real
tree) and a `--self-test` mode (mutates a throwaway copy, asserts each mutant red, a no-op copy
green). `task check` (`hack/check.sh`) sweeps all guards in both modes. CI guards must be wired
into the required `lint` job of `ci.yaml` in both modes (rule CWS-6 in
`hack/test/ci_workflow_security_test.sh`; CFC-6 in the cross-file consistency guard). New
workflows must pass zizmor (`.github/zizmor.yml`) and the CWS rules. Red-first TDD: each test task
must record a red that fails on the target assertion, and test+implementation land in one commit.

## Target question

Would executing `tasks.md` as written deliver the spec (`specs/test-depth/spec.md`)?

Look specifically for:

1. Missing, misordered or unverifiable tasks; tasks or cases that do not name what would expose
   their failure.
2. Whether the fuzz runner as specified can fail open: a crash misread as the deadline flake, a
   zero-target run that exits 0, partial or truncated logs, a matrix target/package mismatch, the
   corpus check (`git status --porcelain` under `testdata/fuzz/`) being blind (e.g. gitignored
   paths, the Go fuzz cache dir outside the repo, untracked-vs-modified files).
3. Shuffle seed reproducibility with `-count=3` (how Go's `-shuffle` and `-count` interact; the
   seed printing; multiple packages each getting the seed).
4. Whether envtest provisioning in a nightly actually works: read how CI test jobs provision it
   (`.github/actions/go-cache/action.yml`, `ci.yaml` test jobs, `Makefile` `setup-envtest`,
   `hack/coverage.sh`, `internal/envtestassets`).
5. Wiring into lint in both modes (CWS-6, CFC-6), and the `hack/check.sh --self-test` sweep.
6. The change-7 reporter list edit (`.github/workflows/ci-failure-report.yaml` and
   `hack/test/ci_failure_report_test.sh`).
7. zizmor and CWS rules for the new workflow (permissions, pinned actions, persist-credentials,
   concurrency, timeout, anything CWS-1..7 demand).
8. Runtime added to required jobs (`lint`), e.g. new self-tests copying the tree per mutant.
9. Anything needing operator accounts or permissions (should be parked as an owner task).
10. Stale `file:line` claims: re-check every `path:line` cited in proposal.md, spec.md, tasks.md
    against the current tree and report each that is wrong.

## Files

- `openspec/changes/test-depth-signals/proposal.md`
- `openspec/changes/test-depth-signals/specs/test-depth/spec.md`
- `openspec/changes/test-depth-signals/tasks.md`
- `openspec/changes/test-depth-signals/loop.md` (context only)
- `.github/workflows/ci.yaml` (jobs `fuzz`, `lint`, test jobs), `.github/workflows/ci-failure-report.yaml`,
  `.github/workflows/e2e-nightly.yaml` (or similarly named), `.github/actions/go-cache/action.yml`,
  `.github/zizmor.yml`
- `hack/check.sh`, `hack/coverage.sh`, `Makefile`, `Taskfile.yml`, `hack/task/Taskfile.test.yml`
- `hack/test/ci_workflow_security_test.sh`, `hack/test/ci_failure_report_test.sh`,
  the cross-file consistency guard (`grep -ln CFC-6 hack/test/*.sh`),
  `hack/test/demo_task_aliases_test.sh`, `hack/test/openspec_evidence_markdown_only_test.sh`
- `README.md` (section `## Quick start (MVP)`), `config/crd/bases/`
- `internal/controller/suite_test.go`, `internal/webhook/v1alpha1/suite_test.go`, `internal/envtestassets/`

## Output format

Markdown. Start with `## Verdict` and one of APPROVE / CONCERNS / BLOCK and one sentence.
Then `## Findings`, each as:

```
### <ID> <CRITICAL|WARNING|NOTE>: <one-line title>
- Where: <file:line> (and any other file:line)
- Evidence: what the file says (quote briefly)
- Why it matters: how executing tasks.md as written would miss the spec, or fail open
- Fix: the exact edit to the spec set
```

CRITICAL = executing the tasks as written would ship a guard that can pass while the spec is
violated, or a task that cannot be executed. WARNING = a real gap or stale claim that will cost a
task round. NOTE = minor. Finish with `## Stale file:line claims` as a table: claim, cited, actual.
