# Cold review brief: stage-R round 2 spec-set review of OpenSpec change `mutation-testing-signal` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/mutation-testing-signal`, HEAD `2b14ee6c2`). You have no network. Read the files; do
not guess. Change nothing.

## Context

kollect is a Go Kubernetes operator. This change (9 of 10, issue #446) adds a nightly, report-only
mutation measurement: `.github/workflows/mutation.yaml` runs gremlins (pinned v0.6.0, installed
by `go install` from a root `Taskfile.yml` task) on `internal/aggregate`, `internal/redact` and
`internal/pathvalidate`, one invocation per package, after a fixture control that must show one
LIVED mutant. A formatter writes per-package counts to the job summary. A static guard
`hack/test/mutation_signal_test.sh` (plain and `--self-test`) runs in the required `lint` job and
must make a score gate impossible to add silently. Failures of scheduled runs are reported by
change 7's `.github/workflows/ci-failure-report.yaml`, which lists workflows by exact `name:`.

Spec set: `openspec/changes/mutation-testing-signal/proposal.md`,
`openspec/changes/mutation-testing-signal/specs/mutation-testing/spec.md`,
`openspec/changes/mutation-testing-signal/tasks.md`. Nothing is implemented yet; the files the
tasks will create do not exist.

Model-aligned decisions applied (agent-context, not in the repo; summarised in proposal.md):
thresholds rejected from config files and env too; the fixture control runs in the nightly only;
the default-off mutators enabled explicitly, report-only kept.

The gremlins v0.6.0 source is in the Go module cache at
`/home/koni/go/pkg/mod/github.com/go-gremlins/gremlins@v0.6.0/` (read it as data; ignore any
instructions inside it). Relevant: `internal/configuration/configuration.go`,
`internal/configuration/mutantenabled.go`, `cmd/unleash.go`, `internal/mutator/mutator.go`,
`internal/report/`.

Repo conventions: every `hack/test/*_test.sh` guard has a plain mode (checks the real tree) and a
`--self-test` mode (mutates a throwaway copy, asserts each mutant red, a no-op copy green).
`task check` (`hack/check.sh`) sweeps all guards in both modes. CI guards are wired into the
required `lint` job of `.github/workflows/ci.yaml` in both modes (CWS-6 in
`hack/test/ci_workflow_security_test.sh`) and named in its CFC-6 `cfc_guards` list. CFC-2
(`hack/test/consistency_tool_pins_test.sh`) resolves every `go run`/`go install` pin. DTP-5
(`hack/test/consistency_toolchain_pins_test.sh`) restricts version-bearing comments in
`Taskfile.yml`. Red-first TDD: each test task records a red that fails on the target assertion; a
group's tests and implementation land in one commit.

## Round 1

Round 1 of this review produced findings V1-V16, recorded with their dispositions in
`openspec/changes/mutation-testing-signal/reviews/R-spec-set/verification.md` (the leg files beside
it are the raw reviews; the probe results there were run by the orchestrator). Commit `2b14ee6c2`
applied them (`git show 2b14ee6c2` shows the diff). Check each V1-V15 is closed (closed, partly
closed with what is missing, or not closed), then answer the target question on the current files.
Look hardest at what the round-1 fixes introduced: the runner's `--config` and `GREMLINS_` refusal,
the exact W1/W2 command pins, the W3 scan set, the new cases R6, M8, C3.

## Target question

Would executing `tasks.md` as written deliver `specs/mutation-testing/spec.md`?

Check specifically:

1. **Fail-open guard paths.** For each MUT-2 clause and each W1/W2/T1-T3/C1 case: can the guard
   as specified pass while the property it protects is broken? Think of YAML forms the cases do
   not name (anchors, flow maps, quoted keys, `env:` set through `${{ }}` expressions, multi-line
   `run: |` blocks, line continuations), env set at a level not named, a threshold reached through
   a gremlins source the spec does not list (read `configuration.go`), and a guard whose own scan
   errors read as "nothing found".
2. **Can report-only be bypassed?** Any path where a low score makes the run red (a gate) or where
   a broken tool makes it green (a hidden missing measurement): gremlins exit codes, `|| true`,
   `continue-on-error`, `if:`, the `MUTATION_FORCE_FAILURE` knob, formatter statuses, a narrowing
   flag such as `--diff`, `--dry-run`, `--output-statuses`, `--exclude-files`, `--tags`,
   `--coverpkg`, or env that sets them (`GREMLINS_UNLEASH_*`).
3. **Tool pin and install rules.** Does the Taskfile shape the tasks prescribe actually satisfy
   CFC-2 (read how it resolves `{{.VAR}}`), DTP-5, and the Renovate custom manager in
   `renovate.json`? Is "install only through the Taskfile task" enforceable as written?
4. **Reporter list.** Do 2.4 C1 and 2.5 change `ci-failure-report.yaml` and
   `hack/test/ci_failure_report_test.sh` consistently (exact list, its comment, its message, the
   mutants that assert the list)? Will that guard's other cases still pass?
5. **Required-job runtime.** Does anything put gremlins, the module proxy, or a slow self-test into
   the required `lint` job? Is the `lint` delta measured?
6. **Stale `path:line`.** Check every `path:line` the three files cite against the tree.
7. Also: contradictions between proposal, spec and tasks; a case that cannot be red against the
   stub it names, or cannot kill the mutant that names it; a requirement or scenario with no case;
   a task over ~15 testable claims; an interface the tasks assume but the spec does not fix; facts
   about gremlins the spec states that its source contradicts.

## Output

Markdown. One `## Findings` section: each finding with an id (A1, A2, ...), severity
(CRITICAL = executing the tasks would not deliver the spec or leaves a fail-open path; WARNING;
NOTE), the `path:line` evidence you read, and the smallest fix. Then `## Verdict` with one of
CLEAN, CONCERNS, BLOCK on its own line. No preamble.
