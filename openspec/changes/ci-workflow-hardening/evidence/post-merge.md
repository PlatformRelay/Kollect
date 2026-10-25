# Evidence — ci-workflow-hardening (post-merge rows)

Recorded 2026-10-25 against `PlatformRelay/Kollect` after #450 merged (2026-10-06,
`3ee21266`). Every row below cites real GitHub Actions runs; all calls are read-only except
the one throwaway probe PR in the CWS-3 section, which was closed unmerged and its branch
deleted.

## CWS-1 — `workflow-security` green on real PR heads

```bash
gh pr checks 494 -R PlatformRelay/Kollect | grep workflow-security
gh pr checks 495 -R PlatformRelay/Kollect | grep workflow-security
gh api repos/PlatformRelay/Kollect/actions/runs/<id> -q '[.id,.event,.head_sha,.conclusion]|@tsv'
```

| PR | PR head | CI run | event | `workflow-security` |
| --- | --- | --- | --- | --- |
| #494 | `13c70fb568d98f9cf8f2684fa5e21ef261d61c91` | 40223387529 | `pull_request` | pass (job 121259583355) |
| #495 | `6e18b5abee9dcb5b84bc3ef4c7052697825b9d6b` | 40227161572 | `pull_request` | pass (job 121270924131) |

Each run's `head_sha` equals the PR head, so the job ran on the PR head, not on a merge
commit. The red side (template-injection fixture, no-`--offline` and skip-switch mutants) is
in `evidence.md`.

**Result: pass.**

## CWS-3 / task 4.2 — `dependency-review` red on a disallowed licence and on a vulnerable module

```bash
gh api --allow-escape-sequences repos/PlatformRelay/Kollect/actions/jobs/<job>/logs \
  | sed -e 's/\x1b\[[0-9;]*m//g' | grep -E 'incompatible|vulnerab|License:|GHSA|##\[error\]'
```

### Disallowed licence (real PRs, no probe needed)

| PR | Run / job | Dependency flagged | Log line |
| --- | --- | --- | --- |
| #470 | 39170622155 / 117802707914 | `go-task/setup-task@a00fbb05…` (Action) | `License: GPL-3.0` → `Dependency review detected incompatible licenses.` |
| #489 (Dependabot, open) | 39987119032 / 120529075026 | `pillow@13.0.0` (docs) | `License: MIT-CMU` → `Dependency review detected incompatible licenses.` |
| #457 (Dependabot) | 37910176591 / 113753166938 | `cairosvg@2.9.1` (docs) | `License: LGPL-3.0-or-later` |
| #456 | 37907543188 / 113744549550 | `golang.org/x/net@0.60.0` | `BSD-3-Clause AND LicenseRef-scancode-google-patent-license-golang` |

In every one of these runs the Vulnerabilities group reports `did not detect any vulnerable
packages`, so the red is the licence policy alone. Each of the last three was resolved with a
package-scoped `allow-dependencies-licenses` exception (with a reason in `ci.yaml`), not by
widening `allow-licenses`; #489 is still red today.

### Vulnerable module (throwaway probe)

No real PR had gone red on a vulnerability, so one throwaway draft PR was opened:

- PR #496, branch `probe/dependency-review-vulnerable`, one commit `a9c1adca` that pins
  `golang.org/x/net` back from v0.60.0 to v0.17.0 in `go.mod` (no other change; x/net's licence
  is already excepted, so only the vulnerability check can fire).
- Run 40231428226 (`pull_request`, head `a9c1adcadb04083a79d6022a15276913b43760c2`),
  `dependency-review` job 121283803587: **fail**.

```text
go.mod » golang.org/x/net@0.17.0 – net/http, x/net/http2: close connections when receiving too many headers (moderate severity)
  ↪ https://github.com/advisories/GHSA-4v7x-pqxf-cx7m
go.mod » golang.org/x/net@0.17.0 – HTTP Proxy bypass using IPv6 Zone IDs in golang.org/x/net (moderate severity)
  ↪ https://github.com/advisories/GHSA-qxp5-gwg8-xv66
go.mod » golang.org/x/net@0.17.0 – golang.org/x/net vulnerable to Cross-site Scripting (moderate severity)
  ↪ https://github.com/advisories/GHSA-vvgc-356p-c3xw
go.mod » golang.org/x/net@0.17.0 – Go Net HTML parser is vulnerable to denial of service (moderate severity)
  ↪ https://github.com/advisories/GHSA-5cv4-jp36-h3mw
##[error]Dependency review detected vulnerable packages.
```

PR #496 was closed without merging and its branch deleted (`gh pr close 496 --delete-branch`).

