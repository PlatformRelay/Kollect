## Verdict: CONCERNS

## Findings
- [WARNING] When the module or build cache misses, the flake is no longer retried. The old inline loop retried it. — `hack/ci/fuzz-retry.sh:67`, `hack/ci/fuzz-retry.sh:53`
  Failure: the runner merges stderr into its log (`2>&1 | tee`). The `fuzz` job has no `go mod download` step (`ci.yaml:818-824`), and `.github/actions/go-cache` only restores with `restore-keys`. After a cold or partial cache, attempt 1 opens with `go: downloading …` lines, and possibly `go: downloading go1.x` toolchain lines. `is_deadline_flake` strips only `^fuzz: ` lines and requires exactly 5 lines, so a real deadline flake on that attempt is reported as `unrecognised failure text` and the leg goes red with no retry. This fails closed, so it is not a safety hole. But the change exists to absorb this flake, and its fixtures (`hack/test/testdata/fuzz-retry/README.md`) all come from warm-cache runs. No case covers the `go:` preamble.
  Fix: before `go test`, run `go mod download` (or `go test -c -o /dev/null "${pkg}"`) as a separate step that is not captured. Alternatively, filter `^go: downloading ` lines along with the `fuzz:` lines, and add a case F16 that adds such a preamble to the deadline fixture.
  Confidence: 60

- [NOTE] Nothing tests the empty-pkg branch, and a mutant that deletes it would survive. — `hack/ci/fuzz-retry.sh:32`, `hack/test/fuzz_retry_test.sh` (F0 loop)
  Failure: F0 tries only `Fuzz.*|./internal/aggregate/...` and a single argument. Deleting `|| -z "${2}"` leaves every case and every self-test mutant green, so `fuzz-retry.sh FuzzX ""` would call `go test` with an empty package argument. The header ("a missing pkg") and evidence row 2b claim this is covered.
  Fix: add `FuzzContentHash|` (two arguments, the second empty) to the F0 list, plus the matching `"|| -z \"\${2}\""` mutant.
  Confidence: 80

- [NOTE] No case replays the real crash fixture. Every deny-marker case uses synthetic awk variants of the deadline output. — `hack/test/testdata/fuzz-retry/crash-local.txt`, evidence `G1.md` "Not replayed by a case today"
  Failure: if Go changes the wording (for example `Failing input written` changes), the synthetic variants keep passing because they embed the old literal. A replayed real crash would still be caught by the allow-list, so the effect is only a less specific diagnostic.
  Fix: one case that replays `crash-local.txt` with rc 1 and expects `deny marker panic:`.
  Confidence: 70

- [NOTE] Evidence row 17 (the runner and the new lint steps on a real ubuntu-latest runner) is `not-run`. The behaviour that matters most, real Go output on GitHub runners, is so far checked only through replayed logs. — `openspec/changes/test-depth-signals/evidence/G1.md` row 17
  Fix: close G1 only after CI on the PR head runs both the `fuzz` matrix and the `lint` steps.
  Confidence: 90

What holds up:
- **Deny markers:** they are checked before the exit status, so a passing attempt that prints `panic:` is red (`:70-75`).
- **Flake match:** it is exact, ordered and pinned to the matrix target. `target` is checked against a regex before it is put into the pattern, so it can't inject regex syntax.
- **Retry cap and logging:** retries are capped at 2 (`:63`, `:95`), the log file is overwritten on each attempt, and `PIPESTATUS[0]` reads go's exit code rather than tee's.
- **Corpus check:** it uses `--untracked-files=all` and fails closed if `git status` fails.
- **Zero-target and no-`elapsed` exits:** both are red.
- **CI wiring:** the `fuzz` step calls the runner with the matrix env (`ci.yaml:824-832`), and the runner has mode 100755. The two lint steps are present, both modes are added to the `cfc_guards` list, and the new CWS mutant deletes both steps.
- **Self-test:** every mutant is an anchored exactly-once replace and asserts its own `FAIL: <case>:` line. The evidence lists 21 mutants plus a no-op control, matching the guard code.

## Could not check
- I did not run any test or self-test. The results above are the ones recorded in `G1.md` "Sensors run", which I could not confirm independently.
- I read `ci_workflow_security_test.sh` only through the diff. I did not read the full `cws6_guards` body, so I could not confirm how it decides that a mode is "run".
- I did not open the `.github/actions/go-cache/action.yml` body past the grep, `pass-*.txt`, `zero-target.txt`, `crash-local.txt`, or the round-1 review legs.
- I could not check golang/go#75804, the source CI runs, or the raw logs held outside the repository.
