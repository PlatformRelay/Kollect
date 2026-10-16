## Verdict: CONCERNS

## Findings
- [WARNING] The two replacement comments say CWS-6 enforces "an unwired gate is not a gate" for every guard. It does not. The meta-test's own comment says CWS-6 only covers guards that CI already invokes, and that a guard whose steps are deleted outright "drops out silently". CFC-6 closes that gap, but only for `consistency_*_test.sh`, `go_patch_lag_test.sh` and `openspec_evidence_markdown_only_test.sh`. Neither hardening guard is on that list. — `hack/test/ci_fetch_lib_hardening_test.sh:844`, `hack/test/ci_install_helm_hardening_test.sh:686-687` (contradicted by `hack/test/ci_workflow_security_test.sh:597-603`)
  Failure: a maintainer believes the comment and deletes the self-wiring block below it as redundant. Then they drop the `CI-FETCHLIB-01` step from `ci.yaml`, and no remaining check reds, because nothing in CWS-6/CFC-6 names this script.
  Fix: reword both comments to "house rule; this guard asserts its own wiring below (CWS-6 only sees guards CI already runs; CFC-6 covers the consistency_* guards)", or drop the CWS-6 attribution.
  Confidence: 80
- [NOTE] `probe` now sends grep's stderr to `/dev/null` for negative probes. Before this change, a negative probe on a missing file also passed: grep exits 2, and `!` turns that into success. So the behaviour is unchanged, as the evidence says. But the "No such file" message was the only sign of a typo in a probe-file argument, and it is now gone. — `hack/test/consistency_toolchain_pins_test.sh:570`
  Failure: a future `!` probe naming a misspelled file passes silently, so a mutant that never landed can still be counted as proof.
  Fix: in `probe`, fail when `${COPY}/${file}` is missing, unless the probe is the explicit `!.` absence form used by the missing-`mise.toml` case.
  Confidence: 55
- [NOTE] The ci-self-drift probe `^[[:space:]]+version: 3\.50\.0$` matches any `version:` line in `kind-e2e-setup/action.yml`, not specifically the setup-task input. The sed range `/go-task\/setup-task@/,/version:/` would also start on a comment line that mentions setup-task. If that happens the gate reds for the wrong reason, which is a loud failure, not a silent one. So this is brittle but not vacuous. — `hack/test/consistency_toolchain_pins_test.sh:837-840`
  Fix: none needed now. Optionally skip comment lines in the sed range.
  Confidence: 40

What I checked and found sound:
- **Plain check vs the old guard:** `setup_task_versions`, `input_versions` and `mise_tool_value` keep the old awk programs unchanged. `mise_assert_pin` makes the same four checks in the same order (empty scan → CI disagreeing with itself → missing value → mismatch). The Go structural checks use the same regexes. The `go.mod` test (`{ -f && grep } || err`) is equivalent to the old one.
- **Scope:** the scan widened from `.github/{workflows,actions}` to all of `.github`. That is stricter, and the evidence records it under *Deviations*.
- **Fail-closed:** under `set -e`, `err` returns 0 and sets `BROKEN`, so `check_mise` always runs to the end. Nothing in `check_all` returns early before `check_mise` (`:499-533`).
- **Each of the 12 old cases has a counterpart** with the same trigger and the same failure message (now prefixed `mise:`), as the mapping table claims (`:803-870`). `mutant_rejected` also requires that no other clause fires (`:600-602`), so each case proves its own clause.
- **Mutant fit with the real `mise.toml`:** `[settings]` comes before `[tools]` and `[tools]` is the last table, so the empty-tools range deletion and the no-tools-table deletion behave as intended. The extra `[settings]` mutant is sound.
- **Wiring:** the new step is in the unconditional `lint` job (`ci.yaml:563-566`). The old step is gone. CFC-6's `consistency_*_test.sh` glob (`ci_workflow_security_test.sh:602`) now covers the folded check, which is stronger than the old grep for its own step. The 1.1 wiring mutants (`:872-889`) still target the renamed step. Removing the step's `- name:` line in `drop_step` still works with the longer name.
- **CWS-6 composite-action mutant:** retargeted at `ci_workflow_security_test.sh:1147-1148`, and the in-command grep is updated with it.
- **References:** `renovate.json:74`, `mise.toml:8`, `.gitignore:26` and `docs/development/setup.md:53` are updated. A grep for `dev_mise_pin_drift` finds it only in openspec records: this change, change 4 (`cross-file-consistency-gates`) and the archive. Change 4's `task-prompt.md:51` and `proposal.md:42` still name the deleted file as a template; the task text says to keep change 4's records as history.

## Could not check
- I ran nothing: as a `claude/` leg my Bash is limited to `git` commands. The self-test, plain-mode, CWS-6 probe and hand-mutant results in `evidence/T1.2.md` are taken from the evidence, not reproduced, and I did no mktemp mutation runs.
- The evidence's claim that the wider `.github` scan reads the same 21 lines.
- Whether change 4 is fully merged, which decides whether its `task-prompt.md:51` counts as history or as live guidance.
- The review legs under `reviews/L-tasks/T1.2/` and the `/tmp/t12/` run scripts, which were out of scope or out of bounds.