**Result: pass** — red on a disallowed licence (#470, #489) and on a vulnerable module (#496).

## CWS-4 — a newer push cancels the PR's in-flight run

`ci.yaml` groups by `${{ github.workflow }}-${{ github.event.pull_request.number || github.sha }}`
with `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`.

```bash
gh run list -R PlatformRelay/Kollect --workflow ci.yaml --event pull_request \
  --json databaseId,headBranch,headSha,conclusion,createdAt
gh api repos/PlatformRelay/Kollect/check-runs/121263420724/annotations -q '.[].message'
```

PR #495 (`docs/archive-synced-debounce-condition`), three pushes:

| Run | Head | Started | Ended | Conclusion |
| --- | --- | --- | --- | --- |
| 40224677510 | `f8a96158` | 19:09:29Z | 19:20:50Z | cancelled |
| 40225572307 | `1f5a0487` | 19:20:34Z | 19:40:59Z | cancelled |
| 40227161572 | `6e18b5ab` | 19:40:41Z | — | success |

Each run ends seconds after the next push's run starts. The cancelled job in run 40224677510
carries the annotation `Canceling since a higher priority waiting request for CI-495 exists`,
naming the PR-keyed group.

**Result: pass.**

## CWS-4 — quick successive merges to `main` each get their own run

The five merges of 2026-10-25 landed within about two and a half hours (17:29 to 20:05
UTC). Filtered by `headSha` (not `--commit`, which can return zero rows):

```bash
gh run list -R PlatformRelay/Kollect --workflow ci.yaml --branch main --limit 60 \
  --json databaseId,headSha,event,conclusion
gh run list -R PlatformRelay/Kollect --workflow e2e-smoke.yaml --branch main --limit 40 \
  --json databaseId,headSha,event,conclusion
```

| Merge | Committed (UTC) | `CI` run | `e2e-smoke` run |
| --- | --- | --- | --- |
| `d2ba2edbb` | 17:29:34 | 40216722422 push success | 40216722466 push success |
| `556f70d1f` | 18:42:33 | 40222532916 push success | 40222532873 push success |
| `44f7a240a` | 19:14:25 | 40225080731 push success | 40225080828 push success |
| `1353f7772` | 19:38:09 | 40226966266 push success | 40226966351 push success |
| `463c9401f` | 20:05:28 | 40229120503 push success | 40229120473 push success |

Every SHA has exactly one push run of each workflow and none was cancelled.

**Result: pass.**

## CWS-6 / task 5.1 — required checks in `protect-main`

```bash
gh api repos/PlatformRelay/Kollect/rulesets/18802410 \
  -q '.rules[]|select(.type=="required_status_checks")|.parameters.required_status_checks[].context'
gh api repos/PlatformRelay/Kollect/rulesets/18802410/history/<version_id> -q '.updated_at'
```

Ten required contexts today: `preflight`, `test`, `kind-smoke`, `Analyze (Go)`, `lint`,
`workflow-security`, `dependency-review`, `tool-pins-selftest`, `vulncheck`, `breaking-marker`.
The ruleset history shows when the four came in: `lint`, `workflow-security` and
`dependency-review` in version 53247538 (2026-10-15), `vulncheck` in version 53474785
(2026-10-16).

**Ruleset half: pass.**

### Skipped required job counts as passing — not proven

Which required jobs can be skipped on a PR:

- `test` and `kind-smoke` are `if: always()` aggregators; `lint`, `workflow-security`,
  `tool-pins-selftest`, `preflight`, `breaking-marker` and `Analyze (Go)` have no job-level
  `if:`; `dependency-review` is `if: github.event_name == 'pull_request'`, so it always runs on
  a PR.
- `vulncheck` is `if: needs.changes.outputs.code == 'true'`, so it **is skipped on every
  docs-only PR**.

Observed on real PRs (`gh pr checks <n>`, and GraphQL `isRequired(pullRequestNumber:)`):

- #489 (Dependabot, docs-only, open): `vulncheck=SKIPPED` while required (run 39987119032).
  The PR is also blocked by the red `dependency-review`, so its merge state cannot show
  whether the skipped `vulncheck` alone would block.
- #462 (docs-only): `vulncheck` skipped and the merge (`0c1456b5`, 2026-10-14) passed the
  ruleset (rule suite 4517903794, `pass`), but `vulncheck` only became required on
  2026-10-16, so that merge is no proof either.
- A scan of every PR merged from 2026-07-17 (first ruleset version with required checks) to
  2026-10-25 found no other merged PR with a skipped check that was required at merge time.

So the case is real (docs-only PRs skip a required job), but no observation yet shows GitHub
treating that skipped job as passing. The probe stays open until a docs-only PR with
otherwise green checks reaches a mergeable state (or does not).

**Result: open.** Task 5.1 stays unticked.

## Task 5.2 — `require_extra_approval_for_unattributed_changes`

```bash
gh api repos/PlatformRelay/Kollect/rulesets/18802410 \
  -q '.rules[]|select(.type=="pull_request")|.parameters'
gh api "repos/PlatformRelay/Kollect/rulesets/rule-suites?ref=refs/heads/main&time_period=month"
```

The `pull_request` rule has `require_extra_approval_for_unattributed_changes: true`, with
`required_approving_review_count: 0`, `require_last_push_approval: false` and
`allowed_merge_methods: ["rebase"]`.

It has not affected bot PRs in practice. Each of these merged with zero reviews and its push to
`main` evaluated as `pass` (not `bypass`) in the rule-suite log:

| PR | Author of the commits | Merge | Rule suite |
| --- | --- | --- | --- |
| #475 (Renovate, Go toolchain digest) | `renovate-bot <renovate@whitesourcesoftware.com>` | `cf448c4a` | 4618177030 pass |
| #476 (Renovate, openspec 1.14.3) | `renovate-bot <renovate@whitesourcesoftware.com>` | `1353f777` | 4713611399 pass |
| #457 (Dependabot, cairosvg) | `dependabot[bot]`, plus one maintainer commit | `3f17512f` | 4460537403 pass |

Dependabot PRs also get the normal check set (#489's list includes every required context).

**Result: pass** — recorded: `true`; no observed effect on Renovate or Dependabot PRs.
