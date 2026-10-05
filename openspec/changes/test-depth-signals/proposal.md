# Proposal

## Why

Three weaknesses in how the suite proves itself, each seen in the tree:

- The fuzz job retries any failure once when no `testdata/fuzz/` file appeared
  (`.github/workflows/ci.yaml`, job `fuzz`, the `for attempt in 1 2` loop). A panic Go fails to
  persist, an out-of-memory kill or a real hang is retried and, if it does not recur, reported green.
- Tests that pass only in source order are invisible: nothing runs the unit suites with
  `-shuffle`. (Golden tests such as `TestMarshalExportEnvelopeGolden` already compare against a
  fixed file and `encoding/json` sorts map keys, so repeating them adds little; an earlier draft
  proposed `-count=2` on them and it was dropped.)
- The README quick start (`README.md:106`) is a claim to users. `hack/test/demo_task_aliases_test.sh`
  checks that `demo-up` and `demo-down` exist and delegate, and `hack/test/demo_03_hero_smoke_test.sh`
  plus the `hero-demo-smoke` job exercise the path, but nothing ties the README's other commands
  (the `kubectl get` short names, `kubectl apply -k` path, `task dev-up`) to what exists.

Mutation measurement is a separate change, `mutation-testing-signal`.

## What Changes

- TDS-1: the fuzz retry moves to `hack/ci/fuzz-retry.sh`, which retries only when the output is
  the known Go engine deadline artifact (golang/go#75804) and nothing else.
- TDS-2: `task test:shuffle` runs the unit packages with `-shuffle=on -count=3`, printing the seed;
  CI runs it nightly, not per PR (it triples unit-test time).
- TDS-3: `hack/test/readme_quickstart_test.sh` checks the README quick-start commands against the
  Taskfile, the sample path and the CRD short names.

## Capabilities

### New Capabilities

- `test-depth`: refusals beyond "tests pass once, in source order".

### Modified Capabilities

None.

## Impact

- Entry points: job `fuzz` in `ci.yaml` calling `hack/ci/fuzz-retry.sh`; the nightly job
  running `task test:shuffle`; the `lint` job running the README test.

## Dependencies

Landing order across the eight proposed changes: developer-toolchain-consistency (with its Go bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation, nightly-failure-reporting, test-depth-signals, mutation-testing-signal, public-agent-contract.

## Non-goals

- Mutation testing (own change); fuzz target changes; new golden tests.
- Executing `task demo-up` (needs kind and Docker; `hero-demo-smoke` already does).
- Re-checking what `demo_task_aliases_test.sh` already asserts.

## Assumptions

- golang/go#75804 is the upstream report for the fuzz engine's "context deadline exceeded" at the
  `-fuzztime` boundary. This comes from the survey and is not verified here; task 1.1 reproduces
  the exact output from a real CI log and records it as a fixture. The real output of that case
  contains `--- FAIL: FuzzX` (every failing `go test -fuzz` prints it), so that line is not a
  discriminator.
- `-shuffle=on` prints the seed (`-test.shuffle <seed>`) at the start of the run, so a failure is
  reproducible with `-shuffle=<seed>`. Probe in task 2.1. Packages that need Docker, envtest or
  kind are excluded: the unit set is `go list ./... | grep -v '/e2e\|/integrationtest'` minus
  packages with tagged tests; task 2.1 fixes the exact list and records the exclusions.
