# Stage B round 2: verification

Round 2 ran because a round-1 leg reported a P0. It doubles as the review of the round-1 fixes
(tasks.md 5.1 to 5.6), since the guard and CI changed: legs `leg-astra-adversarial.md` (brief
`brief-astra.md`), verdict REQUEST_CHANGES, and `leg-claude-opus-diff.md`
(`fanout.sh --legs claude/opus:diff --no-unify`), verdict CONCERNS. Reviewed diff:
`f780d5966..4d5209acf`. Neither leg reported a P0 or a CRITICAL.

Round-1 findings, closed or not (both legs agree, and the orchestrator checked):

| Round-1 id | Closed? | Note |
|---|---|---|
| A1 / O1 | in part | absolute paths gone from `task-prompt.md` and `loop.md`; reviewer and model names in records and in two commit messages stay, for the operator |
| A2 | yes | `docs/development/setup.md` warns before pulling |
| A3 | yes | empty path regex now reds, with a mutant |
| A4 | in part | four-plus aliases closed; negated aliases were still missed (R2-1 below) |
| A5 / O2 | yes | ripgrep unpinned; the step still fails loudly and sits before both guard steps |
| O3 | in part | the three named forms closed; siblings remained (R2-2, R2-3 below) |

New findings, each checked against the tree:

| Id | Finding | Legs | Verified? | Disposition |
|---|---|---|---|---|
| R2-1 | `Host * !name` (ssh_config negation) passes PAC-6: the alias class has no `!` | astra P1, opus NOTE | yes: the new mutant survived before the fix | fixed, `ca5854194` (tasks.md 5.7), with a mutant |
| R2-2 | Bare `/root` before punctuation or a backtick passes | astra P1, opus NOTE | yes: the mutant survived before the fix | fixed, `96d4ec716` (tasks.md 5.8) |
| R2-3 | `/mnt/c/Users/<u>`, `/c/Users/<u>` and an escaped `C:\\Users\\<u>` pass; the forward-slash Windows form has no mutant | opus NOTE | yes: three of the four mutants survived before the fix; `c:/users` was already caught and gets a pin | fixed, `96d4ec716` (tasks.md 5.8) |
| R2-4 | The unbounded Host pattern reds a prose line that starts with "host", and the only control for it was rewritten | opus NOTE | yes (accepted when fixing A4: a leak guard fails closed) | pinned: an `expect_red` control records the accepted false red, `ca5854194` |
| R2-5 | tasks.md had no section 5 for the fix ids the round-1 record cites | opus WARNING | yes | fixed: tasks.md section 5 |
| R2-6 | No sensor run recorded on the fix head | opus WARNING | yes | fixed: the run below |
| R2-7 | Reviewer and model names remain in `loop.md` and `task-prompt.md` | astra P2 | yes, as recorded in round 1 | for the operator, with A1 |

No further review round, as the stage-B rules require. Each round-2 fix went red first (the new
mutant survived on the unfixed guard) and green after, and the full self-test was green at each
commit.

## Sensor run on the final code revision `96d4ec716`

| Sensor | Result |
|---|---|
| `bash hack/test/agent_contract_test.sh` | exit 0, running-host check active |
| `bash hack/test/agent_contract_test.sh --self-test` | exit 0, 150 mutants and controls ok, 105 s |
| `bash hack/test/ci_workflow_security_test.sh` and `--self-test` | exit 0 both (12 s, 163 s) |
| `bash hack/test/repo_root_links_test.sh` | exit 0 |
| `bash hack/test/docs_launch_truth_test.sh` | exit 0 |
| `bash hack/test/task_check_test.sh` | exit 0 |
| `task lint:shell` | exit 0, no warning |
| `task spec:validate` | exit 0, 14 passed |
| `zizmor --offline --min-severity=high` | no findings |
| `gitleaks detect --no-git` with `.github/gitleaks.toml` | no leaks |

## CRITICAL

None open. The operator question (whether review records that name reviewers and models belong in
the public tree, and whether to reword the two spec commits whose messages name reviewer models)
is not a defect in the change and does not block 3.1.
