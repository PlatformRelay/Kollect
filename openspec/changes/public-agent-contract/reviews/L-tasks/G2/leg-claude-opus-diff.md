```text
## Verdict
verdict: APPROVE
findings:
  - [P2] The citation table points back into its own file at line numbers from before the file grew, so `evidence/G2.md:33-37`, `:50`, `:62` and `:66-70` land on the wrong text. For example, the CRD table is at :49-55 and the sink table at :74-80 — `openspec/changes/public-agent-contract/evidence/G2.md:168-187`
  - [P2] Some workflow citations have drifted: the `test` report job is at `ci.yaml:964`, not 950; `kind-smoke-run` is at `e2e-smoke.yaml:131`, `pipeline-cli-smoke` at :150 and `kind-smoke` at :203 — `evidence/G2.md:159-165,191`
  - [P2] The tracked evidence describes private tool set-up: user-level and workspace instruction files loaded from parent directories, "an API key this machine does not use", the "claude/opus" model name and the tool version. The G1 review files already in the tree follow the same pattern, so this is consistent but avoidable in archived text — `evidence/G2.md:27,98-104,213`
  - [P2] The `lint` CI row says the guards run "each plain and, where it has one, with `--self-test`, as separate steps". In fact `ci.yaml:310-313` runs two guards in one step, and the tool-pins guard's `--self-test` runs in another job. The second part is stated in the next row, so this is only a wording issue — `AGENTS.md:70-71`
  - [P2] The commit subject carries "(WIP, folded before review ends)", and tasks 2.1-2.4 are still unticked even though the evidence's scope list names `tasks.md` and `loop.md`, which this diff does not touch. Both are expected to change at the fold, so they should be checked then — `evidence/G2.md:10`
matrix_disputes:
  - #9 (rg on the runner): the evidence says `ripgrep=14.1.0-1` exists in the noble archive, but no CI run has printed `rg --version` yet. The row should be "pending" until a PR run shows it.
could_not_check:
  - I ran no sensors (read-only leg) and relied on the recorded gate record. I did not confirm that the apt pin resolves on the current `ubuntu-latest`, and I did not run gitleaks or the guard self-test.
summary: I checked 14 AGENTS.md statements against their sources: the gate list matches hack/check.sh:42-63 exactly, the required checks match verify-eligibility.sh:19-22, and every Taskfile desc, checklist path and symbol I checked exists (registerTo, Catalog, registeredMetricNames, NewRegistry, ToKollectSinkSpec, family_sink.go, the UPDATE_GOLDEN command, INTEGRATION_PACKAGES, docs/crds). The gitleaks change removes only the AGENTS.md allowlist entry, .gitignore drops only the AGENTS.md line, and the rg step is pinned, fails loudly and sits before both guard steps. I found no leak in AGENTS.md or in the pointer file.
```
