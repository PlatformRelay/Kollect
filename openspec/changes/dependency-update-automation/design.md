# Design

## Context

See proposal.md. Credentials and merge authority are involved, so the trade-offs are recorded.

## Decisions

- **A separate App, never the changelog-sync App.** That App bypasses `protect-main` (always).
  The Renovate App has contents, pull-requests and workflows write and NO bypass, so required
  checks apply to it like to anyone. Identity is compared by App slug, not by secret name: the
  workflow fails when the minted token's `app-slug` output equals a repository variable naming
  the changelog-sync App's slug (set by the operator).
- **Renovate merges itself (`platformAutomerge: false`).** GitHub's auto-merge waits only for
  required checks; Renovate's own merge waits for every check run on the head SHA. A separate
  `pull_request_target` workflow calling `gh pr merge` was rejected: it is a known injection
  surface and splits the policy across two files; it is forbidden by DUA-7.
- **Scope is Go module patch/minor only.** Anything that executes inside a job holding a
  privileged token or inside release and publish workflows is excluded: that is every GitHub
  Action (checkout, setup-task and the App token action run in `changelog-sync.yaml`; most run in
  `release.yaml`), every pinned tool (Task and git-cliff run in `changelog-sync.yaml`, Helm in
  release), and Dockerfile images. Digest and pinDigest updates are excluded because they have no
  release timestamp to age, and are never "exempted" from the age rule. The `go` directive and
  Kubernetes modules change compatibility.
- **Deny rules come last and are asserted structurally.** There is no offline evaluator of the
  final merged rules; the config puts the deny rules after every allow rule with `automerge:
  false` (later `packageRules` win) and the guard asserts with `jq` that they exist and are last.
- **Main must be green.** Before running Renovate the workflow reads the latest push run of
  `ci.yaml` on `main`; when it is not `success`, the run sets `automerge: false` through
  `RENOVATE_FORCE`, so a bot cannot pile merges onto a red main.
- **Daily runs, fewer larger groups.** Under a strict up-to-date rule and a weekly cron, bot PRs
  sit BLOCKED behind each merge and CI re-runs on every rebase; the 7-day cooldown ages updates
  but does not bound freshness. A daily run, `rebaseWhen: "behind-base-branch"`, `prCreation:
  "not-pending"`, one group for non-Kubernetes Go modules and one for Kubernetes modules keep the
  PR count and the rebase churn low.
- **Privileged token isolation in changelog-sync.** The checkout does not persist credentials; the
  App token reaches only the push step. A compromised tool in an earlier step then has no token.

## Risks / Trade-offs

- [A green but weak check merges a bad Go bump] -> scope, cooldown, and the release gate need
  exact-SHA green checks on main; revisit the scope after a month.
- [App private key in repository secrets] -> installation tokens last an hour; rotation is
  operator-owned.
- [Rule order is fragile] -> the guard fails on any rule added after the deny rules.
