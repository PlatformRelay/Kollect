# Design

## Context

See proposal.md. The bot runs with a maintainer token that is a `protect-main` bypass actor
(repository admin role, `bypass_mode: pull_request`), so what it may merge is a trust decision
and is recorded here.

## Decisions

- **The self-hosted Renovate is the bot.** Operator ruling 2026-10-26. It supersedes the
  2026-10-18 ruling for the hosted App, which never opened a PR here. The hosted App stays
  installed until the maintainer removes it. If it ever starts working on this repository, the
  two bots would race on the same `renovate/*` branches. The maintainer docs say so, and the
  removal is listed as an operator action.
- **No token fallback, least privilege for `GITHUB_TOKEN`.** Without `RENOVATE_TOKEN`, Renovate
  under `github.token` opens PRs that start no workflows and can never merge. Failing the run
  with an error naming the secret is louder and costs nothing. With the fallback gone, the job's
  `GITHUB_TOKEN` only checks out, so it gets `contents: read`.
- **Daily, one run at a time.** Renovate merges at most one branch per run and rebases a branch
  that is behind only during a run. Weekly runs left digest PRs blocked for a week behind every
  merge. A `concurrency` group without cancellation stops a manual dispatch from overlapping the
  scheduled run.
- **Pinned Renovate.** The job holds a token that can push to every branch, so the code it runs
  is pinned. `renovate-image` holds `ghcr.io/renovatebot/renovate:<version>@sha256:<digest>`. The
  action (v46.3.10 or later) uses it as given, and Renovate's own `github-actions` manager
  updates tag and digest together. Renovate releases several times a day, so that dependency gets
  a group of its own on a weekly schedule. It is never auto-merged: the next run would execute it
  with the token. The config validator runs from the same reference, so the validator and the
  bot cannot disagree.
- **Auto-merge: Docker image digests only.** Model-aligned decision 2026-10-26 (outcome D of four,
  2 of 3 legs), pending operator validation. Only `digest` updates of the `dockerfile` and
  `devcontainer` managers auto-merge. Today those are the golang builder and Debian base images
  and the devcontainer image. They are the bulk of the manual load and change no version. The
  rejected options:
  - Go module patches are compiled into the shipped binary, and a 7-day age is not enforcement.
  - GitHub native auto-merge under a bypass actor is unverified.
  - No auto-merge leaves the backlog.

  `group:` presets and `extends` that enable automerge are refused by the guard. A branch merges
  only when every upgrade in it may automerge, so a `go toolchain` PR that also moves the `go`
  directive stays manual.
- **Renovate merges, not GitHub (`platformAutomerge: false`).** Renovate merges only after every
  check reported on the head SHA is green, with `automergeStrategy: "rebase"` (the only method
  the ruleset allows). `rebaseWhen: "behind-base-branch"` is explicit, so a PR behind `main` is
  rebased and re-tested before it can merge. The residual risk: the merge runs as a bypass
  actor, so GitHub does not stop a merge when a required check never started (a GitHub outage, or
  a broken workflow file; digest bumps touch neither). Every required-check workflow triggers on
  every `pull_request` without a path filter (verified 2026-10-26). The release is tag-driven and
  needs the maintainer plus two environment approvals. So the maintainer docs ask the maintainer
  to review the bot merges since the last tag before approving a release.
- **No "red main" pause.** A bot PR runs the required checks on a head rebased onto `main`, so a
  red `main` already keeps it red. The exception is a failure in a workflow that does not run on
  PRs, such as a nightly job. That does not make a digest bump wrong. The pause script and its
  `gh` lookup were cut.
- **Validation in a required job.** `hack/ci/renovate-config-validate.sh` reads the image
  reference from `renovate.yaml` and refuses one without a digest. It then runs
  `renovate-config-validator --strict` on `renovate.json` and `.github/renovate-config.json`. The
  `lint` job runs it on every PR; `lint` is required and unconditional. Cost: one image pull
  (about 30 s, run log 39230872281).
- **Structural guard, not an evaluator.** No offline tool evaluates Renovate's merged rules
  without network lookups. `hack/test/renovate_policy_test.sh` therefore asserts the shape with
  `jq` and `yq`:
  - Exactly one `packageRules` entry sets `automerge: true`, with exactly the allowed matchers.
  - No top-level or preset automerge.
  - The automerge settings above.
  - The Renovate-image rule.
  - The dashboard is on.
  - The workflow's token, permissions, schedule, concurrency and pin.
  - The `lint` wiring.

  A local Renovate `lookup` probe (task 2.1) checks the rules against Renovate itself once.
- **Helm rule by `packageName`.** Both the custom regex manager (`depName` = `packageName` =
  `helm/helm`) and the `azure/setup-helm` input (`depName` `helm`, `packageName` `helm/helm`)
  then match one rule. This puts `release.yaml` back in the Helm group.

## Risks / Trade-offs

- [Bypass-identity merge without a started check] → digest-only scope; every required workflow
  runs on every PR; the release review duty is in the maintainer docs. Revisit when a bot
  identity without bypass exists.
- [A compromised upstream image digest] → digests cannot be aged. The release Trivy scan and the
  maintainer's release review are the backstop. The scope is limited to official base images.
- [Two bots if the hosted App wakes up] → operator removes the App; docs name the symptom.
- [Rule order or presets drift] → the guard fails on a second automerge rule, an automerge
  preset or a changed matcher.
- [Daily Renovate-image PRs] → weekly schedule for that group.
