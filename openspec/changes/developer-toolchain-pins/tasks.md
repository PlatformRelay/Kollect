# Tasks

Each check lands in the commit of the task that turns it green, so the `lint` job is green at every
commit. 1.1 creates the guard with only the checks that are green on today's tree and wires its two
`lint` steps in the same commit (CFC-6 reds on any unwired `consistency_*_test.sh`, so the guard
file and its wiring cannot land apart). Every check that is red on today's tree is written in the
task that fixes what it checks, shown red in that task's evidence before the fix, and committed with
the fix: DTP-1 in 2.1, DTP-2 in 2.2, the DTP-5 comment rule in 2.3, 3.2's checks with 3.3, 4.2's
tests with 4.3.

## 1. Guard first (DTP-3, DTP-6, DTP-7)

- [x] 1.1 Write `hack/test/consistency_toolchain_pins_test.sh` (plain and `--self-test`) with the
  checks that are green today, and add its two steps to the `lint` job. Self-test cases on
  throwaway copies, each with the failure it exposes:
  - DTP-3 gitleaks, given 8.30.1 at `ci.yaml:164`, `hack/install-gitleaks.sh:14` and
    `.pre-commit-config.yaml:5` (`v8.30.1`): when the pre-commit `rev` says v8.29.0, then red naming
    that file (a site drifts); when all three say 8.29.0, then red (below floor); when the gitleaks
    scan finds no site, then red (empty scan).
  - DTP-3 git-cliff, given v2.13.1 at `Taskfile.yml:22`, `.github/workflows/release.yaml:420` and
    `hack/verify-changelog.sh:11`: when `release.yaml` says v2.12.0, then red naming it (the release
    notes are built by another git-cliff); when `verify-changelog.sh` says v2.12.0, then red naming
    it (the changelog check runs another git-cliff); when all three say v2.12.0, then red (below
    floor).
  - DTP-3 shellcheck, given 0.11.0 at `ci.yaml:299` and `hack/install-shellcheck.sh:15` and
    `v0.11.0.1` at `.pre-commit-config.yaml:56`: when `ci.yaml` says 0.11.1, then red naming it (a
    bot moved one site); when all three say 0.11.1 (pre-commit `v0.11.1.0`), then red naming the
    missing `pinned_sha256` entry in `install-shellcheck.sh` (a bump that cannot install); when the
    pre-commit `rev` says `v0.11.0.2`, then green (the fourth component is shellcheck-py packaging,
    not a shellcheck release).
  - DTP-6 given `UNVERIFIED_BY_DESIGN` in `hack/test/ci_fetch_lib_hardening_test.sh` with today's
    two entries as the guard's limit, when it gains a third, then red (the exceptions list grows).
  - DTP-7 a no-op copy of the real tree passes (a guard that reds on correct content).
  - DTP-7 wiring, on a copy: when one of the two new steps is removed from `ci.yaml`, then
    `hack/test/ci_workflow_security_test.sh` reds naming the guard and the missing mode (CWS-6, at
    `:590`, fires before CFC-6); when both are removed, then it reds with the CFC-6 enrolment
    diagnostic naming the guard (`:608`).
  Run the plain mode on today's tree and record it green. — closed 2026-10-16, evidence:
  evidence/T1.1.md
