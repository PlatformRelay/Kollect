# Review brief: public-agent-contract, whole branch (stage B, round 1, adversarial)

You are reviewing a whole branch of an OpenSpec change, independently of whoever wrote it. You
did not write any of it. Read-only: do not edit, stage or commit anything, and do not run any
command that writes inside the repository (scratch copies under `mktemp -d` are fine).

Hard rule: do not open, print or quote `AGENTS.local.md`, `CLAUDE.md` or `CLAUDE.local.md`
anywhere on this machine; whether they exist is the only fact you may check. Do not print any
line of `hack/scrub-patterns.txt`, any machine name or any hostname in your output: describe a
match by its file and line, never by its text.

## Inputs

1. SPEC: `openspec/changes/public-agent-contract/specs/agent-contract/spec.md` (PAC-1 to PAC-6),
   `proposal.md`, `tasks.md` (the preamble and sections 0 to 2 are the contract for the guard).
2. DIFF: `git diff origin/main...HEAD` (branch `feat/public-agent-contract`, HEAD `73e92c8c4`).
   Product files: `AGENTS.md`, `.github/copilot-instructions.md`, `.gitignore`,
   `.github/gitleaks.toml`, `.github/workflows/ci.yaml` (`lint` job, ripgrep and guard steps),
   `hack/test/agent_contract_test.sh` (the guard), `hack/test/ci_workflow_security_test.sh`
   (`cfc_guards`), `CONTRIBUTING.md`, `docs/development/setup.md`.
3. EVIDENCE: `openspec/changes/public-agent-contract/evidence/G1.md` and `G2.md`.
4. SENSORS (safe to rerun): `bash hack/test/agent_contract_test.sh` (about 1 s);
   `bash hack/test/agent_contract_test.sh --self-test` (minutes); `bash hack/test/repo_root_links_test.sh`;
   `task --list-all`.

## What to attack (all six are required)

1. TRUTH of `AGENTS.md`: every command, path, symbol, job name, gate and gotcha in it must exist
   and be correct on this tree. Verify each against the tree, not against the evidence:
   `task --list-all` for task names; `hack/check.sh` for the gate list, the guard sweep wording,
   the Docker-skip rule and the exclusion table; `.github/workflows/ci.yaml`, `preflight.yaml`
   and `e2e-smoke.yaml` for each job and what it runs; `hack/release/verify-eligibility.sh`
   `required_checks` for the claim "a release is eligible only when these check runs are green";
   each checklist path (does it exist, does the glob match), each Go symbol (is it declared in the
   named file), each named test (`TestRegisterMatchesRegisteredMetricNames`,
   `TestCRDSpecOpenAPIFragments`) and each command (`make generate manifests`,
   `bash hack/helm-sync-crds.sh`, `python3 hack/gen-glossary.py`, `UPDATE_GOLDEN=1 go test ...`).
   Check the descriptions in the Commands table too (for example "Run golangci-lint v2 and
   go-arch-lint", "fails unless every one of them ran and passed"). A wrong command, a wrong job
   description or a checklist place that does not exist is P1.
2. PRIVACY: read `AGENTS.md`, `.github/copilot-instructions.md`, the evidence files and the
   commit messages (`git log origin/main..HEAD`) as a stranger would. Flag anything that reads as
   private: identity beyond the public git author, internal hosts or addresses, company names,
   local or home paths, machine names, or agent-harness internals (ignored coordination
   directories, private tool set-up, model or session names, local skill names). Run
   `bash hack/scrub.sh`-equivalent matching yourself: `rg -i -f hack/scrub-patterns.txt` over
   the tracked files the branch adds or changes, and report only file:line.
3. LEAK-GUARD FAIL-OPEN paths in `hack/test/agent_contract_test.sh`. Specifically:
   - staged vs disk content: does every content check (scrub, home/harness paths, private
     header, OpenSpec markers, generic hosts, running host) read both the working-tree file and
     the index blob (`git show :AGENTS.md`), for both agent files? What happens when the file is
     staged-deleted, or when it is only in the index, or when the index blob differs from HEAD?
   - the gitleaks TOML allowlist: is the parsing complete (`[allowlist]`, `[[allowlists]]`,
     `[[rules.allowlists]]`, `paths` arrays, single/triple-quoted strings, literal vs basic
     strings and their escape handling, a regex that `grep -P` cannot compile)? Can a TOML parse
     error, a missing `tomllib` (Python < 3.11) or a missing `python3` pass silently? Does the
     PCRE evaluation match Go RE2 semantics closely enough that an allowlist gitleaks honours
     is not missed?
   - `check-ignore --no-index`: is it used with `-c core.excludesFile=/dev/null`, is
     `.git/info/exclude` empty in the copy, are directories probed through a child path, and is
     the exit status of `git check-ignore` (0 ignored, 1 not ignored, 128 error) handled so an
     error does not read as "ignored"?
   - any other path where a tool error, an empty input or an unset variable makes a check pass
     (`rg` exit 2, `set -e` / `pipefail` gaps, `|| true`, a loop over an empty array, a skip
     that triggers too easily such as the PAC-6 hostname skips).
   - does the self-test actually prove each clause (a mutant whose target diagnostic is so
     generic that another clause also produces it)?
4. DATA-LOSS ORDERING: the change makes a previously ignored root `AGENTS.md` tracked. In any
   checkout that still has a private untracked `AGENTS.md`, checking out or pulling this branch
   overwrites or blocks on it. Judge whether tasks.md 0.1/0.2/4.2, the evidence and the INBOX
   notice it mentions close that ordering, whether anything in the branch (commit, CI, docs)
   could still destroy such a file, and whether `docs/development/setup.md` and
   `CONTRIBUTING.md` tell a contributor what to do before pulling.
5. RIPGREP INSTALL step in the `lint` job of `.github/workflows/ci.yaml`: is the version pin
   valid for the runner image the job uses (check `runs-on`), does it fail loudly on a missing
   package or a wrong version, is it placed before both guard steps, does it satisfy the
   repository's own workflow rules (`hack/test/ci_workflow_security_test.sh`, zizmor), and is a
   pinned `apt-get install` of a specific Debian revision fragile across image updates?
6. `.gitignore` EDIT: the branch deletes the `AGENTS.md` line and adds `CLAUDE.local.md`. Can
   this edit, any other ignore rule (negations such as `!*.md`, patterns earlier or later in the
   file, nested `.gitignore` files) or the guard's checks expose `AGENTS.local.md`, `CLAUDE.md`
   or `CLAUDE.local.md` to `git add -A`? Prove it with
   `git -c core.excludesFile=/dev/null check-ignore --no-index -v` on each path in a scratch clone.

Judge, in order: a. INTENT (the branch does what the spec says, nothing more); b. TRUTH;
c. PRIVACY; d. FAIL-OPEN paths; e. AI-code failure modes (prose claiming more than the files do,
hallucinated paths or symbols, silently dropped halves of a requirement).

For each P0/P1, argue the opposite for one line; keep only what survives. Severity: CRITICAL
(P0) = a leak, a check that can pass while the property it guards is broken in a way a real
contributor would hit, or data loss; P1 = a false statement in `AGENTS.md` or a real fail-open
path; P2 = the rest. An APPROVE with no findings is valid when substantiated.

## Return (at most 30 lines, nothing else)

```text
## Verdict
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line, and how you verified it>
checked-true: <count> AGENTS.md statements verified true (list the five riskiest)
```
