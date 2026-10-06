## Verdict: CONCERNS

## Findings
- [WARNING] CWS-2's new "every key needs a comment" walk is bypassable by YAML-equivalent spellings the added mutant does not cover — `hack/test/ci_workflow_security_test.sh:286-297`
  Failure: `rules:\n  # fine\n  template-injection: {disable: true}` (flow-inline value) and `rules:\n  "template-injection":\n    "disable": true` (quoted keys) both exit 0 from the shipped awk — I ran the exact program against both inputs. The yq block-style pre-check at :240-253 also passes (the `.rules` node is still block). Same YAML semantics zizmor parses, so the same `disable: true` bypass the approval leg found survives in two other spellings, un-commented, gate green.
  Fix: stop line-walking the structure: assert via yq that every rule's sub-keys are only `ignore` (`yq '[.rules.[] | keys | .[]]'` catches quoted/flow forms), keep awk only for the comment-above check on what yq says is there; add both spellings as mutants.
  Confidence: 90 (awk pass verified by running it; zizmor's `disable` schema not verified from upstream docs — the branch's own approval-round text is the source)
- [NOTE] A failed installer gate does not stop the audit gate from running a stale `./bin` binary from a previous install — `hack/check.sh:48-53`
  Failure: `hack/install-zizmor.sh` fails (network) but `bin/zizmor` exists from an older version → `workflow-security-audit` reports `ok` against a binary not matching the pin; CI-vs-local drift goes invisible while the gate prints green.
  Fix: have `run_gate` record the install gate's result and skip (and mark) the dependent audit when it failed.
  Confidence: 60 (code read; not reproduced)
- [NOTE] `task check` decides a guard parses `--self-test` by grepping non-comment lines for the literal string — `hack/check.sh:66-68`
  Failure: a guard that only mentions `--self-test` in an error message gets invoked with a flag it does not parse; if it ignores unknown args its untended self-test path still never runs, and the claim "every mode its code declares" is content-guess, not header-declaration as the spec words it. Fail-closed (a non-zero from the extra call is counted), so noise, not a hole.
  Fix: convention on the mode, e.g. presence of a `# modes: bare,--self-test` header line.
  Confidence: 65
- [NOTE] evidence.md claims "the removed duplicate yq block" but the duplicated CWS-2 prelude (`local rules_type flow_style key` + the style check) is still present twice at HEAD, verbatim as on origin/main — `hack/test/ci_workflow_security_test.sh:240,253`. Runtime-idempotent; the record is what's wrong.
  Fix: delete one copy or correct the evidence.
  Confidence: 90
- [NOTE] CWS-3 step allowlist drop of `shell` (`hack/test/ci_workflow_security_test.sh:327`) is a narrowing — verified no existing dependency-review step declares `shell:` (.github/workflows/ci.yaml dependency-review job) so it does not red the tree; direction correct for a ratchet. No widening of any allowlist found in the diff. `gitleaks --redact` and `zizmor --offline` keep secrets off the local log path; installers are SHA256-verified (`hack/install-gitleaks.sh:37-55`); `set -uo pipefail` aggregation verified safe under bash 3.2 (`${#failures[@]}` on empty array tested on /bin/bash 3.2.57, rc=0); `bin/` is gitignored (:46) so `gitleaks --no-git --source .` does not scan the fetched binaries.

## Could not check
- That zizmor's config parser actually honours `disable:` under a rule name, and in flow/quoted form specifically — I verified YAML-node equivalence and the awk/yq gates, not zizmor's serde schema; no zizmor binary on this machine to test against.
- No full `task check`, `hack/test/task_check_test.sh --self-test`, or `ci_workflow_security_test.sh --self-test` run (plan mode, read-only); green-locally claims in evidence.md are attested, not reproduced.
- Whether `preflight`'s CI job really is `lint:markdown + verify + commit-identity guard` (the `coverage:` mapping is taken on trust; TCE-3 only checks the mapped gates are declared).
- The `openspec/changes/task-check-entrypoint/reviews/` directory in the working tree was left unread (prior-review records, out of bounds).
