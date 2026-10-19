## Verdict: CONCERNS

## Findings
- [WARNING] PAC-1 asks for AGENTS.md to list "the CI-enforced rules that exist on main", but nothing keeps that list true after this change lands. The test only checks that each named `task` exists. — `openspec/changes/public-agent-contract/specs/agent-contract/spec.md:15`
  Failure: a later PR adds a `run_gate` to `hack/check.sh:42-58` or drops a required context. AGENTS.md goes stale and every gate stays green. The Purpose line ("safe to publish and true", :6) is not met for this section, and change 6 adding "its own entry" is a convention, not a check.
  Fix: add a PAC-1 clause and scenario that compares the gate names in AGENTS.md with the `run_gate` names in `hack/check.sh` (or with `required_checks` in verify-eligibility).
  Confidence: 70
- [WARNING] PAC-3 says "every repository path and every code symbol" must exist, but it never defines how they are extracted. The proposal's own checklist uses placeholders and globs. — `spec.md:60-62`, `proposal.md:102-112`
  Failure: `internal/sink/<name>`, `docs/crds/<kind>.md` and `api/v1alpha1/*_types.go` are not real paths, so a literal check fails on the correct contract. A loose check passes any word: `Catalog` alone would match anywhere in the tree.
  Fix: say that paths are backticked tokens containing `/`, that placeholders and globs are expanded or skipped, and that a symbol is checked as a Go declaration in the file it is named next to (`func NewRegistry` is at `internal/sink/registry.go:44`).
  Confidence: 65
- [WARNING] The PAC-4 ignore-rule scenario names "a harness directory" from a list that includes `worktrees/`, `.agents/` and `.cursor/`. `.gitignore` has no `worktrees/` entry, and the task cases test only `agent-context/` and `.claude/`. — `spec.md:85-86,106`, `tasks.md:96-99`, `.gitignore:1-13`
  Failure: a literal implementation of the spec reds on today's tree, or the author adds `worktrees/` to `.gitignore`, which nothing asked for. A tasks-literal implementation leaves `.agents/` and `.cursor/` unguarded, so un-ignoring them passes.
  Fix: list exactly which paths must stay ignored (`CLAUDE.md`, `AGENTS.local.md`, `/agent-context/`, `.claude/`, `.agents/`, `.cursor/`) and probe directories with a child path (`git check-ignore .claude/x`).
  Confidence: 60
- [WARNING] The PAC-6 running-host check has no word boundary and no case rule, so it gives false positives on contributors' machines. — `spec.md:145-147`
  Failure: a contributor whose machine is called `build`, `main`, `test` or `kube` (at least 4 characters, not `localhost`) gets a red `task check`, because AGENTS.md must contain `task build` and `task test` (PAC-1).
  Fix: match the hostname case-insensitively on word boundaries, and give a printed skip for hostnames that are already a required command or Taskfile word.
  Confidence: 60
- [WARNING] The PAC-6 generic patterns refuse legitimate public content, and the spec never says where a match has to start and end. — `spec.md:142-144`
  Failure: `git@github.com:PlatformRelay/kollect.git` is a `user@host:` address. A `\.local` match also hits `*.localhost` (dev HTTPS, `docs/development/setup.md`), `mise.local.toml` and the Kubernetes `svc.cluster.local`, all plausible in an operator project's AGENTS.md.
  Fix: anchor host patterns (`\.local\b(?!host)`), exempt `git@github.com:`, and add one no-op mutant that contains those strings and must stay green.
  Confidence: 55
- [WARNING] The PAC-6 "Pattern set can fail" scenario runs the copy's `--self-test`, and that self-test contains the same mutant again. The spec sets no depth bound. — `spec.md:159-162`, `tasks.md:14-15,114-115`
  Failure: the guard collects all failures before exiting (tasks:14), so each nested self-test runs the empty-pattern mutant again. That copies the full tree every level and recurses without limit in the required `lint` job.
  Fix: require a nesting guard, for example an env var set in the inner run that skips the recursive mutant.
  Confidence: 55
