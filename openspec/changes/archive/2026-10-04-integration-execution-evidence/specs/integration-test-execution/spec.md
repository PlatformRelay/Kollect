# Spec Delta

## Purpose

Defines what a required integration-test run must prove before it reports success, so that a green
result means the promised integration tests ran and passed, and what a local exploratory run may
claim when prerequisites are missing.

## ADDED Requirements

### Requirement: IEE-1 Expected tests come from the source

The checker SHALL determine the expected integration tests from the source files of the selected
packages, independently of any test result output. A test is expected when it is a top-level
`TestXxx(*testing.T)` function in a test file whose build constraint holds with the `integration`
tag and does not hold without it.

#### Scenario: Tagged test is expected

- **WHEN** a selected package has a test file constrained by `//go:build integration` that declares `TestExport`
- **THEN** `TestExport` of that package is in the expected set

#### Scenario: Untagged and helper functions are not expected

- **WHEN** a selected package declares `TestUnit` in an untagged file, `TestMain`, a helper such as `Testable`, or a benchmark
- **THEN** none of them is in the expected set

#### Scenario: Empty expected set is refused

- **WHEN** the selected packages contain no integration-tagged tests
- **THEN** the run SHALL fail and SHALL NOT report success

#### Scenario: Tagged tests outside the selected packages

- **WHEN** a package outside the selected patterns has integration-tagged tests
- **THEN** a required run SHALL fail and name those tests, because nothing would ever run them; an exploratory run lists them and is not complete

### Requirement: IEE-2 Required mode passes only when every expected test passed

In required mode the checker SHALL report success only when the `go test` process exited zero, every
expected test reached a `pass` result, no test anywhere failed, and the result stream was complete.

#### Scenario: All expected tests pass

- **WHEN** every expected test passes and `go test` exits zero
- **THEN** the run succeeds and reports expected = executed = passed

#### Scenario: Expected test skipped

- **WHEN** an expected test skips, for any reason
- **THEN** the run SHALL fail and SHALL name the skipped test

#### Scenario: Expected test missing

- **WHEN** an expected test has no result, for example because of a mistyped selection, a build failure or a dropped package
- **THEN** the run SHALL fail and SHALL name the missing test

#### Scenario: Subtest fails

- **WHEN** any subtest fails
- **THEN** the run SHALL fail and SHALL name the failed test

#### Scenario: Every expected test skipped

- **WHEN** all expected tests skip
- **THEN** the run SHALL fail; it SHALL NOT report success because nothing failed

#### Scenario: Same test name declared twice in one package

- **WHEN** a package declares `TestX` in both its internal and its external test package, one run passes and the other skips
- **THEN** the run SHALL fail; every declaration must run and pass, and the worst result counts

#### Scenario: Skipped subtest of an expected test

- **WHEN** an expected test passes but one of its subtests skipped
- **THEN** the run SHALL fail in required mode and SHALL name the subtest

### Requirement: IEE-3 The test process exit status is never masked

The checker SHALL exit non-zero whenever the `go test` process it ran exited non-zero, whatever the
result stream says, in every mode.

#### Scenario: Non-zero exit with a clean-looking stream

- **WHEN** the result stream shows every expected test passing but `go test` exited non-zero
- **THEN** the run SHALL fail

### Requirement: IEE-4 Broken or truncated results fail the run

The checker SHALL fail the run when the result stream contains a line that is not a valid test
event, when a started package has no final result, when a started test has no final result, or
when a selected package never reports.

#### Scenario: Malformed line

- **WHEN** the stream contains a line that is not valid JSON
- **THEN** the run SHALL fail and report the stream as malformed

#### Scenario: Truncated stream

- **WHEN** the stream ends after a test's `run` event without its result, or without a package result
- **THEN** the run SHALL fail and report the stream as truncated

### Requirement: IEE-5 Required mode demands Docker

In required mode the checker SHALL run the tests with `KOLLECT_REQUIRE_DOCKER=true`, so that an
unusable container runtime fails tests instead of skipping them.

#### Scenario: Docker unavailable in required mode

- **WHEN** Docker is unavailable and the checker runs in required mode
- **THEN** the run SHALL fail

### Requirement: IEE-6 Untagged tests in the same packages are reported, not required

Tests in the selected packages that are not expected SHALL NOT be required to run, and their skips
SHALL NOT fail the run. Their failures SHALL fail the run. The report SHALL count them separately.

#### Scenario: Untagged envtest test skips

- **WHEN** an untagged test in a selected package skips because envtest assets are absent
- **THEN** the run is not failed by that skip, and the report lists it as an untagged skip

### Requirement: IEE-7 Exploratory mode never claims a completed verification

An exploratory mode SHALL let a local run without Docker finish without failing on skips. It SHALL
state that the run is not a completed integration verification whenever an expected test did not
pass. It SHALL still fail on any failed test, missing test, broken stream or non-zero `go test` exit.

#### Scenario: Local run without Docker

- **WHEN** Docker is missing and the checker runs in exploratory mode
- **THEN** it exits zero, lists the skipped tests, and states that the run is not a completed integration verification

#### Scenario: Failure in exploratory mode

- **WHEN** a test fails in exploratory mode
- **THEN** the run SHALL fail

### Requirement: IEE-8 A red run explains itself

The report SHALL show, for each failed test, the last lines of its output (where the failure
message is); for each failed build, the compiler output; and for each failed package, its name, plus
the package output when no failed test in it explains the failure. While the tests run, package
results, build output and lines that are not test events SHALL be echoed to stderr, so a cancelled
or timed-out CI job still shows progress; per-test output is left to the report.

#### Scenario: Failure message after long logs

- **WHEN** a test logs 60 lines and then fails with a message
- **THEN** the report shows that message

#### Scenario: Compile error

- **WHEN** a selected package does not compile
- **THEN** the report shows the compiler error text

#### Scenario: Package-level failure

- **WHEN** a package fails outside any test (for example TestMain panics)
- **THEN** the report names the package and shows its output

#### Scenario: Result without a run event

- **WHEN** the stream reports a test result that no run event announced
- **THEN** the run SHALL fail and report the stream as malformed
