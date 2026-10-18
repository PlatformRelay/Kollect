# Cold review brief: stage-B round 2 (last) of OpenSpec change `test-depth-signals` (kollect)

You are an independent, adversarial reviewer with read-only access to this repository (cwd is the
repo root, branch `feat/test-depth-signals`, HEAD `d1dd0930e`, base `origin/main` @ `b05651d20`).
You have no network and cannot write `/tmp` or the tree. Read the files; do not guess. Change
nothing. You have not seen this branch before; nothing below is a conclusion for you to trust.

## Context

kollect is a Go Kubernetes operator. This change adds three test-depth signals:

- TDS-1: the `fuzz` job of `.github/workflows/ci.yaml` calls `hack/ci/fuzz-retry.sh <target> <pkg>`:
  fail closed, retry at most once and only the known Go fuzz deadline flake (an exact line
  sequence), deny markers fail at once, a zero-target or nothing-fuzzed run is red.
- TDS-2: a nightly `.github/workflows/test-shuffle.yaml` runs `task test:shuffle`
  (`hack/ci/test-shuffle.sh`): every `go list ./...` package three times with `-shuffle=<seed>`;
  Ginkgo (`RunSpecs`) packages as three `-count=1` invocations, the rest `-count=3`.
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README `## Quick start` `sh` fences.

Every `hack/test/*_test.sh` guard has a plain mode (checks the real tree) and a `--self-test` mode
(mutates a throwaway copy, asserts each mutant red, plus a no-op control).

A first branch review (round 1) found issues; its verified findings are in
`openspec/changes/test-depth-signals/reviews/B-branch/round-1/verification.md` (rows A1-A4, O1,
and the table "Deferred to stage B by the task loop"). They were addressed in the commits
`git log --oneline 31747e9c0..d1dd0930e` (task group G5, `tasks.md` section 5, evidence in
`openspec/changes/test-depth-signals/evidence/G5.md`). Do not take the evidence's word for it.

## Target questions (answer each explicitly)

1. Closure. For each of A1, A2, A3, A4 and O1, and each row of "Deferred to stage B": is it
   closed in the code at HEAD? Name the line that closes it and the self-test case or mutant that
   would turn red if the fix were reverted. Say "open" or "partly closed" with the reason when it
   is not.
2. A1 depth: can `readme_quickstart_test.sh` still pass a `kubectl get` naming a non-Kollect
   resource? Try operand forms: `kinv/a,foo/b`, `kinv,foo/b`, `-f`, `--all`, `TYPE.GROUP/NAME`,
   `kinv/` (empty name), quoting, and a flag value that looks like a resource.
3. A2 depth: can an `if:`, `continue-on-error:`, `timeout-minutes: 0`, a matrix exclude, a
   `strategy` change, a `needs` change or a `defaults.run.shell` change still skip or soften the
   `fuzz` job or the `test-shuffle` job with every guard green?
4. A3 depth: with the exit-status rule, can `fuzz-retry.sh` still retry something that is not the
   deadline flake, or refuse the real flake (exit status as `go test` reports it through `tee` and
   `PIPESTATUS`)?
5. A4 depth: can `hack/ci/test-shuffle.sh` still exit 0 after any `go test` invocation failed, or
   skip a later invocation? Does the guard's fake `go` now prove both?
6. Newly broken: did the G5 commits break anything that worked at `31747e9c0` (a self-test mutant
   that no longer applies or became equivalent, a case that now passes vacuously, a message the CI
   wiring or another guard greps for, the `internal: true` reader misreading a Taskfile shape the
   repo uses, the amended TDS-3 text claiming something the guard does not do)?
7. Anything else on the whole branch (`git diff b05651d20..d1dd0930e`) that fails open, is a
   tautological test, or claims more than the code does.

For each CRITICAL/WARNING you are about to file, argue the opposite in one line; keep only what
survives.

## Files

- `hack/ci/fuzz-retry.sh`, `hack/test/fuzz_retry_test.sh`, `hack/test/testdata/fuzz-retry/`
- `hack/ci/test-shuffle.sh`, `hack/test/test_shuffle_test.sh`, `hack/test/testdata/shuffle/`,
  `hack/task/Taskfile.test.yml`
- `hack/test/readme_quickstart_test.sh`, `README.md`, `Taskfile.yml`, `config/crd/bases/`
- `.github/workflows/ci.yaml`, `.github/workflows/test-shuffle.yaml`,
  `.github/workflows/ci-failure-report.yaml`
- `hack/test/ci_workflow_security_test.sh`, `hack/check.sh`
- spec: `openspec/changes/test-depth-signals/specs/test-depth/spec.md`, `tasks.md`

## Output format

Markdown. Start with `## Verdict` and one of APPROVE / CONCERNS / BLOCK and one sentence.
Then `## Answers` with one short paragraph per target question 1-7 (question 1 as a table: item,
closed/open, closing line, killing case).
Then `## Findings`, each as:

```
### <ID R2-A1..> <CRITICAL|WARNING|NOTE>: <one-line title>
- Where: <file:line>
- Evidence: what the file says (quote briefly)
- Why it matters: how it fails open or misses the spec
- Fix: the smallest change
```

CRITICAL = a guard or runner that can pass while its requirement is violated (fails open), or CI
wiring that does not run what it claims. WARNING = a real gap that should be fixed before merge.
NOTE = minor. End with `## Could not check`.
