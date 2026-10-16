## Verdict: CLEAN

## Findings
- [NOTE] The step reader matches `uses:` case-sensitively, so a step written `uses: Go-Task/setup-task@…` would not be scanned at all — `hack/test/consistency_toolchain_pins_test.sh:201`
  Failure: GitHub resolves action owner and repo names case-insensitively. A step written that way would still install Task, but DTP-1 would never look at it. If it had no `version:` or an old one, the gate would stay green as long as the correctly spelled sites agree. The removed `setup_task_versions` had the same blind spot, so this task did not introduce it.
  Fix: lowercase `u` (`tolower(u)`) before the `index(u, action "@")` comparison.
  Confidence: 55
- [NOTE] No self-test case covers a mise.toml `task` value that is not a plain version (`"latest"`, `"3"`). DTP-1 now owns that value through `check_tool`'s not-plain branch, but that branch is only tested for gitleaks — `hack/test/consistency_toolchain_pins_test.sh:884-918`
  Failure: if the mise site were ever filtered before `check_tool`, `task = "latest"` would pass and no Task case would fail.
  Fix: add one `mutant_rejected` with `task = "latest"` and the sentinel `task: mise\.toml:[0-9]+ says 'latest', not a plain release`.
  Confidence: 40
- [NOTE] The `check_mise` header comment was rewrapped badly: the sentence breaks at "and" / "is read from go.mod", which reads as a fragment — `hack/test/consistency_toolchain_pins_test.sh:466-468`
  Fix: reflow it to two lines.
  Confidence: 90

Checked, holds:
- **The bump itself:** 18 `go-task/setup-task` `version:` edits (ci.yaml ×9, e2e-nightly ×4, docs, preflight, sonarcloud, changelog-sync, kind-e2e-setup action), plus `mise.toml:54` and its `:52` comment. A repo-wide grep for `3.51.1` outside openspec now only hits the self-test mutant lines (`:888`, `:894-895`).
- **Nothing the old check rejected is lost.** Removing `setup_task_versions` + `mise_assert_pin task`:
  - CI disagreeing with itself → `check_tool` disagreement.
  - mise differing from CI → mise.toml is now a DTP-1 site (`:534`).
  - Empty scan → `:529-530`.
  - mise has no task entry → presence check (`:487-488`), keeping the old `mise: mise.toml has no task entry under [tools]` prefix, so the empty-tools and `[settings]` sentinels still match.
  - The new code is stricter in two places: a `version:` under `env:` no longer counts, and a step with no `version:` now fails instead of being skipped.
- **git-cliff reading is unchanged.** `action_version_sites` changes only the `uses:` matcher, from a regex to a prefix `index`, which also accepts a leading quote as before. The git-cliff call passes `orhun/git-cliff-action` (`:510`).
- **All three DTP-1 spec scenarios have a case:**
  - One site missed → sonarcloud case (`:885`).
  - Whole set below the floor → `:891`. Its probe only proves the mise.toml edit landed, but a partial workflow edit would produce the disagreement message instead and fail the sentinel, so it is still sound.
  - Bot bump to a newer release → 99.0.0 control (`:898`).
- **The no-`version:` mutant is sound** (`:904`). The probe proves the step is still there and its `version:` is gone. The reader then emits `<no version: input>` for the now-empty `with:`.
- **The split no-ci-pins case loses no coverage.** Its node/python sentinel regex still matches `mise_assert_pin`'s message (trailing space after `setup-node`). Its task half is now the `task empty scan` case.
- **Mutant values** (3.50.0, 3.51.1, 99.0.0) are all past releases or implausible, never a likely next release.
- **CI wiring:** only the step name and comment changed. Both `run:` lines are untouched, so CWS-6/CFC-6 still match them.
- **setup.md row** says what the code checks (workflows and actions).

## Could not check
- I ran no tests (read-only leg). The plain-mode red/green, the `--self-test` 50 `ok` lines, the 8 hand mutants (the `/tmp/t21/` scripts are outside the repo), zizmor, the CWS self-test, `hyg_07` and the Task 3.52.0 release lookup are all taken from `evidence/T2.1.md` without re-running them.
- I did not read the renovate config. It is not in 2.1's scope (DTP-5 / task 3.1), so I did not check whether a Renovate bump would move all 19 Task sites together.
- I did not read `hack/test/ci_workflow_security_test.sh`. My CWS-6 conclusion rests on the unchanged `run:` lines in the diff.
