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

- A committed, public `AGENTS.md` at the repository root: how to build and test, the gates CI
  enforces, conventions (commit format, rebase merge, spec workflow), and incident-derived
  checklists for adding a CRD field, a metric and a sink.
- One-line `CLAUDE.md` and `.github/copilot-instructions.md` that point to `AGENTS.md`.
- The maintainer's existing local `AGENTS.md` (a large file headed "LOCAL ONLY, never commit" in
  the main checkout) is renamed to `AGENTS.local.md`, which stays ignored, BEFORE `.gitignore`
  stops ignoring `AGENTS.md` and `CLAUDE.md`; otherwise un-ignoring would leave a private file one
  `git add -A` from being committed. The local harness references to `AGENTS.md` are updated.
- `.github/gitleaks.toml:10` allowlists `AGENTS\.md`; that entry is removed so the committed file
  is secret-scanned like any other.
- `hack/test/agent_contract_test.sh`: scrub-pattern match, machine-local paths, referenced paths
  exist, pointers stay one line, checklists name real files.
- Private gotchas stay in the maintainer's local harness (`agent-context/`, `AGENTS.local.md`).

## Capabilities

### New Capabilities

- `agent-contract`: what the public agent instructions must contain and must never contain.

### Modified Capabilities

None.

## Impact

- Entry point: files at the repository root read by assistants, and the `lint` job running
  `hack/test/agent_contract_test.sh`. `task scrub` only scans staged files
  (`hack/scrub.sh:9`), so the new test scans the committed files directly.
- `CONTRIBUTING.md` gets one line pointing at `AGENTS.md`.

## Dependencies

Landing order across the eight proposed changes: developer-toolchain-consistency (with its Go bump), cross-file-consistency-gates, ci-workflow-hardening, dependency-update-automation, nightly-failure-reporting, test-depth-signals, mutation-testing-signal, public-agent-contract
(last, so its checklists can name the gates the earlier changes add).

## Non-goals

- Restating CONTRIBUTING.md, the testing strategy or the ADRs (the contract links them).
- Publishing the maintainer's local harness, memory or operator instructions.
- Defining skills, slash commands or tool-specific configuration.
- Agent attribution rules for commits: the contract states the repo's commit format only.

## Assumptions

- `hack/scrub-patterns.txt` (10 lines, 91 bytes) is the whole list of forbidden strings and `rg -il -f`
  is how `hack/scrub.sh` applies it. Probe: read `hack/scrub.sh`.
- The checklists below are derived from the code on this tree and re-verified by a dry-run in
  task 2.1 (change one CRD field, one metric, add a stub sink; see which gates fire):
  - CRD field: `api/v1alpha1/*_types.go` then `make generate manifests` (CRDs in `config/crd/bases`,
    `config/rbac/role.yaml`, deepcopy) and `hack/helm-sync-crds.sh` for `charts/kollect/crds`;
    all three are drift-checked by `hack/verify.sh` (`task verify`). The hand-written
    `docs/crds/<kind>.md` and `config/samples/` are not generated (probe: no generator target in
    `Makefile`/`Taskfile.yml`) and need a manual update.
  - Metric: register in `internal/metrics/metrics.go`; add to `Catalog` in
    `metrics_catalog.go` and to `registeredMetricNames` in `metrics_catalog_test.go`; document in
    `docs/operator-manual/metrics.md`; alert rules in `charts/kollect/templates/prometheusrule.yaml`
    if relevant (cross-file-consistency makes these checks mechanical).
  - Sink: package under `internal/sink/<name>`, factory registered in `internal/sink/registry.go`
    `NewRegistry`, the type accepted in `api/v1alpha1` sink config and CRD docs, an ADR note
    (`docs/adr/0406-sink-registry.md`) when behaviour is new, the integration tests list
    (`INTEGRATION_PACKAGES` in `Taskfile.yml`) if it has tagged tests.
