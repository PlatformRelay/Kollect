## Verdict: CONCERNS

## Findings
- [WARNING] Two documents say only auto-merge PRs are rebased when behind, and a mixed group branch can break that — `openspec/changes/dependency-update-automation/design.md:25`, `docs/development/tooling-setup.md` ("Rebasing a bot PR")
  Failure: a group branch such as `docker images` or `go toolchain` can hold a dockerfile digest upgrade plus a tag or `go` change. If the digest upgrade sorts first, the branch takes its `rebaseWhen: behind-base-branch` (evidence/2.4.md:124-130). The branch is then rebased, and CI re-runs, every hour that `main` moves, but it never auto-merges, because `automerge` is recomputed over all upgrades. My judgement: this is a cost and accuracy problem, not a breach of DUA-7 or DUA-8. DUA-8 only asks for the literal settings, and they hold (`renovate.json:7,169`). No merge results, and with the strict up-to-date ruleset the extra rebase even keeps the PR mergeable for a human. It is unlikely today: "go" sorts before "golang", and the observed mixed branches resolved to `conflicted`.
  Fix: reword both sentences to "auto-merge PRs, and a mixed group whose first upgrade is a digest, are rebased when behind", and record this as accepted cost. Keep the T11/T12 contract. Do not put `rebaseWhen: conflicted` on the group rules: the digest rule comes later and wins for digests anyway, and the `auto` value would mean rebasing every PR under the strict ruleset.
  Confidence: 75
- [NOTE] The automerge rule matches every devcontainer dependency, including features, not only images — `renovate.json:166-167`, `.devcontainer/devcontainer.json:4-12`
  Failure: if someone pins a devcontainer feature by digest (for example `docker-in-docker:2@sha256:…`), its digest bumps auto-merge. Features are scripts that run in a `--privileged` container, and DUA-7 says "image digests". This cannot happen today because the features carry no digest.
  Fix: add `matchDepTypes: ["image"]` to the rule and to T11's exact key set (`hack/test/renovate_policy_test.sh:587`), or state in DUA-7 that features are in scope.
  Confidence: 55
- [NOTE] T7 refuses only `RENOVATE_*` variables. The action's default env passthrough also forwards `NODE_OPTIONS` and `HTTPS_PROXY`/`NO_PROXY` into the Renovate container — `hack/test/renovate_policy_test.sh:386-424`
  Failure: a workflow-level `NODE_OPTIONS: --import data:…` or a proxy variable passes the guard. It would run injected code inside the pinned image, or route traffic carrying `RENOVATE_TOKEN` through a proxy. This needs a reviewed workflow edit, but catching what review misses is T7's job.
  Fix: in T7, also refuse `NODE_OPTIONS`, `GITHUB_COM_TOKEN` and `*_PROXY`/`*_proxy` keys at workflow, job and step level, with one mutant.
  Confidence: 60
- [NOTE] `workflow_dispatch` can run `renovate.yaml` from any branch with the bypass-actor `RENOVATE_TOKEN` — `.github/workflows/renovate.yaml:13,48`
  Failure: anyone with write access can push a branch that edits the workflow and dispatch it. The token, which bypasses the `main` ruleset, then runs under unreviewed steps. Repository secrets already reach push-triggered workflows, so this is defence in depth, not a new hole.
  Fix: move the secret into an environment with a `main`-only deployment-branch policy, and set `environment:` on the job.
  Confidence: 50
- [NOTE] The spec's wording is stricter than the code it accepts. DUA-3 says "no `RENOVATE_*` override besides the repository name", but the token-check step sets `RENOVATE_TOKEN` (`renovate.yaml:35`), and T7 allows that on purpose (`renovate_policy_test.sh:401-411`) — `specs/dependency-updates/spec.md:54`
  Fix: add "and the token check step's `RENOVATE_TOKEN`" to DUA-3.
  Confidence: 80
- [NOTE] `CONTRIBUTING.md:139` still allows "Squash and merge", while the ruleset (tasks.md 0.1) and AGENTS.md allow rebase only. The same paragraph names changelog-sync as the only bypass and does not mention the Renovate token. This is outside DUA-1..10, but it is a one-line fix and it describes the merge authority this branch changes, so fix it here or in a follow-up — `CONTRIBUTING.md:139`
  Confidence: 85

**Compliance**
- DUA-1 holds (`renovate.yaml:15-16,26-27,33-40,48`, T1/T2/T3; App unused is documented).
- DUA-2 holds (`:12-13,18-20`, T4/T5).
- DUA-3 holds (`:44,47`, `renovate.json:151-158`; the image is extracted by `github-actions`, so the digest rule cannot match it), with the wording NIT above.
- DUA-4 holds (`renovate.json:3`, T10). Nothing sets `minimumReleaseAge`, so "awaiting-age" never shows on the dashboard.
- DUA-5 holds (`renovate-config-validate.sh:32-46`, `ci.yaml:597-598`, T8/T16).
- DUA-7 holds; the devcontainer rule is broader than "image" (NOTE above).
- DUA-8 holds (`renovate.json:4-7,169`, T9/T12).
- DUA-9 holds (`renovate.json:194-197`, T15).
- DUA-10 holds: every listed topic is in the docs section, and T17 checks it lexically only.

**Code behaviour no requirement mentions**
- Debug logging on manual runs (`renovate.yaml:31`).
- `--network none` and the caller's uid for the validator.
- A rewritten SCA policy table (`sca-remediation-policy.md:109-117`).

**Unreviewed fixes re-checked**
- T10 nested `extends` (`:552-557`), T7 workflow-level env and `GITHUB_ENV` (`:388,420`), T14 `updateNotScheduled` and whole-weekday window (`:698-699`), T13 exact keys (`:653`), the hourly cron with T4 accepting `*`/`*/N` (`:299`), and the T6 `configurationFile` rule (`:374`) are all sound.
- The 1.2 message rewording (`:819`) is accurate.
- Renovate reads `renovate.json` before any other repo config file name, and T13 locks `inheritConfig` out, so no shadow config can take over.

**Fitness**
- The branch tightens supply-chain pinning (the image is pinned by digest, and its bumps are scheduled and never auto-merged).
- It grows the CI gate set (three `lint` steps, enrolled in CWS-6 at `ci_workflow_security_test.sh:622`). Enrolling a guard narrows nothing, and no allow-list or baseline was widened.
- It widens merge authority: the bot now merges digest PRs, and that is guarded by T11/T12 plus 144 self-test mutants.
- Gap: no function guards the branch-level `rebaseWhen` outcome. The smallest worthwhile ratchet is a 2.4-style trace check, outside CI, that no `automerge: false` branch resolves to `behind-base-branch`.

## Could not check
- Renovate 44.179.1 source (`generate.js` sort order, `reuse.js`, devcontainer feature depType): it is not in this repository, so I relied on recorded evidence.
- I did not run any guard, self-test, validator or `task check`; the holistic result is taken from the brief.
- Live ruleset and secret settings; the earlier review `.err` transcripts.
