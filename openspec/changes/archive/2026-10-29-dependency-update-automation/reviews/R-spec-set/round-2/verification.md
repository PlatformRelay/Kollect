# Stage R round 2 (last): verification and triage

Spec set `5e2f421c2`. Reviewer A: BLOCK (1 CRITICAL). Reviewer B: CONCERNS. Round 2 is the last
round: every verified finding is fixed in the spec set without a further review; stage B reviews
the result on the branch.

| ID | Finding | Sev | Raised by | Verified | Disposition |
| --- | --- | --- | --- | --- | --- |
| A-R2-1 / B-4 | a preset nested in a `packageRules` entry (`extends: [":automergeAll"]`) enables automerge without a literal `automerge: true`; T10 pins only the top-level `extends` | CRITICAL | A, B | yes (Renovate resolves nested presets; the schema allows `extends` in a rule) | fix: T10 refuses `extends` anywhere but the top level |
| A-R2-2 / B-3 | T7 misses workflow-level `env` and `GITHUB_ENV` writes | WARNING | A, B | yes | fix: T7 covers workflow, job and step env and refuses `GITHUB_ENV` in `renovate.yaml` run bodies |
| A-R2-3 / B-2 | `updateNotScheduled` defaults to true, so an open image PR is updated outside its window; a narrow window may never meet a run | WARNING | A, B | yes | fix: image rule `updateNotScheduled: false` and a whole-weekday window; T14 asserts both; 0.4 (a) checks the not-scheduled case |
| A-R2-4 / B-5 | T13 cannot be red on today's tree | WARNING | A, B | yes (`.github/renovate-config.json` has exactly those keys) | fix: T13 green today, proven by its mutants |
| A-R2-5 | 0.4's fixtures need not produce a mixed branch | WARNING | A | yes | fix: an older golang digest plus an older `go` directive form one `go toolchain` branch; 0.4 (c) shows its upgrades, then `automerge: false` |
| B-1 | one automerge per run plus a daily run starves a digest PR on busy days (rebased, pending, behind again next day); T4 forbade an hourly cron | WARNING | B | yes (`main` moved on 11 of 21 days, 10-05 to 10-25) | fix: hourly cron; T4 accepts `*` and `*/N` hours; `rebaseWhen` `behind-base-branch` only on the automerge rule and `conflicted` at the top level, so hourly runs do not rebase PRs that wait for a human |
| B-6 | design claims "official base images" without enforcement | NIT | B | yes | fix: design states the scope is every image the Dockerfiles and devcontainer name |
| B-7 | T9 misses `actions/github-script` merges; T9 and DUA-8 disagree on `pull_request_target` | NIT | B | yes | fix: T9 scans `with.script` for `pulls.merge`; DUA-8 forbids `pull_request_target` outright |
| B-8 | T16 fixes argv before 0.5 settles mount, workdir and entrypoint | NIT | B | yes | fix: T16 judges image, command and flags; the mount contract follows 0.5 |
