## Verdict: CONCERNS

## Findings
- [WARNING] The commit puts a red guard on the branch by itself, which breaks this change's own rule that the tree is green at every commit. — `openspec/changes/public-agent-contract/tasks.md:5-7`, `tasks.md:181`, `hack/check.sh:88-104`
  Failure: `hack/check.sh` picks up every `hack/test/*_test.sh` with no edit. So from `ec966638d` on, `task check` fails twice: the plain run, and `--self-test`, which takes about 13 s and fails because its no-op copy has no `AGENTS.md`. The repo merges by rebase, so this red commit would reach `main` as it is. tasks.md still says G2 "(1.1 and 2.1-2.4) closes in one commit", which this commit now contradicts. The split is recorded only in `evidence/G1.md:223` and `loop.md` (Known red); tasks.md was never changed to match.
  Fix: before the PR, fixup this commit into the G2 commit. Or else amend the tasks.md preamble and the §2 heading to describe the split and record the exception in `decisions.md`.
  Confidence: 90
- [WARNING] Temp directories created inside `$(...)` are never removed: the `SCRATCH+=` append happens in a subshell, so the parent's list never sees it. — `hack/test/agent_contract_test.sh:682,694,789,791`
  Failure: each `--self-test` run (and each `task check`) leaves one fake-`rg` directory and one shadow directory of symlinks per PATH entry holding `rg` in `$TMPDIR`. The cleanup trap only removes what is in the parent's `SCRATCH`. `evidence/G1.md:193-195` claims "a trap removes the rest" and "No state through `$(helper)`: yes". Both claims are wrong for these helpers.
  Fix: give the helpers a parent directory created at top level (for example `mktemp -d -p "${BASE}"`, or one `TOOLS="$(mktemp -d)"` added to `SCRATCH` before the calls), or have them return a path through a global instead of stdout.
  Confidence: 90
- [NOTE] `MUT_ENV` leaks to the next mutant when `selected` skips one. — `hack/test/agent_contract_test.sh:548,562`
  Failure: `return 0` runs before the `MUT_ENV=()` reset. In the nested run limited to "PAC-6 host" (line 848), the fake `HOME` from line 810 carries over into the PAC-6 host mutants. A `task` or `rg` that depends on `HOME` (for example mise shims) then reds as "Taskfile could not be loaded" instead of "survived". The outer check at line 848 then fails with "without … survived". It passed on the maintainer's machine, so this is a portability risk.
  Fix: reset `MUT_ENV=()` before both early returns.
  Confidence: 70
- [NOTE] The "ordinary word in tracked Markdown" skip runs `xargs … rg -l` under `pipefail`. If xargs splits the file list into several batches and one batch finds no match, or rg hits an error (exit 1 or 2), xargs returns 123. The skip then fails silently and the host check stays active, causing the false red the skip exists to prevent. — `hack/test/agent_contract_test.sh:240-243`
  Fix: test the output instead, e.g. `[[ -n "$(… | xargs -0 rg -i -w -F -l -- "${name}" 2>/dev/null)" ]]`.
  Confidence: 55
- [NOTE] Evidence goes to a different file than the tasks say. 0.2 and 0.3 say "record … in `evidence/G0.md`" (`tasks.md:64,70`), but the sha, labels and counts are in `evidence/G1.md`, and no G0.md entry is added. The 1.1 "closed … red" note is attached to the last sub-bullet (PAC-5 wrong clause) rather than the task line. — `tasks.md:64,70,179`
  Fix: change the tasks.md wording to point at G1.md, and move the closing note onto the 1.1 line.
  Confidence: 85
- [NOTE] Some evidence rows claim results that are not shown. Row 4 marks "independent-review: pass" for the 0.2 record before any review exists (`reviews/L-tasks/` is untracked and has no verdict). The *Sensors run* table says `task lint:shell` is under "see *Iterations* for the final run", but *Iterations* holds only the red run. — `evidence/G1.md:21,207,214`
  Fix: mark row 4 "pending" until a review lands, and record the actual `task lint:shell` run.
  Confidence: 75

Checked, and found to hold:
- The guard contains no scrub literal or hostname. Mutants read `hack/scrub-patterns.txt` at run time, and stems are random.
- No FAIL message prints a matched pattern or the hostname.
- The scrub uses the same flags as `hack/scrub.sh` (`rg -i -f`), and rg exit 2 reds.
- The pattern file has no blank or `#` lines (counted, not read), so `rg -f` has no empty-match risk.
- `check_gates` correctly excludes the `run_gate()` definition at `hack/check.sh:26`.
- The ignore probe overrides the global excludes and uses an empty `info/exclude`.
- The index is checked through `git show :path`.
- Every listed case in tasks.md 1.1 (lines 77-179) has a mutant or control, including PAC-5 no-op, surviving-mutant and wrong-clause.
- Nested self-test recursion stops at one level (`AGENT_CONTRACT_NESTED`).
- The file mode is 100755.

## Could not check
- I ran nothing (read-only leg). The plain red, the 128-ok scratch self-test, the hand mutants and the shellcheck/lint results come from the evidence file, and I did not re-run them.
- `/tmp/g1-scratch/tree` (the fixture `AGENTS.md`, pointer, `.gitignore` and gitleaks edits). It is outside the repository, so I could not confirm that the fixture follows the parsing rules G2 must follow.
- The 0.1c local edits, the 0.2 label map and the inventory script are local or untracked by design. I did not open `AGENTS.local.md` or `CLAUDE.md`.
- `hack/test/task_check_test.sh` with the new guard present: the evidence does not record a run.
- Bash 3.2 (stock macOS) portability: the guard uses `${x,,}` and `${x^^}`, which need bash 4. I did not check whether the repo already requires bash 4 for contributors.
