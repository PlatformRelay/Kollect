# PR description — developer-toolchain-pins

**Create at:** <https://github.com/platformrelay/kollect/compare/main...feat/developer-toolchain-pins>

- **Source:** `feat/developer-toolchain-pins`
- **Target:** `main`
- **Title:** `:wrench: build: pin and verify the developer toolchain in one place`
- **Opens as:** ready (operator-authorised; merges by rebase once every check on the head is green)

Paste everything below the line.

---

## TL;DR

- **What:** Task 3.52.0 at every site, govulncheck behind one `GOVULNCHECK_VERSION` variable, a new guard `hack/test/consistency_toolchain_pins_test.sh` that replaces `dev_mise_pin_drift_test.sh`, live Renovate managers for every pin site, and mandatory checksums in the git-cliff and helm-docs installers.
- **Why:** tool versions were stated in several files each, half the Renovate managers matched nothing, and two installers ran unverified downloads, one of them in a job that holds a ruleset-bypass token.
- **Risk:** the new guard is fail-closed and runs in required `lint`, so a guard bug or a Renovate bump it misreads blocks every PR until fixed.
- **Reviewer action:** check `renovate.json` (managers and groups) and the installer checksum paths; approving also confirms the two model-aligned decisions below.

Closes #442.

The same tool was pinned in up to four files with nothing keeping them in step, and the Renovate managers meant to move them were partly dead, so "no update PR" looked the same as "up to date". This PR makes every tool's sites agree, gives every site a live Renovate manager, and makes the guard red when they drift.

## Most important things

