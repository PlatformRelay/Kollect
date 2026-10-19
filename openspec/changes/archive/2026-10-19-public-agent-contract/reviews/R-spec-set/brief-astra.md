# Cold review brief: stage-R spec-set review of OpenSpec change `public-agent-contract` (kollect)

You are an independent reviewer with read-only access to this repository (cwd is the repo root,
branch `feat/public-agent-contract`, HEAD `1c880b52f`, spec set at `cb93d3750`). You have no
network. Read the files; do not guess. Change nothing.

Hard rule: do NOT open, read or quote any file outside this worktree, and in particular never any
`AGENTS.local.md` or untracked `CLAUDE.md` anywhere on this host. They are the maintainer's private
notes; this review is about keeping them private. Do not quote any line of
`hack/scrub-patterns.txt` in your output either (refer to patterns by line number).

## Context

kollect is a public Go Kubernetes operator repo. This change (10 of 10, issue #447) commits a
public `AGENTS.md` (agent contract: commands, CI-enforced gates, conventions, checklists for adding
a CRD field, a metric and a sink) and a one-line `.github/copilot-instructions.md` pointer, stops
ignoring `AGENTS.md` in `.gitignore`, removes the `AGENTS.md` allowlist from
`.github/gitleaks.toml`, and adds a guard `hack/test/agent_contract_test.sh` (plain and
`--self-test`) to the required `lint` job.

Until now the maintainer kept a PRIVATE `AGENTS.md` (company strings, hostnames, harness details)
in the main checkout, ignored by git. Git replaces an ignored working-tree file with a tracked one
on checkout or pull without warning, so after this change lands, any checkout still holding the
private `AGENTS.md` loses it on the next pull. Decisions applied (recorded outside the repo,
summarised in proposal.md): the private file was already renamed to `AGENTS.local.md` (still
ignored) and is loaded for Claude Code by a local, ignored root `CLAUDE.md` containing only
`@AGENTS.local.md`; the PR merges only after the rename is verified in every checkout the agent
controls; the workstation hostname is NOT added to the public `hack/scrub-patterns.txt` (that would
publish it); the change lands after change 9 (PR #471) merges.

Spec set: `openspec/changes/public-agent-contract/proposal.md`,
`openspec/changes/public-agent-contract/specs/agent-contract/spec.md`,
`openspec/changes/public-agent-contract/tasks.md` (plus `loop.md` and `task-prompt.md`, the run
state and the per-task subagent prompt). Nothing is implemented yet; `AGENTS.md`, the pointer and
the guard do not exist.

Repo conventions: every `hack/test/*_test.sh` guard has a plain mode (checks the real tree) and a
`--self-test` mode (mutates a throwaway copy, asserts each mutant red, a no-op copy green).
`task check` (`hack/check.sh`) sweeps all guards in both modes. CI guards are wired into the
required `lint` job of `.github/workflows/ci.yaml` in both modes as separate steps (CWS-6 in
`hack/test/ci_workflow_security_test.sh`) and named in its CFC-6 `cfc_guards` list. `task scrub`
runs `hack/scrub.sh` (staged files only). Evidence under `openspec/changes/**/evidence/` is
Markdown only. Red-first TDD: a test task records a red that fails on the target assertion; a
group's tests and implementation land in one commit.

## Target question

Would executing `tasks.md` as written deliver `specs/agent-contract/spec.md` without leaking
private content into the public repository?

Check specifically:

1. **Fail-open guard paths (scrub).** Compare the spec's PAC-4 parity claim with
   `hack/scrub.sh` and `hack/scrub-patterns.txt`: flags, regex vs fixed-string, case, exit-code
   handling, which files are scanned. Can the guard as specified pass while a scrub pattern is in
   a committed agent file? Do the run-time-built mutants (each pattern line planted as text) prove
   a match for every line, given that the lines are regular expressions? Could the guard, its
   mutants or any Markdown in this change itself turn `task scrub` red?
2. **Fail-open guard paths (hostname, PAC-6).** With the hostname kept out of the scrub list, what
   still stops it leaking? Is the runtime `hostname -s` check sound (CI runner names, short or
   common hostnames, FQDN vs short, a hostname that is an ordinary word), and what does it miss
   (another machine's name, a hostname written in a different case or split)? Are the generic
   patterns (`.internal`, `.local`, `.lan`, `.home.arpa`, RFC 1918, link-local, `Host` lines,
   `user@host:`) specified tightly enough to be implementable and to avoid false reds on the
   legitimate content the contract needs (e.g. `cluster.local` service DNS, `localhost`, example
   addresses)?
3. **Data-loss ordering.** Walk the order of 0.1, 0.1b, 0.1c, 0.2, 0.3, 2.3 (the `.gitignore`
   edit), 4.1 and the pull in other checkouts. Is there any point where a private file is
   un-ignored, overwritten or one `git add -A` from a commit? Does keeping `CLAUDE.md` ignored and
   untracked (PAC-2) hold under every step, including OpenSpec's legacy cleanup that rewrites root
   `AGENTS.md`/`CLAUDE.md`, and a contributor's tool creating a `CLAUDE.md`? Is 0.2 verifiable as
   written ("every checkout the agent controls")?
4. **Wiring.** Does 2.4 satisfy CWS-6 and CFC-6 as `ci_workflow_security_test.sh` implements them
   (separate plain and `--self-test` steps in a required job; `cfc_guards`)? Does the guard run on
   docs-only PRs (`ci.yaml` `paths-ignore`, the `changes` job)? Does `task check` pick it up? Is
   the gitleaks claim right (`--no-git` in `ci.yaml` and `hack/check.sh`) and does removing the
   allowlist line leave anything else exempt?
5. **Stale or wrong file:line references.** Check every `path:line` in the three spec files
   against this tree (`Taskfile.yml:20`, `.gitignore:11-13`, `.github/gitleaks.toml:10`,
   `hack/scrub.sh:8-12`/`:14`, `hack/check.sh:60`/`:88-111`, `ci.yaml:170`/`:283`,
   `ci_workflow_security_test.sh:591-598`/`:608-611`, `docs/development/setup.md:382`, the metrics
   and sink line numbers). Flag any claim that is false.
6. **Spec-to-task coverage.** Does every PAC-1 to PAC-6 scenario have a 1.1 case and a
   verification row? Are any 1.1 cases unimplementable as written (e.g. `task --list-all` in a
   throwaway copy, `git check-ignore` semantics, the "staged only" private-header case, `chmod
   000` as root in CI)? Is PAC-1's "written from tracked public sources only" checkable, and is the
   task-prompt's private-file rule enough to keep the author from paraphrasing private notes?

## Output

Markdown. One section per finding: `### <id> <SEVERITY> <title>` with SEVERITY one of CRITICAL
(the tasks would ship a leak, lose a private file, or not deliver a requirement), WARNING (a real
gap a task executor would likely trip on), NIT. For each: the evidence (`path:line`, quoted
minimally), the failure it causes, and a concrete fix to the spec set. End with
`## Verdict` and one of APPROVE, CONCERNS or BLOCK, plus one line naming the main risk.
