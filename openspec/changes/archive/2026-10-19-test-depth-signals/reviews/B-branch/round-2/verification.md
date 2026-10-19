# Stage B round 2 (last): verification of the branch review findings

Tree reviewed: `d1dd0930e`. Legs, both fresh sessions (no round-1 session reused):
`leg-astra-adversarial.md` (brief `brief-astra.md`, a new cold brief), verdict **BLOCK**; and
`leg-claude-opus-spec.md` (spec compliance), verdict **CONCERNS**. Both exited 0. Launch note: the
two were meant to run in parallel, but a zsh glob error stopped the astra launch, so the opus leg
ran first and astra ran right after it. Neither leg saw the other's output.

The orchestrator reproduced every finding with probes on `/tmp` copies of the tree, running the
guard's plain mode on each mutated copy. Holistic run: `holistic.md` (`task check` exit 0,
2138 s). This is the last round. Verified behaviour defects were fixed with a case and a mutant,
with no further review. Everything else is rejected or deferred below.

## Round-1 closure (astra Q1, opus per-requirement check)

| Item | Leg reading | After verification |
|---|---|---|
| A1 later TYPE/NAME operands | astra: partly closed | partly closed at `d1dd0930e`: R2-A1a (`kinv/a,foo/b`) was a G5 regression. Now closed (X24) |
| A2 `if:` / `continue-on-error:` | both: closed for those keys | closed; the wider shell bypass R2-A2 is fixed below |
| A3 exit-1 retry | both: closed (F17, mutant) | closed |
| A4 failing Ginkgo run | both: closed (S13, two mutants) | closed |
| O1 fence scope | both: closed (spec and X13/X23) | closed; the info-string gap R2-O1 is fixed below |
| Deferred F16, envtest error line, F0 empty pkg, F18 crash replay, internal tasks, continuation and indented fence | astra: closed | closed |

## Findings

| Id | Sev (leg) | Verified | Disposition |
|---|---|---|---|
| R2-A1a | CRITICAL (astra), WARNING by our reading | yes: `kubectl get kinv/a,foo/b` passes (exit 0). The G5 code cut `/NAME` before splitting at commas | **fixed**: split at commas, then cut each part. Case X24; hand mutant (old order) killed by X24 |
| R2-A1b | CRITICAL (astra) | yes: `kubectl "get" pods` passes; `kubectl get "kinv"` is a false red | **fixed**: quote characters are dropped from each word. X25 rejects `pods`, X26 accepts `"kinv"`; hand mutant (no stripping) killed by X25 and X26 |
| R2-A1c | CRITICAL (astra) | yes: `kubectl get --context kinv pods` passes (the flag value is read as the type); `kubectl get -f kinv` passes | **fixed**: `--context`, `--cluster`, `--user`, `--kubeconfig`, `-s`/`--server`, `--as`, `-f`/`--filename` take a value. With that, `get -f x` names no resource and fails. X27; hand mutant (`--context` dropped) killed by X27 |
| R2-A1d | astra (answer 2) | yes: `kubectl get kinv/` passes | **reject**: kubectl itself refuses an empty name. The guard checks that the named types exist, not kubectl's syntax |
| R2-A1e | astra (fix text) | partly: other value-taking flags not on the list could still swallow an operand | **defer** to hand-off: an exhaustive kubectl flag table (or refusing unknown flags) is a design choice. The README uses none today |
| R2-A2 | CRITICAL (astra), WARNING by our reading | yes: job `defaults.run.shell: bash -c "true" {0}` passes F11 and S11; so do a step `shell:` (shuffle) and a workflow-level `defaults:` | **fixed**: F11 refuses job `defaults:` and step `shell:`, F16 refuses `shell:` on the download step, S11 refuses workflow and job `defaults:` and step `shell:`. Six `--self-test` mutants, each red on its case |
| R2-A2b | astra (answer 3) | yes: the fuzz job's `needs`, matrix entries and `timeout-minutes` are not pinned | **defer**: matrix coverage and `needs` predate this change and TDS-1 does not cover them. **Reject** `timeout-minutes: 0`: it cancels the job, so it cannot pass green (**believed**, GitHub semantics) |
| R2-A3 | CRITICAL (astra), WARNING by our reading | yes: `status: ["true"]` or `ignore_error: true` on the `shuffle:` task passes S12 | **fixed**: S12 allows only `desc`, `summary` and `cmds` on the task. Two mutants (`status`, `ignore_error`) red on S12 |
| R2-O1 | WARNING (opus) | yes: a column-0 ```` ``` sh ```` fence (space before the info string) is skipped, so `task nosuch` inside it passes | **fixed**: the info string's leading blanks are trimmed, as CommonMark does. Case X28; hand mutant (no trim) killed by X28 |
| R2-O2 | NOTE (opus) | yes: TDS-2 cites `ci_failure_report_test.sh:1226-1233` and `ci-failure-report.yaml:40-42`; the lines are now `:1228-1235` and `:41-43` | **fixed** (spec text) |
| R2-O3 | NOTE (opus) | yes | **reject**: `SHUFFLE_SEED` limited to 1-18 digits is stricter than Go's flag, and the workflow never sets it |
| R2-O4 | NOTE (opus) | yes | **defer** (as round-1 O4): Taskfile `aliases:`, `flatten:` and `task -d` give a false red that fails closed; nothing in the README hits it |
| R2-Q7 | astra (answer 7) | covered by R2-A3 | see R2-A3 |

Hand mutants ran on scratch copies (`/tmp`, the guard plus its inputs). The fuzz and shuffle
mutants live in their guards' `--self-test`.

## Gates after the fixes

Run sequentially on the fixed tree (`umask 022`), all exit 0, each log ending on its pass line:

| Gate | Result |
|---|---|
| `fuzz_retry_test.sh` plain / `--self-test` | `F0-F18` / 144 s |
| `test_shuffle_test.sh` plain / `--self-test` | `S0-S13` / 117 s |
| `readme_quickstart_test.sh` plain / `--self-test` | 0 s / 6 s (X24-X28 included) |
| `ci_workflow_security_test.sh` plain / `--self-test` | 14 s / 149 s |
| `ci_failure_report_test.sh` | 53 s |
| `task_check_test.sh` | 1 s |
| `task lint:shell` | exit 0 |
| `task spec:validate` | `14 passed, 0 failed` |
| `task lint:markdown` | 0 issues |

`task check` was not rerun after these fixes. They touch only the three guards and the spec text,
and the gates above cover each one in both modes. Its last run, on `d1dd0930e`, is
`holistic.md`.

## Round verdict

**CONCERNS → fixed**. The three astra CRITICALs (fail-open guards) and the opus WARNING are fixed,
each with a case or mutant. No CRITICAL remains open. Open for hand-off:

- R2-A1e: no exhaustive kubectl flag table;
- R2-A2b: fuzz matrix and `needs` are not pinned;
- R2-O4: Taskfile aliases and flatten give a false red;
- G5 review N2: the README guard's code mutants are hand-run, not kept in its `--self-test`.
