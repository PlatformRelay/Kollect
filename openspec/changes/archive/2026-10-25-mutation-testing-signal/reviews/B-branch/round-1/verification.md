# Stage B round 1: verification

Change `mutation-testing-signal`, whole branch at `fe7d8e31e` (base `origin/main` @ `3c01608ff`),
reviewed 2026-10-19. Holistic run and the real-chain probes: `holistic.md`.

## Legs

| Leg | File | Exit | Verdict |
| --- | --- | --- | --- |
| astra (adversarial lens, cold brief `brief-astra.md`, `codex exec --sandbox read-only -`) | `leg-astra-adversarial.md` | 0 | BLOCK |
| claude/opus:spec (fanout.sh, `--no-unify`, compliance table per MUT requirement) | `leg-claude-opus-spec.md` | 0 | CONCERNS |

Both legs ran fresh; no stand-in. Opus found every MUT clause held in the code except the W3
partial below, and named three behaviours stronger than the spec (package-file check, strict
`MUTATION_FORCE_FAILURE` values, action SHAs from other workflows only), none harmful.

## Answers to the brief's questions (astra, checked)

| Question | Answer | Checked |
| --- | --- | --- |
| 1 report-only bypass | no gate found: `GREMLINS_*` refused (`mutation.sh:37-44`), failures propagate (`:76`), Task and the workflow swallow nothing | read |
| 2 measure nothing, look green | yes: B1 | reproduced |
| 3 guard passes a bad workflow | no for W1 (astra rejected two in-memory mutants); W3 is a text scan: B4 | B4 reproduced |
| 4 install chain | correct (`Taskfile.yml:32-33`, `:217`, `:223`) | read |
| 5 reporter entry | correct (`ci-failure-report.yaml:26` equals `mutation.yaml:10`; the `:35` filter admits scheduled runs of this repository) | read |
| 6 force-failure knob | fails only; passed through `env:`, never interpolated into the script; the upload then gets an empty `path`: B3 | read |

## Findings

| Id | Sev (after verification) | Legs | Verified | Finding | Disposition |
| --- | --- | --- | --- | --- | --- |
| B1 | CRITICAL | astra; orchestrator runs | yes: 2 of 7 real `task mutation` runs reported every `internal/aggregate` mutant TIMED OUT and exited 0; `mutation-summary.sh` accepts an all-TIMED-OUT report (exit 0, row `0 0 0 5 0`, rerun 2026-10-19); the deadline is coverage time x 3, started before the mutant build (`gremlins@v0.6.0/internal/engine/executor.go:83-101,194`); with `--timeout-coefficient=10` 0 of 6 runs timed out against 4 of 6 at the default | a package whose mutants all time out (or are all not covered or not viable) is reported green: a night that measured nothing reads as a clean one, against MUT-2 "a missing measurement must not look like a clean one" | fix, two commits: S1 the formatter fails a package with no KILLED or LIVED mutant (new case M9 on the recorded `warm-cache-pathvalidate.json`, plus a mutant); S2 the runner passes `--timeout-coefficient=10` (R1 argv, plus a mutant), so a loaded runner stops turning measurements into timeouts. Spec MUT-1 (argv) and MUT-2 (*Tool broken*) updated |
| B2 | WARNING | opus | yes: `mutation.sh:59-63` is a fixed list; nothing compares it with the pinned binary; the bump control passes a `v99.0.0` pin (`mutation_signal_test.sh` `bump`) | a Renovate bump that adds a mutator (default off) leaves it silently disabled, against MUT-1 "every mutator flag of the pinned version" | fix, S3: before the fixture call the runner reads `<gremlins> unleash --help` and fails unless its `enable "..." mutants` flags are exactly the eleven (new case R9, a recorded help fixture, plus mutants) |
| B3 | NOTE | opus, astra | yes by reading: the forced failure and the refusal exit before `reports=` is written, so the upload's `path:` is empty and that step fails too | a second red step on a forced run | reject: the run is red either way, the spec holds ("no effect other than failing the run"), and the failure issue names the first failed step (`hack/ci/ci-failure-issue-body.sh:86-95`), which is the measurement step. Gating the upload on the output would widen W1 for no behaviour change |
| B4 | NOTE | opus | yes: a `test-shuffle.yaml` step `run: \|` / `task \` / `mutation`, and one `run: >-` / `task` / `mutation`, both pass `MUTATION_SIGNAL_ONLY=W3` (scratch worktree, exit 0) | W3 misses `task mutation` split by a shell continuation or folded by YAML | fix, S4: the W3 line scan joins backslash-continued lines, and also scans the parsed text of every block scalar (literal or folded) in the YAML files of the scan set (new W3 mutants for both forms) |
| B5 | NOTE | opus | yes: no case reads `hack/test/testdata/mutation/reports/warm-cache-pathvalidate.json` | a recorded fixture no test uses | fix, by S1: M9 reads it |
| B6 | NOTE | orchestrator | yes: with a 575 MB `bin/` a direct gremlins run filled the `/tmp` quota, panicked and left about 8 GB (`holistic.md`) | `task mutation` on a working copy with a large `bin/` copies `bin/` once per worker | defer: local only (CI's checkout is fresh, and the runner fails closed on the panic); a note belongs in the hand-off, not in the guarded chain |
| B7 | NOTE | orchestrator | yes by reading: the issue body's excerpt keeps only log lines holding `FAIL` (`ci-failure-issue-body.sh:7,132`); no `::error::mutation:` line holds it | the `CI failure: Mutation testing` issue names the failed step but quotes no mutation diagnostic | defer: the reporter's excerpt rule is change 7's (`ci-failure-reporting`); the run page carries the diagnostic, and MUT-4 asks for it in the run |

Astra's NOTE lines for questions 1, 3, 4, 5 and 6 are confirmations, no action.

## Result

One CRITICAL (B1) accepted and fixed, so round 2 runs. B2 and B4 fixed; B3 rejected; B6 and B7
deferred with reasons.
