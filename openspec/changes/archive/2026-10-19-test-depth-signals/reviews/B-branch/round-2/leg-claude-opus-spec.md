## Verdict: CONCERNS

## Findings
- [WARNING] TDS-3 partial: a column-0 fence written as ```` ``` sh ```` (a space before the info string) is an `sh` fence under CommonMark and GitHub, but the guard reads its language as empty and skips its commands without failing — `hack/test/readme_quickstart_test.sh:119`
  Failure: README line 111 changed to ```` ``` sh ```` with `task nosuch` inside. `lang = substr($0,4)` gives `" sh"`, and `sub(/[[:space:]].*/, "", lang)` turns that into `""`, so line 124 skips every line in the fence. Line 125 flags only indented or `~~~` openers, so nothing fails. The second fence (`kubectl apply -k ...`) keeps the extraction non-empty, so the check exits 0. TDS-3 forbids exactly this: fences that "hide `sh` commands from the reader".
  Fix: strip leading whitespace before cutting: `sub(/^[[:space:]]+/, "", lang)` ahead of the existing `sub`. Add a self-test case (```` ``` sh ```` + `task nosuch` → `FAIL: task`). Consider whether `bash`/`shell` fences should fail in the section (the spec allows them today).
  Confidence: 75
- [NOTE] Stale line citation in the TDS-2 scenario "Shuffle failure does not hide the nightly". The spec cites `hack/test/ci_failure_report_test.sh:1226-1233`, but this branch's own two-line insert at :426-427 moved the "both open, nightly success" case to :1228-1235 (`expect` at :1235) — `openspec/changes/test-depth-signals/specs/test-depth/spec.md:159`
  Failure: a reader following the citation lands on the end of the previous case (marker lookalike).
  Fix: cite `:1228-1235`.
  Confidence: 95
- [NOTE] Same scenario: `ci-failure-report.yaml:40-42` is now the comment block. The `concurrency:` group keyed on `workflow_id` is at :41-43, moved by the added `- Test shuffle` line 25 — `spec.md:160`, `.github/workflows/ci-failure-report.yaml:41-43`
  Fix: cite `:41-43`. (`ci-failure-report.sh:133-140`, the marker filter selecting on `$wf`, is still correct.)
  Confidence: 90
- [NOTE] TDS-2 stronger than spec: `SHUFFLE_SEED` must be 1–18 unsigned digits, so a negative or 19-digit int64 that Go's `-shuffle` accepts is refused. Harmless because the workflow never sets the seed — `hack/ci/test-shuffle.sh:39`
  Confidence: 90
- [NOTE] TDS-3 stronger than spec (false red, fail closed): a Taskfile `aliases:` entry, a `flatten: true` include, or a value flag such as `task -d dir x` makes a valid name fail. Nothing in today's README hits this — `readme_quickstart_test.sh:57-65,156-161`
  Confidence: 80

Per requirement, checked against the code:
- **TDS-1 holds.**
  - Exact argv: `fuzz-retry.sh:66`.
  - Deny markers run before the exit status, so an exit-0 attempt is checked too: `:70-75`.
  - Retry only on rc 1 (round-1 A3 closed): `:90`, with F17 testing 2 and 137 and a mutant at `fuzz_retry_test.sh:492`.
  - Exact ordered 5-line flake pinned to the target: `:43-58`.
  - `git status --untracked-files=all` fails closed: `:93`.
  - Cap of 2 attempts with a warning: `:63,99-103`.
  - Zero-target and no-elapsed cases fail: `:77-86`.
- **TDS-2 holds.**
  - Seed printed before make or any test: `test-shuffle.sh:44`.
  - Ginkgo found by grep for `RunSpecs(` and run as 3× `-count=1`; plain packages run once at `-count=3`: `:63,74-80`.
  - Exclusions need a reason and must be e2e: `:32-36`.
  - Envtest is provisioned the same way as `coverage.sh:23-30`, and empty assets fail: `:47-51`.
  - Workflow: schedule only, no seed set; reporter `if:` is schedule-only: `ci-failure-report.yaml:34`.
  - Round-1 A2 (S11 `:458-461`, F11 `:360-368`, F16 `:390`) and A4 (S13 `:350-361` plus 2 mutants) are closed.
- **TDS-3 partial**, on the WARNING above.
  - Round-1 A1 is closed: every TYPE/NAME operand is checked and a plain/slashed mix fails (`:188-201`, X15–X18).
  - The O1 amendment (column-0 backtick fences only; unclosed or `~~~` fences fail) matches `:118-127`.
  - Internal tasks and includes are excluded: `:58-63,85`.
  - No delegation checks are duplicated.

Not covered by any requirement:
- `workflow_dispatch` on `test-shuffle.yaml`: manual runs are never reported.
- The `go mod download` step in the fuzz job.
- The usage-error exit 2 in `fuzz-retry.sh`.
- The exec-bit and job-`if:` pins in F11.
- CFC-6 wiring in `ci_workflow_security_test.sh`.
- The two counter-increment test fixes in `internal/collect` and `internal/metrics`.

## Could not check
- I ran no sensors. Self-test results come from `evidence/G5.md`, which records them as exit 0, including X15–X18, F17 and the 12 `if:`/`continue-on-error:` mutants.
- I did not confirm that `go list ./...` returns no e2e package. There is no `test/e2e/*.go`; the Makefile still filters `/e2e`.
- I did not confirm the captured fixtures in `hack/test/testdata/fuzz-retry/` against the CI runs they cite.
- I did not confirm `task test:shuffle`'s working directory under the include (no `dir:`); round 1 records it green.
