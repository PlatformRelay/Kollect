## Verdict: CONCERNS

## Findings
- [WARNING] CWS-3 can be bypassed at step level: the meta-test allow-lists only the dependency-review job's keys, not its steps' keys or `with:` inputs — `hack/test/ci_workflow_security_test.sh:277-305`
  Failure: two mutations pass the meta-test. (a) Adding `continue-on-error: true` to the `dependency-review-action` step keeps the job green on a vulnerable dependency, which breaks the CWS-3 scenario "PR adds a vulnerable dependency → SHALL fail". (b) Adding `with: config-file: .github/dr.yml` lets that file set `fail-on-severity: critical`, `warn-only: true` or `deny-licenses`. That defeats the "Threshold loosened" and "Deprecated input" scenarios, which only inspect inline `with` keys (`:287-305`).
  Fix: give the dependency-review steps the same step-key allowlist CWS-1 uses (`:165-169`), and turn the `with:` check into an allowlist (`allow-licenses`, plus `fail-on-severity` with `# why:`, plus `allow-dependencies-licenses`) so `config-file` is rejected.
  Confidence: 85
- [WARNING] CWS-6 misses guards that CI runs indirectly. Only literal `hack/test/...` tokens in workflow `run:` bodies count, so a guard reached through `hack/docs/verify.sh` or a `task` target is invisible — `hack/test/ci_workflow_security_test.sh:435-444`, `hack/docs/verify.sh:12-40`
  Failure: a new guard added only to `hack/docs/verify.sh` runs only in the non-required Docs job, and the meta-test still passes. This is the exact defect the branch fixed by hand at `.github/workflows/ci.yaml:450-486`. Today `dist_adr_0708/0709` and `dist_install_docs_test` are covered only because the `dist_*_test.sh` glob happens to match them (`ci.yaml:502`).
  Fix: also scan `hack/docs/verify.sh` (and any script a CI step calls) for `hack/test/` tokens and treat them as non-required rows; or assert that every `hack/test/` token in `verify.sh` has a required-job row.
  Confidence: 80
- [WARNING] CWS-6 "invoked … by its own step" is contradicted. The new docs-side step runs 15 guards in one loop, and the meta-test never checks one step per guard — `.github/workflows/ci.yaml:458-486`, `hack/test/ci_workflow_security_test.sh:435-445`
  Failure: the spec wording is not met. The pre-existing `sonar_ko_*` and `dist_*` glob steps (`ci.yaml:490,502`) have the same shape. Under `bash -e` the first failing guard stops the loop and hides the guards after it.
  Fix: either amend the spec to "by a step" (the cheaper option, and the grouping is reasonable), or split the loop into separate steps.
  Confidence: 75
- [WARNING] CWS-6 "in every mode (… and any other)" holds only partially. `guard_modes` recognises `--self-test` alone, and the code comment says so — `hack/test/ci_workflow_security_test.sh:376-389`
  Failure: a guard with a `--check-history` mode that CI runs only from a nightly job is never flagged.
  Fix: record this as a spec deviation in the change, or narrow the spec text to `--self-test`.
  Confidence: 90
- [NOTE] CWS-2 checks a different granularity from the one the config documents. `.github/zizmor.yml:4-6` requires a comment above each ignore list *item*, but the meta-test checks only the `head_comment` of each *rule key* (`ci_workflow_security_test.sh:262-267`).
  Failure: one comment above `cache-poisoning:` justifies any number of `ignore:` entries, and a failure names the rule rather than the entry, contrary to the spec's "name the entry". `:251` also claims a "4-space rule key" mutant exists; none does.
  Fix: check `head_comment` per `.rules[k].ignore[]` item, or reword the config header. Add the missing mutant.
  Confidence: 80
- [NOTE] CWS-3 checks only that a `# why:` line exists, not that it "cites a measured trial". There is also no check that future `allow-dependencies-licenses` entries carry a reason (there are none today) — `ci_workflow_security_test.sh:296-305`
  Confidence: 70
- [NOTE] Lines 143-144 and 145-146 are an identical duplicated assertion (dead code) — `hack/test/ci_workflow_security_test.sh:143-146`
  Confidence: 99
- [NOTE] Changes outside the spec: a `markdownlint` ignore widened to `openspec/changes/**/reviews/**` (`hack/tooling/markdownlint-cli2.yaml:53`) and `WORKFLOW_KEY_ALLOWLIST` widened with `concurrency` (`hack/test/dist_ci_wiring_test.sh:106`; this one is needed for CWS-4 and is locked by the exact-expression check). Also not in the spec, but sound fixes: `changelog-sync` token scoping and not persisting the token in `.git/config` (`changelog-sync.yaml:79-137`), `cache: false` in `release.yaml`, moving composite-action inputs into `env`, and `fetch-depth: 0` on lint.
  Confidence: 95

Requirements that hold, checked against the code:
- **CWS-1:** exact invocation `ci.yaml:443`, checked at `:208`. The job and step keys and env are allow-listed (`:127-176`). Workflow-level `env`/`defaults` are blocked by `dist_ci_wiring_test.sh:212`.
- **CWS-4:** both files use the exact expressions (`ci.yaml:55-57`, `e2e-smoke.yaml:37-38`, checked at `:326-333`), and the two workflow names differ.
- **CWS-5:** checked at `:339-356`, and `verify-eligibility.sh:21` includes `workflow-security`.
- **CWS-3 "not a release check":** checked at `:312-317`.
- **CWS-7:** a no-op control plus at least one mutant for each of CWS-1 to CWS-6 (`:647-939`), including the "removing `--offline`" mutant.

## Could not check
- I did not run the meta-test, its `--self-test`, zizmor or any CI job. Bash was denied, so every verdict comes from reading the code. "All checks green" is the orchestrator's claim, not something I confirmed.
- I could not confirm that zizmor 1.30.1 rates `${{ github.event.pull_request.title }}` in a `run:` step at `--min-severity=high` (CWS-1 "New unsafe pattern"). No test in the repo exercises zizmor itself.
- I could not confirm from upstream docs that `dependency-review-action` v5.0.0 reports unknown licences without failing, nor that `permission-contents` is the correct input name for the pinned `create-github-app-token`.
- I did not check the GitHub ruleset (which contexts are actually required) or whether the dependency graph is enabled on the repository. Both are post-merge operator evidence.
- I did not read `evidence.md`, `tasks.md` or the earlier review records (the target lists them; I ignored them as claims).
