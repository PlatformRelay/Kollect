# Review brief: public-agent-contract, stage B round 2 (round-1 fixes, adversarial)

You are reviewing fixes to a branch, independently of whoever wrote them. Read-only: do not
edit, stage or commit anything in the repository; scratch copies under `mktemp -d` are fine.

Hard rule: do not open, print or quote `AGENTS.local.md`, `CLAUDE.md` or `CLAUDE.local.md`
anywhere on this machine; whether they exist is the only fact you may check. Do not print any
line of `hack/scrub-patterns.txt`, any machine name or any hostname: cite file:line only.

## Inputs

1. SPEC: `openspec/changes/public-agent-contract/specs/agent-contract/spec.md` (PAC-1 to PAC-6).
2. ROUND-1 RECORD: `openspec/changes/public-agent-contract/reviews/B-branch/round-1/verification.md`
   (findings A1 to A5 and O1 to O6, each with its disposition).
3. DIFF: `git diff f780d5966..HEAD` (six fix commits, one per finding):
   - `5312acd86` A3: an empty gitleaks path regex is no longer skipped;
   - `8f59b5779` A4: the SSH `Host` pattern takes any number of aliases; the prose control moved
     "host" mid-line (a test change);
   - `3148ff5d6` O3: `/var/home/<user>`, `/root/` and `C:\Users\<user>` are home paths;
   - `221eb661e` A5/O2: the `lint` job installs ripgrep without a version pin;
   - `48dae840f` A2: `docs/development/setup.md` warns to rename a private `AGENTS.md` before
     pulling;
   - `4d5209acf` A1/O1 (part): absolute paths in `task-prompt.md` and `loop.md` became
     placeholders.
4. SENSORS (safe to rerun): `bash hack/test/agent_contract_test.sh` (1 s);
   `bash hack/test/agent_contract_test.sh --self-test` (about 90 s; `AGENT_CONTRACT_ONLY=<text>`
   limits it to mutants whose label contains the text); `bash hack/test/ci_workflow_security_test.sh`.

## Questions

1. CLOSED? For each fix: does it close its finding completely, and does its mutant fail for the
   right reason (would it survive if the fix were reverted)? Look for siblings the fix missed:
   other empty or whitespace-only allowlist entries; a `Host` line form still missed (tabs,
   `Host=` forms, a trailing comment after many aliases); home-path forms still missed or
   newly over-matched (does the new pattern red on ordinary content an operator contract needs,
   such as a container path `/root` in a sentence, `docs/var/home`, a URL path); a Windows path
   with forward slashes.
2. NEW DEFECTS: does any fix introduce a false red on the committed `AGENTS.md` or on plausible
   future content, break a self-test control, or weaken another clause? Does the unpinned
   ripgrep step still fail loudly when no package is available, stay before both guard steps,
   and keep `hack/test/ci_workflow_security_test.sh` and zizmor green?
3. DOC: is the new `setup.md` sentence true (git replaces an ignored untracked file with a
   tracked one on pull without a warning) and placed where a contributor would read it?
4. PRIVACY: do the fix commits or their messages (`git log f780d5966..HEAD`) add anything that
   reads as private (machine names, home paths, harness internals, model names)?

For each P0/P1, argue the opposite for one line; keep only what survives. Severity: P0 = a leak,
data loss, or a check that passes while the property it guards is broken in a way a real
contributor would hit; P1 = a finding not closed or a new false red/green; P2 = the rest. An
APPROVE with no findings is valid when substantiated.

## Return (at most 20 lines, nothing else)

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
closed: A1 <yes/part/no> A2 <..> A3 <..> A4 <..> A5 <..> O3 <..>
findings:
  - [P0|P1|P2] <one line, with path:line, and how you verified it>
```
