Paths below are relative to `openspec/changes/public-agent-contract/` unless stated otherwise. Review performed at `1c880b52f`; no files changed.

### R1 CRITICAL The review artifacts themselves disclose local environment details

**Evidence:** `tasks.md:49-53` requires recording “each path and result” in committed `evidence/G0.md`. `loop.md:3` and `task-prompt.md:3,7,15` already contain an absolute checkout path and local harness locations; `task-prompt.md:21` includes machine-specific execution details. PAC-4 scans only the two product files (`specs/agent-contract/spec.md:81-87`).

**Failure:** Following the tasks publishes local environment information through evidence and orchestration files even if the public contract passes every guard. The task prompt’s private-file prohibition does not protect metadata supplied elsewhere in its context.

**Fix:** Keep the checkout inventory and orchestration details outside tracked files. Commit only opaque checkout identifiers, verification results and revision identifiers. Sanitize the already-tracked run-state files before publication, and extend the independent privacy review to **all changed files**, including evidence and review records. If these commits have not been published, remove the disclosures from the commits being published, not merely from the final tree.

### R2 CRITICAL The checkout migration gate is not sufficiently defined to prevent data loss

**Evidence:** `tasks.md:49-55` defines the inventory by checkouts that currently have either ignored agent file, checks only a `"LOCAL ONLY"` header, and requires the main checkout’s checksum wherever a local file exists. It prohibits fast-forwarding shared `main` before verification, but does not explicitly prohibit checking out the feature branch elsewhere. Task 2.3 removes the ignore rule at `tasks.md:137-144`.

**Failure:** There is no bounded inventory or completeness criterion for “every checkout the agent controls.” A private file lacking that particular header can pass; a checkout with legitimately different private notes cannot satisfy the shared checksum requirement. Switching any missed checkout to the product branch can replace its ignored private file before the merge gate is reached. Merely applying the ignore-rule deletion makes an existing private `AGENTS.md` eligible for `git add -A`.

**Fix:** Require an owner-maintained inventory of clones and their worktrees before applying the product change anywhere. For each checkout, verify that no untracked or ignored root `AGENTS.md` remains, regardless of header. Preserve each checkout’s own before/after checksum and refuse overwriting an existing `AGENTS.local.md`. Gate checkout, rebase, reset and pull operations that introduce the tracked file—not just merge and fast-forward of shared `main`. Publish only the sanitized attestation described in R1.

### R3 WARNING The scrub mutants do not prove regular-expression parity

**Evidence:** `tasks.md:86-88` plants each pattern’s literal text and an uppercase version. PAC-4 requires regex semantics (`specs/agent-contract/spec.md:81-87`).

**Failure:** I checked all ten current pattern lines: each matches its own literal text, so these mutants are presently executable. However, an incorrect `rg -F -i` implementation would pass all of them. Pattern lines **6 and 10** also match a runtime-generated dot-substitution witness that does not contain the literal pattern. The prescribed mutants never test that distinction. Future regexes need not match their own source text.

**Fix:** Add runtime-generated regex witnesses, including the dot-substitution cases, and verify each witness against the real regex before testing the guard. Include a deliberate fixed-string implementation mutant that must fail the self-test. Define how future pattern additions supply witnesses without embedding private literals.

The current five tracked change files produced no matches against the scrub list; that does not establish their privacy, as R1 demonstrates.

### R4 CRITICAL The staged-content requirement lacks an index-versus-worktree test

**Evidence:** PAC-4 requires rejection of a private header when “tracked or staged” (`specs/agent-contract/spec.md:109-112`). `tasks.md:100-102` calls for tracked and staged cases without requiring different index and working-tree contents. Repository `hack/scrub.sh:8-14` obtains staged filenames but scans their **working-tree contents**.

**Failure:** Stage a private version, then restore a clean public version without staging it. A guard reading only disk contents passes, and `task scrub` also inspects the clean version. A simple “staged” mutant that leaves identical contents on disk does not expose this failure. Secret scanning is not a substitute for detecting private prose.

**Fix:** Specify which representations the guard checks: working-tree files plus relevant index blobs before committing. Add a mutant with a forbidden header staged and clean content on disk, and require the header-specific diagnostic. Apply the same index protection to scrub-pattern content. Describe `task scrub` accurately as scanning working-tree versions of staged paths.

### R5 CRITICAL Ordinary `git check-ignore` cannot enforce the tracked-file ignore requirement

**Evidence:** `tasks.md:96-99` prescribes `git check-ignore` for detecting an ignore rule reintroduced for `AGENTS.md`; the no-op mutant repository commits tracked files (`tasks.md:16-18`). PAC-4 requires detecting that rule (`specs/agent-contract/spec.md:104-107`).

**Failure:** By default, `git check-ignore` does not report tracked files. Once `AGENTS.md` is committed, adding an ignore rule for it can leave this check green. The task also omits `.agents/` and `.cursor/` from its ignore-rule mutants, although PAC-4 names them as protected harness paths.

**Fix:** Explicitly require `git check-ignore --no-index` for rule evaluation, independently from `git ls-files` checks. Probe representative child paths in every protected directory. Add separate mutants for `.agents/`, `.cursor/` and the newly required `CLAUDE.local.md` rule, with corresponding verification coverage.

