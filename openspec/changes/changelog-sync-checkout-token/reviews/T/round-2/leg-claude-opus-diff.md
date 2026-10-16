The diff, the round-1 reports, the spec, tasks and evidence are read. Writing up the report now.

## Verdict: CONCERNS

## Findings
- [WARNING] The direct step-object reference still gets past the token check. `_output_ref` requires `outputs` to follow the id, so `${{ toJSON(steps.app-token) }}` or `toJSON(steps['app-token'])` in the guard step's `env` passes. That expression returns `{outputs:{token:…}, outcome, conclusion}`. It is the same class as round-1's P1: a direct, named reference with no wildcard. Spec CST-1 says "in any spelling" (`spec.md:15`), and the Limitations note (`evidence/T2.1.md`) lists only renamed outputs, implicit reads and `steps.*`, not this case — `hack/test/changelog_sync_release_guard_test.sh:180-187`
  Failure: a workflow edit that adds `LEAKED: ${{ toJSON(steps.app-token) }}` to a non-push step → `credential_errors` returns no error → the guard is green while the bypass-capable token reaches that step.
  Fix: make the trailing `outputs` group optional and end the id with a boundary, e.g. `(?![\w-])` after the dot/bracket id. Add a `token-step-object` mutant.
  Confidence: 70
- [WARNING] The App-secret check has the same blind spots that round 1 closed for `steps`. It only matches `secrets.NAME` / `secrets['NAME']` inside a step. Three cases get through: `${{ toJSON(secrets) }}` in a step, dynamic indexing such as `secrets[format('CHANGELOG_SYNC_{0}', 'GITHUB_APP_PRIVATE_KEY')]`, and a job-level `jobs.sync.env` or workflow-level `env:` that references the private key, which exposes it to every step. That contradicts "the App's secrets SHALL be referenced by the minting step alone" (`spec.md:15-16`), and the Limitations note doesn't mention it — `hack/test/changelog_sync_release_guard_test.sh:190-199,225-236`
  Failure: `jobs.sync.env.KEY: ${{ secrets.CHANGELOG_SYNC_GITHUB_APP_PRIVATE_KEY }}` → `static_wiring_check` only walks `steps` → exit 0, and every step can re-mint the bypass token.
  Fix: reject `toJSON(secrets)` and a non-literal `secrets[` in any non-minting step. Run `_secret_names` over `wf.get("env")` and `wf["jobs"]["sync"].get("env")` too. At minimum, add these cases to Limitations. Whether zizmor's overprovisioned-secrets audit covers the `toJSON(secrets)` case in CI is unverified.
  Confidence: 65
- [NOTE] The round-1 bracket finding is closed. `_output_ref` (`:180-187`) accepts dot or bracket for the id and for `outputs`, in any mix, case-insensitively, with escaped ids. Mutant `token-bracket-ref` (`:923-926`, `:1056-1058`) covers the all-bracket form, and the mixed forms follow from the regex — `hack/test/changelog_sync_release_guard_test.sh:180`
  Confidence: 90
- [NOTE] `toJSON(steps)` is now rejected in every step except the push step and the minting step (`:227-231`). Its mutant expects the reason `toJSON(steps)`, which only that error message contains, so the mutant can't pass by tripping a different check — `hack/test/changelog_sync_release_guard_test.sh:227`
  Confidence: 90
- [NOTE] The re-mint finding is closed for direct references. The App secrets are the union of what the minting steps reference; the push step is not exempt, and names are compared upper-cased. The `app-secret-remint` mutant's expected reason, "only the create-github-app-token step may", matches the error at `:224` — `hack/test/changelog_sync_release_guard_test.sh:215-226`
  Confidence: 85
- [NOTE] The post-merge row is closed: it now requires the "Pushed regenerated CHANGELOG.md" notice and keeps the row open on `in-sync`. This matches the workflow's soft-fail `else` branch at `changelog-sync.yaml:138-142` — `openspec/changes/changelog-sync-checkout-token/tasks.md:57`
  Confidence: 90
- [NOTE] The commit says "test(ci)" but also adds CST-1 spec text, a new scenario at `spec.md:38-41`, and the round-1 review files. The spec change and the test change are one logical unit, so this is acceptable. The PR description should still call out the spec widening — commit `2e3640092`
  Confidence: 50

Checked, no defect found:
- The workflow at the head has no `token:` on the checkout, keeps `persist-credentials: false`, and only the push step reads `APP_TOKEN` (`changelog-sync.yaml:83-92,119-120`).
- `pushes` is in scope where `credential_errors` is called (`:548,604`).
- Each new mutant builds against the real step layout: the guard step has no `env`, and `setdefault` and the insert at `guards[0]` both work.
- The `|| return 0` keeps `set -e` from ending the run while `fail` still counts the failure.
- Whitespace tolerance in the regexes only over-matches, which is the safe direction.

## Could not check
- I could not run the guard, its mutants, the scratch-copy "switched off" red runs, zizmor, `task lint:shell` or `task spec:validate`. I rely on the claims in `evidence/T2.1.md`.
- I did not confirm on a live runner that GitHub evaluates `toJSON(steps.<id>)` and `secrets[format(...)]` the way I describe above.
- I did not read the untracked `reviews/T/round-2/` directory, `sonar_ko_08_workflow_permissions_test.sh`, `ci.yaml`, or zizmor's configured audits and severity threshold.
- I could not see the post-merge run (task 4.2) or the live `protect-main` ruleset.
