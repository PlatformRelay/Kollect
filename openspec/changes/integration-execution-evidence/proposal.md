# Proposal

## Why

`task test-integration` is a plain `go test`. A green result says no test failed. It does not
say the integration tests ran. `KOLLECT_REQUIRE_DOCKER=true` (#405) turns one skip path into a
failure: a container that cannot start. Other paths still end green. A test can skip on
`git not in PATH` or `testing.Short()`. A mistyped selection, a package dropped from the list, or a
stream cut off mid-run leaves nothing to fail. CI cannot tell "57 tests passed" from "nothing ran".

## What Changes

- Add a checker, `hack/tools/integrationevidence`. It finds the integration tests from the source,
  runs `go test -json` itself, and fails unless every one of those tests passed. It reports the
  expected, executed, passed, failed, skipped and missing counts, and names the tests involved.
- `task test-integration` (what CI runs) goes through the checker in required mode.
- Add `task test-integration:explore` for local runs without Docker. It reports what was
  unavailable, says the run is not a completed verification, and still fails on a test failure.

## Capabilities

### New Capabilities

- `integration-test-execution`: what a required integration run must prove before it reports
  success, and what the exploratory local mode may and may not claim.

### Modified Capabilities

None.

## Impact

- Entry point: `task test-integration`, run by the `test-integration` job in
  `.github/workflows/ci.yaml`. That job is advisory today, not one of the required checks. This
  change does not make it required.
- Local behaviour changes. On a machine without Docker, `task test-integration` now fails;
  `task test-integration:explore` keeps the old skip-and-report behaviour.
- Docs: `docs/development/testing.md`, `docs/COMMAND-REFERENCE.md`, `CONTRIBUTING.md`.

## Non-goals

- Banning `t.Skip`. Untagged tests compiled into the same packages keep their own skip rules (the
  envtest tests in `internal/pipeline` skip without assets). They are reported, not required.
- Making `test-integration` a required check, or changing which packages are integration-tested.
- Diagnosing the Forgejo reaper timeouts (#407 disabled Ryuk in CI).

## Assumptions

- `go test -json` reports `build-fail` and then a package `fail` for a package that does not
  compile. It reports a package `pass` with no `run` events when nothing was selected, a package
  `skip` when there are no test files, and a parent `fail` when a subtest fails. Probed on
  2026-10-04 under Go 1.26.6 (go.mod) and Go 1.27.1. Both emit the same events; only the order
  across packages differs, so the checker must not depend on that order.
- `go list -json` gives `TestGoFiles` and `XTestGoFiles` per package for the tag set it is given.
- An integration test is a top-level `func TestXxx(*testing.T)` in a `_test.go` file whose build
  constraint holds with the `integration` tag and fails without it.
