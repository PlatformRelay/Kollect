# Post-merge evidence — scheduled-vulncheck

Verified 2026-11-16.

| Row | Check | Result |
|---|---|---|
| PR | Pull request #558 `:construction_worker: ci: run govulncheck daily on main and report it` | 40 checks pass, 3 skipped; merge state CLEAN; merged by rebase |
| Merge | Head of `main` after the merge | `1c8f0ea72` |
| Main CI | Runs on `main` whose `headSha` is `1c8f0ea72` | all `success`: CI 43867444894, E2E smoke 43867444774, Docs 43867444880, Preflight 43867444666, CodeQL 43867444864, OpenSSF Scorecard 43867444906, Code Quality 43867442967, changelog-sync 43867444756 |
| Smoke (2.1) | `gh workflow run vulncheck-scheduled.yaml --ref main` | run 43867485889 `success`; step `govulncheck, one line per advisory` logged `No vulnerabilities found.`; its log lines carry the `vulncheck<TAB>govulncheck, one line per advisory<TAB>` prefix (the B round-1 finding, observed live) |
| CFR-6 | CI failure report run for that dispatched run | 43867576891 `skipped` (only scheduled runs report) |
| SVS-2 in production | First scheduled run (cron `41 4 * * *`) and the report it triggers | not yet observed at archive time: handed to the next session's handoff (first run 2026-11-17 04:41 UTC) |

Commands:

```sh
gh run list --branch main --limit 40 --json databaseId,headSha,name,status,conclusion \
  --jq '.[] | select(.headSha == "1c8f0ea72faf816d0a6c32129b81bc582e170b36")'
gh run view 43867485889 --log | grep 'No vulnerabilities'
```
