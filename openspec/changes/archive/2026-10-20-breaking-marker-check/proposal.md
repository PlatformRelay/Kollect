# Proposal

## Why

Pull requests land by rebase merge, so git-cliff (`hack/release/cliff.toml`) reads the commits
and never the pull request. A `!` in the PR title, or a `BREAKING CHANGE:` footer in the PR body,
never reaches the changelog. On a commit, git-cliff renders only the title: a footer flags the
entry `[**breaking**]` but its text is dropped. The project rule (`CONTRIBUTING.md`) is therefore the `!` on the
commit title, and this change checks titles only.
Four breaking changes since v0.21.0 arrived unmarked this way: #455, #478, #480 and #410. #478 is
the clearest case: its title was `:lock: fix(sink)!: …` and its body ended with a
`BREAKING CHANGE:` footer, but every one of its commits had a plain title.

## What Changes

- New script `hack/check-breaking-marker.sh <base> <head>`, reading the PR title and body from
  the environment (`PR_TITLE`, `PR_BODY`) and the commit titles from `git log`. When the PR title
  has a conventional `type!:` marker, or the PR body has a `BREAKING CHANGE:` footer line, at
  least one commit in the range must carry `!` in its title, or the script fails. A commit with
  `!` under a PR title without one is reported as a warning only.
- New workflow `.github/workflows/breaking-marker.yaml`, job `breaking-marker`, on
  `pull_request` (opened, edited, synchronize, reopened). Read-only token, no secrets, pinned
  checkout without persisted credentials; the PR text reaches the script through `env:`, never
  through `${{ }}` inside `run:`. It is **not** added to the required checks.
- New guard `hack/test/breaking_marker_test.sh`: fixture cases (including #478's title and
  footer) run against the script on throwaway repositories, a static check of the workflow's
  shape, and `--self-test` mutants. Both modes run in the required `lint` job and are named in
  CFC-6 (`cfc_guards` in `hack/test/ci_workflow_security_test.sh`).
- `CONTRIBUTING.md`: one line saying the marker must be on a commit title.

## Capabilities

### New Capabilities

- `breaking-change-marking`: how a pull request's breaking-change declaration is held to the
  commit titles that git-cliff reads.

### Modified Capabilities

None.

## Impact

- Entry point: the `breaking-marker` job of `.github/workflows/breaking-marker.yaml`, reported as
  a check on every pull request. It blocks nothing until the operator adds it to `protect-main`.
- No secrets, no write permission, no third-party action beyond the pinned `actions/checkout`.

## Assumptions

- git-cliff marks a commit breaking from its title's `!` (the `^[a-z]+(\(.+\))?!:` parser in
  `hack/release/cliff.toml`) or from a body footer, and renders no footer text; a title its
  preprocessor `^:[a-z0-9_]+:` plus one space does not strip fails to parse and is skipped. Probed with git-cliff
  v2.13.1 and the repository's `cliff.toml` on a scratch repository (review round 1); the #478
  entries in `CHANGELOG.md` `[Unreleased]` are plain Features.
- On `pull_request`, `actions/checkout` checks out the PR merge commit, whose first parent is the
  base tip and second parent the PR head, so `HEAD^1..HEAD^2` is exactly the PR's commits. The
  `preflight` commit identity step relies on the same fact.
- `github.event.pull_request.title` and `.body` are in the payload of the `opened`, `edited`,
  `synchronize` and `reopened` activity types; `body` is null for an empty body and becomes an
  empty string in `env:`. Source: GitHub "Events that trigger workflows", `pull_request`.

## Non-goals

- Rewriting commit titles or the changelog, or checking commit bodies.
- Validating the conventional-commit format of titles in general.
- Making the check required. The operator decides that after it has run on real PRs.
