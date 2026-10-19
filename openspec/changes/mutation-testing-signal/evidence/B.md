# Evidence: stage-B fixes of mutation-testing-signal

Fixes for the verified findings of the branch review (`reviews/B-branch/round-1/verification.md`),
tasks.md section 4. Each fix: the new case red on the unfixed tree on its target assertion, then
the fix, then the guard plain and `--self-test` green on the fixed tree.

## S1 (4.1): a package that measured nothing fails (B1, B5)

Red, `MUTATION_SIGNAL_ONLY=M9 bash hack/test/mutation_signal_test.sh` on the unfixed formatter
(exit 1), the three M9 reports each failing on all three clauses (shown once):

```text
FAIL: M9: the formatter exited 0, want non-zero
FAIL: M9: no '::error::mutation:' line naming 'internal/pathvalidate' and 'no mutant was killed or lived'; stderr:
FAIL: M9: a counts table was printed although a report is invalid: | Package | Killed | Lived | Not covered | Timed out | Not viable | ...
FAIL: the mutation signal breaks MUT-1 to MUT-4 (9 failure(s) above)
```

Fix: `hack/ci/mutation-summary.sh` adds the verdict "no mutant was killed or lived (each one timed
out, was not viable or is not covered): nothing was measured" after the status check.

First self-test after the fix: exit 1, the M4 mutant (zero-mutation check dropped) passed,
because the new check also rejects an empty mutation list and M4 asserted no message. M4 now
asserts its own text ("no mutation: nothing was measured"), so the mutant reds by message.

Green: guard plain exit 0 (M9 three lines `ok`); `--self-test` exit 0 in 242 s, including:

```text
ok - self-test: zero-mutation check dropped (M4): red (exit 1) with 'M4: no '::error::mutation:' line naming 'internal/pathvalidate' and 'no mutation: nothing was measured''
ok - self-test: measured-nothing check dropped (M9, stage-B B1): red (exit 1) with 'M9: the formatter exited 0'
ok - self-test: only TIMED OUT counted as measuring nothing (M9, stage-B B1): red (exit 1) with 'M9: the formatter exited 0'
All mutation signal self-tests passed.
```

## S2 (4.2): a timeout coefficient a loaded runner does not exceed (B1)

Red, `MUTATION_SIGNAL_ONLY=R1 bash hack/test/mutation_signal_test.sh` with R1's argv holding
`--timeout-coefficient=10` and the runner unchanged (exit 1, each of the four calls, cut):

```text
FAIL: R1: call 1 (fixture) argv is 'unleash --config=<root>/hack/ci/gremlins.yaml --output=<dir>/fixture.json --arithmetic-base ...
FAIL: the mutation signal breaks MUT-1 to MUT-4 (4 failure(s) above)
```

Why 10: the coefficient probe in `reviews/B-branch/round-1/holistic.md` (4 of 6 runs with TIMED
OUT mutants at the default 3, 0 of 6 at 10).

Fix: `hack/ci/mutation.sh` passes `--timeout-coefficient=10` after `--output`.

Green: guard plain exit 0; `--self-test` exit 0 in 206 s with
`the timeout coefficient dropped (R1, stage-B B1)` and
`the timeout coefficient back at the default 3 (R1, stage-B B1)` red by R1's message.

Real chain, in a clean detached worktree at `7b650b5cd` with the fixed runner copied in (so
`bin/` holds only gremlins): five `task mutation` runs, each exit 0 with the recorded counts
(aggregate 4/1/0/0/0, redact 12/1/1/0/0, pathvalidate 10/2/0/0/0), no TIMED OUT; one more run
with 16 busy loops loading all 16 CPUs, exit 0, the same counts.
