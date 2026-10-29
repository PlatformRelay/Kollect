## Verdict: BLOCK

## Findings
- [CRITICAL] `ci.yaml` now has a step with a `name` but no `run` or `uses`, which makes the whole CI workflow invalid — `.github/workflows/ci.yaml:446`
  Failure: line 446 `- name: Run Sonar SECURITY remediation meta-tests` is left over from the insertion, and an identical step follows it at :447. GitHub rejects any step without `run`/`uses` ("Every step must define a `uses` or `run` key"). So the CI workflow does not start on any PR or push to main. No required context reports (`lint`, `test`, `workflow-security`, …), every PR stays BLOCKED, and `verify-eligibility.sh` can never pass on a new main SHA. No gate here catches it. `dist_ci_wiring_test.sh` and `ci_workflow_security_test.sh` only check key allow-lists, and a step that has only `name` passes both. The repo has no `actionlint`. I have not run zizmor, so I cannot say whether the "exit 0" in `evidence.md:35-36` was produced on this tree. If zizmor's parser rejects the step, that evidence is stale. If it accepts it, zizmor is not a schema check either.
  Fix: delete line 446. Then add a workflow-schema check to the required `lint` job, for example a pinned `actionlint`, or a yq assertion that every step has `run` or `uses`. That is the ratchet that would have failed here.
  Confidence: 95

- [WARNING] The meta-test claims "installed from a pinned checksum-verified release", but it does not check the install step's `run` body or its `env` — `hack/test/ci_workflow_security_test.sh:168-186`
  Failure: `cws1_version_pin` only reads `env.ZIZMOR_VERSION` and greps the installer file. Step-level `env` is allow-listed (:137). So two edits pass every check and every mutant:
  - `KOLLECT_FORCE_SHA256: <any digest>` on the install step, which silently disables the pinned digest (`hack/install-zizmor.sh:50`).
  - Replacing `bash hack/install-zizmor.sh` with `pipx install zizmor`, or with a stub dropped on `$GITHUB_PATH`.
  Spec CWS-1, "Skip switch", requires that a disabling env var fails the meta-test.
  Fix: pin the install step's comment-stripped `run` body exactly, the way the audit step is pinned at :164. Reject any step `env` key other than `ZIZMOR_VERSION` in `workflow-security`. Add one mutant for each.
  Confidence: 80

- [WARNING] CWS-2 only polices `.github/zizmor.yml`. An inline `# zizmor: ignore[...]` in a workflow is an unjustified suppression the meta-test never sees — `hack/test/ci_workflow_security_test.sh:190-212`
  Failure: a PR adds `${{ github.event.pull_request.title }}` to a `run:` step with a trailing `# zizmor: ignore[template-injection]`. zizmor counts it as "ignored" and exits 0, and `cws2_suppressions` passes. That defeats the CWS-1 "New unsafe pattern" scenario. The tree has zero inline ignores today (I grepped), so the ratchet costs nothing to add now.
  Fix: fail on any `zizmor: ignore` under `.github/` that has no justification comment on the line above. Add one mutant.
  Confidence: 75

- [NOTE] The App-token header is built with GNU `base64` without `-w0`, and a failed push only emits a warning — `.github/workflows/changelog-sync.yaml:136,141`
  Failure: today `x-access-token:` plus a 40-character `ghs_` token is 55 bytes, which encodes to exactly 76 characters, the wrap width. Any longer token format (my belief that one could come: not verified) would put a newline into the header. The push would then fail, and that only produces a `::warning`, so the changelog drift would go unnoticed.
  Fix: `base64 -w0`. actions/checkout encodes without wrapping too.
  Confidence: 55

- [NOTE] The `dist_ci_wiring_test.sh` allow-list grew by `concurrency` (`hack/test/dist_ci_wiring_test.sh:106`). I checked this widening: `cws4_concurrency` (:256-267) pins both expressions exactly, and there are mutants for the ref-keyed group, the missing prefix and an unconditional cancel. The widening looks defended. Recording it because these allow-lists are supposed to shrink only.

What I tried that did not land:
- **CWS-4 behaviour:** group and cancel expressions for `pull_request` vs `push`, the workflow-name prefix ("CI" vs "E2E smoke"), and HY-06 being preserved.
- **Allow-list key shapes:** `cws1_job` blocks job-level `if`/`needs` keys.
- **Glob resolution in CWS-6:** an unmatched glob fails.
- **Guards hidden in action inputs:** no guard is passed through a `with:` input (grepped `.github`).
- **Template-injection fixes in `kind-e2e-setup`:** inputs are now bound to env vars correctly.
- **Release builds:** `cache: false` on both setup-go steps in `release.yaml`.
- **App token scope:** `permission-contents: write` in `changelog-sync.yaml`.

## Could not check
- I could not run any command (Bash was denied). Not run: `ci_workflow_security_test.sh` in either mode, `test-verify-eligibility.sh`, `dist_ci_wiring_test.sh`, zizmor, or a YAML or workflow-schema validator.
- Which commit introduced ci.yaml:446 (`git log -L` was unavailable), and whether the evidence run predates it.
- Live behaviour of `dependency-review-action` v5 against this repo's dependency graph: Go modules reporting a NOASSERTION licence, and whether GitHub Actions dependencies are licence-checked against `allow-licenses`.
- That the Linux zizmor tarball has `zizmor` at its root (`install-zizmor.sh:57`), and that the four pinned digests match the release assets.
- The repository ruleset: whether `workflow-security` and `dependency-review` are actually required contexts. That is post-merge, operator-checked.
- `openspec/changes/ci-workflow-hardening/reviews/` and `loop.md`, other than incidental grep hits. Both are untracked and outside the range.
