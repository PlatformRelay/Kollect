# Post-merge evidence

Change merged as #502 (`2cf23a200`, 2026-10-28T13:28:51Z); follow-up fix #506 (`c0e8629ab`,
2026-10-28T15:39:57Z, `evidence/post-merge-fix.md`). Verified 2026-10-29 with `gh run list`,
`gh pr view`, `gh issue view`, the GitHub check-runs API and the ruleset rule-suite log.

## CI on the PR head and on `main`

| What | Result |
| --- | --- |
| #502 head `16ba6e06c`, CI run 40693053492 | every check green; `lint` ran "Validate the Renovate config (DUA-5)" (success) and both guard modes |
| `main` @ `2cf23a200` (#502) | CI, Preflight, CodeQL, E2E smoke, Docs, Scorecard, changelog-sync: success |
| `main` @ `c0e8629ab` (#506) | all success |
| `main` @ `81c06cfcd` (bot merge of #503) | CI, Preflight, CodeQL, E2E smoke, Scorecard, changelog-sync: success |
| `main` @ `a2d2083f0` (bot merge of #504) | CI, Preflight, CodeQL, E2E smoke, Scorecard, changelog-sync: success |

## Rows

| Req | Check | Result | Evidence |
| --- | --- | --- | --- |
| DUA-1 | dispatched run on `main` | pass | run 40707869797 (`workflow_dispatch`, success, 2026-10-28T14:38Z): the token check step passed, Renovate ran with the secret; runs 40722087032 and 40727649978 likewise |
| DUA-2 | scheduled runs on a day other than Monday | pass | `gh run list --workflow renovate.yaml`: 40756078933 (2026-10-28T19:38Z, Wednesday), 40791388973 (2026-10-29T00:02Z), 40826205277 (05:55Z), 40879892658 (13:12Z), 40936034830 (19:00Z), all `schedule`, `success`. GitHub starts this repository's schedules hours late (first hourly run about 6 h after merge) |
| DUA-3 | the run pulls the digest reference | pass | run 40707869797 log: `docker run ... ghcr.io/renovatebot/renovate:44.179.1@sha256:f30ad41a40ab226734469eb61517d527a198a48b173cf6e0c3ee180927479293`, `"renovateVersion": "44.179.1"` |
| DUA-4 | dashboard issue after the first run | pass | #505 "Dependency Dashboard", open, author `konih` (the token's user), created 2026-10-28T14:41:40Z |
| DUA-5 | `lint` on the PR head runs the validator | pass | #502 CI run 40693053492, step "Validate the Renovate config (DUA-5)" success |
| DUA-7 | first digest-only PR merged by the bot after all checks green | pass | #503 (`docker images`: debian and distroless digests, "Automerge: Enabled") merged by the bot's token at 2026-10-28T16:39:55Z as `81c06cfcd`, by rebase, after its head `66a522c1a` had every check green (no cancelled check run); #504 (`go toolchain`: golang digest only) merged 2026-10-29T00:05:29Z as `a2d2083f0`. Rule suites 4774989774 and 4782351073: `required_status_checks` evaluated `pass`, not `bypass` |
| all | independent review; CI on the PR head | pass | branch review `reviews/B-branch/`, fixes reviewed in `reviews/L-tasks/G5/`; #502 head all green |

## Found after merge

The first two digest PRs could not auto-merge: Renovate's `dependencies` label started a second
E2E extended run that cancelled the first on the same head, and Renovate reads a cancelled check
as pending. Fixed in #506 with guard clause T18 (`evidence/post-merge-fix.md`); after Renovate
rebased both PRs onto the fix, each merged on its own.

## Still open (not a gate)

- Task 4.2: the maintainer removes the hosted Renovate App from the repository.
