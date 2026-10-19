Reviewed `4685ebd33`. The round-1 fixes for index scanning, `git check-ignore --no-index`, and migration ordering are present. The cited repository line numbers check out. Task 2.4 satisfies the existing CWS-6/CFC-6 wiring; `lint` runs on docs-only PRs, and `task check` discovers the new guard automatically. Removing the gitleaks entry leaves neither public agent-file path exempt.

All ten current scrub patterns match their proposed literal and uppercase witnesses; the dot-substitution witnesses for pattern lines 6 and 10 also match. The inspected change Markdown is scrub-clean. No private files or round-1 legs were read.

### R2-1 WARNING The running-host rule still rejects legitimate prose

**Evidence:** `openspec/changes/public-agent-contract/specs/agent-contract/spec.md:180-185` requires a case-insensitive whole-word match, skipping only unreadable names, short names, `localhost`, and task names. `openspec/changes/public-agent-contract/tasks.md:146-149` tests those exceptions.

A contributor whose short hostname is an ordinary word such as `metric`, `commands`, or `operator` can get a red guard on legitimate contract text. The task-name exception does not solve this. It also makes the required no-op control machine-dependent.

The acknowledged blind spots remain: another machine’s bare name, skipped short names, and split names are not covered. Case variation is covered; an intact short name appearing as an FQDN label should be covered if implemented as a literal whole-word match.

**Fix:** Separate deterministic enforcement from the runtime hostname heuristic. Specify literal matching explicitly, make ambiguous ordinary-word matches advisory or subject to an explicit local policy, and add an ordinary-word hostname control. Retain the independent review for names that generic checks cannot identify.

### R2-2 WARNING The fixed-string implementation mutant has no nesting bound

**Evidence:** `openspec/changes/public-agent-contract/tasks.md:112-113` runs the modified guard’s own self-test. Only the empty-pattern mutant receives a nesting exclusion at `tasks.md:150-152`; the corresponding spec exception is at `specs/agent-contract/spec.md:154-155`.

A nested fixed-string self-test still contains the instruction to create another fixed-string self-test. An implementation that accumulates self-test failures can recurse indefinitely. A fail-fast implementation may avoid that by detecting the surviving regex witness first, but termination should not depend on execution order or failure policy.

**Fix:** Apply the nesting guard to **all** mutants that invoke another self-test. Require nested runs to execute ordinary controls and witnesses while skipping implementation-mutant launches. Require the fixed-string mutant to fail specifically because a regex witness survives, not because of a timeout or unrelated failure.

### R2-3 WARNING The wrong-clause control contradicts the diagnostic acceptance rule

**Evidence:** `openspec/changes/public-agent-contract/tasks.md:14-21` deliberately collects failures and accepts a red containing the targeted diagnostic even when another clause also fails. At `tasks.md:156-157`, a mutant that “also lacks `Taskfile.yml`” must instead make the self-test fail.

If the original mutation still produces its intended diagnostic, adding the missing Taskfile produces exactly the multi-failure result the earlier rule accepts. Both instructions cannot reliably be satisfied together.

**Fix:** Define this control as an input that produces **only an unrelated diagnostic**—for example, remove the intended violation while retaining the broken Taskfile. Alternatively, require an exact allowed diagnostic set for each mutant and revise the earlier multi-failure acceptance rule accordingly.

### R2-4 WARNING Post-pull verification assumes private files exist in every checkout

**Evidence:** `openspec/changes/public-agent-contract/tasks.md:54-64` permits inventory entries that never had a private `AGENTS.md`. But `tasks.md:207-209` requires every listed checkout to retain its private-file checksum and a loader containing `@AGENTS.local.md`. Only the main checkout’s loader creation is recorded at `tasks.md:43-49`.

A clean clone or isolated worktree legitimately has neither private file nor loader. Such a checkout can pass 0.2 but cannot satisfy 4.2. The inventory also lacks an explicit instruction to retain the private mapping from opaque labels to checkout locations for that later verification.

**Fix:** Record, locally, each checkout’s baseline presence/absence, checksums where applicable, and opaque-label mapping. After updating, verify preservation of that baseline. Require the loader import only where that loader was established; do not create private loaders in otherwise clean worktrees.

### R2-5 NIT The archive diff cannot verify preservation of an ignored loader

**Evidence:** `openspec/changes/public-agent-contract/tasks.md:203-206` checks that the archive commit touches neither root agent file. PAC-2 deliberately keeps `CLAUDE.md` untracked (`specs/agent-contract/spec.md:47-49`).

A commit diff cannot reveal an edit to ignored `CLAUDE.md`. This is not evidence that the specified archive command damages it, but the check cannot establish loader preservation against the legacy-cleanup risk cited at `spec.md:136-139`.

**Fix:** Keep the tracked-file diff check and add local existence/checksum comparisons for ignored files around any operation whose preservation is being asserted. Publish only pass/fail evidence.

## Verdict

CONCERNS

Main risk: the remaining self-test ambiguities and machine-dependent hostname rule can prevent a correct implementation from passing; migration verification also needs to distinguish clean checkouts from checkouts holding private files.
