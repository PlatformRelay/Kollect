# Design

## Context

See proposal.md. The checker sits between `task test-integration` and `go test`, so it controls
the command line, the environment and the exit status.

## Goals / Non-Goals

**Goals:** a green required run implies that every integration-tagged test ran and passed; the
reason for any red run is named in a few lines.

**Non-Goals:** a general test-report tool, coverage, flake retries, or deciding which packages
are integration-tested (the Taskfile keeps that list).

## Decisions

- **The checker runs `go test` itself** instead of reading a piped stream. A pipe hides the
  producer's exit status unless every caller sets `pipefail`. Running the process lets the
  checker hold the status and apply IEE-3 without depending on the caller's shell.
- **Expected tests come from `go list` plus `go/parser`.** `go list -json -tags=integration`
  names the test files the build would compile. Each file's `//go:build` line is evaluated with
  and without `integration` (`go/build/constraint`), and the file's top-level `Test*` functions
  are read from the AST. The alternative, `go test -list`, needs a successful build and derives
  the list from the same binary whose results are being checked, so a build failure would empty
  both sides. Text grep was also rejected because it misreads build-constraint expressions.
- **Analysis is a pure function** of (expected set, selected packages, event stream, exit status,
  mode), so every refusal is tested on a recorded stream without Docker.
- **Results are keyed by package and test name**, never by order, because event order across
  packages varies between Go versions.
- **Untagged tests are not on an allowlist.** Not being integration-tagged is the narrow,
  reviewable rule: such a test is not expected, so its skip is reported and does not fail the
  run. Adding a skip to a tagged test, by contrast, fails required mode.

## Risks / Trade-offs

- [A tagged test that legitimately skips in CI] → none exist today. If one is ever needed, it
  is a reviewed spec change, not a silent skip.
- [Test functions generated or declared in an unusual way, for example via a var] → not
  detected. All 57 current integration tests are plain top-level functions.
- [Local developers without Docker now get a red `task test-integration`] → intentional;
  `task test-integration:explore` is the documented local path.
