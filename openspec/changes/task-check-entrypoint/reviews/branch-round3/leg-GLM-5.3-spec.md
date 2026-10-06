## Verdict: CONCERNS

## Findings

- [WARNING] CWS-2 (in-range, outside TCE spec): the awk key extraction is dead code — `sub(/"?.*/, "", key)` always yields an empty `key`, so the deliberate bare-`ignore:` exemption never fires — `hack/test/ci_workflow_security_test.sh:283-284`
  Failure: a future legitimate block-style suppression (`rules:` → commented rule → bare `ignore:` → commented entries) false-reds with "has no comment directly above it" on the `ignore:` marker itself; verified by replaying the awk on a synthetic config (`FAIL(key=): ignore:`). Latent today (`rules: {}`), and the mutants can't catch it because no mutant asserts the legitimate spelling passes. The `rest`/`value` no-op assignments (line 285-286) are the same hunk's dead code.
  Fix: extract the key up to the first `:` (e.g. `sub(/:.*/, "", key)` after whitespace trim) instead of `sub(/"?.*/, "", key)`.
  Confidence: 85 (behaviour verified by replay; impact latent until a suppression is actually written).
- [WARNING] TCE-1's central scenario ("everything runnable runs" on a no-Docker machine) has zero execution evidence — `openspec/changes/task-check-entrypoint/tasks.md:28`
  Failure: if any of the 72 swept guards fails bare on a normal dev machine (tooling, args, network), `task check` reds for the wrong reason the first time a contributor runs it; the meta-test pins wiring, not runtime success, and no CI job ever runs `hack/check.sh`.
  Fix: cheapest ratchet — run `task check` on the merge commit (or at least the guard sweep) once before merge, or add a CI job that runs `bash hack/check.sh` and demote the deferral.
  Confidence: 90 that it is unexecuted (stated in tasks.md); 40 that a guard actually breaks it.
- [NOTE] The `coverage: preflight=...` mapping is validated only against gates declared in check.sh, never against preflight.yaml's actual step list — `hack/check.sh:66`
  Failure: preflight gains a new step (e.g. a new lint) → the mapping still validates and the step's local equivalent silently vanishes from `task check`; TCE-3's letter holds (preflight is mapped), the spirit of "no required gate silently missing" weakens.
  Fix: have the guard parse preflight.yaml's `run:` steps the way it parses `required_checks`.
  Confidence: 80 (mechanism gap verified; currently the mapping is accurate — checked against preflight.yaml:31-52).
- [NOTE] Local gitleaks invocation omits `--verbose` (cosmetic) vs ci.yaml's `--verbose --redact --no-git` — `hack/check.sh:59` vs `.github/workflows/ci.yaml:168`; the guard's pinned substring matches both, so nothing notices.
  Fix: add `--verbose` or accept and drop "same invocation shape" wording. Confidence: 95 (verified both lines).
- [NOTE] Evidence bookkeeping: evidence.md says "7 mutants", the script has 8 (`task_check_test.sh:299-353`), tasks.md says 8; a duplicated paragraph also sits at `openspec/changes/task-check-entrypoint/evidence/evidence.md:15-21`.
  Failure: an auditor trusting evidence.md misses the TCE-3 silent-missing mutant's recorded result.
  Fix: correct the count, delete the dup. Confidence: 100.

Spec requirement states (TCE spec, verified against code, not documents): **TCE-1 holds structurally** — glob sweep `hack/check.sh:77`, `--self-test` where code declares `:91`, header-Docker skip `:82` (today exactly `integration_no_docker_test.sh` is skipped, with printed reason), failure aggregation + `exit 1` — but runtime scenario unexecuted (WARNING above). **TCE-2 holds** — Taskfile.yml:111-114, verify = one cmd `bash hack/verify.sh`, desc names generated artifacts; guard pins cmd count, command, and desc. **TCE-3 holds** — all 14 `required_checks` names (verify-eligibility.sh:19-22) resolve: 12 via run_gate, 3 exclusions with reasons, preflight via the coverage mapping; I ran `bash hack/test/task_check_test.sh` myself: green. **TCE-4 holds** — 8 throwaway-copy mutants + no-op control, wired plain + `--self-test` into the required `lint` job on `pull_request` (ci.yaml:444-448; job at :267; trigger at :43).

Beyond-spec behaviour: extra non-required gates run (format:check, scrub, spec:validate, go-mod) — harmless, costs runtime; dependency-review listed as an exclusion though it is not in `required_checks` (informative, not a spec violation); zizmor/gitleaks installers run under required-check names (workflow-security run_gate is the installer, the audit is a separate label) — naming quirk only. I also checked the range's other commit (CWS-2): the yq flow-style check was a duplicate removal only — the original survives, so the flow-map hole I initially suspected is closed; no regression there.

## Could not check

- `task check` end-to-end and the 72-guard sweep: not executed (also barred by my read-only constraint for anything that writes; a green sweep needs network/tool installs too).
- `bash hack/test/task_check_test.sh --self-test`: not run (mutates mktemp copies). I verified all 8 mutants statically — each sed/yq/perl target string exists verbatim in check.sh/Taskfile.yml/verify-eligibility.sh, sentinels match the assertion messages — but its green status rests on evidence.md/tasks.md, not my run.
- shellcheck/yq over check.sh; the full text of the branch-round2 review legs (read only as diff stat); `task helm-test`/`task test` runtime (envtest setup) beyond confirming the task definitions exist.
