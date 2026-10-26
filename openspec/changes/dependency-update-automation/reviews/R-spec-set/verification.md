# Stage R round 1: verification and triage

Spec set `abaea4694`. Reviewer A: BLOCK (3 CRITICAL). Reviewer B: BLOCK (1 CRITICAL). Each finding
was checked against the tree and, where it concerns Renovate, against Renovate 44.167.7's
installed source (`config-validator.js`, `modules/platform/github/index.js` `getBranchStatus`,
`workers/repository/updates/generate.js:249`, `workers/repository/update/branch/reuse.js`,
`workers/repository/update/pr/automerge.js`).

| ID | Finding | Sev | Raised by | Verified | Disposition |
| --- | --- | --- | --- | --- | --- |
| A-R1 / B-4 | T11's exact key set leaves out `automerge`, so a correct rule fails | CRITICAL | A, B | yes | fix: T11 lists `automerge` |
| A-R2 / B-1 | automerge can be enabled outside `packageRules` (top-level `patch`, `vulnerabilityAlerts`, per-manager blocks), by a remote preset, in the global config, or by a `RENOVATE_*` env override | CRITICAL | A, B | yes (schema allows each; `RENOVATE_FORCE` exists at `renovate.yaml:24`) | fix: T11 counts `automerge: true` at any depth in both files; T10 pins `extends` exactly; T13 fixes the global config's keys; T7 refuses `RENOVATE_*` env besides the repository name |
| A-R5 / B-10 | dispatch sets `RENOVATE_FORCE {"schedule":null}`, so manual runs ignore the image rule's weekly window; a schedule is a window, not a frequency | CRITICAL | A (B as note) | yes | fix: `RENOVATE_FORCE` removed (2.1, T7); DUA-3 says "weekly window" |
| A-R3 / B-11 | T4 accepts `0 4 * 1 *` (January only) | WARNING | A, B | yes | fix: T4 requires `<min> <hour> * * *` |
| A-R4 | T5 accepts per-run groups (`github.run_id`) | WARNING | A | yes | fix: T5 requires the literal group `renovate` |
| A-R6 / B-8 | merge-blocking cases (no checks, red, pending, behind) never verified; proposal cites the wrong tasks | WARNING | A, B | yes | fix: 0.4 (e) records them from the pinned Renovate's source; cross-references corrected. Read today in 44.167.7: no statuses and no check runs → pending; failure → red; queued, cancelled or timed out → pending; PR automerge requires green; branch automerge = every upgrade |
| A-R7 | a lookup on today's pins may propose no Helm or mixed branches | WARNING | A | yes | fix: 0.4 and 2.4 use a scratch copy with deliberately older pins for each case |
| B-2 | no task adds the validator step to `lint` | WARNING | B | yes | fix: 2.3 adds it |
| B-3 | T13 (now T9) as a text grep hits the comment at `ci.yaml:13` | WARNING | B | yes | fix: parsed `run:` bodies only; the comment is named as a must-not-count case |
| B-5 | T7/T14 (now T14/T15) ignore rule order; a later `groupName` wins | WARNING | B | yes | fix: order assertions |
| B-6 | T2 can be defeated by `if:`, `continue-on-error:` or a different env source | WARNING | B | yes | fix: T2 pins the step's env and refuses both keys |
| B-7 | the validator treats named files as global config unless `--no-global` | WARNING | B | yes (`config-validator.js:100-109`) | fix: T16 expects two calls, repo mode and global mode |
| B-9 | stale task cross-references in proposal and design | NIT | B | yes | fix |
| B-11 | T3 misses `permissions: write-all` | NIT | B | yes | fix: string permissions refused |
| B-12 | T6 takes the version from the comment; pin the exact SHA | NIT | B | yes | reject: an exact-SHA assertion would red the guard on every legitimate action bump PR from the bot; the floor plus the SHA format is kept, and 0.4 (d) records the v46.3.10 SHA for the reviewer |
| B-13 | verification rows: DUA-3 log row is post-merge; missing rows for the image PR and the behind case | NIT | B | yes | fix |
| B-14 | the new header comment must not contain `github.token` | NIT | B | yes | fix: T1 reads parsed values; 2.1 says so |

Task 1.1 grew past fifteen claims with these fixes and was split into 1.1 (workflow clauses) and
1.2 (config, validator and docs clauses).
