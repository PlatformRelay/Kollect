# Tasks

Each behaviour task runs after the test or guard task that can fail on it. A check that is red on
today's tree is shown red in that task's evidence and lands in the same commit as the behaviour
task that turns it green, so `main` stays green at every commit.

## 1. Guard first (DTP-1, DTP-2, DTP-3, DTP-5, DTP-6, DTP-7)

- [ ] 1.1 Write `hack/test/consistency_toolchain_pins_test.sh` (plain and `--self-test`) with every
  check except manager liveness (that is 3.2), and add its two steps to the `lint` job. Self-test
  cases on throwaway copies, each with the failure it exposes:
  - DTP-1 given 18 setup-task sites and `mise.toml` at 3.52.0, when one site says 3.51.1, then red
    naming that file (a missed site); when all say 3.51.1, then red (set below floor); when all say
    a newer release, then green (exact-value check that would block bot bumps).
  - DTP-2 given `GOVULNCHECK_VERSION` with a `# renovate:` line, when `task vulncheck` carries a
    literal `govulncheck@v1.9.0`, then red (inline pin returns); when the annotation is deleted,
    then red (Renovate stops seeing it).
  - DTP-3 given gitleaks 8.30.1 at `ci.yaml`, `hack/install-gitleaks.sh` and
    `.pre-commit-config.yaml`, when the pre-commit `rev` says v8.29.0, then red (a site drifts);
    when all three say 8.29.0, then red (below floor); when `GIT_CLIFF_VERSION` says v2.12.0, then
    red; when the gitleaks scan finds no site, then red (empty scan).
  - DTP-5 when a `mise.toml` comment says `helm (v3.21.4)`, `go 1.27.2` or "all 17 occurrences",
    then red (stale restated version or count); when a comment names the pin it sits on, then
    green (false positive).
  - DTP-6 when `UNVERIFIED_BY_DESIGN` in `hack/test/ci_fetch_lib_hardening_test.sh` gains an
    entry, then red (the exceptions list grows).
  - DTP-7 a no-op copy of the real tree passes (a guard that reds on correct content).
  Run it on today's tree and record it red on the 18 + 1 Task 3.51.1 sites, the inline
  govulncheck, and the `mise.toml:24`, `:28`, `:39` and `:52` comments. Show CFC-6 red when either
  new step is removed from `ci.yaml`.
- [ ] 1.2 Fold `hack/test/dev_mise_pin_drift_test.sh`: port every existing inline mutant one-to-one
  into `--self-test`, show each red against the new guard, list the mapping in evidence, then
  delete the old guard and its `ci.yaml` step. The mutants to port (old label -> expected
  failure): `honest` (green), `task-drift` (mise task 3.50.0 vs CI), `node-drift` (node 20 vs 22),
  `python-drift` (python 3.11 vs 3.12), `go-pinned` (`go` under `[tools]`), `go-setting-gone`
  (`idiomatic_version_file_enable_tools` removed), `gomod-gone` (no `go` line in `go.mod`),
  `ci-self-drift` (workflow and composite action disagree on task), `no-ci-pins` (empty CI scan),
  `empty-tools` (empty `[tools]`), `no-tools-table` (no `[tools]` table), and the missing
  `mise.toml` case. Its self-wiring check (`grep` for the step in `ci.yaml`) is replaced by CFC-6
  enrolment, shown red in 1.1. Delete `ci.yaml:411-419` (comment and step). Update every live
  reference to the old name: `renovate.json:74` description, `mise.toml:8`, `.gitignore:26`,
  `docs/development/setup.md:53`, the CWS-6 composite-action mutant at
  `hack/test/ci_workflow_security_test.sh:1145-1146` (retarget it to the new guard and show it
  still reds), and the comments at `hack/test/ci_fetch_lib_hardening_test.sh:844` and
  `hack/test/ci_install_helm_hardening_test.sh:687`. Change 4's evidence and review files keep the
  old name as history.

## 2. Pins (DTP-1, DTP-2, DTP-5)

