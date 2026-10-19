# Review brief, round 2: public-agent-contract group G1 (tasks 0.1c, 0.2, 0.3, 1.1)

You are reviewing one task group of an OpenSpec change, independently of whoever implemented it.
Read-only: do not edit, stage or commit anything in the repository.

Hard rule: do not open, print or quote `AGENTS.local.md` or `CLAUDE.md` anywhere on this machine.
Their existence is the only fact you may check. Do not print any line of `hack/scrub-patterns.txt`
or any machine name in your output.

## Inputs

1. TASK: `openspec/changes/public-agent-contract/tasks.md`, items 0.1c, 0.2, 0.3 and 1.1 (1.1 lists
   about 30 given/when/then cases), and its preamble. The spec is
   `openspec/changes/public-agent-contract/specs/agent-contract/spec.md` (PAC-1 to PAC-6).
2. MATRIX: `openspec/changes/public-agent-contract/evidence/G1.md`.
3. DIFF: `git diff ec966638d..10efd0903` is the round-1 fix you judge (the round-1 findings and their dispositions are in the evidence file under *Independent review*); `git diff 3cb6d45df..10efd0903` is the whole group for context. The new guard is
   `hack/test/agent_contract_test.sh`.
4. SENSORS: recorded in the evidence file. The guard is red on this tree by design (no
   `AGENTS.md` until a later group); its self-test was proved on a scratch tree outside the
   repository (`/tmp/g1-scratch/tree`, built by `/tmp/g1-scratch/make-tree.sh` from the tracked
   files plus a throwaway fixture `AGENTS.md`). If you can run commands there, `bash
   hack/test/agent_contract_test.sh` and `--self-test` from inside that tree are the checks.
5. RULES: no scrub pattern and no hostname as a literal in a tracked file; evidence is Markdown
   only, with opaque checkout labels and no paths or checksums of private files; the guard
   collects every failure; each mutant runs on its own throwaway git copy and must red with the
   diagnostic of its own clause; the ignore check uses
   `git -c core.excludesFile=/dev/null check-ignore --no-index`.

Lens: security posture, leak prevention. The guard exists so that a private agent file, a
machine-local path, an internal host or the maintainer's machine name never reaches the public
`AGENTS.md`. Ask how a leak would still get through.

Judge, in order:

a. INTENT: does the diff do what the four tasks ask, all of it, and nothing more? Is every 1.1
   case present as a mutant or control?
b. MATRIX: is each sensor real, would it fail if its claim were false, is each evidence class
   honest? Mutation-check clauses where you can (in a scratch copy, never in the repository).
c. RULES: any rule above broken, including in the evidence file and the commit.
d. AI-code failure modes: vacuous checks, regexes that cannot match what they claim, mutants that
   red for the wrong reason, prose that claims more than the code does.

For each P0/P1, argue the opposite for one line; keep only what survives. An APPROVE with no
findings is valid when substantiated.

## Return (at most 15 lines, nothing else)

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <item>
summary: <one line>
```