- [NOTE] PAC-5 contradicts itself. It requires a mutant for "each scenario" and says the self-test fails "when any mutant leaves the plain check green". But the PAC-3 dry-run scenario, PAC-1 "Fresh clone" and the PAC-6 empty-pattern mutant either cannot be mutated or leave plain green by design. — `spec.md:126-128` vs `:74-77,159-162`
  Fix: list the scenarios that get mutants and state that the expected outcome of the empty-pattern mutant is a red copy self-test.
  Confidence: 70
- [NOTE] PAC-1 says "every `task <name>` it names", but not where those names are read from. Prose such as "the task that…", and env-prefixed or flag forms (`task --list-all`, `KOLLECT_DEV_MINIMAL=1 task dev-up`), would be read as task names. — `spec.md:13-14`
  Fix: read task names only from code spans and code fences, skip flags, and accept the aliases `task --list-all` reports.
  Confidence: 50
- [NOTE] The gitleaks scenario does not say whether "allowlists AGENTS.md" means a text match or a regex match. A broad entry such as `.*\.md$` exempts AGENTS.md without naming it. Separately, removing `.gitignore`'s exemption at `.github/gitleaks.toml:10` takes the exemption off the renamed private `AGENTS.local.md`. The `--no-git` directory scan in `hack/check.sh:60` evidently sees ignored files (that is why `agent-context/` is allowlisted), so the maintainer's local `task check` may now scan the private notes. — `spec.md:119-122`
  Fix: evaluate each allowlist regex against the three paths. Decide explicitly whether `AGENTS.local.md` should be allowlisted.
  Confidence: 45
- [NOTE] "Headed LOCAL ONLY / never commit" leaves the header length and the case rule open, so a public opening line such as "Never commit secrets…" would trip it. — `spec.md:109-112`
  Fix: match the exact first-line marker that the private file carries.
  Confidence: 40
- [NOTE] The documents disagree on the pointer length: the spec allows at most 3 lines, while the proposal and tasks say one line. — `spec.md:39` vs `proposal.md:36`, `tasks.md:137`
  Confidence: 80
- [NOTE] PAC-4 requires `rg` semantics and makes an `rg` error fail the test. No workflow installs ripgrep, and none of the guards visible in the `lint` job uses `rg`. — `spec.md:82-83`, `.github/workflows/ci.yaml:283-620`
  Failure: if the runner image has no `rg`, the test exits 127 and turns `lint` red. It fails closed, but it is still red.
  Confidence: 40
- [NOTE] The plan does things no requirement covers: adding `CLAUDE.local.md` to `.gitignore` (tasks:139) has no requirement or check; nothing requires the `CONTRIBUTING.md` pointer line; and the `docs/development/setup.md:382` correction ("do not commit AGENTS.md") has no check, so if it is skipped the docs contradict the contract and nothing fails.

What I checked and found holds: the symbols the spec names exist (`internal/metrics/metrics.go:303`, `metrics_catalog.go:18`, `metrics_catalog_test.go:18`, `internal/sink/registry.go:44`). Every required task name is defined in `Taskfile.yml`. `scrub.sh:14` does swallow `rg` errors, as the spec says. The links test covers only tracked root Markdown. Every scrub pattern matches its own literal text, so planting each line as a mutant works.

## Could not check
- The other reviewers' records under `reviews/R-spec-set/`: not read, to keep this review independent.
- Whether the GitHub `ubuntu-latest` image ships ripgrep, and whether gitleaks `--no-git` honours `.gitignore`.
- The self-test's runtime cost (one tracked-tree copy plus `git init` and commit per mutant). I did not count tracked files or run anything.
- `hack/release/verify-eligibility.sh`, and the exact current lines of `cfc_guards`/CWS-6 after change 9.
- The maintainer's private files and other checkouts (task 0.x): outside this repository.
