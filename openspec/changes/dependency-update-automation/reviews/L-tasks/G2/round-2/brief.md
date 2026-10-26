You are reviewing one OpenSpec task, independently of whoever implemented it. Read-only: do not
edit the repository. Scratch copies under /tmp are fine (leave mise.toml out of a copy that runs
a guard). Do not run any command whose text contains the gh merge subcommand literally; put such
patterns in a file.

Repository: /home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation
Base: 92791d3106ce859e236d85a96386b280d950d3d5 · Head: 651a19048 (branch spec/dependency-update-automation)
Diff: `git diff 92791d310..651a19048` (whole task); round-1 fixes alone: `git diff be8a369ae..651a19048`

This is round 2, the last. Round 1 (`reviews/L-tasks/G2/leg-reviewer-*-diff.md`) found: T16 took
the image from anywhere in the argv (fixed: image position parsed, decoy `m_t16_image_as_arg`);
set -e was off inside each parallel self-test case (fixed: the case marks itself passed on its
last line); T17's secondary patterns lacked mutants (fixed); T16 never runs against the tree's own
renovate.yaml (deferred, see evidence *Harness gaps*). Dispositions are in `evidence/1.2.md`
*Independent review*. Judge the fixes first (are they right, complete, and do they break
anything), then the whole task as below; do not re-file a deferred finding unless you show it
is a false green the task text forbids.

Inputs:

1. TASK — `openspec/changes/dependency-update-automation/tasks.md`, task 1.2 (cases T10 to T17)
   and the guard preamble above section 1; task 2.2 (the config shape the fixture models);
   requirements DUA-3, DUA-4, DUA-5, DUA-7, DUA-8, DUA-9, DUA-10 in
   `openspec/changes/dependency-update-automation/specs/dependency-updates/spec.md`. The validator
   contract T16 judges is in `evidence/0.5.md` ("The argv that works"): T16 judges image, command
   and flags; a file argument may carry the mount prefix. Dispatch: add T10 to T17 to the guard
   from task 1.1 (its CLAUSES list, MUTANTS table and fixture), plus the stub
   `hack/ci/renovate-config-validate.sh [root]` whose body is `echo "not implemented" >&2; exit 2`.
   Expected on today's tree (plain mode): T10, T11, T12, T14, T15, T17 red; T16 red on the stub;
   T13 green. This red is the required red of a test task; `lint` stays red until sections 2 and 3
   by design: do not file that as a defect. `--self-test` must be green.
2. MATRIX and SENSORS — `openspec/changes/dependency-update-automation/evidence/1.2.md`
   (design choices, verification matrix, red per case, sensors run, probes incl. the real
   validator run on the fixture and the clause-removal proof, guard mutation checklist, gaps).
3. DIFF — the code you judge: `hack/test/renovate_policy_test.sh` (new clauses T10 to T17, fixture
   writers, mutants, controls, and a parallel self-test runner replacing 1.1's sequential loop),
   `hack/ci/renovate-config-validate.sh` (stub), and the evidence file. 1.1's clauses T1 to T9 are
   not under review except where this diff changes them or the runner they depend on.
4. RULES — guard-mutation rules: one mutant per clause judged by its own message, a no-op
   control, a simulated bot-bump control, a decoy, no plausible next-release values as mutants,
   values read parsed (yq), never from comments. Evidence under openspec is Markdown only. Tools
   in CI's lint job: yq (mikefarah v4), jq, bash, docker.

Judge, in order:
a. INTENT — does the diff do what task 1.2 asks for each of T10 to T17, all of it, and nothing it
   did not ask? Is each clause at least as strict as its case text (e.g. T11 "any depth" in both
   files, T12's exemptions, T14's "one whole weekday", T16's four behaviours)? Are the extra
   strictnesses justified by the spec, and would any of them red a correct 2.2/2.3/3.1 result?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify clauses in
   a scratch copy (remove a clause body, confirm the self-test reds). Check the fixture
   renovate.json is a correct 2.2 config and the fixture validator a correct 2.3 script.
c. RULES — any rule above the diff violates.
d. Failure modes — yq expressions that silently yield nothing (precedence of `|`, `,`, `or`), TSV
   fields collapsing under IFS, set -e aborts on a no-match substitution, state lost through
   `$(...)`, false greens on a wrong tree, false reds on a correct tree, the fake docker being
   fooled (a script that calls docker differently, e.g. `docker container run`, a pull, or the
   validator through `sh -c`), the parallel runner hiding a failed case or leaking temp dirs.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 15 lines, nothing else:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row claim> — <why the sensor/class does not prove it>
could_not_check:
  - <what you did not verify>
summary: <one line>