- **New guard replaces `dev_mise_pin_drift_test.sh`.** `hack/test/consistency_toolchain_pins_test.sh` absorbs the old guard's 12 cases 1:1 and adds a 13th. It checks that Task, gitleaks, git-cliff and shellcheck sites agree and meet their floors (never exact values, so a bot bump that moves every site stays green); that shellcheck's installer has a digest for its pin; that `task vulncheck` runs the annotated `GOVULNCHECK_VERSION`; that `mise.toml` and `Taskfile.yml` comments restate no version away from its pin; and that every Renovate custom manager is alive, covers every pin site and keeps each tool in one group. 146 self-test cases, including an all-pins bump control so a correct Renovate bump cannot red the guard.
- **Renovate.** The dead gitleaks and shellcheck managers are replaced by ones matching the real pins. New managers cover gitleaks, shellcheck, shellcheck-py (pre-commit), git-cliff (`Taskfile.yml`, `release.yaml`, `hack/verify-changelog.sh`) and golangci-lint (`Makefile`, `.custom-gcl.yml`). shellcheck and golangci-lint each get their **own group**: a shellcheck bump is red until its digest is added, and a red golangci-lint bump must not hold back Task, gitleaks, git-cliff or govulncheck. A `packageRule` stops the github-actions manager bumping `node-version`/`python-version` inputs without `mise.toml`.
- **Installer checksums are mandatory.** git-cliff verifies its per-asset `.sha512`; helm-docs verifies against `checksums.txt` (64-hex enforced). `UNVERIFIED_BY_DESIGN` in `ci_fetch_lib_hardening_test.sh` is now empty and limited to empty. `hack/lib/verify-sha256.sh` and the git-cliff path fall back to `shasum -a` on macOS and refuse when neither tool exists. **That fallback has had no real Darwin run**; it is covered by tests on Linux only.
- **CFC-2 tightened (change 4's guard).** `hack/test/consistency_tool_pins_test.sh` now requires a full semver for literal and Taskfile-variable pins, so `latest` or a go query such as `v1` no longer passes; its mutants were re-anchored because govulncheck moved into a variable. This is a test change to another change's guard.
- **Self-test moved, not added to `lint`.** The new guard's `--self-test` runs in the existing required job `tool-pins-selftest`, next to CFC-2's. The job now takes about 15 min (45 min timeout). No new job, so no ruleset change. CFC-6 (`ci_workflow_security_test.sh`) now pins both self-tests to that job.
- **Two model-aligned decisions, pending operator validation** (recorded in `proposal.md`): **A, pin ownership:** golangci-lint and govulncheck get no floor or agreement check here, because CFC-2 already owns those pins. **C, guard shape:** a new `consistency_*` guard instead of extending `dev_mise_pin_drift_test.sh`, which is folded in and deleted.

## Risk

- **What can break:** a false red in the guard blocks every PR, since it runs plain in `lint` and with `--self-test` in `tool-pins-selftest`. A Renovate bump that changes a file shape the guard's Renovate model does not know (presets, minimatch edge cases) could red the bot PR. The macOS checksum path could fail on a real Mac.
- **Blast radius:** CI gating, Renovate behaviour and the two installers (`changelog-sync.yaml` runs git-cliff). No product code changes.
- **Rollback:** revert the PR. No ruleset or required-check changes to undo.
- **Residual risk accepted:** BACKLOG CFC2-MAKEFILE-LATEST (a Makefile `go-install-tool` var or literal `@latest` still floats past CFC-2; `ENVTEST_VERSION`'s branch pin needs its own decision), DTP-SHA512-MACOS, the shasum fallback for install-helm/polaris, and case-sensitive `setup-go` matching in change 4's go-toolchain guard. The guard's Renovate model is a Python reading of `renovate.json`, checked against Renovate 44.115.4 extract dry-runs (41 deps), not Renovate itself.

## Files to look at first

- `hack/test/consistency_toolchain_pins_test.sh`: the new guard; start at the Renovate model and the all-pins bump control.
- `hack/test/consistency_tool_pins_test.sh`: CFC-2's full-semver rule and re-anchored mutants.
- `hack/test/ci_workflow_security_test.sh`: CFC-6 now pins both self-tests to `tool-pins-selftest`; also fixes a `grep -v | grep -q` SIGPIPE flake under pipefail.
- `.github/workflows/ci.yaml`: self-test step moved to `tool-pins-selftest`, old `dev_mise` step removed.
- `renovate.json`: new managers, own groups for shellcheck and golangci-lint, node/python uses-with disabled.
- `hack/install-git-cliff.sh`, `hack/install-helm-docs.sh`, `hack/lib/verify-sha256.sh`: supply-chain change.
- `.github/workflows/*`, `.github/actions/kind-e2e-setup/action.yml`, `mise.toml`, `Taskfile.yml`: Task 3.52.0 at every site and `GOVULNCHECK_VERSION`.

**Review this commit-by-commit, not as one flattened diff.** Each check landed with the fix it gates, and the review fixes are separate commits; most of the file count is OpenSpec evidence under `openspec/changes/`.

## Testing protocol

- [x] Locally: guard plain and `--self-test`, CFC-2 and CFC-6 both modes, `task lint:shell`, `task lint:markdown`, `task spec:validate`, `task vulncheck`, evidence-markdown guard; `task check` green at both branch-review rounds.
- [x] Real git-cliff and helm-docs installs on linux/amd64; Renovate 44.115.4 extract and lookup dry-runs.
- [x] Scrub grep and `gitleaks detect` over `origin/main..HEAD` clean.
- [ ] Every check on the PR head SHA is green, SonarCloud included; `tool-pins-selftest` shows both self-tests (15-20 min expected).
- [ ] Rebase-merge: `gh pr merge <n> --rebase --delete-branch`.
- [ ] On a Mac: `bash hack/install-git-cliff.sh` and `bash hack/install-helm-docs.sh` complete; a tampered archive fails with `checksum mismatch`.

---

## Optional reading — the long version

*Nothing above depends on this. Read it if you want the full story, the road not taken, or the deployment fine print — skip it if the top half already told you what you needed.*

### Where a human should look first (from the loop state)

- `hack/test/consistency_toolchain_pins_test.sh`: new guard (absorbs `dev_mise_pin_drift_test.sh`; 146 self-test cases incl. all-pins bump control and a Python Renovate model).
- `hack/test/consistency_tool_pins_test.sh`: change 4's CFC-2 tightened; re-anchored mutants.
- `hack/test/ci_workflow_security_test.sh`: CFC-6 pins both self-tests to `tool-pins-selftest`; guard_modes SIGPIPE flake fix.
- `.github/workflows/ci.yaml`: toolchain self-test in `tool-pins-selftest`; old dev_mise step removed.
- `renovate.json`: new managers (gitleaks, shellcheck, shellcheck-py, git-cliff, golangci-lint), own groups for shellcheck and golangci-lint, uses-with node/python disabled.
- `hack/install-git-cliff.sh`, `hack/install-helm-docs.sh`, `hack/lib/verify-sha256.sh`: checksums mandatory, macOS `shasum` fallback.
- `.github/workflows/*` + `mise.toml`: Task 3.52.0 at 19 sites; govulncheck via `GOVULNCHECK_VERSION`.

### Why the self-test left `lint`

The guard's `--self-test` took about 233 s locally. The operator ruled on 2026-10-16 that slow self-tests stay out of `lint`, so it runs as a second step in `tool-pins-selftest`, which is already required. A new job would have needed a ruleset change after merge.

### Why golangci-lint got its own group

All `custom.regex` dependencies shared the "pinned build tools" group. A golangci-lint bump often needs code fixes, and a red golangci-lint PR would have held back every other tool in that group.

### Review history

Spec-set review, a per-task loop with independent reviewers, and two branch-review rounds; every reproduced finding was fixed with a mutant, and the rest are recorded with dispositions in `openspec/changes/developer-toolchain-pins/loop.md` (Triage table). The G6 fixes were split into ten commits from one tested tree; all eleven passed a per-commit gate in round 2.
