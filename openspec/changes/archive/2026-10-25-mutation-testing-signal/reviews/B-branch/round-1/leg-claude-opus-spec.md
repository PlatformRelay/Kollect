## Verdict: CONCERNS

## Findings
Compliance per MUT requirement (checked against the code, not the evidence docs):
- MUT-1 workflow: **holds.** The name, the `30 1 * * *` cron, `contents: read`, `persist-credentials: false` and the five steps match the spec, and the upload takes `path:` from `steps.mutation.outputs.reports` with `error` — `.github/workflows/mutation.yaml:10-50`. W1 locks this to exact keys — `hack/test/mutation_signal_test.sh:597-660`.
- MUT-1 pin and install task: **holds.** The renovate line sits directly above `GREMLINS_VERSION: v0.6.0` — `Taskfile.yml:32-33`. The install goes to a version-named GOBIN, neither task has a skip or soften key, and W2 checks the exact key set and that cmds are plain strings, so a command-level `ignore_error` is excluded — `Taskfile.yml:214-223`, `mutation_signal_test.sh:685-692`.
- MUT-1 scope (W3): **partial.** Line scan plus deps scan — `mutation_signal_test.sh:702-800`. See the NOTE on lines split by a continuation.
- MUT-1 runner: **holds.** One call per package with an absolute `--config`, all eleven flags, a fresh `mktemp -d`, and `reports=` written before the first call — `hack/ci/mutation.sh:61-78,65-68,96-98`.
- MUT-1 formatter: **holds.** Rejects a missing, duplicate or unknown package; validates every report before printing; writes the five counts to the summary, or stdout outside Actions — `hack/ci/mutation-summary.sh:30-53,57-98,111-115`.
- MUT-1 reporter wiring: **holds** — `.github/workflows/ci-failure-report.yaml:26`. The issue is titled from the workflow name — `hack/ci/ci-failure-report.sh:211`.
- MUT-2 threshold sources: **holds.** T1 matches `--threshold[-_.]` case-insensitively. T2 lists tracked and untracked-but-not-ignored `.gremlins*` and requires the config to parse to `{}`. T3 does a text plus parsed-YAML scan. At run time, any `GREMLINS_*` variable is refused, empty or not, by `compgen -e` — `mutation_signal_test.sh:831,838-848,855-867`, `mutation.sh:37-44`.
- MUT-2 scan errors fail the guard and every failure is collected: **holds.** `to_json`, `ls-files`, the awk passes and the yq queries all route to `scan_error`, and `case_fail` counts rather than exits — `mutation_signal_test.sh:558-588,747,754-776`.
- MUT-2 lint job runs the guard plain and `--self-test`: **holds** — `.github/workflows/ci.yaml:615-618` sits in the `lint:` job at `:283`. Whether that job is a required check is not visible.
- MUT-2 "Tool broken": **holds.** Non-zero exit, no report, empty report, zero mutants, and RUNNABLE, SKIPPED or unknown statuses each fail with no table — `mutation.sh:76-77`, `mutation-summary.sh:64-85`.
- MUT-3 fixture and control: **holds.** `Sum` is executed and its result is not asserted; `Unused` is never called; the fixture has its own `go.mod`. The control must appear exactly once and be LIVED before the real packages run — `hack/test/testdata/mutation/control.go:8-15`, `control_test.go:6-9`, `mutation.sh:80-94`.
- MUT-4 three nights and forced failure: **holds** — `mutation.sh:46-50`, with R5 and R7 at `mutation_signal_test.sh:474-530`.
- **Stronger than spec:**
  - The formatter rejects a report whose `file_name` is not a file in that package's directory — `mutation-summary.sh:87-94`.
  - `MUTATION_FORCE_FAILURE` values other than `true`, `false` or empty (e.g. `TRUE`, `1`) also fail the run — `mutation.sh:49`.
  - W1 accepts only action SHAs used by other *workflows*; the spec says "elsewhere in the repository", which also covers composite actions — `mutation_signal_test.sh:646-650`.
  - None of these breaks a legitimate caller today.

- [WARNING] A Renovate bump of `GREMLINS_VERSION` can silently break "every mutator flag of the pinned version passed explicitly" — `hack/ci/mutation.sh:59-63`
  Failure: Renovate bumps `Taskfile.yml:33` to a version that adds a twelfth mutator, off by default. Every guard stays green; the self-test's "bump" control even proves a v99 bump passes (`mutation_signal_test.sh:1045-1050`). The new mutator is never enabled, and nothing reports it. Only a removed flag would turn the run red. `renovate.json` has no gremlins-specific rule, so this relies entirely on the reviewer reading the comment "a bump re-checks this list".
  Fix: fail the nightly when `"${GREMLINS}" unleash --help` lists a boolean mutator flag that is not in `FLAGS`. Cheaper alternative: put a Renovate `prBodyNotes` on `depName=github.com/go-gremlins/gremlins` naming the flag list.
  Confidence: 65
- [NOTE] Forced failure and the `GREMLINS_` refusal exit before `reports=` is written, so the upload step gets `path: ''` and adds a second error — `hack/ci/mutation.sh:43-50,65-68`
  Failure: with `MUTATION_FORCE_FAILURE=true` the run is red twice: the forced error, then upload-artifact's "path required". The spec ("no effect other than failing the run") still holds, but the failure report can quote the wrong step.
  Fix: create the directory and write `reports=` before the refusal checks, or add `if: steps.mutation.outputs.reports != ''` to the upload step.
  Confidence: 70
- [NOTE] W3 does not catch `task mutation` split across lines — `hack/test/mutation_signal_test.sh:770`
  Failure: a step written as `run: |` / `task \` / `  mutation` (or `run: >-` / `task` / `mutation`) in another workflow reaches the mutation task. Each line is checked on its own, so neither line matches. A split `gremlins … unleash` is still caught by the `unleash` rule at `:762`.
  Fix: join backslash-continued lines before the scan, and scan the parsed `run:` strings from `wf.json`-style yq output.
  Confidence: 55
- [NOTE] `hack/test/testdata/mutation/reports/warm-cache-pathvalidate.json` is recorded but no case reads it. The cached-test failure (finding F1, every mutant timed out) is guarded only indirectly, by `GOFLAGS` in R1 and by the control check.
  Fix: add an R4 variant serving it as `fixture.json`, or drop the file.
  Confidence: 80

## Could not check
- I did not run any guard or its self-test. The `ok`/`FAIL` outcome of `mutation_signal_test.sh` and `--self-test` on this tree is unverified, and I did not read the recorded results in `evidence/G2.md`.
- Two claims about gremlins v0.6.0 itself are unverified: that `--config=<abs>` stops it searching other config locations, and that the eleven-flag list is complete. No binary or source was read.
- Whether `lint` is a required status check in branch protection.
- That `actions/upload-artifact@043fb46d…` and `setup-task` 3.52.0 appear in another workflow on this tree. W1 asserts it, but I did not run W1.
- The earlier review records under `openspec/changes/mutation-testing-signal/reviews/` (I deliberately didn't read them).
