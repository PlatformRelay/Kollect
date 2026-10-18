## Verdict: CONCERNS

## Findings
Compliance per requirement, checked against the code:
- **TDS-1: holds.** `hack/ci/fuzz-retry.sh:66` runs the exact argv. Retry is capped at one (`:63`, `:96`). The flake is an exact ordered 5-line match with `fuzz: ` lines stripped (`:43-58`). Deny markers are checked first with the spec's diagnostic (`:70-75`). `git status --porcelain --untracked-files=all` fails closed (`:90`). The two "fuzzed nothing" branches are at `:77-86`, and the warning names the flake at `:100`. All nine scenarios have a case at F1–F15 in `hack/test/fuzz_retry_test.sh`, and `ci.yaml` calls the runner.
- **TDS-2: holds.** The seed is printed before envtest or any test (`hack/ci/test-shuffle.sh:44`). Ginkgo packages are found by grep and run as three `-count=1` invocations; the rest run at `-count=3` (`:62`, `:72-80`). Envtest is set up as `coverage.sh:23-30` does and fails when empty (`:46-50`). Packages come only from `go list ./...` (`:52`). Each exclusion needs a reason and an e2e path (`:32-36`), and the guard test checks both (S5/S6). The workflow has its own `schedule:`, sets no `SHUFFLE_SEED`, and is listed in `ci-failure-report.yaml:25`, whose `if:` at `:34` limits reports to scheduled runs.
- **TDS-3: holds.** It reads the section starting `## Quick start`, `sh` fences only, with `#` comments split into their own segments (`readme_quickstart_test.sh:105-115`, `:137`). It checks tasks including namespaced ones (`:50-78`, `:142-152`), `apply -k` paths (`:179-181`) and `get` names split on commas against CRD plural/singular/shortNames (`:81-94`, `:171-178`). An empty extraction fails (`:131`). Self-tests R1–R10 cover each scenario. Nothing duplicates `demo_task_aliases_test.sh`.

Stricter than the spec, or not in it:
- [WARNING] TDS-3 rejects a `~~~` fence or an indented fence anywhere in the section, whatever its language. The spec says fences of another language are ignored and the "Commands outside the section" scenario must pass — `hack/test/readme_quickstart_test.sh:114`
  Failure: a contributor adds a `~~~yaml` sample, or a fenced block indented under a list item, inside Quick start → `FAIL: extract`, although the spec says that fence is not read.
  Fix: either amend TDS-3 to say the section allows only column-0 backtick fences (and why), or emit UNSUPPORTED only when the opener's language is `sh`.
  Confidence: 80
- [NOTE] A non-`sh` fence left open into the next section also fails, as X10 tests; the spec says nothing about it — `hack/test/readme_quickstart_test.sh:111`
  Failure: a `text` fence missing its closing line → red, which the spec does not ask for (arguably right, since unclosed fences hide content).
  Fix: add the rule to TDS-3's fail list so spec and code agree.
  Confidence: 70
- [NOTE] A flag before the kubectl verb fails (X9). This is an extra failure mode the spec does not list — `hack/test/readme_quickstart_test.sh:158-161`
  Failure: `kubectl --context kind-kollect get kinv` in Quick start → red, though every name in it exists.
  Fix: record it in TDS-3, or skip known global flags together with their value.
  Confidence: 75
- [NOTE] Several false reds are possible on valid commands; each fails closed. A word after `task x --` is checked as a task name (`:144-149`). Task `aliases:` and includes with `flatten:` or `dir:` are not understood (`:53-77`). None of these appears in today's tree.
  Failure: `task demo-up -- --foo` → "'--foo'" is skipped, but `task demo-up -- bar` → "'bar' is not a task".
  Fix: stop reading task names at `--`.
  Confidence: 60
- [NOTE] `SHUFFLE_SEED` must be 1–18 digits with no sign. The spec only says "a decimal integer", and Go accepts any int64 — `hack/ci/test-shuffle.sh:39`
  Failure: rerunning with a negative or 19-digit seed from some other source is refused. Seeds this script picks are 4-byte unsigned (`:42`), so its own nightlies are unaffected.
  Fix: allow `^-?[0-9]{1,19}$` or note the limit in TDS-2.
  Confidence: 55
- [NOTE] The fuzz runner rejects a target not matching `^Fuzz[A-Za-z0-9_]*$` with exit 2; the spec does not mention this. The flake's `FAIL\t<import path>` line is matched by shape only, not against `<pkg>` — `hack/ci/fuzz-retry.sh:32`, `:49`
  Failure: none with today's matrix. This is extra input validation and a looser match than the wording suggests.
  Fix: none needed; optionally mention it in TDS-1.
  Confidence: 50
- [NOTE] Not in any requirement:
  - `test-shuffle.yaml` also has `workflow_dispatch`. It is harmless, because the reporter ignores non-schedule runs.
  - A separate `go mod download` step was added to the fuzz job (`ci.yaml`, F16).
  - Two Go tests now assert counter increments rather than totals (`internal/collect/metrics_snapshot_test.go:89-107`, `internal/metrics/metrics_test.go:70-76`). These are needed to make `-count=3` pass and are behaviour-neutral.
  - The CWS-6/CFC-6 list now also names the three new tests (`hack/test/ci_workflow_security_test.sh:608-611`).
  Confidence: 90

## Could not check
- I ran nothing (read-only leg) and did not read any recorded results from the tests or their self-test modes, so the claim that the fuzz fixtures match real runs 37911142180 and 37288739312 is unverified.
- I did not read all of `fuzz_retry_test.sh` or `test_shuffle_test.sh`. I relied on their case headers and greps, plus the guard-test lines for the exclusion list.
- I did not confirm that `go list ./...` really lists no e2e package, or that the TDS-2 scenario test fixture (`hack/test/testdata/shuffle/orderdep_test.go`) fails on some seed.
- The TDS-2 "two separate issues" scenario rests on code outside this diff (`ci-failure-report.sh:133-140`, test lines 1226-1233). I did not re-read those.
