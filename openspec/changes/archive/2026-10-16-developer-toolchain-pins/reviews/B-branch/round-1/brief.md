# Stage-B adversarial brief: developer-toolchain-pins (whole branch)

You are reviewing a whole feature branch, independently of whoever implemented it, as a hostile
reviewer looking for defects that cross task boundaries. You have read-only access to the repo at
the current directory. Do not modify anything.

## Inputs

1. SPEC: `openspec/changes/developer-toolchain-pins/specs/developer-toolchain/spec.md`
   (requirements DTP-1..DTP-7), `proposal.md`, `tasks.md` in the same directory.
2. DIFF: `git diff 966e17e10...HEAD -- . ':!openspec'` (20 product files; the core is the new guard
   `hack/test/consistency_toolchain_pins_test.sh`, ~2080 lines, plus `renovate.json`, `mise.toml`,
   `Taskfile.yml`, `.github/workflows/*.yaml`, `hack/install-git-cliff.sh`,
   `hack/install-helm-docs.sh`). The old `hack/test/dev_mise_pin_drift_test.sh` was deleted and
   folded into the new guard.
3. SENSORS: each guard runs as `bash hack/test/<guard>_test.sh` (check the real tree) and
   `bash hack/test/<guard>_test.sh --self-test` (mutants on scratch copies). You may rerun anything
   under a minute. Writes go to a scratch copy under `$TMPDIR` only; free each copy afterwards
   (shared /tmp inodes are nearly exhausted).
4. INTERACTING GUARDS from an earlier change (already on main): `hack/test/consistency_tool_pins_test.sh`
   (CFC-2), `hack/test/consistency_go_toolchain_test.sh`, `hack/test/ci_workflow_security_test.sh`
   (CFC-6 wiring and CWS-6), `hack/test/ci_fetch_lib_hardening_test.sh`.

## Read these first (lowest prior review coverage)

- `hack/install-git-cliff.sh` and `hack/install-helm-docs.sh` (checksum verification), and the
  guard's installer self-test section: `inst_meta`, `inst_fixtures`, `inst_run`, `inst_refused`,
  `inst_cases` (around lines 1406-1580) and the two sed scripts at ~1578-1579 that strip the
  verification to prove the tamper case can see a missing check.
- `unverified_other_uses` and `check_unverified_limit` (~373-430): the shrink-only
  UNVERIFIED_BY_DESIGN list, now limit = empty.
- `cliff_install_sites` (~246) and `action_version_sites` (~194, called for orhun/git-cliff-action at ~1030) — git-cliff pins in
  release.yaml and verify-changelog.sh.
- The python Renovate model (`RENOVATE_MODEL`, `renovate_deps`, `check_managers`, ~628-950): it
  evaluates `renovate.json` customManagers/regexes itself to prove every manager is alive and
  covers its pins, and simulates Renovate's rewrite.

## Questions (answer each; findings must carry path:line)

a. FAIL-OPEN: any path where the guard, an installer, or a self-test exits 0 while the thing it
   claims to check is unchecked: missing tool (python3, sha512sum, sha256sum, curl), empty
   extraction, a regex that matches nothing, a `grep | grep -q` under pipefail, `(( ))` on
   unvalidated numbers, a subshell losing state, a case-statement default that passes.
b. GUARD INTERACTIONS: do the new guard and CFC-2 (`consistency_tool_pins`), the Go-toolchain guard
   and CFC-6/CWS-6 (`ci_workflow_security`) agree on who owns each pin, or does a pin fall between
   them / get double-owned with contradicting rules? Does the ci.yaml wiring satisfy CFC-6 for the
   new guard (both modes) and is the deleted guard's step gone everywhere?
c. SUPPLY CHAIN: can the git-cliff or helm-docs installer be made to install an unverified binary
   (checksum file fetched from an attacker-controlled URL, checksum line for another asset, empty
   digest, digest of wrong length, `sha*sum -c` semantics, arch/OS mapping that skips verify)?
   Does the self-test actually prove the verification, or only that some error string appears?
d. RENOVATE BUMP PR: for each manager in renovate.json touched by this branch (Task, govulncheck,
   gitleaks, shellcheck, shellcheck-py, git-cliff, golangci-lint, helm-docs, mise pins), simulate
   what a real Renovate bump PR would change (which files, which lines, which groups), then say
   whether each guard (this one, CFC-2, go-toolchain, ci_workflow_security, fetch-lib) would go
   red on that PR. A guard that reds every legitimate bump is a P1; a bump that changes one site
   but leaves a sibling pin stale while every guard stays green is a P1.
e. AI-code failure modes: tautological mutants, mutants anchored on text that does not exist,
   prose/comments claiming more than the code does, spec scenarios with no case.

Already known and queued for fixing (do NOT re-report these):
- macOS lacks sha512sum/sha256sum, so the installers fail closed there (fix planned: shasum -a fallback).
- helm-docs digest is not checked as 64 hex.
- CFC-2 accepts `latest` via any Taskfile var (consistency_tool_pins_test.sh:159-163).
- `uses:` is matched case-sensitively.
- No `task = "latest"` case.

For each P0/P1 you are about to file, argue the opposite for one line; keep only what survives.
An APPROVE with no findings is valid when substantiated.

RETURN at most 25 lines, nothing else, no diff quotes, no narration:
verdict: APPROVE | REQUEST_CHANGES | BLOCK
findings:
  - [P0|P1|P2] <one line, with path:line> -- <how you confirmed it (command or reasoning)>
renovate_bump_table:
  - <manager>: <files changed> -> <guards red/green>
could_not_check:
  - <what you could not verify>
summary: <one line>

Severity: P0 (security bypass, vacuous gate presented as proof) -> BLOCK. P1 (correctness gap,
fail-open, a guard that reds every legitimate bump, untested new behaviour) -> REQUEST_CHANGES.
P2 -> note only.