### R6 WARNING PAC-6 promises more hostname protection than its mechanism provides

**Evidence:** `specs/agent-contract/spec.md:142-157` checks the executing machine’s short hostname, except short names and `localhost`. `tasks.md:111-113` describes that as protecting the maintainer’s workstation name. The proposal explicitly relies on this check and independent review (`proposal.md:87-89`).

**Failure:** CI checks its runner’s name, not the workstation’s. Another machine’s bare name is invisible to the check. Case handling, literal versus regex matching, boundaries and failure of `hostname -s` are unspecified. An ordinary-word hostname can reject legitimate prose; a short hostname is explicitly skipped despite the unconditional scenario. Split spellings remain outside the proposed mechanism.

**Fix:** State that this is supplementary detection, not proof that no workstation name leaked. Define case-insensitive literal matching, token boundaries, failure handling and exceptions consistently in the requirement and scenario. Use an injectable hostname provider for deterministic tests. Require a private owner-side check of known machine identifiers before publication, recording only pass/fail, alongside public-source provenance review.

### R7 WARNING Generic host rules have no acceptance boundary or positive fixtures

**Evidence:** `specs/agent-contract/spec.md:142-144` prohibits all hosts under the listed suffixes and private/link-local IPv4 addresses. `tasks.md:108-110` supplies only forbidden examples, including only `172.16.` for the entire corresponding private range.

**Failure:** The literal requirement rejects Kubernetes `cluster.local` service names and private example addresses. Conversely, a narrow implementation could check only the sampled subnet and miss the rest of `172.16.0.0/12`. Case, DNS-label boundaries, valid IPv4 octets, Markdown code blocks and SSH-line anchoring are unspecified. A no-op copy only establishes that one chosen document passes.

**Fix:** Define the accepted grammar and whether legitimate service DNS and example addresses are allowed. If the blanket ban is intentional, require the contract to link to examples instead. Add positive fixtures for allowed prose and addresses, and negative fixtures covering range boundaries, mixed case, all suffixes, SSH aliases and address forms.

### R8 WARNING Several promised negative checks are missing or can pass for the wrong reason

**Evidence:**

- PAC-4 includes a `CLAUDE.md` gitleaks exemption (`specs/agent-contract/spec.md:119-122`), but `tasks.md:105-107` tests only the other two files.
- PAC-3 requires the metric checklist’s named entries (`specs/agent-contract/spec.md:64-67`); `tasks.md:80-85` tests headings and stale sink references, not removal of those metric entries.
- `tasks.md:91-92` skips the error-path mutant as root.
- `task-prompt.md:55` describes mutant success as asserting nonzero exit, although `tasks.md:14-16` expects clause-specific failures.

**Failure:** A guard can fail for an unrelated missing prerequisite and count the mutant as covered. Root execution has no proof that `rg` errors fail closed. The requirement-level verification rows at `tasks.md:170-175` conceal these missing cases.

**Fix:** Require an expected diagnostic for every mutant, plus a green baseline. Add the missing cases and a deterministic error-path test, such as an invalid regex or controlled `rg` exit-2 injection. Exercise the Copilot file as well as `AGENTS.md`. Give the scratch-copy recipe explicitly enough files for `task --list-all`, including `Taskfile.yml`, its included task file and required inputs; missing-tool or task-loading failures must not count as target failures.

### R9 WARNING Public-source provenance and OpenSpec preservation need explicit verification

**Evidence:** PAC-1 limits source material to a short list (`specs/agent-contract/spec.md:19-20`), while task 2.1 necessarily derives checklists from `api/` and `internal/` (`tasks.md:122-126`). Task 2.2 treats absent local files and session starting location as evidence of clean context (`tasks.md:127-135`). The marker prohibition covers committed agent files only (`specs/agent-contract/spec.md:114-117`).

**Failure:** File absence does not establish that inherited context contained no private material. The task prompt’s prohibition is useful, but cannot establish provenance after private context has already been loaded. The source list also omits code needed for the mandated checklists. Finally, a marker-free public contract does not establish that a tool preserves an ignored local loader; ignore rules govern staging, not tool writes.

**Fix:** Expand permitted sources to tracked public code and require source citations for checklist and operational claims. Dispatch the author with a sanitized brief and no inherited private context. Add a scratch-only compatibility test using synthetic ignored loader/private-file fixtures, covering the pinned OpenSpec operations the workflow actually uses and checking their preservation. Never use real private files as fixtures.

### R10 WARNING Archive ordering contradicts the repository workflow

**Evidence:** `tasks.md:160-164` puts merge/pull before archive, while also requiring archive to be the PR’s last commit. `task-prompt.md:54` explicitly says “then archive.” Repository `docs/development/spec-workflow.md:32-36` requires archive, reviewer recheck, then merge on green CI.

**Failure:** Literal execution merges before the living spec and archived evidence are finalized and reviewed.

**Fix:** Order the steps as: implement → review → archive → recheck the archive revision → green CI on that revision → verify migration attestations → merge → protected pull and checksum verification.

## Verdict

**BLOCK** — The tasks can publish private environment metadata through review artifacts, and the migration and staged-content checks do not yet establish the promised protection against disclosure or data loss.
