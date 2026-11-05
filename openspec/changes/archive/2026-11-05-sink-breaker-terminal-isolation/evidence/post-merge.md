# Post-merge evidence — sink-breaker-terminal-isolation

Verified 2026-11-05.

| Row | Check | Result |
|---|---|---|
| PR | Pull request #532 `:bug: fix(sink): keep one inventory's ownership conflicts out of the shared breaker` | all checks on head `6cc25f4b9` green or skipped (45 pass, 1 skipped); merge state CLEAN; merged by rebase |
| Merge | Head of `main` after the merge | `850df538d` (the bot's changelog sync followed as `ae1be083e`) |
| Main CI | Runs on `main` whose `headSha` is `850df538d` | 8 runs, all `success`: CI 42030314133, E2E smoke 42030314193, Docs 42030314421, Preflight 42030314186, CodeQL 42030314100, OpenSSF Scorecard 42030314223, Code Quality 42030312333, changelog-sync 42030314290 |
| Archive | `task spec:cli -- archive sink-breaker-terminal-isolation --yes` | see the archive commit; living spec `openspec/specs/sink-circuit-breaker/spec.md` gains SCB-1..SCB-3 |

Commands:

```sh
gh pr view 532 --json statusCheckRollup,mergeCommit
gh run list --branch main --limit 40 --json databaseId,headSha,name,status,conclusion \
  --jq '.[] | select(.headSha == "850df538d3a4fe19aae9044a6de076a944db6462")'
```