- [ ] 2.1 Task 3.52.0 at all 18 setup-task sites and `mise.toml:54`; 1.1's DTP-1 check turns green.
- [ ] 2.2 Test change on a guard owned by change 4, first: run
  `bash hack/test/consistency_tool_pins_test.sh --self-test` green, then move govulncheck to
  `GOVULNCHECK_VERSION` (Renovate-annotated, value v1.9.0) and show the CFC-2 mutants that `sed`
  the literal `govulncheck@v…` in `Taskfile.yml` (~499-501 "unresolvable pin", ~523-525 "too old
  for go.mod's Go") fail to apply. Re-anchor both onto the `GOVULNCHECK_VERSION:` line, and confirm
  the two that match `govulncheck@` without a version (~503-505 "package path typo", ~527-529 "pin
  missing") still apply and red for their stated reason. Confirm the plain run resolves
  `{{.GOVULNCHECK_VERSION}}` (the variable lookup at ~162) rather than reporting
  `<unresolved:…>`. Record a real `task vulncheck` run. 1.1's DTP-2 check turns green.
- [ ] 2.3 Replace the restated versions and count in `mise.toml:24`, `:28`, `:39` and `:52` with
  pointers to the owning file; 1.1's DTP-5 check turns green. The Go half of DTP-5 needs no
  change: it is the existing DEV-MISE-01 check, carried over by 1.2.

## 3. Managers (DTP-4)

- [ ] 3.1 Probe which files and dependencies each custom manager matches
  (`renovate-config-validator --strict` plus a local dry run). Settle and record what "a manager
  matches a file" means: at least one `managerFilePatterns` hit on a tracked file AND at least one
  `matchStrings` dependency extracted from it. The gitleaks and shellcheck managers pass the first
  half and fail the second, which is why a file-pattern-only check would call them alive.
- [ ] 3.2 Add the manager-liveness and site-coverage checks to the guard, using 3.1's definition,
  with self-test cases: given today's `renovate.json`, then red naming the gitleaks
  (`renovate.json:37-47`) and shellcheck (`renovate.json:48-58`) managers (dead manager); given a
  manager whose file pattern matches no tracked file, then red (pattern rot); given no manager
  covering `Makefile` `GOLANGCI_LINT_VERSION`, `hack/tooling/.custom-gcl.yml`,
  `.pre-commit-config.yaml`, `hack/install-gitleaks.sh`, `ci.yaml:164` or `ci.yaml:299`, then red
  naming the file (uncovered site); given two golangci-lint sites in different groups, then red
  (sites can move apart). Record it red on today's tree.
- [ ] 3.3 Replace the dead gitleaks manager with one that matches `ci.yaml:164`,
  `hack/install-gitleaks.sh:14` and `.pre-commit-config.yaml:5`; replace the dead shellcheck
  manager with one that matches `ci.yaml:299`; add managers for `Makefile:184` and
  `hack/tooling/.custom-gcl.yml:6` in one group. Re-run 3.1's probe; 3.2's checks turn green.

## 4. Installers (DTP-6)

- [ ] 4.1 Probe the upstream checksum files for every platform each installer supports: git-cliff
  per-asset `<asset>.sha512` (the `.sha256` is 404), helm-docs `checksums.txt`. Stop if one is
  missing.
- [ ] 4.2 Tests first, with a fake `fetch_to`, per installer: given a tarball whose digest differs
  from the published one, then the installer exits non-zero and the output path does not exist
  (tampered download installed); given a checksum fetch that fails, then the same (silent
  fallback to unverified). Widen the checksum scan in `hack/test/ci_fetch_lib_hardening_test.sh`
  (`verify_sha256|sha256sum`, ~779 and ~793) to accept `sha512sum`, with a mutant that deletes the
  `sha512sum` call from a copy of `install-git-cliff.sh` and must red once the entry leaves
  `UNVERIFIED_BY_DESIGN`. Watch all red.
- [ ] 4.3 Verify git-cliff with `sha512sum` against `<asset>.sha512` and helm-docs against
  `checksums.txt` through `hack/lib/fetch.sh`; remove the NOTE at `install-git-cliff.sh:40-42` and
  `install-helm-docs.sh:40`; remove both entries from `UNVERIFIED_BY_DESIGN` (~766). 4.2's tests
  and the stale-exemption check turn green.

## 5. Land

- [ ] 5.1 Archive the change as the last commit of the PR, after review and green CI; the review
  record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DTP-1 | guard: one-site and below-floor mutants red; newer set passes | as stated | not-run | |
| DTP-2 | inline-govulncheck and dropped-annotation mutants; CFC-2 self-test after re-anchoring; `task vulncheck` real run | red; CFC-2 green; run result recorded | not-run | |
| DTP-3 | pre-commit-rev, below-floor and empty-scan mutants | red | not-run | |
| DTP-4 | dead-manager, rotted-pattern, uncovered-site and split-group fixtures | red | not-run | |
| DTP-5 | ported go/node/python/task mutants; stale-version and site-count comment mutants | red | not-run | |
| DTP-6 | tampered and missing-checksum cases per installer; grown-exceptions mutant | installer fails, nothing installed; red | not-run | |
| DTP-7 | all mutants plus no-op by exit status; old-to-new mutant mapping; CFC-6 with a step removed | mutants red, no-op green, mapping complete, CFC-6 red | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
