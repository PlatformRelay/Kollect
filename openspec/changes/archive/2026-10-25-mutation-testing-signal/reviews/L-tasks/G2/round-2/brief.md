# Cold review brief, round 2 (last): task group G2 of OpenSpec change `mutation-testing-signal` (kollect)

You are reviewing fixes, independently of whoever wrote them. Read-only access to this repository
(cwd is the repo root, branch `feat/mutation-testing-signal`), no network. Read the files; do not
guess. Change nothing in the repository.

## Inputs

1. ROUND-1 FINDINGS — five P2 gaps in the static guard `hack/test/mutation_signal_test.sh`, from
   two reviewers (verbatim in `openspec/changes/mutation-testing-signal/evidence/G2.md`, section
   *Independent review*):
   - W3 missed a task command with flags or other tasks before `mutation` (`task -s mutation`,
     `task --silent mutation`, `task lint mutation`, `task -t Taskfile.yml mutation`);
   - W3's Taskfile dependency query missed a templated dependency (`deps: ["{{.MEASUREMENT}}"]`
     with `MEASUREMENT: mutation`);
   - the root Taskfile's dependency and includes queries skipped their errors, and the scan
     pipelines (`grep ... || true`, `hits | awk ... || true`) hid a failing command, against C3
     (a scan error must fail the guard, never read as "nothing found");
   - a `GREMLINS_` variable set inline on the gremlins call in the runner was caught by no case;
   - the gremlins command-word regex missed `"gremlins" unleash` and `$G unleash`.
2. DIFF — `git diff 3c8d59194..a71c7a5e9` (only the guard changed). Context: task text in
   `openspec/changes/mutation-testing-signal/tasks.md` 2.5 (W3), 2.6 (C3), 2.2 (R1) and spec
   MUT-1/MUT-2 in `openspec/changes/mutation-testing-signal/specs/mutation-testing/spec.md`.
3. SENSORS — `evidence/G2.md` *Sensors run*: the final matrix on `a71c7a5e9`, all green
   (guard plain 9 s, `--self-test` 125 s with 120 passing lines and 0 FAIL).

## Judge

a. Does each fix close its finding, and does each new self-test mutant (search the guard for
   "Round-1 review" and "round-1 review") red for the reason it names?
b. Did a fix break anything: a false positive on the real tree (the plain run is green; check
   the new `task ... mutation` and `unleash` rules against the scan set: `Makefile`,
   `Taskfile.yml` and its includes, `.github/`, `hack/` outside `hack/test/`), a case that now
   passes vacuously, an awk program whose failure is still swallowed, a templated reference that
   still resolves to nothing?
c. Only behaviour defects count in this round; wording is out of scope.

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
