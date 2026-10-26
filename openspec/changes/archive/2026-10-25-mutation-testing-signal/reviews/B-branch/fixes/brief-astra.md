# Cold review brief: stage-B fixes of OpenSpec change `mutation-testing-signal` (kollect)

You are reviewing fixes, independently of whoever wrote them. Read-only access to this repository
(cwd is the repo root, branch `feat/mutation-testing-signal`, HEAD `49310e2ad`). No network. Read
the files; do not guess. Change nothing in the repository. Short scratch runs (under a minute)
outside the repository are fine where your sandbox allows them.

## Context

A nightly, report-only gremlins (v0.6.0) mutation measurement: `.github/workflows/mutation.yaml`
runs `task mutation` -> `hack/ci/mutation.sh` (runner) -> `hack/ci/mutation-summary.sh`
(formatter). `hack/test/mutation_signal_test.sh` is the static and behavioural guard (plain and
`--self-test`; the self-test mutates throwaway copies and requires each mutant to red with a
named message). Spec: `openspec/changes/mutation-testing-signal/specs/mutation-testing/spec.md`.

## Inputs

1. FINDINGS being fixed (`openspec/changes/mutation-testing-signal/reviews/B-branch/round-1/verification.md`):
   - B1 CRITICAL: a package whose mutants all TIMED OUT (or all NOT VIABLE / NOT COVERED) was
     reported green, and on a loaded runner gremlins' default timeout coefficient (3) did turn
     whole packages TIMED OUT.
   - B2 WARNING: a gremlins bump adding a mutator (default off) would leave it silently disabled.
   - B4 NOTE: W3 missed `task mutation` split by a shell continuation or a YAML folded scalar.
   - B5 NOTE: the recorded `warm-cache-pathvalidate.json` fixture was unused.
2. DIFF: `git diff fa16e8409..HEAD` (four commits, S1-S4; tasks.md section 4, evidence/B.md).
3. SENSORS (evidence/B.md): guard plain exit 0 (13 s); `--self-test` exit 0 (266 s); `task
   lint:shell`, `task lint:markdown`, `task spec:validate` green; real `task mutation` runs
   green with no TIMED OUT at coefficient 10, and red naming the flag when one is removed.

## Judge

a. Does each fix close its finding? S1: can a report still measure nothing and pass (e.g. one
   KILLED out of many TIMED OUT; is that acceptable report-only)? S2: is 10 justified, does it
   bound a hung mutant within the workflow's `timeout-minutes: 15`? S3: can the `--help` parse
   pass when it should fail (regex too loose or too strict, locale, a flag listed twice, a help
   format where mutator lines are absent so everything is 'none')? S4: does the block-scalar
   scan or the continuation join create false positives on the real tree, or swallow an error?
b. Does each new self-test mutant red for the reason it names (M9 x2, M4 message change, R1 x2,
   R9 x4, W3 x3)? Mutation-check one or two in a scratch copy if your sandbox allows.
c. Did a fix break anything else: R8 (`$GITHUB_OUTPUT` before the first gremlins call, now that
   `--help` is a call), R5/R6 (no gremlins call on refusal), the fake gremlins' call counting.
d. Only behaviour defects count; wording is out of scope.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.

## Return, at most 15 lines, nothing else

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
could_not_check:
  - <what you could not verify>
summary: <one line>
```
