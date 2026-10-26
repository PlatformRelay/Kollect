# Tasks

## 0. Prerequisites and probes

- [x] 0.1 (operator) `lint`, `vulncheck`, `workflow-security`, `dependency-review` are required checks in `protect-main`. Done 2026-10-16; re-measured 2026-10-26: ruleset 18802410 requires `preflight`, `test`, `kind-smoke`, `Analyze (Go)`, `lint`, `workflow-security`, `dependency-review`, `tool-pins-selftest`, `vulncheck`, `breaking-marker`; rebase merge only; strict up-to-date; bypass actors: admin role (`pull_request`), the changelog-sync App (`always`)
- [x] 0.2 `install-git-cliff.sh` verifies a checksum (change 5, DTP-6, archived `2026-10-16-developer-toolchain-pins`)
- [x] 0.3 (operator) `require_extra_approval_for_unattributed_changes` has no observed effect on bot-authored merges (2026-10-16)
- [ ] 0.4 Probe the Renovate assumptions in proposal.md with a local Renovate on a committed scratch copy of the tree, outside the repo (`--platform=local`, `RENOVATE_DRY_RUN=extract`, then `lookup` with a read-only token from `gh auth token`; the token is never printed or written). On a copy carrying the planned `renovate.json` and `renovate.yaml`, record: (a) the `renovate-image` input is extracted as a docker dependency with tag and digest; (b) `azure/setup-helm` resolves to `depName` `helm`, `packageName` `helm/helm`, and with the DUA-9 rule its branch is `renovate/helm`; (c) which branches the lookup proposes and which carry `automerge: true` (only digest-only dockerfile or devcontainer branches may); (d) the action at v46.3.10 uses a `renovate-image` that holds `:` as the whole reference (read `src/docker.ts` at the tag). Evidence `evidence/0.4.md`. A premise that turns out false is a SPECIFICATION finding: stop and report
- [ ] 0.5 Probe the validator: pull the pinned image chosen for DUA-3, then run `renovate-config-validator --strict` from it on today's `renovate.json` and `.github/renovate-config.json`. Record whether today's config passes or needs migration, the binary path and entrypoint, whether it needs network, and the wall time. Evidence `evidence/0.5.md`

## 1. Tests first (DUA-1 to DUA-5, DUA-7 to DUA-10)

- [ ] 1.1 Write `hack/test/renovate_policy_test.sh` (plain mode checks a tree, default the repo root; `--self-test` mutates copies) and the stub `hack/ci/renovate-config-validate.sh [root]` (body: `echo "not implemented" >&2; exit 2`). Each mutant is judged by its own clause's message, with one no-op control, per `~/.claude/skills/close-task/reference/guard-mutation.md`. Cases, each given / when / then, with the failure it exposes:
  - **T1 token:** given `renovate.yaml`, when the Renovate step's `token` is anything but exactly `${{ secrets.RENOVATE_TOKEN }}`, or the file mentions `github.token` or `secrets.GITHUB_TOKEN`, then fail. Exposes a fallback that opens CI-less PRs (DUA-1)
  - **T2 fail closed:** when no step before the Renovate step exits non-zero on an empty `RENOVATE_TOKEN` with a message naming it, then fail. Run that step's `run:` body under bash with the variable empty and set; judge by exit status and message. Exposes a silent no-op run (DUA-1)
  - **T3 permissions:** when the workflow or job grants any `write` permission, then fail. Exposes write access to `GITHUB_TOKEN` (DUA-1)
  - **T4 schedule:** when no cron entry has `*` in both its day-of-month and day-of-week fields, or the triggers lack `workflow_dispatch`, then fail. Exposes the weekly cron (DUA-2)
  - **T5 concurrency:** when there is no `concurrency.group` or `cancel-in-progress` is not literally `false`, then fail (DUA-2)
  - **T6 pin:** when the action is below v46.3.10 (by its version comment, with a SHA pin), `renovate-image` does not match `^ghcr\.io/renovatebot/renovate:[0-9]+\.[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}$`, or `renovate-version` is set, then fail. Exposes the floating `:44` tag (DUA-3)
  - **T7 image rule:** when no `packageRules` entry matches `ghcr.io/renovatebot/renovate` with its own `groupName` and a weekly `schedule`, or the image is matched by an automerge rule, then fail (DUA-3)
  - **T8 dashboard:** when `extends` holds `:disableDependencyDashboard` or `dependencyDashboard` is false, then fail (DUA-4)
  - **T9 validator wiring:** when `ci.yaml`'s `lint` job has no step whose `run` is exactly `bash hack/ci/renovate-config-validate.sh`, or that step has `if:` or `continue-on-error:`, then fail (DUA-5)
  - **T10 validator behaviour:** with a fake `docker` on `PATH` that records its argv, the script refuses an image without a digest without calling docker; with a digest it calls docker exactly once with `run --rm` and the image from `renovate.yaml`, running `renovate-config-validator --strict renovate.json .github/renovate-config.json`; it exits non-zero when the fake exits 1 (DUA-5)
  - **T11 automerge scope:** when the number of `packageRules` with `automerge: true` is not exactly 1, that rule's keys are not exactly `matchManagers` `["devcontainer","dockerfile"]` (sorted) plus `matchUpdateTypes` `["digest"]` plus optional `description`, a top-level `automerge` is true, or `extends` names any preset containing `automerge`, then fail. Exposes widened scope (DUA-7)
  - **T12 merge settings:** when `platformAutomerge` is not `false`, `automergeType` is not `pr`, `automergeStrategy` is not `rebase`, `rebaseWhen` is not `behind-base-branch`, or `ignoreTests` is present, then fail (DUA-8)
  - **T13 no merge workflow:** when any file under `.github/workflows/` combines `pull_request_target` with `gh pr merge` or a merge API call, or runs `gh pr merge` at all, then fail (DUA-8)
  - **T14 helm:** when no rule has `matchPackageNames` containing `helm/helm` with `groupName` `helm`, or the helm rule matches by `matchDepNames` alone, then fail (DUA-9)
  - **T15 docs:** when the Renovate section of `docs/development/tooling-setup.md` does not name the daily schedule, `RENOVATE_TOKEN`, the dependency dashboard, the digest-only auto-merge scope, `gh workflow disable`, `gh pr update-branch --rebase`, `hack/ci/renovate-config-validate.sh`, the unused hosted App and the release review, then fail (DUA-10)

  Run it on today's tree and record the red per case (T1 to T9, T11, T12, T14 and T15 red; T10 red on the stub; T13 green, which is correct today). Add both modes to the `lint` job now, and add the guard to CWS-6's enrolled list in `hack/test/ci_workflow_security_test.sh`; `lint` stays red until section 2. Evidence `evidence/1.1.md`

