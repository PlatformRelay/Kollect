# Review brief: public-agent-contract group G2 (tasks 2.1 to 2.4)

You are reviewing one task group of an OpenSpec change, independently of whoever implemented it.
Read-only: do not edit, stage or commit anything in the repository.

Hard rule: do not open, print or quote `AGENTS.local.md` or `CLAUDE.md` anywhere on this machine.
Their existence is the only fact you may check. Do not print any line of `hack/scrub-patterns.txt`
or any machine name in your output.

## Inputs

1. TASK: `openspec/changes/public-agent-contract/tasks.md`, items 2.1, 2.2, 2.3 and 2.4, and its
   preamble. The spec is `openspec/changes/public-agent-contract/specs/agent-contract/spec.md`
   (PAC-1 to PAC-6). The guard `hack/test/agent_contract_test.sh` came from an earlier group (1.1)
   and is not changed here; G2 turns it green.
2. MATRIX: `openspec/changes/public-agent-contract/evidence/G2.md` (dry-run record, author
   session, citation table, clean-clone check, CI wiring, gate record).
3. DIFF: `git diff 15bbd821c..HEAD` (one commit). Product files: `AGENTS.md`,
   `.github/copilot-instructions.md`, `.gitignore`, `.github/gitleaks.toml`,
   `.github/workflows/ci.yaml` (lint job), `hack/test/ci_workflow_security_test.sh` (`cfc_guards`),
   `CONTRIBUTING.md`, `docs/development/setup.md`.
4. SENSORS: recorded in the evidence file, *Gate record*. Under a minute and safe to rerun:
   `bash hack/test/agent_contract_test.sh`, `bash hack/test/repo_root_links_test.sh`,
   `bash hack/test/ci_workflow_security_test.sh` (about 11 s).
5. RULES: `AGENTS.md` is written only from tracked files (README, CONTRIBUTING, `docs/`,
   Taskfile/Makefile, `hack/`, `.github/`, `openspec/`); every sentence true on a fresh clone; the
   gate list names exactly the `run_gate` names of `hack/check.sh`; links, not restatement, for
   CONTRIBUTING and the spec workflow; no scrub pattern and no hostname as a literal in any tracked
   file; evidence is Markdown only; the guard runs in the required `lint` job plain and
   `--self-test` as two steps and is listed in `cfc_guards`.

Lens: security posture of a public contract plus its leak guard. Two questions above the rest:

- Leak: read `AGENTS.md`, `.github/copilot-instructions.md`, the evidence file and the commit
  message as a stranger would. Flag anything that looks like private content: personal identity
  beyond the public git author, internal hosts or addresses, company names, local or home paths,
  machine names, or agent-harness internals (local coordination files, ignored directories,
  private tool set-up, model or session names). Also: does any change weaken secret scanning or
  the ignore rules beyond the one allowlist entry the spec removes?
- Truth: pick at least ten statements in `AGENTS.md` (commands, CI jobs, checklist lines,
  gotchas) and check each against the cited tracked file. A wrong command or a checklist place
  that does not exist is a P1.

Also judge: the `lint` job's ripgrep step (pinned enough? fails loudly? placed before the guard
steps?), and whether the dry-run record supports each checklist line.

Judge, in order: a. INTENT (all of 2.1 to 2.4, nothing more); b. MATRIX (is each sensor real, each
class honest); c. RULES; d. AI-code failure modes (prose claiming more than the files do,
hallucinated paths or symbols, silently dropped halves of a task).

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
