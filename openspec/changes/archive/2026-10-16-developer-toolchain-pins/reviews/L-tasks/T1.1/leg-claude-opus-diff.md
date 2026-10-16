## Verdict: CONCERNS

## Findings
- [WARNING] The shellcheck self-test cases hardcode `0.11.1` as the "other" release and `v0.11.0.2` as the control, so the next legitimate shellcheck bump breaks `--self-test`. — `hack/test/consistency_toolchain_pins_test.sh:541-556`
  Failure: Renovate or a person moves all three sites to 0.11.1 and adds the four digests. Then the `shellcheck ci.yaml drifts` mutant sets ci.yaml to the value every site already has, so there is no drift and the gate exits 0, and the self-test fails with "gate accepted". `shellcheck bumped without digests` now finds the digests and also exits 0. The `v0.11.0.2` control now disagrees with 0.11.1 and reds. A correct bump PR goes red in the required lint job because of the test, not the change. This is the bot-bump path DTP-1 and DTP-4 are meant to keep working.
  Fix: take the values from the tree. Read the current agreed version, use a version that cannot exist (for example `99.0.0`) for the drift and no-digest mutants, and use `v<current>.<n+1>` for the control.
  Confidence: 90
- [WARNING] The DTP-6 shrink-only check misses indexed and other non-`=` writes to the array. — `hack/test/consistency_toolchain_pins_test.sh:302`
  Failure: the definition grep only matches `UNVERIFIED_BY_DESIGN=` / `+=`. Adding `UNVERIFIED_BY_DESIGN[2]="install-kind.sh"` (or `mapfile`/`read -a` into the name) on its own line in `ci_fetch_lib_hardening_test.sh` leaves one `=(...)` line within the limit, so DTP-6 passes. Meanwhile the hardening gate's `"${UNVERIFIED_BY_DESIGN[@]}"` (`ci_fetch_lib_hardening_test.sh:770`) now exempts a third installer. The list grew and the ratchet did not fire.
  Fix: count every non-comment line that writes the name, using `UNVERIFIED_BY_DESIGN(\[[^]]*\])?\+?=|(mapfile|readarray|read[^#]*-a)[^#]*UNVERIFIED_BY_DESIGN`, and require exactly one. Add a self-test mutant for the `[2]=` form.
  Confidence: 75
- [NOTE] Nothing ratchets `UNVERIFIED_LIMIT` itself. — `hack/test/consistency_toolchain_pins_test.sh:68`
  Failure: one PR can add the same name to both `UNVERIFIED_LIMIT` and `UNVERIFIED_BY_DESIGN`, and the guard passes. Only review stops it. The spec says the list "SHALL only shrink".
  Fix: add an assertion that `${#UNVERIFIED_LIMIT[@]} -le 2` (lower the number in later tasks), or accept this and record it as review-enforced.
  Confidence: 60
- [NOTE] `cliff_action_sites` reads any `version:` key inside a git-cliff-action step, including one under `env:` or a sibling map. — `hack/test/consistency_toolchain_pins_test.sh:166`
  Failure: a step with `env: { version: x }` placed before `with:` would be read as the action's input. If anything breaks, the guard fails rather than passing wrongly, so this is low risk.
  Fix: only accept `version:` while inside the step's `with:` block.
  Confidence: 40

What I checked and found correct:
- The scan of each site kind matches today's sites: `ci.yaml:164`/`:299`, `install-gitleaks.sh:14`, `install-shellcheck.sh:15`, `.pre-commit-config.yaml:5`/`:56`, `Taskfile.yml:22`, `release.yaml:420` and `verify-changelog.sh:11`. Comment lines are skipped. `{{.GIT_CLIFF_VERSION}}` at `Taskfile.yml:374` is correctly not counted as a site.
- The floor comparison (`sort -V` on validated N.N.N) is correct, and the floor check only runs once all sites agree.
- The shellcheck platform extraction finds `linux`/`darwin` × `x86_64`/`aarch64`, and the digest lookup requires a 64-hex `echo` on the line after each case label.
- An empty scan and a missing named file both fail.
- The four-component truncation applies only to the shellcheck-py `rev` (`:363-366`).
- The `mutant_rejected` "one clause only" rule and the `probe` check that each mutant actually landed both hold.
- The wiring sentinels match CWS-6 at `ci_workflow_security_test.sh:590` and CFC-6 at `:608`.
- The lint steps sit after "Ensure yq is available" (`ci.yaml:337`), so yq is present when they run.
- Every self-test case required by task 1.1 is present. The four extra mutants are a plus.

## Could not check
- I did not run the guard or its self-test. The pass results (plain, `--self-test` 23 ok lines, `ci_workflow_security_test.sh` both modes, `lint:shell`) come from `evidence/T1.1.md:55-66`, not from my own runs.
- I did not check whether `.devcontainer/` or other non-`hack/` scripts install shellcheck or gitleaks through a pattern the scan does not cover. A text grep found none.
- I did not read the hand-mutant scratch script in `/tmp/t11-mut`; it is outside the repository.
