The duplicate step is confirmed at HEAD. Now reading the meta-test and surrounding files.
Verifying the pinned digests independently by downloading the tarballs (read-only to repo; temp dir only, binary never executed):
## Verdict: BLOCK

## Findings
- [CRITICAL] The ci.yaml added at HEAD is invalid as a GitHub Actions workflow: a duplicate step with only a `name:` and no `run:`/`uses:` — `.github/workflows/ci.yaml:446`
  Failure: GitHub rejects any step mapping lacking `run` or `uses`, so the *entire* ci.yaml fails validation — no runs on PR or push-to-main, no required check ever reports (lint, workflow-security included), every PR wedges and `verify-eligibility.sh` can never see a complete suite on a main SHA. The gate under review is not merely weakened, it is absent. No sensor catches this: yq parses the YAML fine (meta-test passes), no actionlint or schema validation exists in the lint job.
  Fix: delete the duplicated `- name: Run Sonar SECURITY remediation meta-tests` line (446); add `actionlint` (or schema validation of `.github/workflows/`) as a step of the lint job so the class is ratcheted.
  Confidence: 92
- [WARNING] changelog-sync's per-push authentication is computed with line-wrapping base64, so the extraheader is corrupt on ubuntu-latest and the push always fails — silently — `.github/workflows/changelog-sync.yaml:133`
  Failure: GNU coreutils `base64` wraps output at 76 columns; a GitHub App installation token (~360 chars) yields a multi-line `GIT_CONFIG_VALUE_0`; libcurl rejects (or header-splits) a newline-bearing extraheader, `git push origin HEAD:main` fails, and the `if … else ::warning` branch swallows it — the CHANGELOG self-heal this change is supposed to preserve stops working with a green job. actions/checkout's own pattern uses unwrapped base64.
  Fix: `base64 -w0` (or pipe through `tr -d '\n'`).
  Confidence: 85
- [NOTE] `KOLLECT_FORCE_SHA256` silently overrides the pinned digest at runtime and no meta-test forbids the variable in the job's env — `hack/install-zizmor.sh:49`
  Failure: a PR adding `KOLLECT_FORCE_SHA256` to the install step's env installs an arbitrary tarball while every assertion (`verify_sha256` present, `pins >= 4`) stays green; the cws1 step-key allowlist permits `env` without inspecting it. Same trust level as editing the digest itself, so no new privilege — but the "test only" comment is enforced by nothing.
  Fix: drop the override and have the self-test inject a wrong digest by editing the copy's `PINNED_SHA256` instead.
  Confidence: 70
- [NOTE] `--min-severity=high` filters, rather than suppresses, every medium zizmor audit (e.g. unsound-inputs) on the tree the release gate trusts — `.github/workflows/ci.yaml` audit step
  Failure: a medium-severity workflow defect can reach main; documented as a pinned threshold decision in tasks.md, which I did not read.
  Fix: none if the recorded breakdown genuinely shows nothing at medium worth keeping; otherwise raise scope, not severity.
  Confidence: 60

Checked and clean: all four zizmor v1.30.1 digests in `hack/install-zizmor.sh` match the real release assets (computed independently); fetch helper pins https/proto-redir/TLS 1.2 and fails closed; `verify_sha256` empty-digest path; the three template-injection fixes use the env-var pattern correctly and `SETUP_SCRIPT`/`RUN_MODE`/`SCENARIO_SCRIPT` collide with no script in the repo; app-token scoping to `contents: write`; `persist-credentials: false` on the new jobs' checkouts; `cache: false` on the publish jobs; the CWS-4 concurrency expressions are equivalent on every trigger ci.yaml actually declares; `.github/zizmor.yml` is `rules: {}` (no suppression widening); APP_TOKEN reaches git only via env, and create-github-app-token masks it.

## Could not check
- Ran nothing: the meta-test, its `--self-test`, or zizmor itself (read-only review); pass claims are read-verified only.
- Branch-protection/ruleset config: whether `workflow-security`/`dependency-review` are real required PR checks (CWS-5/CWS-6 operator evidence lives outside this repo).
- dependency-review-action v5 Go-module advisory and licence coverage — the CWS-3 scenario depends on dependency-graph Go support; no live trial.
- `openspec/changes/ci-workflow-hardening/evidence/evidence.md` and the tasks.md severity breakdown.
- Whether the CWS-6 mode sweep stays green with the four newly relocated guard steps (read the four scripts, they parse no `--self-test`; did not execute the sweep).
