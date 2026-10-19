# Proposal

## Why

`AGENTS.md` and `CLAUDE.md` are gitignored (`.gitignore:11-13`: `CLAUDE.md`, `AGENTS.md`,
`AGENTS.local.md`), and `git ls-files` shows no agent file. A fresh clone, or a contributor's
coding assistant, starts with no commands, no conventions and no list of what CI enforces; it
learns them from failed CI runs. The maintainer's local file mixes public conventions with private
gotchas, so it cannot just be committed. attune ships a committed agent contract (`AGENTS.md`
with commands, rules and "when adding X, update these places" checklists) with one-line pointers
for other assistants.

## What Changes

- A committed, public `AGENTS.md` at the repository root, written in a worktree where no private
  agent file is loaded (so private text cannot be paraphrased from context), from the public
  sources the spec lists (PAC-1): how to build and test, the gates CI enforces, conventions
  (commit format, rebase merge, spec workflow), and checklists for adding a CRD field, a metric
  and a sink.
- A one-line `.github/copilot-instructions.md` that points to `AGENTS.md`. Claude Code, Cursor
  and Codex read the root `AGENTS.md` themselves, so no committed `CLAUDE.md` is added: the root
  `CLAUDE.md` stays ignored, because it is the maintainer's local loader (below).
- The maintainer's private local file (headed "LOCAL ONLY, never commit") was renamed from
  `AGENTS.md` to `AGENTS.local.md` in the main checkout on 2026-10-19, before `.gitignore` stops
  ignoring `AGENTS.md`; a local, ignored `CLAUDE.md` containing only `@AGENTS.local.md` keeps it
  loading for Claude Code (decisions 2026-10-19, change 10: rename ordering A, private loading A).
  Git replaces an ignored file on checkout or pull without warning, so a checkout that still has
  the private `AGENTS.md` loses it on the first pull after this change lands, and a committed
  `CLAUDE.md` would replace the local loader the same way. The PR therefore merges only after the
  rename is verified in every checkout the agent controls (task 0.2), and the operator's other
  machines get an INBOX notice.
- `.github/gitleaks.toml:10` allowlists `(?:^|/)AGENTS\.md$`; that entry is removed so the
  committed file is secret-scanned like any other. CI runs gitleaks with `--no-git`
  (`.github/workflows/ci.yaml:170`, `hack/check.sh:60`), so it scans the working tree and the new
  file is covered from the PR that adds it.
- `hack/test/agent_contract_test.sh` (plain and `--self-test`): every `task <name>` exists,
  required commands and links are present, the pointer is one line, checklist paths and symbols
  exist, no scrub-pattern match (same `rg -i -f` semantics as `hack/scrub.sh`), no home or
  harness path, no hostname or internal host by generic pattern, ignore rules, no "LOCAL ONLY"
  header, no OpenSpec marker, no gitleaks allowlist for agent files.
- `docs/development/setup.md:382` says not to commit `AGENTS.md`; it is corrected.
- Private gotchas stay in the maintainer's local harness.

## Capabilities

### New Capabilities

- `agent-contract`: what the public agent instructions must contain and must never contain.

### Modified Capabilities

None.

## Impact

- Entry point: files at the repository root read by assistants, and the required `lint` job
  running `hack/test/agent_contract_test.sh` plain and `--self-test` as two steps. `task scrub`
  only scans staged files (`hack/scrub.sh:8-12`) and is a no-op on a clean CI checkout, so the new
  test scans the committed files directly.
- `hack/test/repo_root_links_test.sh` checks every tracked root Markdown file, so every relative
  link in the new `AGENTS.md` must resolve.
- `CONTRIBUTING.md` gets one line pointing at `AGENTS.md`.
- `hack/test/ci_workflow_security_test.sh` gains the new guard in its CFC-6 `cfc_guards` list
  (`:608-611`), a test change on change 4's guard.

## Dependencies

Change 10 lands after change 9 (mutation-testing-signal, PR #471) merges (decision 2026-10-19,
landing A). The public `AGENTS.md` names only gates that exist on `main` at that point. Change 6
(dependency-update-automation) is not landed; when it lands it adds its own `AGENTS.md` entry
(this change does not describe Renovate automerge).

## Hard gates

- The independent read of the committed text (task 3.1): a pattern test cannot know every
  private string, and the scrub list is incomplete by construction.
- The rename is verified in every checkout the agent controls before merge (task 0.2).

## Non-goals

- Restating CONTRIBUTING.md, the testing strategy or the ADRs (the contract links them).
- Publishing the maintainer's local harness, memory or operator instructions.
- Defining skills, slash commands or tool-specific configuration.
- Agent attribution rules for commits: the contract states the repo's commit format only.
- Private notes for Cursor and Codex: they read only the public `AGENTS.md`; the private file
  loads in Claude Code only, through the local `CLAUDE.md` (decision 2026-10-19).
- Adding the maintainer's workstation hostname to `hack/scrub-patterns.txt`: that file is public,
  so listing it would publish it (decision 2026-10-19, hostname A). PAC-6's generic patterns, the
  runtime host check and the 3.1 review cover it instead.

## Assumptions

- `hack/scrub-patterns.txt` (10 lines, 91 bytes) is the whole list of forbidden strings, and
  `hack/scrub.sh:14` applies it with `rg -il -f`: case-insensitive, regular-expression, substring
  matches, so a `.` in a pattern matches any character and an ordinary word or phrase can match.
  `scrub.sh` also treats an `rg` error (exit 2) as "no match"; the new test does not copy that.
- Claude Code reads the root `AGENTS.md` without a `CLAUDE.md` import (believed: on this host a
  workspace with only `AGENTS.md` loads it, Claude Code 2.1.302, 2026-10-19); task 2.3 confirms it
  in a fresh session on a clean clone.
- The checklists below are derived from the code on this tree and re-verified by a dry-run in
  task 2.1 (change one CRD field, one metric, add a stub sink; see which gates fire):
  - CRD field: `api/v1alpha1/*_types.go` then `make generate manifests` (CRDs in `config/crd/bases`,
    `config/rbac/role.yaml`, deepcopy) and `hack/helm-sync-crds.sh` for `charts/kollect/crds`;
    all three are drift-checked by `hack/verify.sh` (`task verify`). The hand-written
    `docs/crds/<kind>.md` and `config/samples/` are not generated (probe: no generator target in
    `Makefile`/`Taskfile.yml`) and need a manual update.
  - Metric: register in `internal/metrics/metrics.go` (`Register`, `:303`); add to `Catalog` in
    `metrics_catalog.go` (`:18`) and to `registeredMetricNames` in `metrics_catalog_test.go`
    (`:18`); document in `docs/operator-manual/metrics.md`; alert rules in
    `charts/kollect/templates/prometheusrule.yaml` if relevant (`hack/test/consistency_metrics_test.sh`,
    CFC-4, checks the docs and rules).
  - Sink: package under `internal/sink/<name>`, factory registered in `internal/sink/registry.go`
    `NewRegistry` (`:44`), the type accepted in `api/v1alpha1` sink config and CRD docs, an ADR
    note (`docs/adr/0406-sink-registry.md`) when behaviour is new. Integration-tagged tests need
    no list edit: `INTEGRATION_PACKAGES` (`Taskfile.yml:20`) already includes `./internal/sink/...`.
