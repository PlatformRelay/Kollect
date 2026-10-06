Verifying the extra-step bypass concretely against a scratch copy (repo untouched):
## Verdict: BLOCK

## Findings
- [CRITICAL] The `workflow-security` job accepts an extra step that replaces the `zizmor` binary with a green stub; the meta-test passes and the gate is silently defeated — `hack/test/ci_workflow_security_test.sh:133`
  Failure: I applied a real mutant — a 4th step doing `printf '#!/bin/bash\nexit 0\n' > "$RUNNER_TEMP/zizmor"` after the reviewed install step (which the reviewed `GITHUB_PATH` echo puts first on PATH). `check_all` passes on the mutated tree (verified: `All workflow-security meta-tests passed`, exit 0), and the exact-match audit line at `ci.yaml:194` then executes the stub. Key-shape allowlists (`:128-147`) validate each step's keys but never step count or order, so "shell: true {0}"-class defeat returns one layer up. This is the release-eligibility check (CWS-5) nullifiable by a PR no assertion reds.
  Fix: `ci_workflow_security_test.sh:133` change `-ge 3` to `-eq 3` (or require the audit step index to equal the install index + 1 and no step after it), and add a self-test mutant inserting a stub step.
  Confidence: 95 (mutant applied and run, not theorised)
- [WARNING] `--min-severity=high` filters, not suppresses: future medium-and-below findings pass silently, contradicting the spec's "the job SHALL fail on any unsuppressed finding" — `.github/workflows/ci.yaml:194`
  Failure: I ran the pinned zizmor 1.30.1 offline over `.github/` at `--min-severity=medium`: zero findings today. So the sensitivity costs nothing now, yet e.g. a future `persist-credentials` regression (the very class fixed in this diff for changelog-sync/release) in a medium-severity audit would land green with no entry in `zizmor.yml` and no visible decision.
  Fix: pin `--min-severity=medium` in `ci.yaml:194` and the exact-match string + probes in `ci_workflow_security_test.sh:179,640,645,655`; the tree is clean at medium so it is free today.
  Confidence: 70 (severity-classification of specific audits is my belief, not checked against each audit's docs)
- [NOTE] The shipped installer honours a checksum-bypass env, `KOLLECT_FORCE_SHA256`, in every context — `hack/install-zizmor.sh:50`
  Failure: the meta-test blocks it only as an env key on `workflow-security` steps; any other future job, runner or local invocation that exports it installs an unverified binary while the comment still promises "fails closed". Its only consumer is `hack/test/install-checksum-negative_test.sh:25`.
  Fix: require an explicit companion flag (e.g. `KOLLECT_INSTALLER_UNSAFE_TEST=1`) before honouring the override, or take the digest as an installer argument used only by the negative test.
  Confidence: 60 (limited exploitability today; it is a latent footgun, not a live hole)
- [NOTE] The installer comment claims the digests are "values the GitHub release API reports"; the release API publishes no asset digests — `hack/install-zizmor.sh:21-22`
  Failure: provenance of the pin is misdescribed; I independently verified the pins are correct for two platforms (downloaded v1.30.1 `zizmor-x86_64-unknown-linux-gnu` and `zizmor-aarch64-apple-darwin`, sha256 matches `install-zizmor.sh:27,35`), so the values are good but the stated method is not.
  Fix: one-line comment correction ("computed from the release assets at pin time; re-verified against a fresh download 2026-10-06").
  Confidence: 85

## Checked (found nothing)
- `kind-e2e-setup/action.yml:100-124`: inputs moved to env bindings — injection genuinely closed; callers pass repo-static values.
- `changelog-sync.yaml:116-141`: token reaches git only via `GIT_CONFIG_*`, scoped to `https://github.com/`, not in argv or `.git/config`, no `set -x`; artipacked fix is real and the minted token is now scoped to `contents: write`.
- CWS-4 concurrency on both workflows: main pushes never cancelled; HY-06 preserved (strictly stricter than the old `github.ref != main` form).
- Ran both modes of the meta-test green (31 mutants, message-asserted); the CWS-6 required-set derivation, glob resolution and composite-action rejection read correct.
- `dependency-review`: SHA-pinned, PR-only, no `warn-only`/`deny-licenses`, absent from eligibility — matches CWS-3.

## Could not check
- The actual `protect-main` ruleset / required-context list (needs `gh api`; spec defers it to post-merge evidence).
- That `dependency-review-action@a1d282b…` is the true v5.0.0 tag SHA.
- Runtime behaviour of the four guard scripts newly pinned into `lint` (only their wiring was verified); no evidence run this session that the new `workflow-security` job is green on GitHub.
- `create-github-app-token` v2.2.2 add-mask default for step outputs (assumed from upstream behaviour, not read).
