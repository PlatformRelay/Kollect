## Verdict: CONCERNS

## Findings
- [WARNING] CWS-2: the bare-suppression parser can be bypassed in three ways — `hack/test/ci_workflow_security_test.sh:225-243`
  Failure: (a) a flow-style `rules: {cache-poisoning: {ignore: [release.yaml]}}` is not `{}`, so the parser sets `in_rules=1` and then sees no more lines. The bare suppression passes. (b) Rule keys indented by 4 spaces never match `^[[:space:]]{2}[a-zA-Z…]`, so they are never checked. (c) `prev` is only reset when a rule key is seen, so a comment inside rule A's `ignore:` list counts as the justification for a bare rule B that follows it.
  Fix: parse with `yq '.rules | keys'` and use `yq '... | head_comment'` to require a comment on each rule (or each `ignore` item). Reject flow style outright.
  Confidence: 85
- [WARNING] CWS-1 "removed/skip": nothing pins the job's display name, which is the status context — `hack/test/ci_workflow_security_test.sh:124-131`
  Failure: rename `jobs.workflow-security.name` to something else and add a decoy job `x: {name: workflow-security, runs-on: ubuntu-latest, steps: [{run: "true"}]}`. Every CWS-1 check still passes, because they key on the YAML job id. The ruleset and `verify-eligibility.sh` then see a green `workflow-security` that zizmor never produced.
  Fix: assert `.jobs["workflow-security"].name == "workflow-security"`, and assert that no other job in any workflow uses that name. Add a mutant for each.
  Confidence: 75
- [WARNING] CWS-6: the "required set" is taken from the release-eligibility list (13 names), not from the merge-required contexts — `hack/test/ci_workflow_security_test.sh:485-494`
  Failure: `ci_docs_gate_test.sh:17` documents four PR-required contexts. Under this test, a guard placed in `helm`, `verify`, `build` or `audit-rbac` counts as "pinned by a required job" even though those contexts may not block a merge. That defeats the "a guard that cannot block a merge is not a gate" rule the spec states.
  Fix: declare the merge-required set explicitly in the test: the spec's four, plus the existing ruleset contexts. Do not borrow `required_checks`.
  Confidence: 65 (depends on the live ruleset, which I could not read)
- [NOTE] CWS-6 "required set shrinks" can never fire for `dependency-review`, because the name is hardcoded into `set_names` before the membership check — `hack/test/ci_workflow_security_test.sh:346,349`
  Failure: the check on `dependency-review` is tautological. Removing the job is caught only by CWS-3.
  Fix: either drop it from the loop or take it from the parsed ci.yaml jobs.
  Confidence: 90
- [NOTE] CWS-6 "its own step" is contradicted by grouped multi-script steps — `.github/workflows/ci.yaml:296-297,455-485,490,502`
  Failure: 15 docs guards run in one step, and further guards run through globs. The spec text says "by its own step". This holds only if "own step" is read loosely.
  Fix: either amend the spec wording or split the steps.
  Confidence: 60
- [NOTE] CWS-6 cannot see indirect invocations. Only literal `hack/test/…` tokens in `run:` bodies are scanned — `hack/test/ci_workflow_security_test.sh:372-374`
  Failure: a new guard added to `hack/docs/verify.sh`, a Taskfile target, or a composite-action input (`with: scenario-script:`) runs in CI without ever being checked. The 15 docs guards were wired in by hand for exactly this reason.
  Fix: also scan the scripts reached through `task`/`verify.sh`, or record this as an accepted limitation in the spec.
  Confidence: 80
- [NOTE] CWS-6 detects modes per line by substring, not per invocation — `hack/test/ci_workflow_security_test.sh:421`
  Failure: `bash a_test.sh; bash b_test.sh --self-test` on one line marks `a_test.sh --self-test` as pinned.
  Fix: decide the mode for each matched token from the text that follows that token.
  Confidence: 85
- [NOTE] CWS-3 is stronger than the spec and not scoped to the job — `hack/test/ci_workflow_security_test.sh:279-287`
  Failure: the spec only objects to raising the threshold "above the default", but setting `fail-on-severity: low` (the default) also requires a `# why:`. The line lookup also takes the first `fail-on-severity:` anywhere in ci.yaml, not the one in the dependency-review block.
  Fix: compare the value against `low`, and resolve the line from the job's `with` block.
  Confidence: 80
- [NOTE] Stronger than spec: the workflow-security job must have exactly 3 steps (`:136`), and every inline `# zizmor: ignore` is banned (`:247`). Both block legitimate future edits, such as a setup step. They are defensible, but the spec does not ask for them.

Things the change does that no requirement mentions:
- The allow-list grows by `concurrency` at `hack/test/dist_ci_wiring_test.sh:106`.
- The markdownlint exclusions grow by `openspec/changes/**/reviews/**` at `hack/tooling/markdownlint-cli2.yaml:53`.
- e2e-smoke's HY-06 cancel switches from ref-based to event-based (`e2e-smoke.yaml:37-38`), so `workflow_dispatch`/`schedule` runs off main are no longer cancelled.
- `changelog-sync.yaml` reworks token handling (`permission-contents`, `persist-credentials: false`, extraheader via `GIT_CONFIG_*`).
- `release.yaml` sets `cache: false`.
- The kind-e2e-setup action now passes its inputs through env bindings.
- The lint job sets `fetch-depth: 0`.
- 19 guards are moved into `lint`.
- The `schema_steps` ratchet is new.
- `.gitignore` gains `*.err`.

Requirements that hold, per the code:
- CWS-1: invocation `ci.yaml:192`, pin `:182/install-zizmor.sh:18`.
- CWS-3: job `ci.yaml:201-214`, eligibility exclusion `verify-eligibility.sh:20`.
- CWS-4: `ci.yaml:55-57`, `e2e-smoke.yaml:37-38`.
- CWS-5: `ci.yaml:33-34`, `verify-eligibility.sh:21`.
- CWS-7: a mutant exists for each of CWS-1 to CWS-6, plus the no-op control (`:628-891`).

## Could not check
- I did not run the meta-test or its self-test: Bash was denied in this session. Every verdict above comes from reading the code only.
- The live ruleset's required contexts. The CWS-6 required set depends on them.
- How zizmor 1.30.1 handles severity for `pull_request.title` injection, and that its exit code is non-zero on findings.
- That dependency-review-action v5 does not fail on unknown licences, and that the repo has the dependency graph enabled.
- Whether `dist_ci_wiring_test.sh` blocks a workflow-level `env:`/`defaults:` in ci.yaml. That is a possible CWS-1 skip switch which this test does not check itself.
- Whether mawk on ubuntu-latest supports the `{2}` interval regex at `:234`.
- The review records under `openspec/changes/ci-workflow-hardening/reviews/` (I did not read them).
