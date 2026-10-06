## Provisional register (legs only, not unified) — legs ok: 5/6

### leg-GLM-5.3-fitness
## Verdict: BLOCK
- [CRITICAL] Duplicate step with `name` only and no `run`/`uses` makes `ci.yaml` invalid for GitHub Actions — `.github/workflows/ci.yaml:446`
- [WARNING] changelog-sync push will fail on ubuntu-latest: GNU `base64` wraps at 76 chars, splitting the `AUTHORIZATION` extraheader across lines — `.github/workflows/changelog-sync.yaml:136`
- [NOTE] Evidence undercounts the mutant harness: 19 mutants claimed, 22 exist (`evidence.md:11` vs 22 `*_mutant_rejected` calls in `ci_workflow_security_test.sh`; tasks.md CWS-1 row says 8, there are 7) — `openspec/changes/ci-workflow-hardening/evidence/evidence.md:11`
- [NOTE] Spec scenario CWS-1 "New unsafe pattern" (template-injection fixture) and CWS-3 "PR adds a vulnerable dependency" have no executed probe — deferred post-merge per tasks 4.2/verification table — `openspec/changes/ci-workflow-hardening/tasks.md:23`
- [NOTE] Allow-list widening flagged per lens: `WORKFLOW_KEY_ALLOWLIST` gains `concurrency` — `hack/test/dist_ci_wiring_test.sh:105`

### leg-GLM-5.3-spec
## Verdict: BLOCK
- [CRITICAL] ci.yaml is unparseable at HEAD: orphan duplicated step name with no `run:`/`uses:` — `.github/workflows/ci.yaml:446`
- [WARNING] The gate never validates GH Actions schema, so the CWS-7 no-op control passes on the broken tree — `hack/test/ci_workflow_security_test.sh:515-524`
- [WARNING] Spec CWS-6 says "any other mode", but `guard_modes` only detects bare/`--self-test` — `hack/test/ci_workflow_security_test.sh:310-315` vs header claim at :20-21
- [NOTE] `KOLLECT_FORCE_SHA256` env override of the checksum pin is not covered by any mutant or step-env check — `hack/install-zizmor.sh:28`
- [NOTE] Stronger-than-spec exactness: the audit step must equal the exact string (cws1_invocation, :164), and `runs-on` must equal `ubuntu-latest` (:146), so any benign CLI/runner addition (e.g. `--color=never`, `ubuntu-24.04`) reds the gate and requires editing the lock itself.

### leg-Qwen3.8-Flash-Next-security
## Verdict: BLOCK
- [CRITICAL] The ci.yaml added at HEAD is invalid as a GitHub Actions workflow: a duplicate step with only a `name:` and no `run:`/`uses:` — `.github/workflows/ci.yaml:446`
- [WARNING] changelog-sync's per-push authentication is computed with line-wrapping base64, so the extraheader is corrupt on ubuntu-latest and the push always fails — silently — `.github/workflows/changelog-sync.yaml:133`
- [NOTE] `KOLLECT_FORCE_SHA256` silently overrides the pinned digest at runtime and no meta-test forbids the variable in the job's env — `hack/install-zizmor.sh:49`
- [NOTE] `--min-severity=high` filters, rather than suppresses, every medium zizmor audit (e.g. unsound-inputs) on the tree the release gate trusts — `.github/workflows/ci.yaml` audit step

### leg-claude-opus-adversarial
## Verdict: BLOCK
- [CRITICAL] `ci.yaml` now has a step with a `name` but no `run` or `uses`, which makes the whole CI workflow invalid — `.github/workflows/ci.yaml:446`
- [WARNING] The meta-test claims "installed from a pinned checksum-verified release", but it does not check the install step's `run` body or its `env` — `hack/test/ci_workflow_security_test.sh:168-186`
- [WARNING] CWS-2 only polices `.github/zizmor.yml`. An inline `# zizmor: ignore[...]` in a workflow is an unjustified suppression the meta-test never sees — `hack/test/ci_workflow_security_test.sh:190-212`
- [NOTE] The App-token header is built with GNU `base64` without `-w0`, and a failed push only emits a warning — `.github/workflows/changelog-sync.yaml:136,141`
- [NOTE] The `dist_ci_wiring_test.sh` allow-list grew by `concurrency` (`hack/test/dist_ci_wiring_test.sh:106`). I checked this widening: `cws4_concurrency` (:256-267) pins both expressions exactly, and there are mutants for the ref-keyed group, the missing prefix and an unconditional cancel. The widening looks defended. Recording it because these allow-lists are supposed to shrink only.

### leg-claude-opus-spec
## Verdict: BLOCK
- [CRITICAL] `ci.yaml` now has a step with only a `name:` and no `run:` or `uses:`. That makes the workflow invalid, so on every PR and every push to main no CI job starts at all: lint, workflow-security and the release-eligibility checks never report. — `.github/workflows/ci.yaml:446`
- [WARNING] CWS-1 skip switch is only partly covered: the step allowlist accepts `shell:` and `env:` on workflow-security steps. — `hack/test/ci_workflow_security_test.sh:137`
- [WARNING] CWS-2 is stricter than the spec: a properly justified suppression written over several lines is rejected. — `hack/test/ci_workflow_security_test.sh:203-208`
- [WARNING] CWS-3 "threshold loosened" only looks at `fail-on-severity`. `warn-only: true` and a job-level `continue-on-error` both silence dependency-review without being caught. — `hack/test/ci_workflow_security_test.sh:226-237`
- [WARNING] The `# why:` check takes the first `fail-on-severity` text anywhere in `ci.yaml`, comments included. — `hack/test/ci_workflow_security_test.sh:233`
- [WARNING] CWS-6 is partial: it only scans `.jobs[].steps[].run`. Guards reached through composite actions (they are skipped at :340), reusable workflows, `task` targets or wrapper scripts (`lab_harness_meta_suite.sh`, `hack/docs/verify.sh`) are invisible to it. Mode detection also knows only `--self-test`, while the spec says "and any other". — `hack/test/ci_workflow_security_test.sh:310-315,338-362`
- [NOTE] CWS-1, CWS-4, CWS-5 and CWS-7 hold as written. The invocation is pinned exactly (:164), both workflows declare the exact concurrency (:256-266, `e2e-smoke.yaml:37-38`), the push-to-main check names `verify-eligibility.sh` (:275-280), and there is one mutant per requirement plus a no-op control (:515-709). CWS-3 "not a release check" holds: `verify-eligibility.sh:21`.
- [NOTE] Things no requirement mentions:
- [NOTE] `WORKFLOW_KEY_ALLOWLIST` in `dist_ci_wiring_test.sh:106` gained `concurrency`. This widens an allowlist. CWS-4 requires it and the meta-test pins the exact value, but it is still a widening.
