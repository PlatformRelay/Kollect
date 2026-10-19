## Verdict: CONCERNS

## Findings
- [WARNING] The change's own committed records publish private machine and harness details, which is the class of leak PAC-4 bans from the agent files — `openspec/changes/public-agent-contract/task-prompt.md:3`, `:7`, `:15`, `:17`, `:53`; `loop.md:3`; `tasks.md:133` (a test-literal home path)
  Failure: after merge, the public repository holds the maintainer's absolute home path and workspace layout, `~/.claude/skills/...` harness script paths, internal reviewer and model names, and private review workflow ("operator choice"). Leg files named `leg-claude-opus-*.md` add model names to product files. The guard only scans `AGENTS.md` and `.github/copilot-instructions.md`, so nothing catches this. Six earlier changes on main follow the same pattern (for example `openspec/changes/mutation-testing-signal/task-prompt.md`), so this repeats an existing pattern rather than starting one.
  Fix: before 4.1 archives the change, either drop `task-prompt.md` and `loop.md` from the commit (keep them in the ignored harness) or redact them to repo-relative paths. Ask the operator in the INBOX whether review records belong in the public tree at all.
  Confidence: 80
- [WARNING] The CI step pins a noble-only package while the job runs on `ubuntu-latest` — `.github/workflows/ci.yaml:634-640`, `:285`
  Failure: when `ubuntu-latest` moves past 24.04 (26.04 is already out), `apt-get install ripgrep=14.1.0-1` fails with "version not found", and the required `lint` check goes red on every PR. The evidence records this risk (`evidence/G2.md:226-230`) but nothing mitigates it.
  Fix: give `lint` `runs-on: ubuntu-24.04`, or install rg through a checksum-pinned `hack/install-ripgrep.sh` like gitleaks and zizmor.
  Confidence: 55
- [WARNING] PAC-4 "SHALL NOT contain an absolute home path" is only partly met — `hack/test/agent_contract_test.sh:265`
  Failure: `/root/.kube/config`, `/var/home/alice/x` (blocked by the `[^A-Za-z0-9_.~-]` prefix rule, since the character before `/home` is `r`), `C:\Users\alice` and `$HOME/x` all pass the plain check. Only `/home/` and `/Users/` with a non-word prefix are refused.
  Fix: add `/root/`, `/var/home/`, `[A-Za-z]:\\Users\\` and `\$HOME/` to the pattern, with one mutant each.
  Confidence: 70
- [NOTE] The PAC-6 running-host check turns itself off when the host name already appears in tracked Markdown — `hack/test/agent_contract_test.sh:240-243`
  Failure: if a hostname has already leaked into any tracked `.md`, the check prints "ordinary word" and skips, so the earlier leak disables the guard against the next one. The spec allows this skip (spec.md:192); the risk remains.
  Fix: print how many files matched, so a reviewer can tell a real dictionary word from a leak.
  Confidence: 60
- [NOTE] The scrub self-test plants each pattern line as literal text — `hack/test/agent_contract_test.sh:812`
  Failure: no pattern contains a regex metacharacter other than `.` today (I counted; nothing printed). A future pattern using `\b`, `|` or `(...)` would not match its own text, the mutant would survive, and the required `lint` job would go red in this guard for an unrelated scrub-list edit.
  Fix: document in the header that plants assume literal-style patterns, or skip the literal plant when the line has metacharacters and keep only the witness.
  Confidence: 60
- [NOTE] The self-test adds about 80 s to the required `lint` job and to `task check` (`evidence/G2.md:238`). Acceptable, but worth recording.

**PAC compliance (checked against code, not documents):**
- **PAC-1 holds.** Gate list `AGENTS.md:30-48` matches exactly the 17 `run_gate` calls in `hack/check.sh:42-63`. Every `task X` it names is defined in `Taskfile.yml` (spot-checked test-integration:explore :323, changelog:verify :413, coverage :230, docker:build:pipeline :356). The links resolve. `repo_root_links_test.sh:127` covers root `*.md`.
- **PAC-2 holds,** and the path check is stricter than the spec asks: a pointer target must be a tracked file inside the repo (`:292-296`).
- **PAC-3 holds** (`:149-209`). The paths I spot-checked exist. `TestCRDSpecOpenAPIFragments` exists (`test/schema/contract_test.go:25`) but is never checked, because its line has no `.go` path, which the spec permits.
- **PAC-4 is partial** (home-path finding above). The rest holds: rg errors are fatal (`:256-263`), the index is checked (`:411-414`), the probe repo runs with excludesFile set to /dev/null (`:418-440`), and the gitleaks allowlist goes through a real TOML walk plus `grep -P` (`:449-491`). The `AGENTS.md` allowlist entry is removed (`.github/gitleaks.toml`).
- **PAC-5 holds.** There is a mutant per scenario, wrong-clause and surviving-mutant probes (`:758-766`), and the nested-run guard (`:612`).
- **PAC-6 holds.** I checked the octet ranges, the ULA prefix and the allowed look-alikes (`:82-89`, `:894`).

**AGENTS.md claims, every one checked true on this tree:**
- `required_checks` matches `hack/release/verify-eligibility.sh:19-22`.
- Job bodies match: `ci.yaml:156-196`, `:917-949`, `:964-991`; `preflight.yaml:40-58`, `:61-68`; `e2e-smoke.yaml:131-165`, `:203-206`.
- The `hack/verify.sh:68` schema test runs under `task verify`.
- Scrub semantics match `hack/scrub.sh:8,14`; the Docker-header skip matches `check.sh:92-95`; the exclusion table matches `check.sh:120-124`; the gitmoji convention matches `CONTRIBUTING.md:51`.

**Not covered by any requirement:**
- The CI apt install of ripgrep.
- The `cfc_guards` extension (`ci_workflow_security_test.sh:615`).
- The `docs/development/setup.md` and `CONTRIBUTING.md` edits.
- The committed loop records and review records.

## Could not check
- I ran nothing (read-only leg). The pass results are taken from the recorded evidence (`evidence/G2.md:253-256`).
- Whether any committed openspec file contains this machine's or another machine's hostname: I cannot read the hostname, and I deliberately printed no hostname-shaped matches.
- `hack/scrub-patterns.txt` against the new openspec records: I did not open the pattern lines, by rule.
- Live branch protection, and whether `ubuntu-latest` has already moved to 26.04.
- Whether 0.2's clone inventory covered machines other than this host.
- `AGENTS.local.md` and `CLAUDE.md` were not opened, by rule.