- [x] 1.2 Fold `hack/test/dev_mise_pin_drift_test.sh`: port every existing inline mutant one-to-one
  into `--self-test`, show each red against the new guard, list the mapping in evidence, then
  delete the old guard and its `ci.yaml` step. The mutants to port (old label -> expected
  failure): `honest` (green), `task-drift` (mise task 3.50.0 vs CI), `node-drift` (node 20 vs 22),
  `python-drift` (python 3.11 vs 3.12), `go-pinned` (`go` under `[tools]`), `go-setting-gone`
  (`idiomatic_version_file_enable_tools` removed), `gomod-gone` (no `go` line in `go.mod`),
  `ci-self-drift` (workflow and composite action disagree on task), `no-ci-pins` (empty CI scan),
  `empty-tools` (empty `[tools]`), `no-tools-table` (no `[tools]` table), and the missing
  `mise.toml` case. All are green on today's tree. Its self-wiring check (`grep` for the step in
  `ci.yaml`) is replaced by CWS-6 and CFC-6, shown red in 1.1. Delete `ci.yaml:411-419` (comment and
  step). Update every live reference to the old name: `renovate.json:74` description, `mise.toml:8`,
  `.gitignore:26`, `docs/development/setup.md:53`, the CWS-6 composite-action mutant at
  `hack/test/ci_workflow_security_test.sh:1145-1146` (retarget it to the new guard and show it
  still reds), and the comments at `hack/test/ci_fetch_lib_hardening_test.sh:844` and
  `hack/test/ci_install_helm_hardening_test.sh:687`. Change 4's evidence and review files keep the
  old name as history. — closed 2026-10-16, evidence: evidence/T1.2.md

## 2. Pins (DTP-1, DTP-2, DTP-5)

- [x] 2.1 Add the DTP-1 check, with self-test cases given 18 setup-task sites and `mise.toml` at
  3.52.0: when one site says 3.51.1, then red naming that file (a missed site); when all say 3.51.1,
  then red (set below floor); when all say a newer release, then green (an exact-value check would
  block bot bumps). Record the plain mode red on today's 18 + 1 Task 3.51.1 sites. Then set Task
  3.52.0 at all 18 sites and `mise.toml:54`, and rewrite `docs/development/setup.md:39` so the row
  names the `go-task/setup-task` `version:` input without a version. Plain mode green; one commit.
  — closed 2026-10-16, evidence: evidence/T2.1.md
