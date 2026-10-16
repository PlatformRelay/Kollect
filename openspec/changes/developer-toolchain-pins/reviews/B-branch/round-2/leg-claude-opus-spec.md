## Verdict: CONCERNS

## Findings
- [WARNING] The bump control covers only pins the bash model of Renovate's custom regex managers extracts, not "every pin Renovate extracts" (DTP-7). — `hack/test/consistency_toolchain_pins_test.sh:1260-1265`
  Failure: Renovate's built-in github-actions manager may extract `node-version`/`python-version` from `actions/setup-node`/`setup-python` (I'm not certain it does). If it does, a Renovate PR that moves `node-version: "22"` in `docs.yaml`/`preflight.yaml` leaves `mise.toml` `node = "22"` untouched, because the mise manager is off. DTP-5 then reds that PR, and the control never tries this case. The `ci.yaml` comment "nothing else manages the node/python inputs" would also be wrong.
  Fix: check whether the G3 dry-run lists `node-version`/`python-version` deps. If it does, either disable them with a packageRule or narrow the DTP-7 wording to "every pin the custom managers extract".
  Confidence: 45
- [WARNING] Nobody has checked the golangci-lint grouping (DTP-4, 6.7) against real Renovate. The code says the 44.115.4 dry-runs were "not re-run after golangci-lint got its own group". — `hack/test/consistency_toolchain_pins_test.sh:634-637`, `renovate.json:187-191`
  Failure: the model does not expand `extends` presets. If a preset's group rule also matches `golangci/golangci-lint`, real Renovate would put it in a different group than the model reports, and the guard stays green.
  Fix: re-run the `--platform=local` dry-run on HEAD and record the golangci-lint `groupName` in the evidence.
  Confidence: 35
- [NOTE] CFC-2's `PIN_VERSION_RE` is stricter than task 6.5 asked for: it now also rejects literal partial pins (`@v1.9`), not only values coming through a Taskfile var. — `hack/test/consistency_tool_pins_test.sh:211,225`
  Failure: a legitimate literal `@vX.Y` pin that used to pass is now red. That is arguably right (it floats), but it is a semantic change to change 4's guard that the commit subject only partly states.
  Fix: record it as stronger-than-spec in the CFC-2 spec/decisions.
  Confidence: 70
- [NOTE] `tool-pins-selftest` now runs two self-tests: 15 min measured locally, 15–25 min according to the `ci.yaml` comment. The 45-minute timeout was set when the job ran one 7–17 min test. — `.github/workflows/ci.yaml:602`
  Failure: a cold module/vuln-DB cache on a slow runner could push past 45 min. The two tests run serially, so one stall costs both.
  Fix: no change needed now; watch the first CI timings, or split the two into parallel jobs.
  Confidence: 30
- [NOTE] `install-helm.sh:71` and `install-polaris.sh:47` still call `sha256sum` directly, with no `shasum` fallback. They fail closed through pipefail, so DTP-6 (which names git-cliff and helm-docs only) still holds, but on macOS `task tools:helm` dies with "command not found".
  Fix: switch both to `verify_sha256`. It is a follow-up, outside this spec.
  Confidence: 80
- [NOTE] G6 evidence says intermediate commits "were not each run". `AGENTS.md` requires a green tree at each commit, and the tree at `632246c28`→`4869b4d40` is unverified. — `evidence/G6.md:89`
  Confidence: 50

### DTP compliance (whole branch, checked against code)
- DTP-1 **holds**: `check_tool task` uses `TASK_FLOOR=3.52.0` (`:100,1062-1073`). The `uses:` match is now case-insensitive (`:225`). Mutants cover mixed case, `latest` and the whole-set move.
- DTP-2 **holds**: `check_govulncheck` (`:979`). The mutant values now come from the tree (`GV_CUR`).
- DTP-3 **holds**: gitleaks and git-cliff floors (`:98-99`), shellcheck compared on its first three components (`:291`), digest table (`:354`), empty scan (`:311`). git-cliff sentinels are read from the tree (`GC_CUR`).
- DTP-4 **holds, with a stronger-than-spec clause stated in the spec**: the `ALONE` loop covers shellcheck and golangci-lint (`:899-956`), and the "golangci-lint group rule removed" mutant is present. Not checked against real Renovate (see above).
- DTP-5 **holds**; the stronger-than-spec dotted-number clause is now in the spec text.
- DTP-6 **holds**:
  - git-cliff looks for `sha512sum`, then `shasum`, and refuses before downloading if neither exists (`install-git-cliff.sh:45-52`); it requires a 128-hex digest (`:55`).
  - helm-docs requires a 64-hex digest (`install-helm-docs.sh:49`), and the shared helper falls back to `shasum` (`verify-sha256.sh:22-29`).
  - The self-test runs the installers with `shasum` only, with no hash tool, and with a short digest (`:1715-1741`). It probes that `sha*sum` are really hidden, and the `[0-9]*` in the pattern also excludes `shasum` in the no-hash case.
- DTP-7 **holds**:
  - The self-test is wired into `tool-pins-selftest` (`ci.yaml:615-616`), and the plain mode stays in lint.
  - The CFC-6 loop pins both self-tests to that job (`ci_workflow_security_test.sh:157-162`), with a separate move-back-into-lint mutant for each script.
  - The bump control rewrites 41 sites and checks both modes, with a nested-run guard so it doesn't recurse.
- Not in any requirement: the CFC-2 changes (outside this capability) and the `ci.yaml` comment rewording.

## Could not check
- Never ran any guard or self-test (read-only leg). The pass/fail results above are the ones recorded in `evidence/G6.md`, not reproduced.
- Real Renovate extraction for setup-node/setup-python and for preset group rules: no dry-run output exists at HEAD.
- The full `consistency_toolchain_pins_test.sh` body before line 286 and the `renovate_deps` model (`:644-860`): only the parts touched by the diff were read in detail.
- Earlier review records and anything outside this worktree.
