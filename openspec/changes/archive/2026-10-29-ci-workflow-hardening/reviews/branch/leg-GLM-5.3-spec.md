I spotted a suspicious duplicated step line in the ci.yaml diff. Let me verify the actual file content at HEAD, then read the meta-test script in full.
Now let me verify the strongest claims: run the meta-test both ways locally (writes only to temp dirs), check `actionlint` on the duplicated step, and check the ci.yaml trigger block at HEAD.
actionlint confirms the parse-level defect. Let me check the remaining changed files (dist_ci_wiring_test.sh, evidence) before writing the report.
One quick check whether the repo's own lint already runs actionlint (affects finding 2's validity):
## Verdict: BLOCK

Verified by running `hack/test/ci_workflow_security_test.sh` (check + `--self-test`, all pass) and `actionlint` locally; all spec deltas read against HEAD.

## Findings

- [CRITICAL] ci.yaml is unparseable at HEAD: orphan duplicated step name with no `run:`/`uses:` — `.github/workflows/ci.yaml:446`
  Failure: every push/PR event → GitHub Actions rejects the whole workflow ("step must run script with `run` section or run action with `uses` section", confirmed by `actionlint` syntax-check at ci.yaml:446) → zero required contexts ever report, release eligibility is structurally unpassable — the exact CWS-5 failure class this change exists to prevent. Its own meta-test cannot run because its host workflow is invalid.
  Fix: delete the duplicated `- name: Run Sonar SECURITY remediation meta-tests` line (446).
  Confidence: 99

- [WARNING] The gate never validates GH Actions schema, so the CWS-7 no-op control passes on the broken tree — `hack/test/ci_workflow_security_test.sh:515-524`
  Failure: any future edit that leaves valid YAML but invalid workflow syntax (orphan step, bad key) passes every local gate; it only surfaces as a fully bricked CI run. `actionlint` is used nowhere in the repo (verified by grep across Taskfile/hack/.github).
  Fix: add an actionlint step over `.github/` next to the zizmor step, plus a CWS-7 mutant "workflow becomes invalid".
  Confidence: 90

- [WARNING] Spec CWS-6 says "any other mode", but `guard_modes` only detects bare/`--self-test` — `hack/test/ci_workflow_security_test.sh:310-315` vs header claim at :20-21
  Failure: a guard invoked with a third mode (e.g. `<tree-root>` arg mode, as this script itself accepts at :65) is mode-pinned only by accident of documentation, not enforcement.
  Fix: either detect other flags in the guard's arg-parse code or narrow the header comment to what is enforced.
  Confidence: 75

- [NOTE] `KOLLECT_FORCE_SHA256` env override of the checksum pin is not covered by any mutant or step-env check — `hack/install-zizmor.sh:28`
  Failure: an env added to the install step (step-key allowlist permits `env` unchecked, ci.yaml test at :137) silently bypasses the fail-closed digest pin the CWS-1 mutants assert.
  Fix: add a CWS-7 mutant asserting no such override env exists in the install step.
  Confidence: 60

- [NOTE] Stronger-than-spec exactness: the audit step must equal the exact string (cws1_invocation, :164), and `runs-on` must equal `ubuntu-latest` (:146), so any benign CLI/runner addition (e.g. `--color=never`, `ubuntu-24.04`) reds the gate and requires editing the lock itself.
  Failure: legitimate hardening additions are rejected as mutants would be.
  Fix: none needed if intended; else assert required substrings instead of equality.
  Confidence: 95 (intentional, by-design)

Requirement status: CWS-1 holds in shape (ci.yaml:177-198, exact invocation verified by meta-test) but is inert at HEAD because the host workflow fails to parse; CWS-2 holds (`rules: {}`, release.yaml:154,553 `cache: false`); CWS-3 holds (pull_request-only, SHA-pinned, allow-list, absent from verify-eligibility.sh:21); CWS-4 holds exactly in both workflows; CWS-5 holds (push:[main] retained, `workflow-security` added to required_checks); CWS-6 holds per executed meta-test; CWS-7 holds — 23 mutants + no-op control all verified green by execution.

## Could not check

- zizmor itself (not installed locally): the "clean tree at --min-severity=high" claim rests on evidence.md's recorded run, not my run.
- Authenticity of the four pinned SHA256 digests against the zizmor v1.30.1 release API (offline).
- GitHub-side live behaviour: that `workflow-security`/`dependency-review` actually report as required contexts on docs-only PRs, and the ruleset wiring (operator-checked post-merge per spec).
- Any Actions run logs — outside repository scope.