- [x] 2.2 Add the DTP-2 check, then move govulncheck, in one commit. Self-test cases given an
  annotated `GOVULNCHECK_VERSION` used by `task vulncheck`: when `task vulncheck` carries a literal
  `govulncheck@v1.9.0`, then red (inline pin returns); when the annotation is deleted, then red
  (Renovate stops seeing it); when `task vulncheck` reads another variable holding v1.9.0 while the
  annotated `GOVULNCHECK_VERSION` stays defined, then red naming the missing `GOVULNCHECK_VERSION`
  reference (the annotated pin is not the one that runs); when the `GOVULNCHECK_VERSION` definition
  is removed, then red with a missing-definition message (the reference resolves to nothing).
  Record the plain mode red on today's inline `Taskfile.yml:204`. Test change on a guard owned by
  change 4, first: run `bash hack/test/consistency_tool_pins_test.sh --self-test` green. Then define
  `GOVULNCHECK_VERSION: v1.9.0` under `# renovate: datasource=go depName=golang.org/x/vuln` (the Go
  module datasource: a `github-releases` annotation would still extract, so DTP-4 passes while
  lookups fail) and use it in `task vulncheck`. Show the CFC-2 mutants that `sed` the literal
  `govulncheck@v…` in `Taskfile.yml` (~499-501 "unresolvable pin", ~523-525 "too old for go.mod's
  Go") fail to apply, re-anchor both onto the `GOVULNCHECK_VERSION:` line, and confirm the two that
  match `govulncheck@` without a version (~503-505 "package path typo", ~527-529 "pin missing")
  still apply and red for their stated reason. Confirm the plain run resolves
  `{{.GOVULNCHECK_VERSION}}` (the variable lookup at ~162) rather than reporting `<unresolved:…>`.
  Record a real `task vulncheck` run. Both guards green, both modes. — closed 2026-10-16, evidence:
  evidence/T2.2.md
- [ ] 2.3 Add the DTP-5 comment rule, then fix the comments, in one commit. Self-test cases, each
  with the failure it exposes:
  - when a `mise.toml` comment says `helm (v3.21.4)`, `go 1.27.2` or "all 17 occurrences", then red
    (a restated version or count away from its pin);
  - when the block directly above `task = "3.52.0"` says `version: 3.51.1`, then red (an attached
    comment gone stale);
  - when a comment naming `node-version: "22"` is separated from `node = "22"` by a blank line, then
    red (not attached);
  - when the block above `node = "22"` names `gitleaks 8.30.1`, then red (another tool's version);
  - the existing Node block (`mise.toml:56-58`, `node-version: "22"` above `node = "22"`) and Python
    block (`:61-62`, `python-version: "3.12"` above `python = "3.12"`), then green (false positive
    on a correct attached comment);
  - in `Taskfile.yml`, a comment saying `helm v3.22.0` directly above `HELM_VERSION: v3.22.0`
    (`:31`), then green; a comment saying `helm v3.21.4` anywhere else, then red (Taskfile comments
    are in scope).
  Record the plain mode red on today's `mise.toml:24`, `:28`, `:39` and `:52`. Replace those
  restated versions and the count with pointers to the owning file. The Go half of DTP-5 needs no
  change: it is the existing DEV-MISE-01 check, carried over by 1.2.

## 3. Managers (DTP-4)

- [ ] 3.1 Probe which files and dependencies each custom manager matches
  (`renovate-config-validator --strict` plus a local dry run). Settle and record:
  - what "a manager matches a file" means: at least one `managerFilePatterns` hit on a tracked file
    AND at least one `matchStrings` dependency extracted from it. The gitleaks and shellcheck
    managers pass the first half and fail the second, which is why a file-pattern-only check would
    call them alive;
  - the guard's mechanism for 3.2, with the reason: Renovate itself run in `lint` (record the pinned
    Renovate version and how it runs without network), or a bash copy of the managers' regexes and
    the `packageRules` order (a later `groupName` wins), with a self-test that reds when the copy
    and `renovate.json` disagree (for example on the `\n` escapes at `renovate.json:80`);
  - that each gitleaks pin form is rewritten correctly by a dry-run bump: `8.30.1` without `v` at
    `ci.yaml:164` and `hack/install-gitleaks.sh:14`, `v8.30.1` at `.pre-commit-config.yaml:5` (no
    `vv`, no dropped `v`); likewise shellcheck's `0.11.0` and the shellcheck-py `v0.11.0.1` rev.
- [ ] 3.2 Add the manager-liveness, site-coverage and grouping checks to the guard, using 3.1's
  definition and mechanism, with self-test cases: given today's `renovate.json`, then red naming
  the gitleaks (`renovate.json:37-47`) and shellcheck (`renovate.json:48-58`) managers (dead
  manager); given a manager whose file pattern matches no tracked file, then red (pattern rot);
  given no manager covering any one of `Makefile:184`, `hack/tooling/.custom-gcl.yml:6`,
  `.pre-commit-config.yaml:5`, `.pre-commit-config.yaml:56`, `hack/install-gitleaks.sh:14`,
  `hack/install-shellcheck.sh:15`, `ci.yaml:164`, `ci.yaml:299`, `release.yaml:420` or
  `hack/verify-changelog.sh:11`, then red naming the file (uncovered site); given two golangci-lint
  sites in different groups, then red (sites can move apart); given a shellcheck site in the
  "pinned build tools" group, then red naming the group (a red shellcheck bump holds other tools).
  Record it red on today's tree; it commits with 3.3.
- [ ] 3.3 Replace the dead gitleaks manager with one that matches `ci.yaml:164`,
  `hack/install-gitleaks.sh:14` and `.pre-commit-config.yaml:5`; replace the dead shellcheck manager
  with one that matches `ci.yaml:299`, `hack/install-shellcheck.sh:15` and
  `.pre-commit-config.yaml:56`, in a `shellcheck` group of its own; add managers for `Makefile:184`
  and `hack/tooling/.custom-gcl.yml:6` in one group, and for `release.yaml:420` and
  `hack/verify-changelog.sh:11` in the group of `Taskfile.yml:22`. Any new group rule goes after the
  "pinned build tools" rule (`renovate.json:126-129`, a later `groupName` wins) and before the "go
  toolchain" rule (`renovate.json:142`), which must stay last; run
  `hack/test/consistency_go_toolchain_test.sh` both modes. Re-run 3.1's probe; 3.2's checks turn
  green. A shellcheck bump PR stays red until its digests are added to `install-shellcheck.sh`'s
  `pinned_sha256` table: that is the intended manual step, and the own group keeps it from holding
  other tools.

## 4. Installers (DTP-6)

- [ ] 4.1 Probe the upstream checksum files for every platform each installer supports: git-cliff
  per-asset `<asset>.sha512` (the `.sha256` is 404), helm-docs `checksums.txt`. Stop if one is
  missing.
- [ ] 4.2 Tests first, in the new guard's `--self-test`, with a fake `fetch_to`, per installer:
  given a valid archive and a checksum file in the published format, then the installer exits 0 and
  the expected executable exists at the output path (an installer that always fails would pass
  every negative case); from that passing fixture, flip one byte of the archive, then the installer
  exits non-zero with its checksum diagnostic and the output path does not exist (tampered download
  installed); make only the checksum fetch fail, then the same (silent fallback to unverified); on a
  copy of the installer with its verification removed, the tampered case fails its assertion (the
  test can see a missing check). Widen the checksum scan in
  `hack/test/ci_fetch_lib_hardening_test.sh` (`verify_sha256|sha256sum`, ~779 and ~793) to accept
  `sha512sum`, with a mutant that deletes the `sha512sum` call from a copy of
  `install-git-cliff.sh` and must red once the entry leaves `UNVERIFIED_BY_DESIGN`. Watch all red;
  the tests commit with 4.3.
- [ ] 4.3 Verify git-cliff with `sha512sum` against `<asset>.sha512` and helm-docs against
  `checksums.txt` through `hack/lib/fetch.sh`; remove the NOTE at `install-git-cliff.sh:40-42` and
  `install-helm-docs.sh:40`; remove both entries from `UNVERIFIED_BY_DESIGN` (~766). Set the new
  guard's limit for `UNVERIFIED_BY_DESIGN` to empty, with a self-test case: when
  `install-git-cliff.sh` is re-added, then red (the ratchet holds after the list empties). 4.2's
  tests and the stale-exemption check turn green.

## 5. Land

- [ ] 5.1 Archive the change as the last commit of the PR, after review and green CI; the review
  record names the reviewed and the archive revision

## Verification

| Req | Check | Expected | Status | Evidence |
| --- | --- | --- | --- | --- |
| DTP-1 | guard: one-site and below-floor mutants red; newer set passes; today's tree red before 2.1 | as stated | not-run | |
| DTP-2 | inline-govulncheck, dropped-annotation, other-variable and missing-definition mutants; CFC-2 self-test after re-anchoring; `task vulncheck` real run | red; CFC-2 green; run result recorded | not-run | |
| DTP-3 | gitleaks, git-cliff and shellcheck drift, below-floor and empty-scan mutants; shellcheck missing-digest mutant; four-component rev passes | red; rev case green | not-run | |
| DTP-4 | dead-manager, rotted-pattern, uncovered-site, split-group and shared-shellcheck-group fixtures; gitleaks pin forms rewritten correctly | red; forms recorded | not-run | |
| DTP-5 | ported go/node/python/task mutants; stale, detached, other-tool and site-count comment mutants; Node, Python and Taskfile attached comments pass | red; positives green | not-run | |
| DTP-6 | valid, tampered and missing-checksum cases per installer; verification-removed copy; grown-exceptions mutant before and after the list empties | valid installs; others fail, nothing installed; red | not-run | |
| DTP-7 | all mutants plus no-op by exit status; old-to-new mutant mapping; one step removed; both steps removed | mutants red, no-op green, mapping complete, CWS-6 red, CFC-6 red | not-run | |
| all | independent review; CI on the PR head | APPROVE, green; both revisions recorded | not-run | |
