# Post-merge evidence — synced-debounce-condition

Verified 2026-10-25.

| Row | Check | Result |
|---|---|---|
| PR | Pull request #493 `:bug: fix(controller)!: report debounced sinks as synced` | all 46 checks on head `3eea0ead8` green or skipped (45 success, 1 skipped: `Deploy to GitHub Pages`), incl. required `breaking-marker` and `vulncheck`; merged by rebase |
| Merge | Head of `main` after the merge | `556f70d1f` (the bot's changelog sync followed as `7a7e1acd4`) |
| Main CI | Runs on `main` whose `headSha` is `556f70d1f` | 8 runs, all `success`: CI 40222532916, E2E smoke 40222532873, Docs 40222532884, Preflight 40222533118, CodeQL 40222532907, OpenSSF Scorecard 40222532980, Code Quality 40222533206, changelog-sync 40222532933 |
| Archive | `task spec:cli -- archive synced-debounce-condition --yes` | archived as `2026-10-25-synced-debounce-condition`; living spec `openspec/specs/inventory-sync-status/spec.md` gains SDC-1..SDC-7 in 17 requirement parts (the deltas were split on `main` in `44f7a240a`); Purpose written |

Commands:

```sh
gh pr view 493 --json statusCheckRollup,mergeCommit
gh run list --branch main --limit 60 --json databaseId,headSha,name,status,conclusion \
  --jq '.[] | select(.headSha == "556f70d1f3fe95cd6c537bbe197708745c31956e")'
```
