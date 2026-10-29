## Verdict: BLOCK

## Findings
- [CRITICAL] `ci.yaml` now has a step with only a `name:` and no `run:` or `uses:`. That makes the workflow invalid, so on every PR and every push to main no CI job starts at all: lint, workflow-security and the release-eligibility checks never report. — `.github/workflows/ci.yaml:446`
  Failure: line 446 `- name: Run Sonar SECURITY remediation meta-tests` is followed by a second step with the same name. GitHub rejects a step that has neither `run` nor `uses` when it loads the workflow. None of the new gates catches this: `cws6_guards` reads the empty `run` as "" and moves on (`hack/test/ci_workflow_security_test.sh:351`), and the step key allowlist accepts a step made of `name` alone.
  Fix: delete line 446. Then add a check to the meta-test (or actionlint) that every step has `run` or `uses`.
  Confidence: 85
- [WARNING] CWS-1 skip switch is only partly covered: the step allowlist accepts `shell:` and `env:` on workflow-security steps. — `hack/test/ci_workflow_security_test.sh:137`
  Failure: `shell: "true {0}"` on the audit step means zizmor never runs and the job passes. `env: KOLLECT_FORCE_SHA256: <any>` on the install step overrides the pinned digest (`hack/install-zizmor.sh:50`). The meta-test catches neither.
  Fix: drop `shell` from that allowlist. Fail if any workflow-security step's env contains anything other than `ZIZMOR_VERSION`.
  Confidence: 80
- [WARNING] CWS-2 is stricter than the spec: a properly justified suppression written over several lines is rejected. — `hack/test/ci_workflow_security_test.sh:203-208`
  Failure: `# why…` / `cache-poisoning:` / `ignore:` / `- release.yaml`. `prev` is cleared after the first key line, so the check fails on `ignore:` even though the suppression has its comment. The comment would also have to sit above every line of the entry, not just the first.
  Fix: require a comment above each top-level rule key (two-space indent) only, not above every line beneath it.
  Confidence: 85
- [WARNING] CWS-3 "threshold loosened" only looks at `fail-on-severity`. `warn-only: true` and a job-level `continue-on-error` both silence dependency-review without being caught. — `hack/test/ci_workflow_security_test.sh:226-237`
  Failure: adding `warn-only: true` under `with:` passes the meta-test, and dependency-review never fails a PR.
  Fix: allowlist the `with` keys (`allow-licenses`, `allow-dependencies-licenses`, plus `fail-on-severity` with a `# why:` line), and allowlist the job keys the same way `cws1_job` does.
  Confidence: 75
- [WARNING] The `# why:` check takes the first `fail-on-severity` text anywhere in `ci.yaml`, comments included. — `hack/test/ci_workflow_security_test.sh:233`
  Failure: a comment mentioning `fail-on-severity` with `# why:` on the line above it satisfies the check for an unrelated real setting further down.
  Fix: grep for `^[[:space:]]+fail-on-severity:` inside the dependency-review job.
  Confidence: 60
- [WARNING] CWS-6 is partial: it only scans `.jobs[].steps[].run`. Guards reached through composite actions (they are skipped at :340), reusable workflows, `task` targets or wrapper scripts (`lab_harness_meta_suite.sh`, `hack/docs/verify.sh`) are invisible to it. Mode detection also knows only `--self-test`, while the spec says "and any other". — `hack/test/ci_workflow_security_test.sh:310-315,338-362`
  Failure: a guard invoked only from `.github/actions/*/action.yml` inside a nightly job would never be flagged.
  Fix: scan `runs.steps[].run` of composite actions and attribute the hits to every job that uses the action, or state these limits in the spec.
  Confidence: 70
- [NOTE] CWS-1, CWS-4, CWS-5 and CWS-7 hold as written. The invocation is pinned exactly (:164), both workflows declare the exact concurrency (:256-266, `e2e-smoke.yaml:37-38`), the push-to-main check names `verify-eligibility.sh` (:275-280), and there is one mutant per requirement plus a no-op control (:515-709). CWS-3 "not a release check" holds: `verify-eligibility.sh:21`.
- [NOTE] Things no requirement mentions:
  - The `changelog-sync` push now authenticates with a `GIT_CONFIG_*` extraheader (`changelog-sync.yaml:132-137`). It uses plain `base64`, which wraps at 76 columns, so a longer App token would put a newline inside the header. Use `base64 -w0`. (Confidence 50.)
  - `release.yaml` sets `cache: false`.
  - The `kind-e2e-setup` action now binds its inputs through `env`.
  - Four e2e contract guards were moved into lint.
- [NOTE] `WORKFLOW_KEY_ALLOWLIST` in `dist_ci_wiring_test.sh:106` gained `concurrency`. This widens an allowlist. CWS-4 requires it and the meta-test pins the exact value, but it is still a widening.

## Could not check
- I ran nothing: Bash was denied, so I did not run the meta-test, its `--self-test`, zizmor or actionlint. Everything above comes from reading the files.
- I did not verify that GitHub actually rejects the workflow at `ci.yaml:446`. That is from my knowledge of the Actions schema, not a run.
- I did not check whether zizmor 1.30.1 would parse that step without error. The evidence's "exit 0" claim may predate the duplicate.
- I did not check what zizmor's "27 ignored" covers.
- I did not check the dependency-review-action v5 defaults (default severity, how unknown licences are handled).
- The live GitHub ruleset and the post-merge evidence for CWS-4 and CWS-6 are outside the repo.
