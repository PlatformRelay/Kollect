You are reviewing one OpenSpec task, independently of whoever implemented it. Read-only: do not
edit the repository. Scratch copies under /tmp are fine (leave mise.toml out of a copy that runs
a guard). Do not run any command whose text contains the gh merge subcommand literally; put such
patterns in a file.

Repository: /home/koni/Projects/PlatformRelay/worktrees/kollect/dependency-update-automation
Base: 8d9c271e5d29fae9b91510ed14bc33fc96761af5 · Head: HEAD of branch spec/dependency-update-automation
Diff: `git diff 8d9c271e5..HEAD`

Inputs:

1. TASK — `openspec/changes/dependency-update-automation/tasks.md`, task 1.1 (cases T1 to T9) and
   the guard preamble above section 1; requirements DUA-1, DUA-2, DUA-3, DUA-5, DUA-7, DUA-8 in
   `openspec/changes/dependency-update-automation/specs/dependency-updates/spec.md`. Dispatch
   additions: wire both modes into the `lint` job as two separate steps (plain, then --self-test,
   one invocation per line, near the other CFC guard steps, with a short comment citing DUA and the
   openspec change); enrol the guard in CWS-6's CFC-6 list in
   `hack/test/ci_workflow_security_test.sh` so both modes are mandatory; design the guard so task
   1.2 can add clauses T10 to T17 (clause table, a --self-test that applies one mutant per clause to
   a copy, judged by that clause's own message, plus a no-op control); plain mode takes an optional
   tree root; --self-test runs on a known-good fixture so it is green now and after 2.1/2.3 land.
   Expected today: plain mode red on T1 to T8, T9 green (this is the required red of a test task;
   `lint` stays red until section 2 by design — do not file that as a defect).
2. MATRIX and SENSORS — `openspec/changes/dependency-update-automation/evidence/1.1.md`
   (verification matrix, red per case, sensors run, guard mutation checklist, design choices).
3. DIFF — the only code you judge: `hack/test/renovate_policy_test.sh` (new),
   `.github/workflows/ci.yaml` (two lint steps), `hack/test/ci_workflow_security_test.sh`
   (enrolment + one self-test mutant), the evidence and loop.md/tasks.md records.
4. RULES — guard-mutation rules: one mutant per clause judged by its own message, a no-op
   control, a simulated bot-bump control, a decoy, no plausible next-release values as mutants,
   workflow values read parsed (yq), never from comments (the action version comment is read with
   yq line_comment); a check over `run:` bodies reads the parsed strings. Evidence under openspec
   is Markdown only. Tools in CI's lint job: yq (mikefarah v4), jq, bash.

ROUND 2 (last). A first round found that T2 judged a body with no `shell:` under pipefail while
the runner uses `bash -e {0}`, plus several notes; the fixes and their dispositions are in the
evidence's *Independent review* table and iterations 5 and 6. Code is identical between
`b47d89d02` (where the final sensors ran) and HEAD (records only). Check the fixes first: does the
shell emulation now match the runner for no shell, `shell: bash`, and a job or workflow
`defaults.run.shell`; can the T7 token rule or the `T2+T7` harness allowance hide a real defect;
can the `yqv` helper turn an error into a pass anywhere. Then the whole diff as before.

Judge, in order:
a. INTENT — does the diff do what task 1.1 asks for each of T1 to T9, all of it, and nothing it
   did not ask? Is each clause at least as strict as its case text, and are the extra strictnesses
   listed in the evidence justified by the spec?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify clauses in
   a scratch copy: remove a clause's logic, confirm the self-test reds, restore. Check the fixture
   workflow is a correct DUA-1..DUA-3 workflow (it is the known-good tree every mutant starts from).
   Check the T2 body execution cannot hang, leak a secret, or run with the wrong shell semantics.
c. RULES — any rule above the diff violates.
d. Failure modes — tautological checks, yq expressions that silently yield nothing (precedence of
   `|`, `,`, `or`), set -e aborts on a no-match substitution, false greens on a wrong tree, false
   reds on a correct tree (including after 2.1 lands the real workflow), CWS-6 wiring gaps.

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