## 2. Implementation

- [ ] 2.1 `.github/workflows/renovate.yaml`: daily cron, `concurrency` (no cancel), job `permissions: contents: read`, a fail-closed token step, `renovatebot/github-action` at v46.3.10 SHA-pinned with `renovate-image` from 0.5, `token: ${{ secrets.RENOVATE_TOKEN }}` only, header comment rewritten. Update the stale `renovate.yaml` remark in `hack/test/sonar_ko_08_workflow_permissions_test.sh`. T1 to T6 green
- [ ] 2.2 `renovate.json`: drop `:disableDependencyDashboard`; set `platformAutomerge`, `automergeType`, `automergeStrategy`, `rebaseWhen`; add the digest automerge rule; add the Renovate image rule (own group, weekly, `automerge: false`); move the Helm rule to `matchPackageNames`. Keep `go toolchain` the last group rule and the node/python rule last (`consistency_go_toolchain_test.sh`, `consistency_toolchain_pins_test.sh`, both modes). T7, T8, T11, T12, T14 green
- [ ] 2.3 Implement `hack/ci/renovate-config-validate.sh` against T10; run it for real on the final config (0.5's image); T9, T10 green
- [ ] 2.4 Re-run 0.4's lookup on the final tree and record the branches and their `automerge` flags in `evidence/2.4.md`

## 3. Docs

- [ ] 3.1 Rewrite the Renovate section of `docs/development/tooling-setup.md` and its maintainer table row (DUA-10), plus the `RENOVATE_TOKEN` line in `docs/security/sca-remediation-policy.md` if it contradicts; T15 green; `task lint:markdown` and `task docs:verify` (see the known local-only red in the repo notes)

## 4. Land

- [ ] 4.1 Merge with the `post-merge` rows open; archive in a follow-up evidence PR once they pass
- [ ] 4.2 (operator) Remove the hosted Renovate App from the repository (not a gate for the archive; listed so it is not lost)

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DUA-1 | guard T1, T2, T3 and their mutants | green on tree, each mutant red with its message | not-run | |
| DUA-1 | dispatched run on `main` after merge | run succeeds under `RENOVATE_TOKEN` | post-merge | follow-up evidence PR |
| DUA-2 | guard T4, T5 and mutants | as above | not-run | |
| DUA-2 | a scheduled (cron) run on a day other than Monday | run exists, `event=schedule` | post-merge | follow-up evidence PR |
| DUA-3 | guard T6, T7 and mutants; run log shows the digest reference | as above; log names `@sha256:` | not-run | |
| DUA-4 | guard T8 and mutant | as above | not-run | |
| DUA-4 | dashboard issue after the first run on `main` | open issue by the token's user | post-merge | follow-up evidence PR |
| DUA-5 | guard T9, T10 and mutants; real validator run on the final config | as above; exit 0 | not-run | |
| DUA-5 | `lint` on the PR head runs the validator | step green | not-run | |
| DUA-7 | guard T11 and mutants; 0.4/2.4 lookup: only digest-only dockerfile/devcontainer branches carry `automerge` | as stated | not-run | |
| DUA-7 | first digest PR after merge is auto-merged by the bot after all checks green | merged, rebase | post-merge | follow-up evidence PR (needs an upstream digest) |
| DUA-8 | guard T12, T13 and mutants | as above | not-run | |
| DUA-9 | guard T14 and mutant; 2.4 lookup puts `release.yaml`'s helm in `renovate/helm` | as stated | not-run | |
| DUA-10 | guard T15 and mutant; docs read against the config | as stated | not-run | |
| all | CWS-6 enrolment, `ci_workflow_security_test.sh` both modes, CFC/DTP guards both modes | green | not-run | |
| all | independent review; CI on the PR head | APPROVE, green | not-run | |
