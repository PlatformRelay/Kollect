## Verdict: CONCERNS

## Findings
- [WARNING] The commit adds a guard that is red on its own and records it as Known red. `tasks.md:5-7` says the opposite: "a red guard cannot be committed alone… the tree is green at every commit". — `openspec/changes/public-agent-contract/loop.md:59-60`
  Failure: `hack/check.sh:91` sweeps `hack/test/*_test.sh`, so `task check` is red at `ec966638d` and at `10efd0903`. Bisecting through the range hits a red that no defect caused. Rebase-merge keeps both commits on `main`, so this lands there.
  Fix: fixup both guard commits into the G2 commit before the PR, as the evidence recommends. The alternative is to amend the preamble and record that in `decisions.md`. Leaving the choice open after the merge is not acceptable.
  Confidence: 90 (round 1 already raised it; the disposition is still open)
- [WARNING] PAC-2 checks that a pointer path exists on disk, but not that it is tracked or inside the repo. — `hack/test/agent_contract_test.sh:385-391`
  Failure: case 1: a pointer `[x](../CLAUDE.md)` resolves to `./CLAUDE.md`. That file is the ignored loader and it exists in the main checkout, so the pointer is green locally and dangles in every public clone. Case 2: a code span `` `../AGENTS.md` `` resolves from ROOT, which leaves the repo; in the main checkout the workspace parent has that file, so the result depends on the machine. Links resolve from `.github/` but code spans resolve from the root, so the two rules disagree.
  Fix: normalise the path, refuse anything outside ROOT, and require `git ls-files --error-unmatch`, the same way PAC-3 uses the tracked list. Add one mutant that links to an existing but ignored file.
  Confidence: 70
- [NOTE] PAC-3 only checks spans that contain `/`. A root file such as `Taskfile.yml`, `go.mod` or `PROJECT` in a checklist is never checked as a path, even though the header says "every path… matches a tracked file". — `hack/test/agent_contract_test.sh:190`
  Failure: when a root file is renamed, a checklist that still cites the old name stays green.
  Fix: also treat a span that matches a tracked root file name, or the extension form `\w+\.\w+`, as a path. Add one mutant.
  Confidence: 60
- [NOTE] The PAC-6 `Host` and scp patterns will likely give false reds on wrapped prose. — `hack/test/agent_contract_test.sh:87-88`
  Failure: a wrapped Markdown line such as `host the chart in your registry.` matches the SSH Host directive. So does `security@example.com: report…` for the scp pattern. G2's real `AGENTS.md` meets these lines, not the fixture.
  Fix: add one look-alike control with a wrapped "host …" prose line, and have G2 check its prose against it. Alternatively, only match `Host` inside fences.
  Confidence: 50
- [NOTE] The running-host skip scans all tracked Markdown. If the host name has leaked into any other tracked `.md` (evidence included), the check turns itself off with a printed skip instead of going red. — `hack/test/agent_contract_test.sh:240-243`
  Failure: a name leaked into one evidence file switches off PAC-6 for the agent files on that machine.
  Fix: limit the ordinary-word corpus to the docs and README, or exclude `openspec/changes/**`. Accepted by spec; recording it so the trade-off is visible.
  Confidence: 45
- [NOTE] The gitleaks parser reads allowlist tables only. A rule-level inline or dotted allowlist (`[[rules]]` … `allowlist = { paths = [...] }` / `allowlist.paths = [...]`) is not parsed. — `hack/test/agent_contract_test.sh:439`
  Failure: a per-rule exemption for `AGENTS.md` written in inline-table form passes. The exemption covers only that rule.
  Fix: also match `^\s*(allowlists?|allowlist\.paths)\s*=` inside `[[rules]]`, or reject inline allowlists outright.
  Confidence: 40
- [NOTE] The Sensors table contradicts the narrative in places. — `openspec/changes/public-agent-contract/evidence/G1.md:207-208`
  Failure: rows 207-208 say "128 ok" and "hand mutants (6), 6 caught". Lines 132 and 167 say 131 and 8 of 8. A reader can't tell which run backs the close.
  Fix: mark rows 207-208 as pre-round-1, or update them.
  Confidence: 85

What I checked and found sound:
- **Round-1 fixes.** I read all of `ec966638d..10efd0903`. TOML escape decoding, the `Host` comment form, the `git@github.com` scp exception, `HELPER_OUT` replacing `$(...)` (so the trap sees the directories), `MUT_ENV` reset on skip, and the xargs output test are all correct.
- **Gate parsing.** `check_gates` skips the `run_gate()` definition and gets the 17 calls in `hack/check.sh:42-63`.
- **Required-task mutants.** `drop_task`'s lookahead keeps `lint` and `test` from hitting `lint:shell` or `test-integration`.
- **Diagnostics.** No diagnostic prints a scrub pattern, a host name or file content beyond task, gate and path tokens.
- **Temp files.** Temp copies are created with `mktemp -d` (0700).
- **0.2 evidence.** It holds labels and counts only.

## Could not check
- I did not run the guard or `--self-test` (read-only leg). The scratch tree `/tmp/g1-scratch/tree` and its fixture `AGENTS.md` were not read, so the claims of 131 ok and 8/8 hand mutants rest on the evidence alone.
- I did not open `hack/scrub-patterns.txt`, `AGENTS.local.md` or `CLAUDE.md`, by instruction. Whether each pattern matches its own literal text (which the per-line plant needs) is unverified.
- 0.1c (local ignored files), the 0.2 inventory and the label map sit outside the repo or are ignored. Not checked.
- `gh pr view 471` and the merge sha `d8716e11e` were not verified.
