verdict: APPROVE
findings:
  - [P2] errexit is silently off for every self-test case. `(run_case …) … && touch ok &` puts the subshell on the left of `&&`, and bash ignores `set -e` there. A mutant step that fails without its own `die` now carries on instead of aborting: if `m_t17_moved`'s `line="$(grep …)"` came back empty, the "moved" decoy would quietly turn into a plain deletion and still pass. Today every jmut/smut/dmut/ymut reads its result back, so nothing is hidden right now — `hack/test/renovate_policy_test.sh:1711`
  - [P2] T17's secondary patterns have no mutant of their own (`unused|not used|not installed`, `digest`, `dockerfile`, `devcontainer`, `release`). `m_t17_app`, `m_t17_scope` and `m_t17_release` each drop the whole fixture line, so the primary pattern is what fires and the secondary rows could be deleted from T17_ITEMS without any self-test going red — `hack/test/renovate_policy_test.sh:510-525,817-825`
  - [P2] T16 only runs the script under test against the fixture `renovate.yaml` (step index 2), never the tree's own workflow. A 2.3 script that hard-codes the fixture's step layout would pass T16. Today only 2.3's real run in `lint` would catch that — `hack/test/renovate_policy_test.sh:377-378`
  - [P2] The T17 comment and failure message say a level-2 heading, but the awk also starts and ends the section on `# ` headings. This is wording only; today's doc is unaffected (`## Renovate dependency updates`, line 157) — `hack/test/renovate_policy_test.sh:533-536`
matrix_disputes:
  - Row 2 (T11 "across … patch, minor, digest") — mutants cover `patch`, `vulnerabilityAlerts`, `lockFileMaintenance` and a manager block, but not `minor` or `digest`. The `..` walk is generic, so the class is still proven.
  - Row 8 (T17 "names the nine items") — the mutants prove each item's primary pattern can fail, not every pattern row (see the second finding).
could_not_check:
  - I ran nothing (read-only leg). The self-test result (136 mutants, 12 controls), the clause-removal probe and the real validator run are taken from evidence/1.2.md.
  - Whether yq's `@json` of an absent field prints an empty string, which T14's "absent" messages and the `"="` field logic depend on. I inferred it from the recorded green mutants `m_t14_automerge_absent` and `m_t14_group_absent`.
  - Whether `$(jobs -rp | wc -l)` really caps parallelism at `nproc`, or runs all 148 cases at once. That affects runtime only, not judging.
  - Whether yq is reachable by the scratch validator under `env -i` with `HOME` overridden on a mise-shim or snap-yq machine.
summary: T10–T17 match the task text, their extra strictness follows the spec and would not red a correct 2.2, 2.3 or 3.1 against today's renovate.json/docs shape, the fake docker fails closed (an unusual call shape gives a red, never a false green), and the parallel runner counts every failed case; only robustness and coverage nits remain.
