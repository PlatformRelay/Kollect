## Verdict: CONCERNS

## Findings
- [WARNING] Under PAC-4, `git check-ignore --no-index` also reads the user's global `core.excludesFile`, so the ignore-rule results depend on the machine — `spec.md:128`
  Failure: a maintainer or contributor whose global gitignore lists `.claude/` or `agent-context/` passes the "stops ignoring `.claude/`" check even when `.gitignore` lost the rule. The matching self-test mutant then survives, so `task check` (which runs `--self-test`, `hack/check.sh:102-109`) goes red locally while CI stays green. A global ignore that lists `AGENTS.md` turns the plain check red with no fault in the repo.
  Fix: make the spec require evaluation against the repository's `.gitignore` only, for example `git -c core.excludesFile=/dev/null check-ignore --no-index` with an empty `info/exclude` in the copy.
  Confidence: 75
- [WARNING] The PAC-6 `user@host:` ban catches image digests and DSNs that the repo's own docs use — `spec.md:179-180`
  Failure: an `AGENTS.md` that states the digest-pinning rule as `ghcr.io/platformrelay/kollect@sha256:…` (as `docs/security/security-architecture.md:347` and `docs/adr/0708-operator-distribution-hubs.md:65` do) or quotes the sample DSN (`docs/examples/postgres-state-store.md:46`) goes red. The "Allowed look-alikes" scenario (`spec.md:199`) does not cover either case.
  Fix: exempt `@sha256:`, and URIs that carry `://` before the `@`, or narrow the pattern to the SSH/scp form (`^\S+@[^/\s]+:[^/\d]`). Add `kollect@sha256:` to the allowed-look-alikes scenario.
  Confidence: 70
- [WARNING] The PAC-3 symbol rule ("declared in that file", the `.go` path on the same line) contradicts the PAC-3 Metric scenario — `spec.md:71-78`
  Failure: the scenario requires `internal/metrics/metrics.go`, `Catalog` and `registeredMetricNames`. But `Catalog` is declared in `internal/metrics/metrics_catalog.go:18` and `registeredMetricNames` in `internal/metrics/metrics_catalog_test.go:18`, not in `metrics.go`. If the line names only `metrics.go`, the symbol check fails. If the symbols sit on a line with no `.go` path, they are never checked. The proposal's line names three `.go` files (`proposal.md:107-109`), and the spec does not say which of them a symbol must be declared in.
  Fix: name `metrics_catalog.go` and `metrics_catalog_test.go` in the scenario. Define the rule for a line with several `.go` paths (declared in any of them, or in the nearest preceding path).
  Fix also: say that methods (`func (r *Registry) Register`, `internal/sink/registry.go:61`) and grouped `var (`/`const (` blocks count as declarations.
  Confidence: 80
- [WARNING] The spec never says where in `AGENTS.md` the gate list and the three checklists are, yet PAC-1 "names exactly" and the PAC-3 heading and stale-path checks depend on it — `spec.md:16-17,68-70`
  Failure: the gate names overlap with task names and prose (`lint`, `test`, `build`, `scrub`, `helm` vs `helm-test`). A test that scans the whole file cannot tell "names a gate check.sh does not run" from an ordinary mention, so it either false-reds or never fires. Two implementations can both claim conformance and disagree.
  Fix: specify anchors, for example a fixed heading or HTML-comment markers around the gate list and named headings for the CRD-field, metric and sink checklists. Say how `run_gate` names are parsed: the first argument of a non-comment `run_gate` call, ignoring the `run_gate()` definition at `hack/check.sh:26`.
  Confidence: 70
- [WARNING] The PAC-4 gitleaks clause names no regex engine and no set of allowlist tables — `spec.md:141-144`
  Failure: `(?:^|/)AGENTS\.md$` (`.github/gitleaks.toml:10`) is RE2/PCRE syntax. Bash `[[ =~ ]]` (ERE) treats it as invalid or non-matching, which reads as "no allowlist" and passes. Gitleaks v8 also accepts `[[allowlists]]` and `[[rules.allowlists]]` path lists, which a check of `[allowlist]` alone never sees.
  Fix: require an RE2- or PCRE-compatible matcher (for example `rg -P` or `grep -P`) that fails closed on a regex error. Name every allowlist table form gitleaks reads, or forbid any allowlist other than `[allowlist]`.
  Confidence: 65
- [NOTE] The OpenSpec marker clause "or similar" cannot be tested — `spec.md:138`
  Fix: list the exact markers, for example `<!-- OPENSPEC:START -->` and `<!-- OPENSPEC:END -->`, case-insensitive.
  Confidence: 85
- [NOTE] The PAC-6 private ranges miss `100.64.0.0/10` (CGNAT, used by Tailscale) and IPv6 ULA `fd00::/8`, both common on a maintainer's private network — `spec.md:177-178`
  Fix: add them, or record the omission as an accepted gap that review 3.1 covers.
  Confidence: 50
- [NOTE] The PAC-6 hostname check has no skip for a dictionary-word hostname such as `docs`, `main` or `kollect`, which `AGENTS.md` will contain — `spec.md:181-183`
  Failure: a contributor's `task check` goes red. The only escape is `AGENT_CONTRACT_HOSTNAME`, which the spec documents as being for tests.
  Fix: document the override for contributors, or skip names that are also words in the tracked docs.
  Confidence: 45
- [NOTE] The PAC-5 rule "reds with another clause's diagnostic" is ambiguous when a mutant trips its own clause and another one; tasks.md says all failures are collected — `spec.md:152-153`
  Fix: say that the target diagnostic must be present, and that extra diagnostics are allowed or forbidden.
  Confidence: 55
- [NOTE] The PAC-6 empty-pattern-set mutant runs the copy's whole `--self-test`, about 40 or more mutants, each a fresh `git init` plus `task --list-all`. That roughly doubles self-test time in the required `lint` job — `spec.md:154-155`
  Confidence: 60

What holds:
- The required tasks exist (`Taskfile.yml`: build, test, lint, verify, check, scrub, lint:markdown, spec:validate, lint:shell, helm-test, test-integration).
- `task` is installed in the `lint` job (`ci.yaml:297`).
- `hack/scrub-patterns.txt` has no regex metacharacters other than `.` and no blank lines, so the `.` witness in PAC-4 is enough today.
- `scrub.sh:14` does `2>/dev/null` with `if`, so an `rg` error reads as "no match", as the spec states.
- The gitleaks entry is at `.github/gitleaks.toml:10`.
- `.gitignore:12` ignores `AGENTS.md`, `CLAUDE.local.md` is not ignored yet, and `agent-context` is anchored as `/agent-context/`.
- `docs/development/setup.md:382` says not to commit `AGENTS.md`.
- `repo_root_links_test.sh` scans the tracked root Markdown files.
- `NewRegistry` is at `internal/sink/registry.go:44`, `Register` at `internal/metrics/metrics.go:303`, and the linked docs exist.
- Nothing in the spec is stronger than the code: no test exists yet.

## Could not check
- Running anything: `task --list-all` output format and namespacing of the `test:` include, gitleaks' accepted allowlist forms on the pinned version, and `rg` on the `lint` runner. Bash was denied apart from git commands; `git show` was refused too.
- `hack/test/ci_workflow_security_test.sh` CWS-6 and CFC-6 (`:591-611`), not read.
- The other reviewers' records under `reviews/`, deliberately not read.
