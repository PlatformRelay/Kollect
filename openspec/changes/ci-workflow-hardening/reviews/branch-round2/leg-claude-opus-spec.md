## Verdict: CONCERNS

## Findings
- [WARNING] CWS-6 is only partly met. The meta-test sees direct `hack/test/` invocations in `run:` bodies only, so guards that CI runs indirectly are never checked. — `hack/test/ci_workflow_security_test.sh:361-414`
  Failure: `docs.yaml:134` runs `task docs:verify`, which calls `hack/docs/verify.sh:12-40`. That script runs `docs_launch_truth_test.sh`, `docs_adr_kollectsink_retcon_test.sh`, `docs_coverage_floor_drift_test.sh`, `security_architecture_docs_test.sh`, `docs_removed_api_fields_test.sh`, `docs_map_wiring_test.sh`, `ui_removal_reference_test.sh`, `hyg_ui_gitignore_test.sh`, `docs_pages_concurrency_test.sh`, `docs_lab_*` (×5) and `lab_adr_0707_indexed_test.sh`. It runs them from a job that is path-filtered and not required, and no step of `lint` runs any of them. So CI runs these guards, but they cannot block a merge, and the meta-test still prints "every hack/test guard CI runs … is pinned" (`:489`).
  Fix: Resolve one level of `task <x>` / `hack/docs/verify.sh` into the invocation table, or add the missing guards as `lint` steps. At minimum, narrow the spec and the pass message to "directly invoked".
  Confidence: 80

- [WARNING] CWS-2's bare-suppression check only recognises block-list items (`- x`). — `hack/test/ci_workflow_security_test.sh:221-231`
  Failure: `rules:\n  cache-poisoning:\n    ignore: [release.yaml]` (a flow sequence) has no `- ` line, so it passes with no comment. The same applies to a per-rule `disable: true` if zizmor 1.30 supports it. I believe it does but have not checked. Either form silences a finding with no justification.
  Fix: Parse with yq and treat every scalar under `rules.*.ignore` (any style), plus any `disable`, as a suppression that needs the comment check.
  Confidence: 75

- [NOTE] The CWS-6 required-job match uses the job's display name across every workflow. It does not check the workflow, the trigger or step-level `if:`. — `hack/test/ci_workflow_security_test.sh:403,474-478`
  Failure: a job named `lint` in `e2e-nightly.yaml` (schedule only) would count as a required home, as would a guard step with `if: false` in `test`. Neither breaks the rule "invoked on pull_request". I checked and no such name collision exists today.
  Fix: Key rows on (workflow file, job id), limit them to files whose `on` has `pull_request`, and reject a step-level `if` on guard steps.
  Confidence: 60

- [NOTE] CWS-6 says "by its own step", but the `sonar_ko_*` and `dist_*` loops run many guards in one step. The meta-test accepts this. — `.github/workflows/ci.yaml:446-470`
  Failure: this is not a broken gate, because default bash `-e` still fails the step. It is a literal departure from the spec text.
  Fix: Relax the spec wording to "a step", or split the loops.
  Confidence: 55

- [NOTE] CWS-3's `# why:` check goes beyond the spec in one direction and falls short in the other. — `hack/test/ci_workflow_security_test.sh:268-277`
  Failure: an explicit `fail-on-severity: low` (the default) still needs a why-line, which is stricter than the spec. Meanwhile any `# why:` text passes, even without "a measured trial", which is looser. Future `allow-dependencies-licenses` entries are never checked for the reason the spec requires.
  Fix: Fire only when the value is above `low`, and add a comment check on `allow-dependencies-licenses`.
  Confidence: 70

- [NOTE] The CWS-1 "version pin removed" mutant is caught by the env-key allowlist ("is not ZIZMOR_VERSION"). It never reaches `cws1_version_pin`'s own messages. — `hack/test/ci_workflow_security_test.sh:670-673`
  Failure: if `cws1_version_pin` regressed, CWS-7 would not notice. The case where the installer default drifts from the version in `ci.yaml` (`:191`) has no mutant at all.
  Fix: Add a mutant that edits the installer's `ZIZMOR_VERSION:-` default, with "version drift" as the sentinel.
  Confidence: 70

- [NOTE] The change does several things no requirement mentions:
  - It widens `WORKFLOW_KEY_ALLOWLIST` with `concurrency` (`hack/test/dist_ci_wiring_test.sh:106`). This widens an allow-list. It is covered only because CWS-4 pins the exact expression.
  - It changes credential handling for the changelog-sync push to main: `persist-credentials: false` and a `GIT_CONFIG_*` extraheader (`changelog-sync.yaml:92,128-137`). No test covers this change.
  - It sets `cache: false` on `release.yaml` and moves inputs into env bindings in `kind-e2e-setup`.
  - It adds 4 guard steps to `lint`.
  - It commits a 0-byte review leg (`reviews/branch/leg-DeepSeek-V4.1-Flash-adversarial.md`).

**Requirement status, checked against the code:**
- CWS-1 holds: `ci.yaml:176-196` and exact-match `:179`.
- CWS-2: the comment-check rule is partial (flow-sequence gap above). The "existing findings fixed" part holds as far as I could check.
- CWS-3 holds: `ci.yaml:200-214`, and `verify-eligibility.sh:19-21` does not list it.
- CWS-4 holds: `ci.yaml:55-57`, `e2e-smoke.yaml:37-38`.
- CWS-5 holds: `ci.yaml:33-34`, `verify-eligibility.sh:21`.
- CWS-6 is partial (indirect invocations, "own step").
- CWS-7 holds: a mutant per requirement plus a no-op control, `:603-845`.

## Could not check
- I could not run anything because Bash was denied. Neither the meta-test, its `--self-test`, `ci_docs_gate_test.sh`, `dist_ci_wiring_test.sh` nor zizmor was run, so nothing here is backed by a run.
- Whether zizmor 1.30.1 rates `${{ github.event.pull_request.title }}` in `run:` as high (the CWS-1 scenario), and whether it exits non-zero on findings.
- Whether zizmor's config accepts `disable:` or flow-style `ignore`.
- That the pinned SHA256 digests and the dependency-review-action v5.0.0 commit SHA are genuine.
- That dependency-review's default leaves unknown licences non-failing.
- The live ruleset's required contexts, and the post-merge evidence for CWS-4 and CWS-6.
- Earlier round-one review records outside this range.
