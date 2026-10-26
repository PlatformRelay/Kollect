## Verdict: CONCERNS

Round-1 fixes checked, as the brief asks:
- **Shell emulation is correct.** No shell runs the body under `bash -e`. `shell: bash`, whether set on the step, as the job's `defaults.run.shell` or as the workflow's, runs it under `bash --noprofile --norc -eo pipefail`. The step value takes precedence, which matches the runner (`hack/test/renovate_policy_test.sh:152-162,183-187`).
- **The body run cannot hang or leak a secret.** It is bounded by `timeout -k 5 20` with stdin closed, runs under `env -i`, and gets a probe value that is not a secret.
- **The `T2+T7` allowance hides nothing.** It is scoped to one mutant, and that mutant still has to match T2's own message (`:813-814`).
- **The T7 token rule hides nothing.** It allows one `run:` step, without `uses:`, before the Renovate step in the same job. A job-level, second-step or after-Renovate `RENOVATE_TOKEN` reds (`:367-376`).
- **Wiring is correct.** The fixture is a correct DUA-1..DUA-3 workflow. The `lint` wiring sits after yq is installed (`ci.yaml:342-347,588-591`). The CWS-6 enrolment and its mutant are correct.
- **Mutant counts are correct.** They add up to 59.

## Findings
- [WARNING] T6 accepts any `configurationFile` value, so the global-config clause planned in 1.2 (T13, on `.github/renovate-config.json`) can be bypassed by pointing the action at another file — `hack/test/renovate_policy_test.sh:337-345`
  Failure: `configurationFile: .github/other.json` holding `force: {automerge: true}` or `onboarding: true`. T1 to T9 stay green, and T13 keeps judging a file Renovate no longer reads.
  Fix: in T6, require `with.configurationFile` to be exactly `.github/renovate-config.json`, and add a mutant that points it at another file.
  Confidence: 65
- [NOTE] The `yqv` header says an empty result is always a violation. That is not true where a read feeds a loop (T7, `:378`) or the T9 trigger read (`:430`): there, an error yields no rows, and no rows means a pass.
  Failure: any renovate.yaml on which the T7 key walk errors would clear every `RENOVATE_*` check silently. I found no concrete construct that triggers this today; the mutants only prove the read works on the fixture.
  Fix: read the T7 rows with plain `yq eval` and red T7 when it fails, as T9's body read already does (`:421-425`). Alternatively, correct the comment at `:68-69`.
  Confidence: 55
- [NOTE] The `|| echo '<unreadable>'` fallback is dead code, because `yqv` already ends in `|| true` — `hack/test/renovate_policy_test.sh:183`
  Failure: an unreadable shell value becomes `""` and the body is judged under `bash -e`. That is the stricter direction, so nothing is let through, but the fallback the line meant to add never fires.
  Fix: drop the fallback, or call `yq eval` directly at this line.
  Confidence: 90
- [NOTE] Two design-choice entries in the evidence file are stale since the round-1 fixes — `openspec/changes/dependency-update-automation/evidence/1.1.md:43-47,53-54`
  Failure: line 43 still says every body runs under `-eo pipefail`. Lines 53-54 still say T7 allows the token on "any `run:` step" and that moving the check "trips T2 only". The code and the `T2+T7` row now say otherwise.
  Fix: rewrite both bullets to match `run_body` and `re_tok` as they are now.
  Confidence: 95
- [NOTE] T9 only scans `run:` and `with.script`. A workflow that merges through a third-party `uses:` action (an automerge action) passes. This goes beyond T9's case text, but DUA-8 is about "no merging workflow", and the gap is not listed under *Harness gaps* — `hack/test/renovate_policy_test.sh:421-429`
  Fix: list the gap in the evidence, or deny merge actions by name in `uses:`.
  Confidence: 70
- [NOTE] T2 runs the body under `env -i`, so `CI` and `GITHUB_ACTIONS` are unset there but set on the runner.
  Failure: a body that fails only when `$CI` is empty passes T2 but fails open on the runner. Only a deliberately evasive body would do this.
  Fix: export `CI=true GITHUB_ACTIONS=true` inside `run_body`.
  Confidence: 40

## Could not check
- I ran no sensors: the self-test, the plain red and the CWS runs are taken from the evidence as recorded.
- I did not mutation-verify any clause in a scratch copy, because I am read-only.
- I did not confirm yq v4's short-circuit of `and` in the T7 key walk on non-string keys, or how `.run.shell` resolves on a scalar `defaults:`.
- I did not read the spec's DUA-* text in `specs/dependency-updates/spec.md` beyond the case lines in `tasks.md`.
- I did not read the round-1 review legs under `reviews/L-tasks/G1/`.
- I checked the runner's shell defaults from memory of the documentation, not live.
