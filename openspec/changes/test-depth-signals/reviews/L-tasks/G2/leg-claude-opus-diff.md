## Verdict: CLEAN

## Findings
- [NOTE] S12's reporter check uses yq `contains`, which matches substrings inside array elements — `hack/test/test_shuffle_test.sh` (S12, `contains(["Test shuffle"])`)
  Failure: if the reporter lists `"Test shuffle (old)"`, S12 still passes. The exact-list assertion in `hack/test/ci_failure_report_test.sh:1552` catches it, so this is cosmetic in S12 only.
  Fix: `any_c(. == "Test shuffle")`, or compare the exact list the way `ci_failure_report_test.sh` does.
  Confidence: 70
- [NOTE] The exclusion rule's e2e regex is narrower than the Makefile's e2e filter — `hack/ci/test-shuffle.sh:36` vs `Makefile:70`
  Failure: the regex `(^|/)e2e(/|$)` rejects excluding something like `.../test/e2e-kind`, but `make test` (`grep -v /e2e`) silently drops that package. If such a package were ever added, it would run in the nightly and could not be excluded. No such package exists today (G2 evidence: 0 e2e packages), so this is latent.
  Fix: none needed now. Record the divergence next to `EXCLUDE` if a sibling e2e package appears.
  Confidence: 40
- [NOTE] If `make -s echo-kubebuilder-assets` fails, the script exits through `set -e` with no `::error::test-shuffle:` line — `hack/ci/test-shuffle.sh:48`
  Failure: when make exits non-zero, there is no annotation and the "envtest assets missing" message is skipped. The job is still red. Only the diagnostic is weaker.
  Fix: `KUBEBUILDER_ASSETS="$(make -s echo-kubebuilder-assets)" || die "..."`.
  Confidence: 60
- [NOTE] The required `lint` job gains about 87 s for `--self-test`, according to the G2 sensor table. Each mutant re-runs S1–S3 against the real `go test` (S3 can run up to 20 times). This is a cost, not a defect. A cheaper option is to skip S3's loop in mutants that target only S11/S12, which are YAML-only.
  Confidence: 80

What I checked and found correct:
- **Seed handling:**
  - `SHUFFLE_SEED` accepts at most 18 digits, which fits int64.
  - `10#` strips leading zeros.
  - The `od -tu4` seed's trailing newline is removed by the command substitution.
  - The seed line prints before `make setup-envtest`, and setup's newline-less stderr is closed with `echo >&2`.
- **Ginkgo split** (`RunSpecs(` searched in `_test.go`; a directory with no test files is classed as plain):
  - Three `-count=1` calls for Ginkgo packages, one `-count=3` call for the rest.
  - `rc` is collected across all calls, so a plain-package failure doesn't skip the Ginkgo runs.
  - The final `die` names the seed.
  - The Ginkgo `-count` refusal and the per-binary runtimes (well under go test's 10-minute default timeout) are in the G2 evidence.
- **Fixture:** it is red only under order (S2 control). The seed-2 precondition is re-checked at runtime, not just assumed.
- **Guard design:** fake `go`/`make` on PATH, the exact-once `replace_once`, and the self-test asserting `FAIL: <id>:` per mutant. The S4 regex covers go's `-test.shuffle` first line. In S3 the printed seed is cross-checked against the seed go actually used.
- **Workflow:**
  - Contents read-only, SHA-pinned actions, `persist-credentials: false`, no `SHUFFLE_SEED`.
  - The trigger set is pinned (S11 key-difference check).
  - The reporter's `if:` limits reports to scheduled runs of this repository (`ci-failure-report.yaml:34`), as the spec requires.
- **Lint wiring:** both modes run in the `lint` job. `cfc_guards` gains the guard, and its CWS-7 mutant uses `yq del` and asserts the step is really gone.
- **Spec match:** the change matches TDS-2 (spec.md:84-121) clause by clause, and the commit message matches the diff.

## Could not check
- I did not run any sensor. Pass results (check, the 27-mutant self-test, CFR/CWS self-tests, zizmor, the real 383 s `task test:shuffle`) come from `evidence/G2.md` and were not reproduced.
- I did not read the root `Taskfile.yml` include stanza (`dir:` for the `test:` namespace) or `.github/actions/go-cache`. I assumed both behave as for the existing `test:` tasks.
- I did not verify that the `lint` job in `ci.yaml` installs go and yq before the new steps; I relied on the in-file comment and the existing `fuzz_retry_test.sh` steps.
- The first scheduled run and the reporter firing on it are post-merge only (the evidence marks them that way).
