## Verdict: CONCERNS

## Findings
- [WARNING] The evidence says a kubectl global flag that takes a value before the verb makes the guard fail. In fact it passes silently — `hack/test/readme_quickstart_test.sh:222-226`
  Failure: in the README line `kubectl --context kind-kollect get kinv,foo -A`, `--context` falls into `-*) ;;`. That makes `kind-kollect` the verb and `get` the resource. Because `verb != get`, nothing is checked and the guard exits 0, even though `foo` doesn't exist. The same happens with `--kubeconfig <path>`, `--cluster x` and `--user x`. This contradicts G3.md's "Known limits (fail closed …)" paragraph, which says these cases make the guard fail.
  Fix: fail when the first non-flag word after `kubectl` is neither a known verb nor in a small allow-set. Alternatively, add `--context|--kubeconfig|--cluster|--user|-s|--server` to the value-flag list. Either way, add an X-case (`kubectl --context x get kinv,foo`) and correct the evidence wording.
  Confidence: 90
- [WARNING] The "Mutation proof" table (20 of 20 killed) rests on `/tmp/claude/g3/hand_mutants.py`, which is not in the repo and not inlined in the evidence. The claim can't be reproduced, and the evidence class says "machine" — `openspec/changes/test-depth-signals/evidence/G3.md:107-109`
  Failure: a reviewer or the stage-B holistic run cannot re-run the mutants. The repo's own decision of 2026-10-16 (cited at `.github/workflows/ci.yaml:603-606`) is to inline probes in the evidence .md.
  Fix: inline the mutant list as sed/patch snippets in G3.md, or tag the table **believed**.
  Confidence: 75
- [NOTE] Token splitting relies on GNU `sed`, which interprets `\n` in the replacement as a newline. BSD/macOS sed inserts a literal `n`, so the real README fails locally on macOS (`task demo-up n preferred: …` → `'n'` is not a task) — `hack/test/readme_quickstart_test.sh:201`
  Failure: this is not a CI risk (ubuntu runner). It is a dev-machine false red, and the repo already guards against GNU-only sed elsewhere (`hack/test/dist_operatorhub_pr_test.sh:33-37`). The awk interval `/^ {0,3}…/` at `:158` also needs a mawk with repetition support.
  Fix: do the split in bash (`${text//&&/$'\n'}` and similar) or in awk. Use `/^ ? ? ?[^[:space:]]/` or `/^[^ ]|^ [^ ]|^  [^ ]|^   [^ ]/`.
  Confidence: 60
- [NOTE] The guard only treats a `## ` line inside an sh fence as "unclosed". An `# ` (H1) heading, or a non-sh fence left open, swallows the rest of the file, and the END branch's message says "the sh fence opened at …" even when the open fence is a `text` fence. The END/EOF branch has no self-test case, as the evidence itself says (`G3.md:63`) — `hack/test/readme_quickstart_test.sh:179-182`
  Fix: match `/^##?[[:space:]]/` in the unclosed rule, and add one EOF case (delete everything after FENCE2's opener).
  Confidence: 55
- [NOTE] As the spec allows, ```` ```bash ````/```` ```shell ```` fences are not read. If one is added next to an existing `sh` fence, `task nosuch` inside it passes. R8 only catches the case where every fence changes language. Spec-conformant (`specs/test-depth/spec.md:158-160`), so this is recorded rather than blocking — `hack/test/readme_quickstart_test.sh:176-180`
  Fix (optional): fail `extract` on any fence in the section whose language is a shell dialect other than `sh`.
  Confidence: 70
- [NOTE] Tasks marked `internal: true` count as existing. A README naming one would pass, yet `task <it>` errors for users — `hack/test/readme_quickstart_test.sh:124-129`
  Fix: drop keys whose body has `internal: true`, or record this as a known limit.
  Confidence: 50

Checked and found OK:
- CI wiring. Both modes run as separate steps in `lint` (`ci.yaml:603-606`), and `lint` has no `needs:`/`if:`, so README-only PRs still run it (`ci.yaml:24-28,279`). On push to main, `README.md` is in `paths-ignore`, but PRs gate the change.
- `cfc_guards` addition and its yq mutant. They delete both steps and assert the CFC-6 message (`ci_workflow_security_test.sh:1142-1148`).
- Assertions. `reject` checks the clause and the token, not just the exit code (`:304-314`).
- Self-test hygiene. Anchors fail loudly if they don't match (`:279-300`), the R3b post-check works on stale `OUT`, temp copies are cleaned up, and `set -e` is safe in `((…))`/`&&` lists.
- Include parsing against `Taskfile.yml:58-60` and the shortNames in CRDs.
- No duplication of `demo_task_aliases_test.sh` (this guard checks names only, no delegation).
- tasks.md 3.1/3.2: R1–R10 and R3b are all present.

## Could not check
- I did not run the guard, its self-test or the cws self-test (read-only leg). The sensor results in G3.md are taken as recorded.
- `/tmp/claude/g3/hand_mutants.py` is outside the repo and not readable.
- I did not confirm the mawk version on the CI runner, or whether `task lint:shell` covers the new script.
