# Design

## Context

See proposal.md. Credentials and merge authority are involved, so the trade-offs are recorded.

## Decisions

- **A separate App, never the changelog-sync App.** That App bypasses `protect-main` (always) so
  it can push the regenerated changelog. A Renovate token with bypass rights could merge a red
  bot PR. The Renovate App has contents, pull-requests and workflows write and NO ruleset bypass,
  so required checks apply to it like to anyone else.
- **Renovate's own automerge with `automergeStrategy: "rebase"`**, not a separate workflow that
  runs `gh pr merge --rebase`. One system owns the decision (scope, age, strategy) in
  `renovate.json`, and it uses GitHub auto-merge, so the merge waits for required checks.
  Alternative: a workflow on `pull_request_target` calling `gh pr merge --auto --rebase` for
  bot authors; rejected because `pull_request_target` with a bot author filter is a known
  injection surface (the zizmor audit in `ci-workflow-hardening` flags it) and splits the policy
  across two files. Rebase because the workspace merge policy is rebase-merge, linear history,
  never squash or merge commits.
- **Scope: `patch` and `minor` of `gomod` and the `custom.regex` pinned-tools group; `patch`,
  `minor`, `digest` and `pinDigest` of `github-actions`.** Major updates, `k8s.io/` and
  `sigs.k8s.io/` modules, the Dockerfile base image and the `go` directive are reviewed by a
  human: they change behaviour or compatibility.
- **Deny rules come last and are asserted structurally.** There is no offline evaluator of the
  final merged rule set. Instead the config puts the deny rules (major, `k8s.io/`, dockerfile,
  go directive) after every allow rule with `automerge: false` (later `packageRules` win), and the
  guard asserts with `jq` that those rules exist, set `automerge: false`, and are the last
  entries. Adding a rule after them fails the guard, which is the intended friction.
- **Pinned-tool automerge stays**, which is why `developer-toolchain-consistency` asserts a
  version floor and agreement between sites, not exact values (an exact-value test would make
  every bot PR red).
- **Cooldown 7 days** (`minimumReleaseAge`) so a compromised or yanked release is usually
  caught upstream first; security advisories are raised by the existing sweep, outside this
  config.
- **Weekly schedule stays**; the cooldown, not the cron, bounds freshness.

## Risks / Trade-offs

- [A required check that is green but weak merges a bad bump] -> scope limits to patch/minor,
  the cooldown, and the release gate (`verify-eligibility.sh` needs green exact-SHA checks on
  main before any release). Revisit the scope after the first month.
- [Auto-merge of GitHub Actions bumps runs new action code with `contents: read` on main] ->
  digest pins plus the cooldown; `zizmor` and harden-runner audit (separate change) add visibility.
- [App private key in repository secrets] -> installation tokens are short-lived (1 hour) and
  the App has no bypass; key rotation is operator-owned.
