# Review brief — task 2.1 of developer-toolchain-pins (diff-only, independent)

You are reviewing one task, independently of whoever implemented it. Read-only: do not edit files.
Repo root: the current directory (kollect, branch `feat/developer-toolchain-pins`).

1. TASK — `openspec/changes/developer-toolchain-pins/tasks.md` item 2.1: add the DTP-1 check (every
   `go-task/setup-task` `version:` input under `.github/workflows/` and `.github/actions/`, plus
   `mise.toml` `task`, equal and at least 3.52.0), with self-test cases: one site at 3.51.1 → red
   naming that file; all at 3.51.1 → red (below floor); all at a newer release → green. Record the
   plain-mode red on today's 18 + 1 sites, then set Task 3.52.0 at all 18 sites and `mise.toml:54`,
   and rewrite `docs/development/setup.md:39` to name the input without a version. One commit.
   Added by the dispatcher: a setup-task step with NO `version:` input was silently skipped before
   (Task installs unpinned); a self-test case must red on it, plus a hand mutant.
   Spec: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md` (DTP-1, DTP-7).
   Repo rules: AGENTS.md, docs/development/spec-workflow.md.
2. MATRIX and SENSORS — `openspec/changes/developer-toolchain-pins/evidence/T2.1.md` (verification
   matrix, red before the bump, sensors run, hand mutants, test changes, deviations).
3. DIFF — the working tree against base d8cba74dd: `git diff d8cba74dd` (nothing committed yet; the
   evidence file is intent-to-add). The guard before this task: `git show
   d8cba74dd:hack/test/consistency_toolchain_pins_test.sh`.
4. RULES — guards: plain mode checks the real tree, `--self-test` corrupts throwaway copies per
   mutant and asserts non-zero WITH the clause's own message and no other clause tripped, plus a
   no-op copy that must pass. Tests written by earlier tasks (1.1, 1.2) are fixed points: a change to
   one needs evidence from the spec. Self-test mutant values must never be a plausible next release.

Judge, in order:
a. INTENT — all of 2.1 and nothing else? All 18 setup-task sites and mise.toml at 3.52.0
   (`grep -rn -A3 go-task/setup-task@ .github`)? Does the DTP-1 check reject everything the removed
   DEV-MISE-01 task comparison (`setup_task_versions` + `mise_assert_pin task`) rejected, and does
   the generalised step reader (`action_version_sites`, formerly `cliff_action_sites`) still read
   git-cliff exactly as before?
b. MATRIX — is each sensor real, would it fail if the claim were false? Mutation-verify in a scratch
   copy (mktemp, never the repo) any clause you doubt: `bash <guard copy> --self-test <repo>` runs in
   about 50 s. Does each probe prove its mutant landed? Are the test changes to 1.1/1.2 cases
   justified by the spec, and is any old coverage lost (the split no-ci-pins case especially)?
c. RULES — CI wiring (the lint step was renamed; CWS-6 / CFC-6 must still hold the guard in both
   modes); workflow security (only `version:` values and one step name/comment changed in YAML).
d. AI-code failure modes — tautological probes, weakened assertions, dropped halves, prose (comments,
   evidence, the setup.md row) that claims more than the code does.

For each P0/P1, argue the opposite for one line; keep only what survives. An APPROVE with no
findings is valid when substantiated.

RETURN at most 15 lines, nothing else, under a `## Verdict` heading:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line>
matrix_disputes:
  - <row> — <why>
could_not_check:
  - <what>
summary: <one line>
