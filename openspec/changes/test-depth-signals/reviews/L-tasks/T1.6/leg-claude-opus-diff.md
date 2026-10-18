## Verdict: CONCERNS

## Findings
- [WARNING] Task is ticked and marked CLOSED before the independent review it depends on has run — `openspec/changes/test-depth-signals/tasks.md:109`, `openspec/changes/test-depth-signals/evidence/T1.6.md:14`, `openspec/changes/test-depth-signals/loop.md:43`
  Failure: the evidence file says `Status: CLOSED (closed 2026-10-18)`, but its "Independent review", "Verdict" and "Learnings" sections still read "Filled after review" / "Filled at close". The loop.md row says `CLOSED` with "rounds: pending". If this review or another one returns findings, the task record already claims a verdict it never reached.
  Fix: mark the task as in review (or leave it unticked) until the review rounds are recorded, then close it.
  Confidence: 85
- [NOTE] F16 matches the step's `run:` text and its position, but nothing that controls whether the step runs or whether its failure counts — `hack/test/fuzz_retry_test.sh:340-349`
  Failure: adding `if: false`, `continue-on-error: true` or a different `working-directory:` to the download step keeps F16 green, and the cold-cache `go: downloading` lines come back into the runner's output. The runner fails closed, so the result is a real flake red-lining the build with "unrecognised failure text" rather than a false green. That is why this is a NOTE and not higher.
  Fix: in the same yq select, require `(.value.if // "") == ""`, `(.value."continue-on-error" // false) == false` and no `working-directory`. Add one mutant for one of these.
  Confidence: 60
- [NOTE] The core claim, that pre-downloading removes every `go: downloading` line from the fuzz output, is checked only by a local probe the evidence classes as agent-review. No CI run backs it — `openspec/changes/test-depth-signals/evidence/T1.6.md:32-47`
  Failure: by Go's documented behaviour (not run here), a bare `go mod download` (go ≥1.17 module graph) fetches only modules the main module requires directly. A test-only dependency missing from go.mod, or a `toolchain` line newer than what setup-go installs, could still print download lines during `go test -fuzz`. The probe used one target out of five matrix packages.
  Fix: no code change needed. Record a cold-cache CI run (or run the probe once per matrix package) before calling the question settled.
  Confidence: 40
- [NOTE] Things I checked that hold:
  - The step sits after `go-cache`, which installs Go from `go.mod`, and before the runner step (`.github/workflows/ci.yaml:821-836`).
  - The download step is separate, and F11 pins the runner's `run:` to exactly one line, so the download output really is uncaptured (`hack/test/fuzz_retry_test.sh:310-318`).
  - The go-cache `restore-keys` fall back to `${{ runner.os }}-go-` (`.github/actions/go-cache/action.yml:28-30`). A partial cache hit after a go.sum bump is therefore the common real case, and the new step covers it.
  - F16's count, presence and order branches are sound: indexes come from `to_entries`, and `wc -w` guards against more than one match.
  - The after-runner mutant inserts a valid step at the end of the job. The before-cache mutant's anchor is unique. The "red only F16" count pattern `^FAIL: F[0-9]*[a-z]*:` does not match the final summary `fail` line.
  - The `rm -rf "${COPY:?}"` cleanup and the `replace_once` loud-failure paths are correct.
  - The 1.6 line in tasks.md is now outside 1.5's wrapped text.
  Confidence: 80

## Could not check
- I did not run `fuzz_retry_test.sh` (plain or `--self-test`), `ci_workflow_security_test.sh`, zizmor or the linters. I relied on the results recorded in `evidence/T1.6.md`.
- No CI run on the PR head, so the cold-cache behaviour was not seen on a real runner.
- I did not read go.mod's `go`/`toolchain` lines, so I could not rule out a toolchain-download line in the fuzz step.
- I did not read `hack/ci/fuzz-retry.sh`, so I did not confirm from its code that `go: downloading` lines fall outside its allow-list. I took that from the test comments.
